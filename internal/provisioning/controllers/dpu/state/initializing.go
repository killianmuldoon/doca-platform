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
	"sort"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type dpuInitializingState struct {
	dpu *provisioningv1.DPU
}

func (st *dpuInitializingState) Handle(ctx context.Context, c client.Client, _ dutil.DPUOptions) (provisioningv1.DPUStatus, error) {
	state := st.dpu.Status.DeepCopy()
	if isDeleting(st.dpu) {
		state.Phase = provisioningv1.DPUDeleting
		return *state, nil
	}

	// allocate the first DPUCluster (in alphabetical order) to this DPU
	dcList := &provisioningv1.DPUClusterList{}
	if err := c.List(ctx, dcList); err != nil {
		return *state, fmt.Errorf("failed to list DPUCluster, err: %v", err)
	}
	if len(dcList.Items) == 0 {
		return *state, nil
	}
	sort.Slice(dcList.Items, func(i, j int) bool { return dcList.Items[i].Name < dcList.Items[j].Name })
	dc := dcList.Items[0]
	if dc.Status.Phase != provisioningv1.PhaseReady {
		state.Phase = provisioningv1.DPUInitializing
		cond := cutil.DPUCondition(provisioningv1.DPUCondInitialized, "DPUClusterNotReady", "")
		cond.Status = metav1.ConditionFalse
		cutil.SetDPUCondition(state, cond)
		return *state, nil
	}
	st.dpu.Spec.Cluster.Name = dc.Name
	st.dpu.Spec.Cluster.NameSpace = dc.Namespace
	if err := c.Update(ctx, st.dpu); err != nil {
		return *state, fmt.Errorf("failed to set cluster, err: %v", err)
	}

	state.Phase = provisioningv1.DPUNodeEffect
	cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondInitialized, "", ""))
	return *state, nil
}
