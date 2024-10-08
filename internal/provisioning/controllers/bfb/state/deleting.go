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
	"os"
	"path/filepath"

	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	butil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/bfb/util"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/events"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type bfbDeletingState struct {
	bfb      *provisioningv1.BFB
	recorder record.EventRecorder
}

func (st *bfbDeletingState) Handle(ctx context.Context, client client.Client) (provisioningv1.BFBStatus, error) {
	state := st.bfb.Status.DeepCopy()
	// make sure the bfb task is deleted from the downloading task map
	bfbtaskName := cutil.GenerateBFBTaskName(*st.bfb)
	butil.DownloadingTaskMap.Delete(bfbtaskName)
	butil.DownloadingTaskMap.Delete(bfbtaskName + "cancel")

	bfbFile := cutil.GenerateBFBFilePath(st.bfb.Spec.FileName)
	err := os.Remove(bfbFile)
	if err != nil && !os.IsNotExist(err) {
		msg := fmt.Sprintf("Deleting BFB: (%s/%s) failed", st.bfb.Namespace, st.bfb.Name)
		st.recorder.Eventf(st.bfb, corev1.EventTypeWarning, events.EventFailedDeleteBFBReason, msg)
		return *state, err
	}

	if err := DeleteTmpBFBFiles(bfbFile); err != nil {
		msg := fmt.Sprintf("Deleting BFB: (%s/%s) temp file failed", st.bfb.Namespace, st.bfb.Name)
		st.recorder.Eventf(st.bfb, corev1.EventTypeWarning, events.EventFailedDeleteBFBReason, msg)
		return *state, err
	}

	controllerutil.RemoveFinalizer(st.bfb, provisioningv1.BFBFinalizer)
	if err := client.Update(ctx, st.bfb); err != nil {
		return *state, err
	}

	return *state, nil
}

func DeleteTmpBFBFiles(bfbFile string) error {
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
