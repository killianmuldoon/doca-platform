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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type dpuErrorState struct {
	dpu *provisioningv1.Dpu
}

func (st *dpuErrorState) Handle(ctx context.Context, client client.Client, _ dutil.DPUOptions) (provisioningv1.DpuStatus, error) {
	state := st.dpu.Status.DeepCopy()
	if isDeleting(st.dpu) {
		state.Phase = provisioningv1.DPUDeleting
		return *state, nil
	}

	if err := RemoveNodeEffect(ctx, client, *st.dpu.Spec.NodeEffect, st.dpu.Spec.NodeName, st.dpu.Namespace); err != nil {
		return *state, err
	}
	return *state, nil
}

func RemoveNodeEffect(ctx context.Context, k8sClient client.Client, nodeEffect provisioningv1.NodeEffect, nodeName string, namespace string) error {
	if nodeEffect.NoEffect {
		return nil
	}

	nn := types.NamespacedName{
		Namespace: "",
		Name:      nodeName,
	}
	node := &corev1.Node{}
	if err := k8sClient.Get(ctx, nn, node); err != nil {
		return err
	}
	if nodeEffect.Drain {
		maintenanceNN := types.NamespacedName{
			Namespace: namespace,
			Name:      nodeName,
		}
		maintenance := &nodeMaintenancev1beta1.NodeMaintenance{}
		if err := k8sClient.Get(ctx, maintenanceNN, maintenance); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if err := cutil.DeleteObject(k8sClient, maintenance); err != nil {
			return err
		}
	}
	originalNode := node.DeepCopy()
	needPatch := false
	if len(nodeEffect.CustomLabel) != 0 {
		needPatch = true
		for k := range nodeEffect.CustomLabel {
			delete(node.Labels, k)
		}
	} else if nodeEffect.Taint != nil {
		for i, taint := range node.Spec.Taints {
			if taint.Key == nodeEffect.Taint.Key {
				node.Spec.Taints = append(node.Spec.Taints[:i], node.Spec.Taints[i+1:]...)
				needPatch = true
				break
			}
		}
	}
	if needPatch {
		patch := client.StrategicMergeFrom(originalNode)
		if err := k8sClient.Patch(ctx, node, patch); err != nil {
			return fmt.Errorf("failed to patch node %s ,err: %v", nodeName, err)
		}
	}
	return nil
}
