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

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	butil "github.com/nvidia/doca-platform/internal/provisioning/controllers/bfb/util"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/events"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util/bfbdownloader"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type bfbDownloadingState struct {
	bfb      *provisioningv1.BFB
	recorder record.EventRecorder
}

func (st *bfbDownloadingState) Handle(ctx context.Context, k8sClient client.Client, option butil.BFBOptions, bfbDownloader bfbdownloader.BFBDownloader) (provisioningv1.BFBStatus, error) {
	state := st.bfb.Status.DeepCopy()
	bfbJobName := cutil.GenerateBFBJobName(*st.bfb)

	if isDeleting(st.bfb) {
		// Delete the Job if it exists
		job := &batchv1.Job{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: bfbJobName, Namespace: st.bfb.Namespace}, job)
		if err != nil {
			if apierrors.IsNotFound(err) {
				state.Phase = provisioningv1.BFBDeleting
				return *state, nil
			}
			state.Phase = provisioningv1.BFBError
			msg := fmt.Sprintf("get job %s (%s/%s) failed with error :%s", bfbJobName, st.bfb.Namespace, st.bfb.Name, err.Error())
			st.recorder.Eventf(st.bfb, corev1.EventTypeWarning, events.EventFailedDownloadBFBReason, msg)
			return *state, err
		} else {
			if err := k8sClient.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
				state.Phase = provisioningv1.BFBError
				msg := fmt.Sprintf("delete job %s (%s/%s) failed with error :%s", bfbJobName, st.bfb.Namespace, st.bfb.Name, err.Error())
				st.recorder.Eventf(st.bfb, corev1.EventTypeWarning, events.EventFailedDownloadBFBReason, msg)
				return *state, err
			}
		}
		state.Phase = provisioningv1.BFBDeleting
		return *state, nil
	}

	exist, err := IsBFBExist(st.bfb)
	if err != nil {
		state.Phase = provisioningv1.BFBError
		msg := fmt.Sprintf("Download BFB: (%s/%s) failed with error :%s", st.bfb.Namespace, st.bfb.Name, err.Error())
		st.recorder.Eventf(st.bfb, corev1.EventTypeWarning, events.EventFailedDownloadBFBReason, msg)
		return *state, err
	}

	if exist {
		state.Phase = provisioningv1.BFBReady
		return *state, nil
	}

	nn := types.NamespacedName{
		Namespace: st.bfb.Namespace,
		Name:      bfbJobName,
	}
	job := &batchv1.Job{}
	if err := k8sClient.Get(ctx, nn, job); err != nil {
		if apierrors.IsNotFound(err) {
			err = bfbDownloader.CreateBFBDownloadJob(ctx, k8sClient, st.bfb, option)
			if err == nil {
				return *state, nil
			}
			state.Phase = provisioningv1.BFBError
			msg := fmt.Sprintf("Failed to create download job for BFB: (%s/%s), error: %s", st.bfb.Namespace, st.bfb.Name, err.Error())
			st.recorder.Eventf(st.bfb, corev1.EventTypeWarning, events.EventFailedDownloadBFBReason, msg)
			return *state, err
		}
		state.Phase = provisioningv1.BFBError
		msg := fmt.Sprintf("Failed to get job for BFB: (%s/%s), error: %s", st.bfb.Namespace, st.bfb.Name, err.Error())
		st.recorder.Eventf(st.bfb, corev1.EventTypeWarning, events.EventFailedDownloadBFBReason, msg)
		return *state, err
	}
	if jobSuccess, err := bfbDownloader.ProcessJobConditions(job, option.BFBDownloaderPodTimeout); err != nil {
		msg := fmt.Sprintf("Download BFB: (%s/%s) failed", st.bfb.Namespace, st.bfb.Name)
		st.recorder.Eventf(st.bfb, corev1.EventTypeWarning, events.EventFailedDownloadBFBReason, msg)
		state.Phase = provisioningv1.BFBError
		return *state, err
	} else if !jobSuccess {
		return *state, nil
	}

	msg := fmt.Sprintf("Download BFB: (%s/%s) successful", st.bfb.Namespace, st.bfb.Name)
	st.recorder.Eventf(st.bfb, corev1.EventTypeNormal, events.EventSuccessfulDownloadBFBReason, msg)

	bfbVer, err := bfbDownloader.GetBFBVersion(cutil.GenerateBFBVersionFilePath(st.bfb.Spec.FileName))
	if err != nil {
		msg := fmt.Sprintf("Failed getting BFB: (%s/%s) versions, err: %s", st.bfb.Namespace, st.bfb.Name, err.Error())
		st.recorder.Eventf(st.bfb, corev1.EventTypeWarning, events.EventFailedGetBFBVersionReason, msg)
		state.Phase = provisioningv1.BFBError
		return *state, err
	}

	state.Versions = bfbVer
	state.Phase = provisioningv1.BFBReady
	return *state, nil
}

func IsBFBExist(bfb *provisioningv1.BFB) (bool, error) {
	if bfb == nil {
		return false, fmt.Errorf("BFB object is nil")
	}

	if bfb.Status.Versions.DOCA != "" {
		return true, nil
	}
	return false, nil
}
