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
	"strings"
	"time"

	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/networkhelper"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/ovsclient"

	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	kexec "k8s.io/utils/exec"
	"k8s.io/utils/ptr"
)

const (
	// brInt is the name of the OVN integration bridge
	brInt = "br-int"
	// brOVN is the name of the bridge that is used to communicate with OVN. This is the bridge where the rest of the HBN
	// will connect.
	brOVN = "br-ovn"

	// ovsSystemdConfigPath is the configuration file of the openvswitch systemd service
	ovsSystemdConfigPath = "/etc/default/openvswitch-switch"
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

	// brEx is the name of the OVN external bridge and must match the name of the PF on the host.
	brEx string
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
	pf0 string) *DPUCNIProvisioner {
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
		brEx:                      pf0,
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
			// We need to ensure that the OVN Management VF is configured in a loop to ensure that when the host
			// restarts and the VFs are removed and recreated, the VF is configured again so that OVN Kubernetes can
			// function.
			err := p.configureOVNManagementVF()
			if err != nil {
				klog.Errorf("failed to ensure OVN management VF configuration: %s", err.Error())
			}
		}
	}
}

// configure runs the provisioning flow without checking existing configuration
func (p *DPUCNIProvisioner) configure() error {
	klog.Info("Configuring OVS Daemon")
	err := p.configureOVSDaemon()
	if err != nil {
		return err
	}

	klog.Info("Cleaning up initial OVS setup")
	err = p.cleanUpBridges()
	if err != nil {
		return err
	}

	klog.Info("Configuring OVS bridges and ports")
	// TODO: Create a better data structure for bridges.
	// TODO: Parse IP for br-int via the interface which will use DHCP.
	for bridge, controller := range map[string]string{
		brInt:  "ptcp:8510:169.254.55.1",
		p.brEx: "ptcp:8511",
		brOVN:  "",
	} {
		err := p.setupOVSBridge(bridge, controller)
		if err != nil {
			return err
		}
	}

	brExTobrOVNPatchPort, _, err := p.connectOVSBridges(p.brEx, brOVN)
	if err != nil {
		return err
	}

	klog.Info("Configuring system to enable pod to pod on different node connectivity")
	err = p.configurePodToPodOnDifferentNodeConnectivity(brExTobrOVNPatchPort)
	if err != nil {
		return err
	}

	klog.Info("Configuring system to enable host to service connectivity")
	err = p.configureHostToServiceConnectivity()
	if err != nil {
		return err
	}

	klog.Info("Configuring VF used for OVN Management")
	err = p.configureOVNManagementVF()
	if err != nil {
		return err
	}

	return nil
}

// setupOVSBridge creates an OVS bridge tailored to work with OVS-DOCA
func (p *DPUCNIProvisioner) setupOVSBridge(name string, controller string) error {
	err := p.ovsClient.AddBridgeIfNotExists(name)
	if err != nil {
		return err
	}

	err = p.ovsClient.SetBridgeDataPathType(name, ovsclient.NetDev)
	if err != nil {
		return err
	}

	if len(controller) == 0 {
		return nil
	}

	// This is required so that OVN on the host can access the OVS on the DPU via TCP endpoints
	return p.ovsClient.SetBridgeController(name, controller)
}

// connectBridges connects two bridges in OVS using patch ports
func (p *DPUCNIProvisioner) connectOVSBridges(brA string, brB string) (string, string, error) {
	portFormat := "%s-to-%s"

	brAPatchPort := fmt.Sprintf(portFormat, brA, brB)
	brBPatchPort := fmt.Sprintf(portFormat, brB, brA)
	err := p.addOVSPatchPortWithPeer(brA, brAPatchPort, brBPatchPort)
	if err != nil {
		return "", "", err
	}
	err = p.addOVSPatchPortWithPeer(brB, brBPatchPort, brAPatchPort)
	if err != nil {
		return "", "", err
	}
	return brAPatchPort, brBPatchPort, nil
}

