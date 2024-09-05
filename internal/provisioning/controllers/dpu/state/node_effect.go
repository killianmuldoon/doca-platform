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

	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	dutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/util"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"

	nodeMaintenancev1beta1 "github.com/medik8s/node-maintenance-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type dpuNodeEffectState struct {
	dpu *provisioningv1.Dpu
}

func (st *dpuNodeEffectState) Handle(ctx context.Context, client client.Client, _ dutil.DPUOptions) (provisioningv1.DpuStatus, error) {
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
		if apierrors.IsNotFound(err) {
			return *state, err

		} else {
			return *state, fmt.Errorf("Faild to Get Node %v", err)
		}
	}

	needUpdate := false
	if len(nodeEffect.CustomLabel) != 0 {
		logger.V(3).Info("NodeEffect is set to Custom Label", "node", nodeName)
		for k, v := range nodeEffect.CustomLabel {
			if _, ok := node.Labels[k]; !ok {
				node.Labels[k] = v
				needUpdate = true
			}
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
			needUpdate = true
		}
	} else if nodeEffect.Drain != nil {
		logger.V(3).Info("NodeEffect is set to Drain", "node", nodeName)

		maintenanceNN := types.NamespacedName{
			Namespace: st.dpu.Namespace,
			Name:      nodeName,
		}
		maintenance := &nodeMaintenancev1beta1.NodeMaintenance{}
		if err := client.Get(ctx, maintenanceNN, maintenance); err != nil {
			if apierrors.IsNotFound(err) {
				logger.V(3).Info("NodeMaintenance CR not found, creating new NodeMaintenance CR", "node", nodeName)
				err = createNodeMaintenance(ctx, client, nodeName, st.dpu.Namespace)
				if err == nil {
					logger.V(3).Info("Successfully created NodeMaintenance CR", "node", nodeName, "NodeMaintanence", maintenance)
					state.Phase = provisioningv1.DPUPending
					cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondNodeEffectReady, "", ""))
					return *state, nil
				}
				logger.V(3).Info("Error creating NodeMaintenance CR", "node", nodeName, "NodeMaintanence", maintenance, "error", err)
			} else {
				logger.V(3).Info("Error getting NodeMaintenance CR", "node", nodeName, "NodeMaintanence", maintenance, "error", err)
			}
			state.Phase = provisioningv1.DPUError
			cond := cutil.DPUCondition(provisioningv1.DPUCondNodeEffectReady, "", "")
			cond.Status = metav1.ConditionFalse
			cond.Reason = errorOccurredReason
			cond.Message = err.Error()
			cutil.SetDPUCondition(state, cond)
			return *state, err
		}
	}

	if needUpdate {
		if err := client.Update(ctx, node); err != nil {
			return *state, err
		}
	}

	state.Phase = provisioningv1.DPUPending
	cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondNodeEffectReady, "", ""))

	return *state, nil
}

func createNodeMaintenance(ctx context.Context, client client.Client, nodeName string, namespace string) error {
	newMaintenance := &nodeMaintenancev1beta1.NodeMaintenance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: namespace,
		},
		Spec: nodeMaintenancev1beta1.NodeMaintenanceSpec{
			NodeName: nodeName,
			Reason:   "DPU provisioning",
		},
	}
	if err := client.Create(ctx, newMaintenance); err != nil {
		return err
	}
	return nil
}
