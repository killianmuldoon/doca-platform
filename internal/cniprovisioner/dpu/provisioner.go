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
	"os"
	"path/filepath"
	"time"

	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/networkhelper"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/ovsclient"
	dpu "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/state"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

	// ovnInputGatewayOptsPath is the path to the file in which kubeovn-controller expects the additional gateway opts
	ovnInputGatewayOptsPath = "/etc/init-output/ovn_gateway_opts"
	// ovnInputRouterSubnetPath is the path to the file in which kubeovn-controller expects the Gateway Router Subnet
	ovnInputRouterSubnetPath = "/etc/init-output/ovn_gateway_router_subnet"
)

type DPUCNIProvisioner struct {
	ctx                       context.Context
	ensureConfigurationTicker clock.Ticker
	ovsClient                 ovsclient.OVSClient
	networkHelper             networkhelper.NetworkHelper
	exec                      kexec.Interface
	kubernetesClient          kubernetes.Interface

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
	// pfIP is the IP that should be added to the PF on the host
	pfIP *net.IPNet
	// dpuHostName is the name of the DPU.
	dpuHostName string

	// dhcpCmd is the struct that holds information about the DHCP Server process
	dhcpCmd kexec.Cmd
}

// New creates a DPUCNIProvisioner that can configure the system
func New(ctx context.Context,
	clock clock.WithTicker,
	ovsClient ovsclient.OVSClient,
	networkHelper networkhelper.NetworkHelper,
	exec kexec.Interface,
	kubernetesClient kubernetes.Interface,
	vtepIPNet *net.IPNet,
	gateway net.IP,
	vtepCIDR *net.IPNet,
	hostCIDR *net.IPNet,
	pfIP *net.IPNet,
	dpuHostName string,
) *DPUCNIProvisioner {
	return &DPUCNIProvisioner{
		ctx:                       ctx,
		ensureConfigurationTicker: clock.NewTicker(30 * time.Second),
		ovsClient:                 ovsClient,
		networkHelper:             networkHelper,
		exec:                      exec,
		kubernetesClient:          kubernetesClient,
		FileSystemRoot:            "",
		vtepIPNet:                 vtepIPNet,
		gateway:                   gateway,
		vtepCIDR:                  vtepCIDR,
		hostCIDR:                  hostCIDR,
		pfIP:                      pfIP,
		dpuHostName:               dpuHostName,
	}
}

// RunOnce runs the provisioning flow once and exits
func (p *DPUCNIProvisioner) RunOnce() error {
	if err := p.configure(); err != nil {
		return err
	}
	klog.Info("Configuration complete.")
	if err := p.startDHCPServer(); err != nil {
		return fmt.Errorf("error while starting DHCP server: %w", err)
	}
	klog.Info("DHCP Server started.")

	return nil
}

// stop stops the provisioner
func (p *DPUCNIProvisioner) Stop() {
	p.dhcpCmd.Stop()
	klog.Info("Provisioner stopped")
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
	klog.Info("Configuring Kubernetes host name in OVS")
	if err := p.findAndSetKubernetesHostNameInOVS(); err != nil {
		return fmt.Errorf("error while setting the Kubernetes Host Name in OVS: %w", err)
	}

	klog.Info("Configuring system to enable pod to pod on different node connectivity")
	if err := p.configurePodToPodOnDifferentNodeConnectivity(); err != nil {
		return err
	}

	klog.Info("Writing OVN Kubernetes expected input files")
	if err := p.writeFilesForOVN(); err != nil {
		return err
	}

	return nil
}

// findAndSetKubernetesHostNameInOVS discovers and sets the Kubernetes Host Name in OVS
func (p *DPUCNIProvisioner) findAndSetKubernetesHostNameInOVS() error {
	nodeClient := p.kubernetesClient.CoreV1().Nodes()
	n, err := nodeClient.Get(p.ctx, p.dpuHostName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error while getting Kubernetes Node: %w", err)
	}
	hostName, ok := n.Labels[dpu.HostNameDPULabelKey]
	if !ok {
		return fmt.Errorf("required label %s is not set on node %s in the DPU cluster", dpu.HostNameDPULabelKey, p.dpuHostName)
	}

	if err := p.ovsClient.SetKubernetesHostNodeName(hostName); err != nil {
		return fmt.Errorf("error while setting the Kubernetes Host Name in OVS: %w", err)
	}
	return nil
}

