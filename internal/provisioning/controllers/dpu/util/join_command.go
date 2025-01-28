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

package util

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
)

type NodeJoinCommandGenerator interface {
	GenerateJoinCommand(dc *provisioningv1.DPUCluster) (string, error)
}

type KubeadmJoinCommandGenerator struct{}

func (k *KubeadmJoinCommandGenerator) GenerateJoinCommand(dc *provisioningv1.DPUCluster) (string, error) {
	fp := cutil.AdminKubeConfigPath(*dc)
	if _, err := os.Stat(fp); err != nil {
		return "", fmt.Errorf("failed to stat kubeconfig file, err: %v", err)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("kubeadm", "token", "create", "--print-join-command", fmt.Sprintf("--kubeconfig=%s", fp))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run \"kubeadm token create\", err: %v, stderr: %s", err, stderr.String())
	}
	joinCommand := strings.TrimRight(stdout.String(), "\r\n") + " --v=5"
	return joinCommand, nil
}
