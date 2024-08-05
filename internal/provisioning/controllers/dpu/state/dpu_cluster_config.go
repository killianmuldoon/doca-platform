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

	provisioningdpfv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	dutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/util"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type dpuClusterConfig struct {
	dpu *provisioningdpfv1alpha1.Dpu
}

func (st *dpuClusterConfig) Handle(ctx context.Context, client crclient.Client, _ dutil.DPUOptions) (provisioningdpfv1alpha1.DpuStatus, error) {
	logger := log.FromContext(ctx)

	state := st.dpu.Status.DeepCopy()
	if isDeleting(st.dpu) {
		state.Phase = provisioningdpfv1alpha1.DPUDeleting
		return *state, nil
	}

	dpuName := st.dpu.Name

	tenantNamespace := st.dpu.Spec.Cluster.NameSpace
	tenantName := st.dpu.Spec.Cluster.Name

	newClient, err := util.RetrieveK8sClientUsingKubeConfig(ctx, client, tenantNamespace, tenantName)
	if err != nil {
		return *state, err
	}

	node := &corev1.Node{}
	if err := newClient.Get(ctx, types.NamespacedName{Namespace: tenantNamespace, Name: dpuName}, node); err != nil {
		return *state, err
	}

	if !util.IsNodeReady(node) {
		return *state, nil
	}

	// Patch DPU labels to Kamaji Node
	if err = util.AddLabelsToNode(ctx, newClient, node, st.dpu.Spec.Cluster.NodeLabels); err != nil {
		return *state, err
	}

	logger.V(3).Info(fmt.Sprintf("DPU %s joined Kamaji Cluster", dpuName))

	state.Phase = provisioningdpfv1alpha1.DPUReady
	util.SetDPUCondition(state, util.DPUCondition(provisioningdpfv1alpha1.DPUCondReady, "", "cluster configured"))
	return *state, nil
}