// addPatchPortWithPeer adds a patch port to a bridge and configures a peer for that port
func (p *DPUCNIProvisioner) addOVSPatchPortWithPeer(bridge string, port string, peer string) error {
	// TODO: Check for this error and validate expected error, otherwise return error
	_ = p.ovsClient.AddPortIfNotExists(bridge, port)
	err := p.ovsClient.SetPortType(port, ovsclient.Patch)
	if err != nil {
		return err
	}
	return p.ovsClient.SetPatchPortPeer(port, peer)
}

// configurePodToPodOnDifferentNodeConnectivity configures a VTEP interface and the ovn-encap-ip external ID so that
// traffic going through the geneve tunnels can function as expected.
func (p *DPUCNIProvisioner) configurePodToPodOnDifferentNodeConnectivity(uplinkPort string) error {
	vtep := "vtep0"
	err := p.ovsClient.AddPortIfNotExists(brOVN, vtep)
	if err != nil {
		return err
	}

	err = p.ovsClient.SetPortType(vtep, ovsclient.Internal)
	if err != nil {
		return err
	}

	err = p.setLinkIPAddressIfNotSet(vtep, p.vtepIPNet)
	if err != nil {
		return fmt.Errorf("error while setting VTEP IP: %w", err)
	}
	err = p.networkHelper.SetLinkUp(vtep)
	if err != nil {
		return fmt.Errorf("error while setting link %s up: %w", vtep, err)
	}

	_, vtepNetwork, err := net.ParseCIDR(p.vtepIPNet.String())
	if err != nil {
		return fmt.Errorf("error while parsing network from VTEP IP %s: %w", p.vtepIPNet.String(), err)
	}

	if vtepNetwork.String() != p.vtepCIDR.String() {
		// Add route related to traffic that needs to go from one Pod running on worker Node A to another Pod running
		// on worker Node B.
		err = p.addRouteIfNotExists(p.vtepCIDR, p.gateway, vtep, nil)
		if err != nil {
			return fmt.Errorf("error while adding route %s %s %s: %w", p.vtepCIDR, p.gateway.String(), vtep, err)
		}
	}

	// Add route related to traffic that needs to go from one Pod running on worker Node A to another Pod running on
	// control plane A (and vice versa).
	//
	// In our setup, we will already have a route pointing to the same CIDR via the SF designated for kubelet traffic
	// which gets a DHCP IP in that CIDR. Given that, we need to set the metric of this route to something very high
	// so that it's the last preferred route in the route table for that CIDR. The reason for that is this OVS bug that
	// selects the route with the highest prio as preferred https://redmine.mellanox.com/issues/3871067.
	err = p.addRouteIfNotExists(p.hostCIDR, p.gateway, vtep, ptr.To[int](10000))
	if err != nil {
		return fmt.Errorf("error while adding route %s %s %s: %w", p.hostCIDR, p.gateway.String(), vtep, err)
	}

	err = p.ovsClient.SetOVNEncapIP(p.vtepIPNet.IP)
	if err != nil {
		return err
	}

	// This is also needed for pods to access the internet
	return p.ovsClient.SetBridgeUplinkPort(p.brEx, uplinkPort)
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

// configureHostToServiceConnectivity configures br-ex so that Service ClusterIP traffic from the host to the DPU finds
// it's way to the br-int
func (p *DPUCNIProvisioner) configureHostToServiceConnectivity() error {
	pfRep := "pf0hpf"
	err := p.ovsClient.AddPortIfNotExists(p.brEx, pfRep)
	if err != nil {
		return err
	}
	err = p.ovsClient.SetPortType(pfRep, ovsclient.DPDK)
	if err != nil {
		return err
	}
	err = p.ovsClient.SetBridgeHostToServicePort(p.brEx, pfRep)
	if err != nil {
		return err
	}

	mac, err := p.networkHelper.GetPFRepMACAddress(pfRep)
	if err != nil {
		return err
	}
	return p.ovsClient.SetBridgeMAC(p.brEx, mac)
}

// configureOVNManagementVF configures the VF that is going to be used by OVN Kubernetes for the management
// (ovn-k8s-mp0_0). We need to do that because OVN Kubernetes will rename the interface on the host, but it won't be
// able to plug that interface on the OVS since the DPU won't have such interface.
func (p *DPUCNIProvisioner) configureOVNManagementVF() error {
	vfRepresentorLinkName := "pf0vf0"
	expectedLinkName := "ovn-k8s-mp0_0"
	vfRepresentorLinkExists, err := p.networkHelper.LinkExists(vfRepresentorLinkName)
	if err != nil {
		return fmt.Errorf("error while checking whether link %s exists: %w", vfRepresentorLinkName, err)
	}

	if vfRepresentorLinkExists {
		err := p.networkHelper.SetLinkDown(vfRepresentorLinkName)
		if err != nil {
			return fmt.Errorf("error while setting link %s down: %w", vfRepresentorLinkName, err)
		}

		err = p.networkHelper.RenameLink(vfRepresentorLinkName, expectedLinkName)
		if err != nil {
			return fmt.Errorf("error while renaming link %s to %s: %w", vfRepresentorLinkName, expectedLinkName, err)
		}
	}

	err = p.networkHelper.SetLinkUp(expectedLinkName)
	if err != nil {
		return fmt.Errorf("error while setting link %s up: %w", expectedLinkName, err)
	}

	return nil
}

// configureVFs renames the existing VFs to map the fake environment we have on the host
//
//nolint:unused
func (p *DPUCNIProvisioner) configureVFs() error { panic("unimplemented") }

// cleanUpBridges removes all the relevant bridges
func (p *DPUCNIProvisioner) cleanUpBridges() error {
	// Default bridges that exist in freshly installed DPUs. This step is required as we need to plug in the PF later,
	// see plugOVSUplink(). Without p0 plugged into OVS, adding ports of type DPDK fails.
	err := p.ovsClient.DeleteBridgeIfExists("ovsbr1")
	if err != nil {
		return err
	}
	err = p.ovsClient.DeleteBridgeIfExists("ovsbr2")
	if err != nil {
		return err
	}
	return nil
}

// configureOVSDaemon configures the OVS Daemon and triggers a restart of the daemon via systemd
func (p *DPUCNIProvisioner) configureOVSDaemon() error {
	configuredNow, err := p.exposeOVSDBOverTCP()
	if err != nil {
		return err
	}

	// Enable OVS DOCA. It requires hugepages which are going to be configured by the provisioning workstream.
	err = p.ovsClient.SetDOCAInit(true)
	if err != nil {
		return err
	}

	// We avoid restarting OVS because it takes time if the OVS is already configured. We don't check for OVS being
	// configured in DOCA to avoid extra code for now. It's very unlikely we hit this case.
	if configuredNow {
		cmd := p.exec.Command("systemctl", "restart", "openvswitch-switch.service")
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("error while restarting OVS systemd service: %w", err)
		}
	}
	return nil
}

// exposeOVSDBOverTCP reconfigures OVS to expose ovs-db via TCP
func (p *DPUCNIProvisioner) exposeOVSDBOverTCP() (bool, error) {
	configPath := filepath.Join(p.FileSystemRoot, ovsSystemdConfigPath)
	content, err := os.ReadFile(configPath)
	if err != nil {
		return false, fmt.Errorf("error while reading file %s: %w", configPath, err)
	}

	var configured bool
	// TODO: Could do better parsing here but that _should_ be good enough given that the default is that
	// OVS_CTL_OPTS is not specified.
	if !strings.Contains(string(content), "--remote=ptcp:8500") {
		content = append(content, "\nOVS_CTL_OPTS=\"--ovsdb-server-options=--remote=ptcp:8500\""...)
		err := os.WriteFile(configPath, content, 0644)
		if err != nil {
			return false, fmt.Errorf("error while writing file %s: %w", configPath, err)
		}
		configured = true
	}
	return configured, nil
}
