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
	"path/filepath"

	provisioningdpfv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/provisioning/v1alpha1"
	butil "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/provisioning/controllers/bfb/util"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/provisioning/controllers/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type bfbDeletingState struct {
	bfb *provisioningdpfv1alpha1.Bfb
}

func (st *bfbDeletingState) Handle(ctx context.Context, client client.Client) (provisioningdpfv1alpha1.BfbStatus, error) {
	state := st.bfb.Status.DeepCopy()
	// make sure the bfb task is deleted from the downloading task map
	bfbtaskName := cutil.GenerateBFBTaskName(*st.bfb)
	butil.DownloadingTaskMap.Delete(bfbtaskName)
	butil.DownloadingTaskMap.Delete(bfbtaskName + "cancel")

	bfbFile := cutil.GenerateBFBFilePath(st.bfb.Spec.FileName)
	err := os.Remove(bfbFile)
	if err != nil && !os.IsNotExist(err) {
		return *state, err
	}

	if err := DeleteTmpBfbFiles(bfbFile); err != nil {
		return *state, err
	}

	st.bfb.Finalizers = nil
	if err := client.Update(ctx, st.bfb); err != nil {
		return *state, err
	}

	return *state, nil
}

func DeleteTmpBfbFiles(bfbFile string) error {
	// Remove all .tmp files in the directory
	dir := filepath.Dir(bfbFile)
	tmpFiles, err := filepath.Glob(filepath.Join(dir, "bfb-*.tmp"))
	if err != nil {
		return err
	}
	for _, tmpFile := range tmpFiles {
		if err := os.Remove(tmpFile); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
