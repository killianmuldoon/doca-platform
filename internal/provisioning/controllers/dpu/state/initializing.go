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

	provisioningdpfv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	dutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/util"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type dpuInitializingState struct {
	dpu *provisioningdpfv1alpha1.Dpu
}

func (st *dpuInitializingState) Handle(ctx context.Context, _ client.Client, _ dutil.DPUOptions) (provisioningdpfv1alpha1.DpuStatus, error) {
	state := st.dpu.Status.DeepCopy()
	if isDeleting(st.dpu) {
		state.Phase = provisioningdpfv1alpha1.DPUDeleting
		return *state, nil
	}

	state.Phase = provisioningdpfv1alpha1.DPUNodeEffect
	cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningdpfv1alpha1.DPUCondInitialized, "", ""))
	return *state, nil
}
