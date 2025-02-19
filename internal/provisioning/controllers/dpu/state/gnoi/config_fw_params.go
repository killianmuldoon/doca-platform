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

package gnoi

import (
	"context"
	"fmt"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"

	"github.com/openconfig/gnmi/proto/gnmi"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func ConfigFWParameters(ctx context.Context, dpu *provisioningv1.DPU, ctrlCtx *dutil.ControllerContext) (provisioningv1.DPUStatus, error) {
	state := dpu.Status.DeepCopy()
	logger := log.FromContext(ctx)
	flavor := &provisioningv1.DPUFlavor{}
	if err := ctrlCtx.Get(ctx, types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      dpu.Spec.DPUFlavor,
	}, flavor); err != nil {
		return *state, err
	}

	if len(flavor.Spec.DpuMode) == 0 {
		flavor.Spec.DpuMode = provisioningv1.DpuMode
	}

	if flavor.Spec.DpuMode != provisioningv1.DpuMode {
		state.Phase = provisioningv1.DPUError
		return handleDMSPodFailure(state, "Requested DPU mode is unsupported ", fmt.Sprintf("DPU mode: %s is unsupported in gnoi. Only dpu mode is supported", flavor.Spec.DpuMode))
	}
	dmsTaskName := generateDMSTaskName(dpu)

	logger.V(3).Info(fmt.Sprintf("DMS %s start config fw parameters installation", dmsTaskName))

	conn, err := createGRPCConnection(ctx, ctrlCtx.Client, dpu, ctrlCtx)
	if err != nil {
		msg := fmt.Sprintf("Error creating gRPC connection: %v", err)
		logger.Error(err, msg)
		return handleDMSPodFailure(state, "Error creating gRPC connection ", msg)
	}
	defer conn.Close() //nolint: errcheck
	gnmiClient := gnmi.NewGNMIClient(conn)

	logger.V(3).Info(fmt.Sprintf("TLS Connection established between DPU controller to DMS %s", dmsTaskName))

	setModeRequest := setModeRequest()
	if resp, err := gnmiClient.Set(ctx, setModeRequest); err == nil {
		logger.V(3).Info(fmt.Sprintf("Set DPU mode to DPU %s successfully, %v", dpu.Name, resp.String()))
		state.Phase = provisioningv1.DPUOSInstalling
		return *state, nil
	} else {
		logger.Error(err, "failed to set DPU mode", "DPU", dpu.Name)
		return handleDMSPodFailure(state, "failed to set DPU mode", dpu.Name)
	}

}

func setModeRequest() *gnmi.SetRequest {
	return &gnmi.SetRequest{
		Update: []*gnmi.Update{
			{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "nvidia"},
						{Name: "mode"},
						{Name: "config"},
						{Name: "mode"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_StringVal{StringVal: "DPU"},
				},
			},
		},
	}
}
