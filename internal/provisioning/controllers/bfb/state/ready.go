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
	"os"

	provisioningdpfv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/provisioning/v1alpha1"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/provisioning/controllers/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type bfbReadyState struct {
	bfb *provisioningdpfv1alpha1.Bfb
}

func (st *bfbReadyState) Handle(ctx context.Context, _ client.Client) (provisioningdpfv1alpha1.BfbStatus, error) {
	state := st.bfb.Status.DeepCopy()
	if isDeleting(st.bfb) {
		state.Phase = provisioningdpfv1alpha1.BfbDeleting
		return *state, nil
	}

	if exist, err := checkingBFBFile(*st.bfb); !exist {
		state.Phase = provisioningdpfv1alpha1.BfbDownloading
		return *state, err
	}

	return *state, nil
}

// check whether BFB file exist
func checkingBFBFile(bfb provisioningdpfv1alpha1.Bfb) (bool, error) {
	fullFileName := cutil.GenerateBFBFilePath(bfb.Spec.FileName)
	if _, err := os.Stat(fullFileName); err != nil {
		return false, err
	}
	return true, nil
}
