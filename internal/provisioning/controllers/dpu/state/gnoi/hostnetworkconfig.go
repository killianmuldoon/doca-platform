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

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util/hostnetwork"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	InitContainerMaxRestartCount = 10
)

func SetupNetwork(ctx context.Context, dpu *provisioningv1.DPU, ctrlCtx *dutil.ControllerContext) (provisioningv1.DPUStatus, error) {
	state := dpu.Status.DeepCopy()

	if !dpu.DeletionTimestamp.IsZero() {
		state.Phase = provisioningv1.DPUDeleting
		return *state, nil
	}

	hostNetworkPodName := cutil.GenerateHostnetworkPodName(dpu)

	nn := types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      hostNetworkPodName,
	}
	pod := &corev1.Pod{}
	if err := ctrlCtx.Get(ctx, nn, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return *state, hostnetwork.CreateHostNetworkSetupPod(ctx, ctrlCtx.Client, dpu, ctrlCtx.Options)
		}
		return *state, err
	} else {
		if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].Ready {
			state.Phase = provisioningv1.DPUClusterConfig
			cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondHostNetworkReady, "", "waiting for dpu joining cluster"))
			return *state, nil
		} else {
			cond := cutil.DPUCondition(provisioningv1.DPUCondHostNetworkReady, "", "")
			cond.Status = metav1.ConditionFalse
			for _, container := range pod.Status.InitContainerStatuses {
				if container.RestartCount != 0 && container.RestartCount < InitContainerMaxRestartCount && container.State.Waiting != nil {
					cond.Reason = "Initializing"
					cond.Message = container.State.Waiting.Message
					cutil.SetDPUCondition(state, cond)
				} else if container.RestartCount >= InitContainerMaxRestartCount {
					cond.Reason = "InitializationFailed"
					if container.State.Waiting != nil {
						cond.Message = container.State.Waiting.Message
					}
					state.Phase = provisioningv1.DPUError
					cutil.SetDPUCondition(state, cond)
					return *state, fmt.Errorf("host network setup error")
				}
			}
		}
	}

	return *state, nil
}
