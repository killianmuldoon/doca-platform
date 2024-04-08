//go:build linux

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

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"

	dpucniprovisioner "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/dpu"
	dpucniprovisionertypes "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/dpu/types"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/networkhelper"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/ovsclient"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/readyz"

	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	kexec "k8s.io/utils/exec"
)

const (
	// configPath is the path to the DPU CNI Provisioner configuration file
	configPath = "/etc/dpucniprovisioner/config.yaml"
)

func main() {
	klog.Info("Starting DPU CNI Provisioner")
	config, err := parseConfig()
	if err != nil {
		klog.Fatalf("error while parsing config: %s", err.Error())
	}

	vtepIP, err := getVTEPIP(config)
	if err != nil {
		klog.Fatalf("error while parsing VTEPIP from config: %s", err.Error())
	}

	ovsClient, err := ovsclient.New()
	if err != nil {
		klog.Fatal(err)
	}

	provisioner := dpucniprovisioner.New(ovsClient, networkhelper.New(), kexec.New(), vtepIP)

	err = provisioner.RunOnce()
	if err != nil {
		klog.Fatal(err)
	}

	err = readyz.ReportReady()
	if err != nil {
		klog.Fatal(err)
	}

	klog.Info("DPU CNI Provisioner is ready")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

// parseConfig parses the DPUCNIProvisionerConfig from the filesystem. Notice that the config is cluster scoped and is
// not dedicated to this particular process. Additional filtering must be done to determine correct configuration for that
// instance.
func parseConfig() (*dpucniprovisionertypes.DPUCNIProvisionerConfig, error) {
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config dpucniprovisionertypes.DPUCNIProvisionerConfig
	err = json.Unmarshal(configContent, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// getVTEPIP figures out the VTEP IP to be configured by the provisioner
func getVTEPIP(c *dpucniprovisionertypes.DPUCNIProvisionerConfig) (*net.IPNet, error) {
	node := os.Getenv("NODE_NAME")
	if node == "" {
		return nil, errors.New("NODE_NAME environment variable is not found. This is supposed to be configured via Kubernetes Downward API in production")
	}

	vtepIPRaw, ok := c.VTEPIPs[node]
	if !ok {
		return nil, fmt.Errorf("VTEP IP not found in config for node %s", node)
	}

	vtepIP, err := netlink.ParseIPNet(vtepIPRaw)
	if err != nil {
		return nil, fmt.Errorf("error while parsing VTEP IP to net.IPNet: %w", err)
	}

	return vtepIP, nil
}
