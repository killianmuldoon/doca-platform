/*
Copyright 2024 NVIDIA

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package state

import (
	"context"
	"fmt"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	maintenancev1alpha1 "github.com/Mellanox/maintenance-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type dpuNodeEffectState struct {
	dpu *provisioningv1.DPU
}

func (st *dpuNodeEffectState) Handle(ctx context.Context, client client.Client, _ dutil.DPUOptions) (provisioningv1.DPUStatus, error) {
	logger := log.FromContext(ctx)
	state := st.dpu.Status.DeepCopy()
	if isDeleting(st.dpu) {
		state.Phase = provisioningv1.DPUDeleting
		return *state, nil
	}

	nodeEffect := st.dpu.Spec.NodeEffect

	if nodeEffect.NoEffect {
		logger.V(3).Info("NodeEffect is set to No Effect")
		state.Phase = provisioningv1.DPUPending
		cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondNodeEffectReady, "", ""))
		return *state, nil
	}

	nodeName := st.dpu.Spec.NodeName

	nn := types.NamespacedName{
		Namespace: "",
		Name:      nodeName,
	}
	node := &corev1.Node{}
	if err := client.Get(ctx, nn, node); err != nil {
		return *state, fmt.Errorf("get node %s: %v", nodeName, err)
	}

	if len(nodeEffect.CustomLabel) != 0 {
		logger.V(3).Info("NodeEffect is set to Custom Label", "node", nodeName)
		if err := cutil.AddLabelsToNode(ctx, client, node, nodeEffect.CustomLabel); err != nil {
			return *state, err
		}
	} else if nodeEffect.Taint != nil {
		logger.V(3).Info("NodeEffect is set to Taint", "node", nodeName)
		taintExist := false
		for _, t := range node.Spec.Taints {
			if t.Key == nodeEffect.Taint.Key {
				taintExist = true
				break
			}
		}
		if !taintExist {
			node.Spec.Taints = append(node.Spec.Taints, *nodeEffect.Taint)
			if err := client.Update(ctx, node); err != nil {
				return *state, err
			}
		}
	} else if nodeEffect.Drain != nil {
		logger.V(3).Info("NodeEffect is set to Drain", "node", nodeName)

		maintenanceNN := types.NamespacedName{
			Namespace: st.dpu.Namespace,
			Name:      nodeName,
		}
		maintenance := &maintenancev1alpha1.NodeMaintenance{}
		if err := client.Get(ctx, maintenanceNN, maintenance); err != nil {
			if apierrors.IsNotFound(err) {
				logger.V(3).Info("NodeMaintenance CR not found, creating new NodeMaintenance CR", "node", nodeName)
				if err = createNodeMaintenance(ctx, client, nodeName, st.dpu.Namespace); err != nil {
					logger.V(3).Info("Error creating NodeMaintenance CR", "node", nodeName, "NodeMaintanence", maintenance, "error", err)
					setDPUCondNodeEffectReady(state, metav1.ConditionFalse, errorOccurredReason, err.Error())
					state.Phase = provisioningv1.DPUError
					return *state, err
				}
			} else {
				logger.V(3).Info("Error getting NodeMaintenance CR", "node", nodeName, "NodeMaintanence", maintenance, "error", err)
				setDPUCondNodeEffectReady(state, metav1.ConditionFalse, errorOccurredReason, err.Error())
				state.Phase = provisioningv1.DPUError
				return *state, err
			}
		}
		if done, err := checkNodeMaintenanceProgress(ctx, client, nodeName, st.dpu.Namespace); err != nil {
			setDPUCondNodeEffectReady(state, metav1.ConditionFalse, errorOccurredReason, err.Error())
			state.Phase = provisioningv1.DPUError
			return *state, err
		} else if done {
			state.Phase = provisioningv1.DPUPending
			cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondNodeEffectReady, "", ""))
			return *state, nil
		}
	}

	state.Phase = provisioningv1.DPUPending
	cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondNodeEffectReady, "", ""))

	return *state, nil
}

func createNodeMaintenance(ctx context.Context, client client.Client, nodeName string, namespace string) error {
	logger := log.FromContext(ctx)
	nodeMaintenance := &maintenancev1alpha1.NodeMaintenance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: namespace,
		},
		Spec: maintenancev1alpha1.NodeMaintenanceSpec{
			RequestorID:          cutil.NodeMaintenanceRequestorID,
			NodeName:             nodeName,
			DrainSpec:            &maintenancev1alpha1.DrainSpec{Force: true, DeleteEmptyDir: true},
			AdditionalRequestors: []string{cutil.ProvisioningGroupName},
		},
	}
	if err := client.Create(ctx, nodeMaintenance); err != nil {
		return err
	}
	logger.V(3).Info("Successfully created NodeMaintenance CR", "node", nodeName, "NodeMaintanence", nodeMaintenance)
	return nil
}

func checkNodeMaintenanceProgress(ctx context.Context, client client.Client, nodeName string, namespace string) (bool, error) {
	logger := log.FromContext(ctx)
	maintenanceNN := types.NamespacedName{
		Namespace: namespace,
		Name:      nodeName,
	}
	nodeMaintenance := &maintenancev1alpha1.NodeMaintenance{}
	if err := client.Get(ctx, maintenanceNN, nodeMaintenance); err != nil {
		logger.V(3).Info("Failed to get NodeMaintenance CR", "node", nodeName, "NodeMaintanence", nodeMaintenance, "error", err)
		return false, err
	}
	condition := meta.FindStatusCondition(nodeMaintenance.Status.Conditions, maintenancev1alpha1.ConditionTypeReady)
	if condition.Status == metav1.ConditionTrue {
		logger.V(3).Info("NodeMaintenance succeeded", "node", nodeName, "NodeMaintenance", nodeMaintenance, "reason", condition.Reason)
		return true, nil
	}
	logger.V(3).Info("Node Maintenance in progress", "node", nodeName, "state", condition.Reason, "Drain Progress", nodeMaintenance.Status.Drain)
	return false, nil
}

func setDPUCondNodeEffectReady(state *provisioningv1.DPUStatus, status metav1.ConditionStatus, reason, message string) {
	cond := cutil.DPUCondition(provisioningv1.DPUCondNodeEffectReady, "", "")
	cond.Status = status
	cond.Reason = reason
	cond.Message = message
	cutil.SetDPUCondition(state, cond)
}
