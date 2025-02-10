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
	"os"
	"path/filepath"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/bfcfg"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	BFCFGDir = "bfcfg"
)

func PrepareBFB(ctx context.Context, dpu *provisioningv1.DPU, ctrlCtx *dutil.ControllerContext) (provisioningv1.DPUStatus, error) {
	state := dpu.Status.DeepCopy()
	flavor := &provisioningv1.DPUFlavor{}
	if err := ctrlCtx.Get(ctx, types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      dpu.Spec.DPUFlavor,
	}, flavor); err != nil {
		return *state, err
	}
	dc := &provisioningv1.DPUCluster{}
	if err := ctrlCtx.Get(ctx, types.NamespacedName{Namespace: dpu.Spec.Cluster.Namespace, Name: dpu.Spec.Cluster.Name}, dc); err != nil {
		return *state, fmt.Errorf("failed to get DPUCluster, err: %v", err)
	}
	node := &corev1.Node{}
	if err := ctrlCtx.Get(ctx, types.NamespacedName{
		Namespace: "",
		Name:      dpu.Spec.NodeName,
	}, node); err != nil {
		return *state, err
	}
	bfCFGPath := filepath.Join(BFCFGDir, fmt.Sprintf("%s_%s_%s", dpu.Namespace, dpu.Name, dpu.UID))
	pathInFS := filepath.Join("/", cutil.BFBBaseDir, bfCFGPath)
	if err := os.MkdirAll(filepath.Dir(pathInFS), os.ModePerm); err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondBFBPrepared), err, "FailedToCreateDirectory", ""))
		return *state, fmt.Errorf("failed to create directory for bf.cfgs, dir: %s, err: %v", filepath.Dir(pathInFS), err)
	}

	joinCommand, err := ctrlCtx.JoinCommandGenerator.GenerateJoinCommand(dc)
	if err != nil {
		return *state, fmt.Errorf("failed to generate join command, err: %v", err)
	}
	cfg, err := bfcfg.GenerateBFConfig(ctx, ctrlCtx.Options.BFCFGTemplateFile, dpu, node, flavor, joinCommand, ctrlCtx.Options.DPUInstallInterface)
	if err != nil {
		return *state, fmt.Errorf("failed to generate bf.cfg, err: %v", err)
	}
	log.FromContext(ctx).Info(fmt.Sprintf("write bf.cfg to %s", pathInFS))
	if err := os.WriteFile(pathInFS, cfg, os.ModePerm); err != nil {
		cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondBFBPrepared), err, "FailedToPushBFCFG", ""))
		return *state, fmt.Errorf("failed to push bf.cfgs, path: %s, err: %v", pathInFS, err)
	}
	state.BFCFGFile = bfCFGPath
	cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondBFBPrepared), nil, "", ""))
	state.Phase = provisioningv1.DPUOSInstalling
	return *state, nil
}
