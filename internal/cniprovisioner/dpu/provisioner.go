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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/networkhelper"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/ovsclient"

	"k8s.io/klog/v2"
	kexec "k8s.io/utils/exec"
)

const (
	// brInt is the name of the OVN integration bridge
	brInt = "br-int"
	// brEx is the name of the OVN external bridge and must match the name of the PF on the host
	brEx = "ens2f0np0"
	// brOVN is the name of the bridge that is used to communicate with OVN. This is the bridge where the rest of the HBN
	// will connect.
	brOVN = "br-ovn"

	// ovsSystemdConfigPath is the configuration file of the openvswitch systemd service
	ovsSystemdConfigPath = "/etc/default/openvswitch-switch"
)

type DPUCNIProvisioner struct {
	ovsClient     ovsclient.OVSClient
	networkHelper networkhelper.NetworkHelper
	exec          kexec.Interface

	// FileSystemRoot controls the file system root. It's used for enabling easier testing of the package. Defaults to
	// empty.
	FileSystemRoot string

	// vtepIPNet is the IP that should be added to the VTEP interface.
	vtepIPNet *net.IPNet
}

// New creates a DPUCNIProvisioner that can configure the system
func New(ovsClient ovsclient.OVSClient, networkHelper networkhelper.NetworkHelper, exec kexec.Interface, vtepIPNet *net.IPNet) *DPUCNIProvisioner {
	return &DPUCNIProvisioner{
		ovsClient:      ovsClient,
		networkHelper:  networkHelper,
		exec:           exec,
		FileSystemRoot: "",
		vtepIPNet:      vtepIPNet,
	}
}

// RunOnce runs the provisioning flow once and exits
func (p *DPUCNIProvisioner) RunOnce() error {
	isConfigured, err := p.isSystemAlreadyConfigured()
	if err != nil {
		return err
	}
	if isConfigured {
		klog.Info("System is already configured, skipping configuration")
		return nil
	}
	err = p.configure()
	if err != nil {
		return err
	}
	klog.Info("Configuration complete.")
	return nil
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
		brInt: "ptcp:8510:10.100.1.1",
		brEx:  "ptcp:8511",
		brOVN: "",
	} {
		err := p.setupOVSBridge(bridge, controller)
		if err != nil {
			return err
		}
	}

	brExTobrOVNPatchPort, _, err := p.connectOVSBridges(brEx, brOVN)
	if err != nil {
		return err
	}

	err = p.plugOVSUplink()
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
	err := p.ovsClient.AddBridge(name)
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
	_ = p.ovsClient.AddPort(bridge, port)
	err := p.ovsClient.SetPortType(port, ovsclient.Patch)
	if err != nil {
		return err
	}
	return p.ovsClient.SetPatchPortPeer(port, peer)
}

// plugOVSUplink plugs the uplink for the bridge setup this component is creating.
// TODO: Replace p0 with patch port on br-sfc
func (p *DPUCNIProvisioner) plugOVSUplink() error {
	uplink := "p0"
	err := p.ovsClient.AddPort(brOVN, uplink)
	if err != nil {
		return err
	}
	return p.ovsClient.SetPortType(uplink, ovsclient.DPDK)
}

// configurePodToPodOnDifferentNodeConnectivity configures a VTEP interface and the ovn-encap-ip external ID so that
// traffic going through the geneve tunnels can function as expected.
func (p *DPUCNIProvisioner) configurePodToPodOnDifferentNodeConnectivity(uplinkPort string) error {
	vtep := "vtep0"
	err := p.ovsClient.AddPort(brOVN, vtep)
	if err != nil {
		return err
	}

	err = p.ovsClient.SetPortType(vtep, ovsclient.Internal)
	if err != nil {
		return err
	}

	err = p.networkHelper.SetLinkIPAddress(vtep, p.vtepIPNet)
	if err != nil {
		return fmt.Errorf("error while setting VTEP IP: %w", err)
	}
	err = p.networkHelper.SetLinkUp(vtep)
	if err != nil {
		return fmt.Errorf("error while setting link %s up: %w", vtep, err)
	}

	err = p.ovsClient.SetOVNEncapIP(p.vtepIPNet.IP)
	if err != nil {
		return err
	}

	// This is also needed for pods to access the internet
	return p.ovsClient.SetBridgeUplinkPort(brEx, uplinkPort)
}

