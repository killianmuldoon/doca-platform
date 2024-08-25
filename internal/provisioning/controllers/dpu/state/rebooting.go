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
	"strconv"
	"strings"

	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	dutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/util"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"
	gutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util/powercycle"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type dpuRebootingState struct {
	dpu *provisioningv1.Dpu
}

func (st *dpuRebootingState) Handle(ctx context.Context, client client.Client, _ dutil.DPUOptions) (provisioningv1.DpuStatus, error) {
	logger := log.FromContext(ctx)

	state := st.dpu.Status.DeepCopy()
	if isDeleting(st.dpu) {
		state.Phase = provisioningv1.DPUDeleting
		return *state, nil
	}

	nn := types.NamespacedName{
		Namespace: st.dpu.Namespace,
		Name:      st.dpu.Name,
	}

	dpu := provisioningv1.Dpu{}
	if err := client.Get(ctx, nn, &dpu); err != nil {
		return *state, err
	}

	_, cond := gutil.GetDPUCondition(state, provisioningv1.DPUCondOSInstalled.String())
	if cond == nil || cond.Status != metav1.ConditionTrue {
		err := fmt.Errorf("trying to reboot the host before %s", provisioningv1.DPUCondOSInstalled.String())
		c := gutil.DPUCondition(provisioningv1.DPUCondRebooted, "InvalidState", err.Error())
		c.Status = metav1.ConditionFalse
		gutil.SetDPUCondition(state, c)
		return *state, err
	}

	duration := int(metav1.Now().Sub(cond.LastTransitionTime.Time).Seconds())
	// If we can not get uptime, the host should be rebooting
	uptime, err := HostUptime(st.dpu.Namespace, gutil.GenerateDMSPodName(st.dpu.Name), "")
	if err != nil {
		return *state, err
	}

	logger.V(3).Info(fmt.Sprintf("Rebooting duration is %d, host uptime is %d", duration, uptime))

	// If the pod is available after rebooting, move to next phase
	if duration > uptime {
		state.Phase = provisioningv1.DPUHostNetworkConfiguration
		cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondRebooted, "", ""))
		return *state, nil
	}

	// Power cycle the host
	cmd, err := powercycle.GenerateCmd(st.dpu.Annotations)
	if err != nil {
		logger.Error(err, "failed to generate ipmitool command")
		return *state, err
	}

	// Return early and set node to ready if we should skip the powercycle command.
	// Note: skipping the powercycle may cause issues with the firmware installation and configuration.
	if cmd == powercycle.Skip {
		logger.Info("Warning not rebooting: this may cause issues with DPU firmware installation and configuration")
		state.Phase = provisioningv1.DPUHostNetworkConfiguration
		cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondRebooted, "", ""))
		return *state, nil
	}
	if cmd == "" {
		logger.Info("waiting for manual power cycle")
		c := gutil.DPUCondition(provisioningv1.DPUCondRebooted, "WaitingForManualPowerCycle", "")
		c.Status = metav1.ConditionFalse
		gutil.SetDPUCondition(state, c)
		return *state, nil
	}

	logger.Info(fmt.Sprintf("powercycle with command %q", cmd))
	if _, err := HostPowerCycle(st.dpu.Namespace, gutil.GenerateDMSPodName(st.dpu.Name), "", cmd); err != nil {
		// TODO: broadcast an event
		return *state, err
	}

	return *state, nil
}

func HostPowerCycle(ns, name, container, cmd string) (string, error) {
	return gutil.RemoteExec(ns, name, container, cmd)
}

func HostUptime(ns, name, container string) (int, error) {
	uptimeStr, err := gutil.RemoteExec(ns, name, container, "cat /proc/uptime")
	if err != nil {
		return -1, err
	}

	ts := strings.Fields(uptimeStr)
	if len(ts) != 2 {
		return -1, fmt.Errorf("uptime incorrect: %#v", ts)
	}

	uptime, err := strconv.ParseFloat(strings.TrimSpace(ts[0]), 64)
	if err != nil {
		return -1, err
	}

	return int(uptime), nil
}
