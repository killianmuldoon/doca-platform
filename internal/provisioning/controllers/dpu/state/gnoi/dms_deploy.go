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

package gnoi

import (
	"context"
	"fmt"
	"net"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util/dms"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	errorOccurredReason string = "ErrorOccured"
)

func DeployDMS(ctx context.Context, dpu *provisioningv1.DPU, ctrlCtx *dutil.ControllerContext) (provisioningv1.DPUStatus, error) {
	logger := log.FromContext(ctx)
	state := dpu.Status.DeepCopy()
	if !dpu.DeletionTimestamp.IsZero() {
		state.Phase = provisioningv1.DPUDeleting
		return *state, nil
	}
	dmsPodName := cutil.GenerateDMSPodName(dpu)

	nn := types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      dmsPodName,
	}
	pod := &corev1.Pod{}
	if err := ctrlCtx.Get(ctx, nn, pod); err != nil {
		if apierrors.IsNotFound(err) {
			err = dms.CreateDMSPod(ctx, ctrlCtx.Client, dpu, ctrlCtx.Options)
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
	case corev1.PodPending:
		// Verify if the container of the DMS Pod is in waiting state.
		if len(pod.Status.ContainerStatuses) == 0 ||
			pod.Status.ContainerStatuses[0].State.Waiting == nil {
			return *state, nil
		}
		// Verify that the initContainer of the DMS Pod has a last terminated state.
		if len(pod.Status.InitContainerStatuses) == 0 ||
			pod.Status.InitContainerStatuses[0].LastTerminationState.Terminated == nil {
			return *state, nil
		}
		// Verify NFS server connection using the DMS container startup probe.
		if pod.Status.ContainerStatuses[0].State.Waiting != nil {
			for _, condition := range pod.Status.Conditions {
				if condition.Type != corev1.PodReadyToStartContainers || condition.Status != corev1.ConditionFalse {
					continue
				}
				message := fmt.Sprintf("the DMS server %s is not ready yet, wait for the NFS server to become available", dmsPodName)
				cond := cutil.DPUCondition(provisioningv1.DPUCondDMSRunning, "NFSServerNotAvailable", message)
				cond.Status = metav1.ConditionFalse
				cutil.SetDPUCondition(state, cond)
				return *state, fmt.Errorf("%s", message)
			}
		}
		message := fmt.Sprintf("the DMS server %s is not ready yet, wait for %q", dmsPodName, pod.Status.InitContainerStatuses[0].LastTerminationState.Terminated.Message)
		cond := cutil.DPUCondition(provisioningv1.DPUCondDMSRunning, "ExistingRshimInstallDetected", message)
		cond.Status = metav1.ConditionFalse
		cutil.SetDPUCondition(state, cond)
		return *state, fmt.Errorf("%s", message)

	case corev1.PodRunning:
		// a simple probe to check if the DMS server is ready
		addr := dms.Address(pod.Status.PodIP, dpu)
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			logger.V(3).Info(fmt.Sprintf("the DMS server %s (%s) is not ready yet, err: %v", addr, dmsPodName, err))
			return *state, nil
		}
		defer func() {
			if err := conn.Close(); err != nil {
				logger.Error(fmt.Errorf("failed to close connection of %s (%s), err: %v", addr, dmsPodName, err), "")
			}
		}()
		state.Phase = provisioningv1.DPUOSInstalling
		cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondDMSRunning, "", ""))

	case corev1.PodFailed:
		return handleDMSPodFailure(state, "DMSPodFailed", "DMS Pod Failed")

	default:
		if isTimeout(pod, ctrlCtx.Options.DMSPodTimeout) {
			return handleDMSPodFailure(state, "DMSPodTimedout", "DMS Pod didn't run and timed out")
		}
	}
	return *state, nil
}

func isTimeout(pod *corev1.Pod, timeoutDuration time.Duration) bool {
	return time.Since(pod.CreationTimestamp.Time) > timeoutDuration
}
