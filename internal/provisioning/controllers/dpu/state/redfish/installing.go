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
	"path/filepath"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	rc "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/state/redfish/client"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ServiceLocation = "bfb"
)

func Installing(ctx context.Context, dpu *provisioningv1.DPU, ctrlCtx *dutil.ControllerContext) (provisioningv1.DPUStatus, error) {
	state := dpu.Status.DeepCopy()

	taskName := fmt.Sprintf("%s-%s", dpu.Name, dpu.UID)
	defer func(oldPhase provisioningv1.DPUPhase) {
		if oldPhase == state.Phase {
			return
		}
		dutil.OsInstallTaskMap.Delete(taskName)
	}(state.Phase)

	client, err := rc.NewTLSClient(ctx, dpu, ctrlCtx.Client)
	if err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondOSInstalled), err, "FailedToCreateClient", ""))
		return *state, err
	}
	taskID, ok := dutil.OsInstallTaskMap.Load(taskName)
	if !ok {
		imageURI := filepath.Join(ctrlCtx.Options.BFBRegistry, ServiceLocation, fmt.Sprintf("??%s,%s?", filepath.Base(dpu.Status.BFBFile), dpu.Status.BFCFGFile), "bfb-to-install")
		resp, taskInfo, err := client.InstallBFB(imageURI)
		if err != nil {
			log.FromContext(ctx).Error(err, "failed to install BFB")
			state.Phase = provisioningv1.DPUError
			cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondOSInstalled), err, "FailToInstall", ""))
			// when we transition to ERROR phase, we should return nil to make sure the next Reconcile is trigger by the UPDATE event.
			// If an error is returned, the next Reconcile may be triggered as a retry, leading to installing BFB again.
			// todo: other phases trasitioning to ERROR phase should also follow this pattern
			return *state, nil
		} else if resp.StatusCode() != http.StatusAccepted {
			err = fmt.Errorf("get status: %s", resp.Status())
			log.FromContext(ctx).Error(err, "failed to install BFB, get unexpected status")
			state.Phase = provisioningv1.DPUError
			cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondOSInstalled), err, "FailToInstall", ""))
			return *state, nil
		}
		dutil.OsInstallTaskMap.Store(taskName, taskInfo.ID)
		log.FromContext(ctx).Info(fmt.Sprintf("new install task: %+v", *taskInfo))
		return *state, nil
	}

	// check progress
	resp, prog, err := client.CheckTaskProgress(taskID.(string))
	if err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondOSInstalled), err, "FailToCheckProgress", ""))
		return *state, err
	} else if resp.StatusCode() != http.StatusOK {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondOSInstalled), fmt.Errorf("get status: %s", resp.Status()), "FailToCheckProgress", ""))
		return *state, err
	}
	log.FromContext(ctx).Info(fmt.Sprintf("taskProgress: %+v", prog))
	if prog.PercentComplete < 100 {
		return *state, nil
	}

	log.FromContext(ctx).Info("installation finished")
	state.Phase = provisioningv1.DPURebooting
	cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondOSInstalled), nil, "", ""))
	return *state, nil
}
