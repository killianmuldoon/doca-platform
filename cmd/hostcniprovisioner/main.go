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

	hostcniprovisioner "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/host"
	hostcniprovisionerconfig "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/host/config"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/networkhelper"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/readyz"

	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	kexec "k8s.io/utils/exec"
)

const (
	// configPath is the path to the HOST CNI Provisioner configuration file
	configPath = "/etc/hostcniprovisioner/config.yaml"
)

func main() {
	klog.Info("Starting Host CNI Provisioner")
	config, err := parseConfig()
	if err != nil {
		klog.Fatalf("error while parsing config: %s", err.Error())
	}

	pfIP, err := getPFIP(config)
	if err != nil {
		klog.Fatalf("error while parsing PF IP from config: %s", err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := clock.RealClock{}
	provisioner := hostcniprovisioner.New(ctx, c, networkhelper.New(), kexec.New(), config.HostPF0, pfIP)

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

	klog.Info("Host CNI Provisioner is ready")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
	klog.Info("Received termination signal, terminating.")
	cancel()
	wg.Wait()
}

// parseConfig parses the HostCNIProvisionerConfig from the filesystem. Notice that the config is cluster scoped and is
// not dedicated to this particular process. Additional filtering must be done to determine correct configuration for that
// instance.
func parseConfig() (*hostcniprovisionerconfig.HostCNIProvisionerConfig, error) {
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config hostcniprovisionerconfig.HostCNIProvisionerConfig
	err = json.Unmarshal(configContent, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// getPFIP figures out the PF IP to be configured by the provisioner
func getPFIP(c *hostcniprovisionerconfig.HostCNIProvisionerConfig) (*net.IPNet, error) {
	node := os.Getenv("NODE_NAME")
	if node == "" {
		return nil, errors.New("NODE_NAME environment variable is not found. This is supposed to be configured via Kubernetes Downward API in production")
	}

	pfIPRaw, ok := c.PFIPs[node]
	if !ok {
		return nil, fmt.Errorf("PF IP not found in config for node %s", node)
	}

	pfIP, err := netlink.ParseIPNet(pfIPRaw)
	if err != nil {
		return nil, fmt.Errorf("error while parsing PF IP to net.IPNet: %w", err)
	}

	return pfIP, nil
}
