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

package bfcfg

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util/reboot"

	"github.com/Masterminds/sprig/v3"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// The maximum size of the bf.cfg file is expanded to 128k since DOCA 2.8
	MaxBFSize = 1024 * 128
)

var (
	//go:embed bf.cfg.template
	defaultBFCFGTemplateData string
	// ConfigMapDataKey is the key in the configmap where the bfb.cfg.template is stored if overwritten.
	ConfigMapDataKey = "BF_CFG_TEMPLATE"
	defaultTemplate  = template.Must(template.New("").Funcs(sprig.FuncMap()).Parse(defaultBFCFGTemplateData))
)

func getTemplate(bfbCFGFilePath string) (*template.Template, error) {
	if bfbCFGFilePath == "" {
		return defaultTemplate, nil
	}
	data, err := os.ReadFile(bfbCFGFilePath)
	if err != nil {
		return nil, err
	}
	return template.New("").Funcs(sprig.FuncMap()).Parse(string(data))
}

type BFCFGData struct {
	KubeadmJoinCMD             string
	DPUHostName                string
	BFGCFGParams               []string
	UbuntuPassword             string
	NVConfigParams             string
	Sysctl                     []string
	ConfigFiles                []BFCFGWriteFile
	OVSRawScript               string
	KernelParameters           string
	ContainerdRegistryEndpoint string
	SFNum                      int
	// AdditionalReboot adds an extra reboot during the DPU provisioning. This is required in some environments.
	AdditionalReboot bool
	// OOBNetwork is a flag to indicate if the DPU accesses DPU cluster via the OOB interface.
	OOBNetwork bool
}

type BFCFGWriteFile struct {
	Path        string
	IsAppend    bool
	Content     string
	Permissions string
}

func GenerateBFConfig(ctx context.Context, bfCFGTemplateFile string, dpu *provisioningv1.DPU, node *corev1.Node, flavor *provisioningv1.DPUFlavor, joinCommand, installInterface string) ([]byte, error) {
	logger := log.FromContext(ctx)

	additionalReboot := false
	cmd, _, err := reboot.GenerateCmd(node.Annotations, dpu.Annotations)
	if err != nil {
		logger.Error(err, "failed to generate ipmitool command")
		return nil, err
	}

	if cmd == reboot.Skip {
		additionalReboot = true
	}

	buf, err := Generate(flavor, cutil.GenerateNodeName(dpu), joinCommand, additionalReboot, bfCFGTemplateFile, installInterface)
	if err != nil {
		return nil, err
	}
	if buf == nil {
		return nil, fmt.Errorf("failed bf.cfg creation due to buffer issue")
	}
	if len(buf) > MaxBFSize {
		return nil, fmt.Errorf("bf.cfg for %s size (%d) exceeds the maximum limit (%d)", dpu.Name, len(buf), MaxBFSize)
	}
	logger.V(3).Info(fmt.Sprintf("bf.cfg for %s has len: %d data: %s", dpu.Name, len(buf), string(buf)))

	return buf, nil
}

func Generate(flavor *provisioningv1.DPUFlavor, dpuName, joinCmd string, additionalReboot bool, bfbCFGFilepath string, installInterface string) ([]byte, error) {
	config := &BFCFGData{
		KubeadmJoinCMD:   joinCmd,
		DPUHostName:      dpuName,
		AdditionalReboot: additionalReboot,
		KernelParameters: strings.TrimSpace(strings.Join(flavor.Spec.Grub.KernelParameters, " ")),
	}

	config.ContainerdRegistryEndpoint = flavor.Spec.ContainerdConfig.RegistryEndpoint

	config.BFGCFGParams, config.UbuntuPassword = bfcfgParams(flavor)
	// currently, there can be at most one nvconfig, and that nvconfig applies to all devices on a host
	if len(flavor.Spec.NVConfig) > 0 {
		config.NVConfigParams = strings.Join(flavor.Spec.NVConfig[0].Parameters, " ")
	}
	config.Sysctl = flavor.Spec.Sysctl.Parameters
	for _, f := range flavor.Spec.ConfigFiles {
		config.ConfigFiles = append(config.ConfigFiles, BFCFGWriteFile{
			Path:        f.Path,
			Permissions: f.Permissions,
			IsAppend:    f.Operation == provisioningv1.FileAppend,
			Content:     f.Raw,
		})
	}
	config.OVSRawScript = flavor.Spec.OVS.RawConfigScript
	if installInterface == string(provisioningv1.InstallViaRedFish) {
		config.OOBNetwork = true
	}

	if num, ok := getPFTotalSFFromFlavor(flavor); ok {
		config.SFNum = num
	}

	bfbCFGTemplate, err := getTemplate(bfbCFGFilepath)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	if err := bfbCFGTemplate.Execute(buf, config); err != nil {
		return nil, fmt.Errorf("execute bfbCFGTemplate failed, err: %v", err)
	}
	return buf.Bytes(), nil
}

func bfcfgParams(flavor *provisioningv1.DPUFlavor) ([]string, string) {
	var ret []string
	var passwd string
	for _, param := range flavor.Spec.BFCfgParameters {
		info := strings.Split(param, "=")
		if len(info) != 2 {
			ret = append(ret, param)
			continue
		}
		key := strings.TrimSpace(info[0])
		if key != "ubuntu_PASSWORD" {
			ret = append(ret, param)
			continue
		}
		passwd = strings.TrimSpace(info[1])
	}
	if passwd == "" {
		return ret, passwd
	}
	if !strings.HasPrefix(passwd, "'") {
		passwd = fmt.Sprintf("'%s'", passwd)
	}
	return ret, passwd
}

func getPFTotalSFFromFlavor(flavor *provisioningv1.DPUFlavor) (int, bool) {
	regex := regexp.MustCompile(`^PF_TOTAL_SF=([0-9]+)`)
	for _, nvconfig := range flavor.Spec.NVConfig {
		for _, parmeter := range nvconfig.Parameters {
			matches := regex.FindStringSubmatch(parmeter)
			if len(matches) == 2 {
				if num, err := strconv.Atoi(matches[1]); err == nil {
					return num, true
				}
			}
		}
	}
	return 0, false
}
