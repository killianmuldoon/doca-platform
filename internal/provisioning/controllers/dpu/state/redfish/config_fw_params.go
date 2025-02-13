/*
Copyright 2025 NVIDIA

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

package redfish

import (
	"context"
	"fmt"
	"net/http"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	rc "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/state/redfish/client"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func ConfigFWParameters(ctx context.Context, dpu *provisioningv1.DPU, ctrlCtx *dutil.ControllerContext) (provisioningv1.DPUStatus, error) {
	state := dpu.Status.DeepCopy()

	client, err := rc.NewTLSClient(ctx, dpu, ctrlCtx.Client)
	if err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), err, "FailedToCreateClient", ""))
		return *state, err
	}
	_, data, err := client.CheckBMCFirmware()
	if err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), err, "FailedToGetCheckBMCFW", ""))
		return *state, err
	}
	log.FromContext(ctx).Info(fmt.Sprintf("BMC FW version %q", data.Version))
	if !validateBMCFW(data.Version, "") {
		state.Phase = provisioningv1.DPUError
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), err, "OldBMCFW", ""))
		return *state, nil
	}

	_, data, err = client.CheckDPUNIC()
	if err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), err, "FailedToGetCheckDPUNIC", ""))
		return *state, err
	}
	log.FromContext(ctx).Info(fmt.Sprintf("DPU NIC version %q", data.Version))
	if !validateDPUNIC(data.Version, "") {
		state.Phase = provisioningv1.DPUError
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), err, "OldDPUNIC", ""))
		return *state, nil
	}

	flavor := &provisioningv1.DPUFlavor{}
	if err := ctrlCtx.Get(ctx, types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      dpu.Spec.DPUFlavor,
	}, flavor); err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), err, "FailedToGetDpuFlavor", ""))
		return *state, err
	}

	_, bios, err := client.GetBios()
	if err != nil {
		return *state, err
	}

	if CheckDpuModeChangeNeeded(bios, flavor.Spec.DpuMode) {
		_, err := client.SetDpuMode(flavor.Spec.DpuMode)
		if err != nil {
			cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), err, "FailedToSetDpuMode", ""))
			return *state, err
		}
	}

	log.FromContext(ctx).Info(fmt.Sprintf("Set DPU mode %s to DPU %s successfully. Host power cycle required to take affect.", flavor.Spec.DpuMode, dpu.Name))

	_, data, err = client.CheckDPUOS()
	if err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), err, "FailedToGetCheckDPUOS", ""))
		return *state, err
	}
	log.FromContext(ctx).Info(fmt.Sprintf("DPU OS version %q", data.Version))
	if !validateDPUOS(data.Version, "") {
		state.Phase = provisioningv1.DPUError
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), err, "OldDPUOS", ""))
		return *state, nil
	}

	// Note: this does NOT terminate running rshim on host
	resp, _, err := client.DisableHostRshim()
	if err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), err, "FailedToDisableHostRshim", ""))
		return *state, err
	} else if resp.StatusCode() != http.StatusOK {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), fmt.Errorf("status code: %d", resp.StatusCode()), "FailedToDisableHostRshim", ""))
		return *state, err
	}
	log.FromContext(ctx).Info("successfully disabled host RShim")

	resp, _, err = client.EnableBMCRShim()
	if err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), err, "FailedToEnableBMCRshim", ""))
		return *state, err
	} else if resp.StatusCode() != http.StatusOK {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), fmt.Errorf("status code: %d", resp.StatusCode()), "FailedToEnableBMCRshim", ""))
		return *state, err
	}
	log.FromContext(ctx).Info("successfully enabled BMC RShim")

	state.Phase = provisioningv1.DPUPrepareBFB
	cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUConfigFWParameters), nil, "", ""))
	return *state, nil
}

func validateBMCFW(cur, expect string) bool {
	return true
}

func validateDPUNIC(cur, expect string) bool {
	return true
}

func validateDPUOS(cur, expect string) bool {
	return true
}

func CheckDpuModeChangeNeeded(currentMode *rc.Bios, desiredMode provisioningv1.DpuModeType) bool {
	switch desiredMode {
	case provisioningv1.DpuMode:
		return currentMode.Attributes.HostPrivilegeLevel != rc.Privileged && currentMode.Attributes.NicMode != rc.DpuMode
	case provisioningv1.ZeroTrustMode:
		return currentMode.Attributes.HostPrivilegeLevel != rc.Restricted && currentMode.Attributes.NicMode != rc.DpuMode
	case provisioningv1.NicMode:
		return currentMode.Attributes.NicMode != rc.NicMode
	default:
		return false
	}
}
