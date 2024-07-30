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
	"strings"

	provisioningdpfv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/provisioning/v1alpha1"
	dutil "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/provisioning/controllers/dpu/util"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/provisioning/controllers/util"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/provisioning/controllers/util/hostnetwork"

	"github.com/nmstate/kubernetes-nmstate/api/shared"
	nmstatev1 "github.com/nmstate/kubernetes-nmstate/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	InitContainerMaxRestartCount = 10
)

type dpuHostNetworkConfigState struct {
	dpu *provisioningdpfv1alpha1.Dpu
}

func (st *dpuHostNetworkConfigState) Handle(ctx context.Context, client client.Client, option dutil.DPUOptions) (provisioningdpfv1alpha1.DpuStatus, error) {
	state := st.dpu.Status.DeepCopy()

	if isDeleting(st.dpu) {
		state.Phase = provisioningdpfv1alpha1.DPUDeleting
		return *state, nil
	}

	pfName, ok := st.dpu.Labels[cutil.DpuPFNameLabel]
	if !ok {
		return *state, fmt.Errorf("can not get PF name from the label")
	}

	nn := types.NamespacedName{
		Name:      fmt.Sprintf("br-%s", st.dpu.Spec.NodeName),
		Namespace: "",
	}
	nncp := &nmstatev1.NodeNetworkConfigurationPolicy{}
	if err := client.Get(ctx, nn, nncp); err != nil {
		if apierrors.IsNotFound(err) {
			return *state, createBridge(ctx, client, st.dpu, pfName)
		}
		return *state, err
	}

	hostNetworkPodName := cutil.GenerateHostnetworkPodName(st.dpu.Name)

	nn = types.NamespacedName{
		Namespace: st.dpu.Namespace,
		Name:      hostNetworkPodName,
	}
	pod := &corev1.Pod{}
	if err := client.Get(ctx, nn, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return *state, hostnetwork.CreateHostNetworkSetupPod(ctx, client, st.dpu, option)
		}
		return *state, err
	} else {
		if pod.Status.Phase == corev1.PodRunning {
			state.Phase = provisioningdpfv1alpha1.DPUClusterConfig
			cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningdpfv1alpha1.DPUCondHostNetworkReady, "", "waiting for dpu joining cluster"))
			return *state, nil
		} else {
			cond := cutil.DPUCondition(provisioningdpfv1alpha1.DPUCondHostNetworkReady, "", "")
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
					state.Phase = provisioningdpfv1alpha1.DPUError
					cutil.SetDPUCondition(state, cond)
					return *state, fmt.Errorf("host network setup error")
				}
			}
		}
	}

	return *state, nil
}

func createBridge(ctx context.Context, client client.Client, dpu *provisioningdpfv1alpha1.Dpu, pfName string) error {
	logger := log.FromContext(ctx)
	var hostIP string

	if address, ok := dpu.Labels[cutil.DpuHostIPLabel]; ok {
		hostIP = address
	} else {
		return fmt.Errorf("can not get host IP from the label")
	}

	lastThreeChars := pfName[len(pfName)-3:]
	vf1Name := strings.Replace(pfName, lastThreeChars, "v1", 1)
	vf2Name := strings.Replace(pfName, lastThreeChars, "v2", 1)

	desiredState := `interfaces:
- name: br-dpu
  type: linux-bridge
  state: up
  ipv4:
    dhcp: false
    enabled: true
    address:
    - ip: ` + hostIP + `
      prefix-length: 32
  bridge:
    options:
      stp:
        enabled: false
    port:
    - name: ` + vf1Name + `
      vlan: {}
- name: ` + vf2Name + `
  type: ethernet
  state: up
  ipv4:
    dhcp: false
    enabled: true
    address:
    - ip: 169.254.55.2
      prefix-length: 30`

	logger.V(3).Info("br-ex bridge", "raw data", desiredState)
	bridge := &nmstatev1.NodeNetworkConfigurationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("br-%s", dpu.Spec.NodeName),
		},
		Spec: shared.NodeNetworkConfigurationPolicySpec{
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": dpu.Spec.NodeName,
			},
			DesiredState: shared.State{
				Raw: []byte(desiredState),
			},
		},
	}
	if err := client.Create(ctx, bridge); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}