// configurePodToPodOnDifferentNodeConnectivity configures the VTEP interface (br-ovn) and the ovn-encap-ip external ID
// so that traffic going through the geneve tunnels can function as expected.
func (p *DPUCNIProvisioner) configurePodToPodOnDifferentNodeConnectivity() error {
	if err := p.setLinkIPAddressIfNotSet(brOVN, p.vtepIPNet); err != nil {
		return fmt.Errorf("error while setting VTEP IP: %w", err)
	}
	if err := p.networkHelper.SetLinkUp(brOVN); err != nil {
		return fmt.Errorf("error while setting link %s up: %w", brOVN, err)
	}

	_, vtepNetwork, err := net.ParseCIDR(p.vtepIPNet.String())
	if err != nil {
		return fmt.Errorf("error while parsing network from VTEP IP %s: %w", p.vtepIPNet.String(), err)
	}

	if vtepNetwork.String() != p.vtepCIDR.String() {
		// Add route related to traffic that needs to go from one Pod running on worker Node A to another Pod running
		// on worker Node B.
		if err := p.addRouteIfNotExists(p.vtepCIDR, p.gateway, brOVN, nil); err != nil {
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
	if err := p.addRouteIfNotExists(p.hostCIDR, p.gateway, brOVN, ptr.To[int](10000)); err != nil {
		return fmt.Errorf("error while adding route %s %s %s: %w", p.hostCIDR, p.gateway.String(), brOVN, err)
	}

	if err = p.ovsClient.SetOVNEncapIP(p.vtepIPNet.IP); err != nil {
		return fmt.Errorf("error while setting the OVN Encap IP: %w", err)
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
	if err := p.networkHelper.SetLinkIPAddress(link, ipNet); err != nil {
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
	if err := p.networkHelper.AddRoute(network, gateway, device, metric); err != nil {
		return fmt.Errorf("error adding route: %w", err)
	}
	return nil
}

// writeFilesForOVN writes the input files that the ovnkube-controller expects
func (p *DPUCNIProvisioner) writeFilesForOVN() error {
	if err := p.writeOVNInputGatewayOptsFile(); err != nil {
		return fmt.Errorf("error while writing the file OVN expects to find additional gateway options: %w", err)
	}

	if err := p.writeOVNInputRouterSubnetPath(); err != nil {
		return fmt.Errorf("error while writing the file OVN expects to find the gateway router subnet: %w", err)
	}

	return nil
}

// writeOVNInputGatewayOptsFile writes the file in which kubeovn-controller expects the additional gateway opts
func (p *DPUCNIProvisioner) writeOVNInputGatewayOptsFile() error {
	configPath := filepath.Join(p.FileSystemRoot, ovnInputGatewayOptsPath)
	content := fmt.Sprintf("--gateway-nexthop=%s", p.gateway.String())
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("error while writing file %s: %w", configPath, err)
	}
	return nil
}

// writeOVNInputRouterSubnetPath writes the file in which kubeovn-controller expects the Gateway Router Subnet
func (p *DPUCNIProvisioner) writeOVNInputRouterSubnetPath() error {
	configPath := filepath.Join(p.FileSystemRoot, ovnInputRouterSubnetPath)
	_, vtepNetwork, err := net.ParseCIDR(p.vtepIPNet.String())
	if err != nil {
		return fmt.Errorf("error while parsing network from VTEP IP %s: %w", p.vtepIPNet.String(), err)
	}
	content := vtepNetwork.String()
	err = os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("error while writing file %s: %w", configPath, err)
	}
	return nil
}

// startDHCPServer starts a DHCP Server to enable the PF on the host to get an IP.
func (p *DPUCNIProvisioner) startDHCPServer() error {
	if p.dhcpCmd != nil {
		klog.Warning("DHCP Server already running")
		return nil
	}

	_, vtepNetwork, err := net.ParseCIDR(p.vtepIPNet.String())
	if err != nil {
		return fmt.Errorf("error while parsing network from VTEP IP %s: %w", p.vtepIPNet.String(), err)
	}

	mac, err := p.networkHelper.GetPFRepMACAddress("pf0hpf")
	if err != nil {
		return fmt.Errorf("error while parsing MAC address of the PF on the host: %w", err)
	}

	cmd := p.exec.Command("dnsmasq",
		"--keep-in-foreground",
		"--port=0",         // Disable DNS Server
		"--log-facility=-", // Log to stderr
		fmt.Sprintf("--interface=%s", brOVN),
		fmt.Sprintf("--dhcp-option=option:router,%s", p.gateway.String()),
		fmt.Sprintf("--dhcp-range=%s,static", vtepNetwork.IP.String()),
		fmt.Sprintf("--dhcp-host=%s,%s", mac, p.pfIP.IP.String()),
	)
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error while starting the DHCP server: %w", err)
	}

	p.dhcpCmd = cmd
	return nil
}
