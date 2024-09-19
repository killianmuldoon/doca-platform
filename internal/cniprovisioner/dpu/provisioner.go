/*
Copyright 2024 NVIDIA.

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

package dpucniprovisioner

import (
	"context"
	"fmt"
	"net"
	"time"

	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/networkhelper"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/ovsclient"

	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	kexec "k8s.io/utils/exec"
	"k8s.io/utils/ptr"
)

const (
	// brOVN is the name of the bridge that is used by OVN as the external bridge (br-ex). This is the bridge that is
	// later connected with br-sfc. In the current OVN IC w/ DPU implementation, the internal port of this bridge acts
	// as the VTEP.
	brOVN = "br-ovn"
)

type DPUCNIProvisioner struct {
	ctx                       context.Context
	ensureConfigurationTicker clock.Ticker
	ovsClient                 ovsclient.OVSClient
	networkHelper             networkhelper.NetworkHelper
	exec                      kexec.Interface

	// FileSystemRoot controls the file system root. It's used for enabling easier testing of the package. Defaults to
	// empty.
	FileSystemRoot string

	// vtepIPNet is the IP that should be added to the VTEP interface.
	vtepIPNet *net.IPNet
	// gateway is the gateway IP that is configured on the routes related to OVN Kubernetes reaching its peer nodes
	// when traffic needs to go from one Pod running on Node A to another Pod running on Node B.
	gateway net.IP
	// vtepCIDR is the CIDR in which all the VTEP IPs of all the DPUs in the DPU cluster belong to. This CIDR is
	// configured on the routes related to traffic that needs to go from one Pod running on worker Node A to another Pod
	// running on worker Node B.
	vtepCIDR *net.IPNet
	// hostCIDR is the CIDR of the host machines that is configured on the routes related to OVN Kubernetes reaching
	// its peer nodes when traffic needs to go from one Pod running on worker Node A to another Pod running on control
	// plane A (and vice versa).
	hostCIDR *net.IPNet
}

// New creates a DPUCNIProvisioner that can configure the system
func New(ctx context.Context,
	clock clock.WithTicker,
	ovsClient ovsclient.OVSClient,
	networkHelper networkhelper.NetworkHelper,
	exec kexec.Interface,
	vtepIPNet *net.IPNet,
	gateway net.IP,
	vtepCIDR *net.IPNet,
	hostCIDR *net.IPNet,
) *DPUCNIProvisioner {
	return &DPUCNIProvisioner{
		ctx:                       ctx,
		ensureConfigurationTicker: clock.NewTicker(30 * time.Second),
		ovsClient:                 ovsClient,
		networkHelper:             networkHelper,
		exec:                      exec,
		FileSystemRoot:            "",
		vtepIPNet:                 vtepIPNet,
		gateway:                   gateway,
		vtepCIDR:                  vtepCIDR,
		hostCIDR:                  hostCIDR,
	}
}

// RunOnce runs the provisioning flow once and exits
func (p *DPUCNIProvisioner) RunOnce() error {
	err := p.configure()
	if err != nil {
		return err
	}
	klog.Info("Configuration complete.")
	return nil
}

// EnsureConfiguration ensures that particular configuration is in place. This is a blocking function.
func (p *DPUCNIProvisioner) EnsureConfiguration() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.ensureConfigurationTicker.C():
		}
	}
}

// configure runs the provisioning flow once
func (p *DPUCNIProvisioner) configure() error {
	klog.Info("Configuring system to enable pod to pod on different node connectivity")
	err := p.configurePodToPodOnDifferentNodeConnectivity()
	if err != nil {
		return err
	}

	return nil
}

// configurePodToPodOnDifferentNodeConnectivity configures the VTEP interface (br-ovn) and the ovn-encap-ip external ID
// so that traffic going through the geneve tunnels can function as expected.
func (p *DPUCNIProvisioner) configurePodToPodOnDifferentNodeConnectivity() error {
	err := p.setLinkIPAddressIfNotSet(brOVN, p.vtepIPNet)
	if err != nil {
		return fmt.Errorf("error while setting VTEP IP: %w", err)
	}
	err = p.networkHelper.SetLinkUp(brOVN)
	if err != nil {
		return fmt.Errorf("error while setting link %s up: %w", brOVN, err)
	}

	_, vtepNetwork, err := net.ParseCIDR(p.vtepIPNet.String())
	if err != nil {
		return fmt.Errorf("error while parsing network from VTEP IP %s: %w", p.vtepIPNet.String(), err)
	}

	if vtepNetwork.String() != p.vtepCIDR.String() {
		// Add route related to traffic that needs to go from one Pod running on worker Node A to another Pod running
		// on worker Node B.
		err = p.addRouteIfNotExists(p.vtepCIDR, p.gateway, brOVN, nil)
		if err != nil {
			return fmt.Errorf("error while adding route %s %s %s: %w", p.vtepCIDR, p.gateway.String(), brOVN, err)
		}
	}

	// Add route related to traffic that needs to go from one Pod running on worker Node A to another Pod running on
	// control plane A (and vice versa).
	//
	// In our setup, we will already have a route pointing to the same CIDR via the SF designated for kubelet traffic
	// which gets a DHCP IP in that CIDR. Given that, we need to set the metric of this route to something very high
	// so that it's the last preferred route in the route table for that CIDR. The reason for that is this OVS bug that
	// selects the route with the highest prio as preferred https://redmine.mellanox.com/issues/3871067.
	err = p.addRouteIfNotExists(p.hostCIDR, p.gateway, brOVN, ptr.To[int](10000))
	if err != nil {
		return fmt.Errorf("error while adding route %s %s %s: %w", p.hostCIDR, p.gateway.String(), brOVN, err)
	}

	err = p.ovsClient.SetOVNEncapIP(p.vtepIPNet.IP)
	if err != nil {
		return err
	}

	return nil
}

// setLinkIPAddressIfNotSet sets an IP address to a link if it's not already set
func (p *DPUCNIProvisioner) setLinkIPAddressIfNotSet(link string, ipNet *net.IPNet) error {
	hasIP, err := p.networkHelper.LinkIPAddressExists(link, ipNet)
	if err != nil {
		return fmt.Errorf("error checking whether IP exists: %w", err)
	}
	if hasIP {
		klog.Infof("Link %s has IP %s, skipping configuration", link, ipNet)
		return nil
	}
	err = p.networkHelper.SetLinkIPAddress(link, ipNet)
	if err != nil {
		return fmt.Errorf("error setting IP address: %w", err)
	}
	return nil
}

// addRouteIfNotExists adds a route if it doesn't already exist
func (p *DPUCNIProvisioner) addRouteIfNotExists(network *net.IPNet, gateway net.IP, device string, metric *int) error {
	hasRoute, err := p.networkHelper.RouteExists(network, gateway, device)
	if err != nil {
		return fmt.Errorf("error checking whether route exists: %w", err)
	}
	if hasRoute {
		klog.Infof("Route %s %s %s exists, skipping configuration", network, gateway, device)
		return nil
	}
	err = p.networkHelper.AddRoute(network, gateway, device, metric)
	if err != nil {
		return fmt.Errorf("error adding route: %w", err)
	}
	return nil
}
