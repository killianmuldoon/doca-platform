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
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util/dms"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util/hostnetwork"

	nodeMaintenancev1beta1 "github.com/medik8s/node-maintenance-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type dpuReadyState struct {
	dpu *provisioningv1.Dpu
}

func (st *dpuReadyState) Handle(ctx context.Context, client client.Client, option dutil.DPUOptions) (provisioningv1.DpuStatus, error) {
	state := st.dpu.Status.DeepCopy()
	if isDeleting(st.dpu) {
		state.Phase = provisioningv1.DPUDeleting
		return *state, nil
	}

	if err := healthyCheck(ctx, st.dpu, client, option); err != nil {
		state.Phase = provisioningv1.DPUError
		updateFalseDPUCondReady(state, "DPUNodeNotReady", err.Error())
	}

	if err := HandleNodeEffect(ctx, client, *st.dpu.Spec.NodeEffect, st.dpu.Spec.NodeName, st.dpu.Namespace); err != nil {
		state.Phase = provisioningv1.DPUError
		updateFalseDPUCondReady(state, "NodeEffectError", err.Error())
		return *state, err
	}

	nn := types.NamespacedName{
		Namespace: st.dpu.Namespace,
		Name:      st.dpu.Name,
	}

	dpu := provisioningv1.Dpu{}
	if err := client.Get(ctx, nn, &dpu); err != nil {
		updateFalseDPUCondReady(state, "DPUGetError", err.Error())
		return *state, err
	}

	dpuName := dpu.Name

	tenantNamespace := dpu.Spec.Cluster.NameSpace
	tenantName := dpu.Spec.Cluster.Name

	newClient, err := cutil.RetrieveK8sClientUsingKubeConfig(ctx, client, tenantNamespace, tenantName)
	if err != nil {
		updateFalseDPUCondReady(state, "KamajiClientGetError", err.Error())
		return *state, err
	}

	node := &corev1.Node{}
	if err := newClient.Get(ctx, types.NamespacedName{Namespace: tenantNamespace, Name: dpuName}, node); err != nil {
		updateFalseDPUCondReady(state, "KamajiNodeGetError", err.Error())
		return *state, err
	}

	if !cutil.IsNodeReady(node) {
		updateFalseDPUCondReady(state, "DPUNodeNotReady", fmt.Errorf("kamaji Node %s is not Ready", node).Error())
		return *state, err
	} else {
		cond := cutil.DPUCondition(provisioningv1.DPUCondReady, "DPUNodeReady", "")
		cutil.SetDPUCondition(state, cond)
		return *state, nil
	}
}

// Check if the DMS pod exist, if not, restore the missing pod
func healthyCheck(ctx context.Context, dpu *provisioningv1.Dpu, client client.Client, option dutil.DPUOptions) error {
	nn := types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      cutil.GenerateDMSPodName(dpu.Name),
	}
	dmsPod := &corev1.Pod{}
	if err := client.Get(ctx, nn, dmsPod); err != nil {
		if apierrors.IsNotFound(err) {
			return dms.CreateDMSPod(ctx, client, dpu, option)
		}
		return err
	}

	nn = types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      cutil.GenerateHostnetworkPodName(dpu.Name),
	}
	hostnetworkPod := &corev1.Pod{}
	if err := client.Get(ctx, nn, hostnetworkPod); err != nil {
		if apierrors.IsNotFound(err) {
			return hostnetwork.CreateHostNetworkSetupPod(ctx, client, dpu, option)
		}
		return err
	}
	return nil
}

func updateFalseDPUCondReady(status *provisioningv1.DpuStatus, reason string, message string) {
	cond := cutil.DPUCondition(provisioningv1.DPUCondReady, reason, message)
	cond.Status = metav1.ConditionFalse
	cutil.SetDPUCondition(status, cond)
}

func HandleNodeEffect(ctx context.Context, k8sClient client.Client, nodeEffect provisioningv1.NodeEffect, nodeName string, namespace string) error {
	logger := log.FromContext(ctx)
	if nodeEffect.NoEffect {
		return nil
	}

	nn := types.NamespacedName{
		Namespace: "",
		Name:      nodeName,
	}
	node := &corev1.Node{}
	if err := k8sClient.Get(ctx, nn, node); err != nil {
		return fmt.Errorf("failed to get node %s in HandleNodeEffect, err: %v", nodeName, err)
	}

	if nodeEffect.Taint != nil {
		originalNode := node.DeepCopy()
		taintFound := false
		for i, taint := range node.Spec.Taints {
			if taint.Key == nodeEffect.Taint.Key {
				node.Spec.Taints = append(node.Spec.Taints[:i], node.Spec.Taints[i+1:]...)
				taintFound = true
				break
			}
		}
		if taintFound {
			patch := client.StrategicMergeFrom(originalNode)
			if err := k8sClient.Patch(ctx, node, patch); err != nil {
				return fmt.Errorf("failed to patch node %s after removing the Taint: %+v, err: %v", nodeName, nodeEffect.Taint, err)
			}
		}
	}

	if nodeEffect.Drain {
		maintenanceNN := types.NamespacedName{
			Namespace: namespace,
			Name:      nodeName,
		}
		maintenance := &nodeMaintenancev1beta1.NodeMaintenance{}
		if err := k8sClient.Get(ctx, maintenanceNN, maintenance); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("Error getting NodeMaintenance %v", maintenance)
			}
		}
		switch maintenance.Status.Phase {
		case nodeMaintenancev1beta1.MaintenanceRunning:
			break
		case nodeMaintenancev1beta1.MaintenanceSucceeded:
			logger.V(3).Info("NodeMaintenance succeeded, deleting NodeMaintenance CR", "node", nodeName, "NodeMaintanence", maintenance)
			if err := cutil.DeleteObject(k8sClient, maintenance); err != nil {
				logger.V(3).Info("Error deleting NodeMaintenance CR", "node", nodeName, "NodeMaintanence", maintenance, "error", err)
				return err
			}
		case nodeMaintenancev1beta1.MaintenanceFailed:
			logger.V(3).Info("NodeMaintenance failed", "node", nodeName, "NodeMaintanence", maintenance)
			return fmt.Errorf("NodeMaintenance %v failed", maintenance)
		}
	}
	return nil
}