// configureHostToServiceConnectivity configures br-ex so that Service ClusterIP traffic from the host to the DPU finds
// it's way to the br-int
func (p *DPUCNIProvisioner) configureHostToServiceConnectivity() error {
	pfRep := "pf0hpf"
	err := p.ovsClient.AddPort(brEx, pfRep)
	if err != nil {
		return err
	}
	err = p.ovsClient.SetPortType(pfRep, ovsclient.DPDK)
	if err != nil {
		return err
	}
	err = p.ovsClient.SetBridgeHostToServicePort(brEx, pfRep)
	if err != nil {
		return err
	}

	mac, err := p.networkHelper.GetPFRepMACAddress(pfRep)
	if err != nil {
		return err
	}
	return p.ovsClient.SetBridgeMAC(brEx, mac)
}

// configureOVNManagementVF configures the VF that is going to be used by OVN Kubernetes for the management
// (ovn-k8s-mp0_0). We need to do that because OVN Kubernetes will rename the interface on the host, but it won't be
// able to plug that interface on the OVS since the DPU won't have such interface.
func (p *DPUCNIProvisioner) configureOVNManagementVF() error {
	vfRepresentorLinkName := "pf0vf0"
	expectedLinkName := "ovn-k8s-mp0_0"
	err := p.networkHelper.SetLinkDown(vfRepresentorLinkName)
	if err != nil {
		return fmt.Errorf("error while setting link %s down: %w", vfRepresentorLinkName, err)
	}

	err = p.networkHelper.RenameLink(vfRepresentorLinkName, expectedLinkName)
	if err != nil {
		return fmt.Errorf("error while renaming link %s to %s: %w", vfRepresentorLinkName, expectedLinkName, err)
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

// isSystemAlreadyConfigured checks if the system is already configured. No thorough checks are done, just a high level
// check to avoid re-running the configuration.
// TODO: Make rest of the calls idempotent and skip such check.
func (p *DPUCNIProvisioner) isSystemAlreadyConfigured() (bool, error) {
	exists, err := p.ovsClient.BridgeExists(brInt)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	exists, err = p.ovsClient.BridgeExists(brEx)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	exists, err = p.ovsClient.BridgeExists(brOVN)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	return true, nil
}

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

	// Bridges that are created as part of this process
	err = p.ovsClient.DeleteBridgeIfExists(brInt)
	if err != nil {
		return err
	}
	err = p.ovsClient.DeleteBridgeIfExists(brEx)
	if err != nil {
		return err
	}
	return p.ovsClient.DeleteBridgeIfExists(brOVN)
}

// configureOVSDaemon configures the OVS Daemon and triggers a restart of the daemon via systemd
func (p *DPUCNIProvisioner) configureOVSDaemon() error {
	err := p.exposeOVSDBOverTCP()
	if err != nil {
		return err
	}

	// Enable OVS DOCA. It requires hugepages which are going to be configured by the provisioning workstream.
	err = p.ovsClient.SetDOCAInit(true)
	if err != nil {
		return err
	}

	cmd := p.exec.Command("systemctl", "restart", "openvswitch-switch.service")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error while restarting OVS systemd service: %w", err)
	}
	return nil
}

// exposeOVSDBOverTCP reconfigures OVS to expose ovs-db via TCP
func (p *DPUCNIProvisioner) exposeOVSDBOverTCP() error {
	configPath := filepath.Join(p.FileSystemRoot, ovsSystemdConfigPath)
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("error while reading file %s: %w", configPath, err)
	}

	// TODO: Could do better parsing here but that _should_ be good enough given that the default is that
	// OVS_CTL_OPTS is not specified.
	if !strings.Contains(string(content), "--remote=ptcp:8500") {
		content = append(content, "\nOVS_CTL_OPTS=\"--ovsdb-server-options=--remote=ptcp:8500\""...)
		err := os.WriteFile(configPath, content, 0644)
		if err != nil {
			return fmt.Errorf("error while writing file %s: %w", configPath, err)
		}
	}
	return nil
}
