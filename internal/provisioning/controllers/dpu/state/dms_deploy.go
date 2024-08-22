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
	"time"

	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	dutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/util"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util/dms"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	errorOccurredReason string = "ErrorOccured"
)

type dmsDeploymentState struct {
	dpu *provisioningv1.Dpu
}

func (st *dmsDeploymentState) Handle(ctx context.Context, client client.Client, option dutil.DPUOptions) (provisioningv1.DpuStatus, error) {
	logger := log.FromContext(ctx)
	state := st.dpu.Status.DeepCopy()
	if isDeleting(st.dpu) {
		state.Phase = provisioningv1.DPUDeleting
		return *state, nil
	}
	dmsPodName := cutil.GenerateDMSPodName(st.dpu.Name)

	nn := types.NamespacedName{
		Namespace: st.dpu.Namespace,
		Name:      dmsPodName,
	}
	pod := &corev1.Pod{}
	if err := client.Get(ctx, nn, pod); err != nil {
		if apierrors.IsNotFound(err) {
			err = dms.CreateDMSPod(ctx, client, st.dpu, option)
			if err == nil {
				return *state, nil
			}
			cond := cutil.DPUCondition(provisioningv1.DPUCondDMSRunning, errorOccurredReason, err.Error())
			cond.Status = metav1.ConditionFalse
			cutil.SetDPUCondition(state, cond)
			return *state, err
		}
		return *state, err
	}
	if !pod.DeletionTimestamp.IsZero() {
		logger.V(3).Info(fmt.Sprintf("DMS pod %s is in terminating state", dmsPodName))
		return *state, nil
	}
	switch pod.Status.Phase {
	case corev1.PodRunning:
		state.Phase = provisioningv1.DPUOSInstalling
		cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondDMSRunning, "", ""))
	case corev1.PodFailed:
		return handleDMSPodFailure(state, "DMSPodFailed", "DMS Pod Failed")
	default:
		if isTimeout(pod, option.DMSPodTimeout) {
			return handleDMSPodFailure(state, "DMSPodTimedout", "DMS Pod didn't run and timed out")
		}

	}
	return *state, nil
}

func isTimeout(pod *corev1.Pod, timeoutDuration time.Duration) bool {
	return time.Since(pod.CreationTimestamp.Time) > timeoutDuration
}

func handleDMSPodFailure(state *provisioningv1.DpuStatus, reason string, message string) (provisioningv1.DpuStatus, error) {
	cond := cutil.DPUCondition(provisioningv1.DPUCondDMSRunning, reason, message)
	cond.Status = metav1.ConditionFalse
	cutil.SetDPUCondition(state, cond)
	state.Phase = provisioningv1.DPUError
	return *state, fmt.Errorf("%s", message)
}
