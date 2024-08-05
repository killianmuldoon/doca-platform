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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"

	dpucniprovisioner "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/dpu"
	dpucniprovisionerconfig "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/dpu/config"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/networkhelper"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/ovsclient"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/readyz"

	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
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
		klog.Fatalf("error while parsing VTEP IP from config: %s", err.Error())
	}

	gateway, err := getGateway(config)
	if err != nil {
		klog.Fatalf("error while parsing Gateway from config: %s", err.Error())
	}

	_, vtepCIDR, err := net.ParseCIDR(config.VTEPCIDR)
	if err != nil {
		klog.Fatalf("error while parsing VTEP CIDR %s as net.IPNet: %s", config.VTEPCIDR, err.Error())
	}

	_, hostCIDR, err := net.ParseCIDR(config.HostCIDR)
	if err != nil {
		klog.Fatalf("error while parsing Host CIDR %s as net.IPNet: %s", config.HostCIDR, err.Error())
	}

	ovsClient, err := ovsclient.New()
	if err != nil {
		klog.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := clock.RealClock{}
	provisioner := dpucniprovisioner.New(ctx, c, ovsClient, networkhelper.New(), kexec.New(), vtepIP, gateway, vtepCIDR, hostCIDR, getHostPF0(config))

	err = provisioner.RunOnce()
	if err != nil {
		klog.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		provisioner.EnsureConfiguration()
	}()

	err = readyz.ReportReady()
	if err != nil {
		klog.Fatal(err)
	}

	klog.Info("DPU CNI Provisioner is ready")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
	klog.Info("Received termination signal, terminating.")
	cancel()
	wg.Wait()
}

// parseConfig parses the DPUCNIProvisionerConfig from the filesystem. Notice that the config is cluster scoped and is
// not dedicated to this particular process. Additional filtering must be done to determine correct configuration for that
// instance.
func parseConfig() (*dpucniprovisionerconfig.DPUCNIProvisionerConfig, error) {
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config dpucniprovisionerconfig.DPUCNIProvisionerConfig
	err = json.Unmarshal(configContent, &config)
	if err != nil {
		return nil, err
	}

	node := os.Getenv("NODE_NAME")
	if node == "" {
		return nil, errors.New("NODE_NAME environment variable is not found. This is supposed to be configured via Kubernetes Downward API in production")
	}

	if _, ok := config.PerNodeConfig[node]; !ok {
		return nil, errors.New("perNodeConfig for node %s doesn't exist")
	}

	return &config, nil
}

// getVTEPIP figures out the VTEP IP to be configured by the provisioner
func getVTEPIP(c *dpucniprovisionerconfig.DPUCNIProvisionerConfig) (*net.IPNet, error) {
	node := os.Getenv("NODE_NAME")
	vtepIPRaw := c.PerNodeConfig[node].VTEPIP
	vtepIP, err := netlink.ParseIPNet(vtepIPRaw)
	if err != nil {
		return nil, fmt.Errorf("error while parsing VTEP IP to net.IPNet: %w", err)
	}

	return vtepIP, nil
}

// getGateway returns the Gateway IP to be configured by the provisioner
func getGateway(c *dpucniprovisionerconfig.DPUCNIProvisionerConfig) (net.IP, error) {
	node := os.Getenv("NODE_NAME")
	gatewayRaw := c.PerNodeConfig[node].Gateway
	gateway := net.ParseIP(gatewayRaw)
	if gateway == nil {
		return nil, errors.New("error while parsing Gateway IP to net.IP: input is not valid")
	}

	return gateway, nil
}

// getHostPF0 figures out the name of the host pf0 from the given config
func getHostPF0(c *dpucniprovisionerconfig.DPUCNIProvisionerConfig) string {
	node := os.Getenv("NODE_NAME")
	hostPF0 := c.PerNodeConfig[node].HostPF0
	return hostPF0
}
