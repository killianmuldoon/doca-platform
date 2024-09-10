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

	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type bfbInitializingState struct {
	bfb *provisioningv1.Bfb
}

func (st *bfbInitializingState) Handle(ctx context.Context, client client.Client) (provisioningv1.BfbStatus, error) {
	state := st.bfb.Status.DeepCopy()
	if isDeleting(st.bfb) {
		state.Phase = provisioningv1.BfbDeleting
		return *state, nil
	}

	state.Phase = provisioningv1.BfbDownloading
	return *state, nil
}
