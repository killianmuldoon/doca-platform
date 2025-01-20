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

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	butil "github.com/nvidia/doca-platform/internal/provisioning/controllers/bfb/util"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/events"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util/bfbdownloader"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type bfbReadyState struct {
	bfb      *provisioningv1.BFB
	recorder record.EventRecorder
}

func (st *bfbReadyState) Handle(ctx context.Context, client client.Client, option butil.BFBOptions, _ bfbdownloader.BFBDownloader) (provisioningv1.BFBStatus, error) {
	state := st.bfb.Status.DeepCopy()
	if isDeleting(st.bfb) {
		state.Phase = provisioningv1.BFBDeleting
		return *state, nil
	}

	bfbJobName := cutil.GenerateBFBJobName(*st.bfb)
	if err := cutil.DeleteJobIfExists(ctx, client, bfbJobName, st.bfb.Namespace); err != nil {
		state.Phase = provisioningv1.BFBError
		msg := fmt.Sprintf("Deleting job %s (%s/%s) failed with error :%s", bfbJobName, st.bfb.Namespace, st.bfb.Name, err.Error())
		st.recorder.Eventf(st.bfb, corev1.EventTypeWarning, events.EventFailedDownloadBFBReason, msg)
		return *state, err
	}

	if exist, err := checkingBFBFile(*st.bfb); !exist {
		state.Phase = provisioningv1.BFBDownloading
		return *state, err
	}

	return *state, nil
}

// check whether BFB file exist
func checkingBFBFile(bfb provisioningv1.BFB) (bool, error) {
	fullFileName := cutil.GenerateBFBFilePath(bfb.Status.FileName)
	if _, err := os.Stat(fullFileName); err != nil {
		return false, err
	}
	return true, nil
}
