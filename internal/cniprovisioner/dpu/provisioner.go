/*
Copyright 2024.

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

	utilsTypes "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/types"
)

const (
	// brInt is the name of the OVN integration bridge
	brInt = "br-int"
	// brEx is the name of the OVN external bridge and must match the name of the PF on the host
	brEx = "ens2f0np0"
	// brOVN is the name of the bridge that is used to communicate with OVN. This is the bridge where the rest of the HBN
	// will connect.
	brOVN = "br-ovn"
)

type DPUCNIProvisioner struct {
	ovsClient utilsTypes.OVSClient
}

// New creates a DPUCNIProvisioner that can configure the system
func New(ovsClient utilsTypes.OVSClient) *DPUCNIProvisioner {
	return &DPUCNIProvisioner{
		ovsClient: ovsClient,
	}
}

// RunOnce runs the provisioning flow once and exits
func (p *DPUCNIProvisioner) RunOnce() error {
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

	err = p.configurePodToPodOnDifferentNodeConnectivity(brExTobrOVNPatchPort)
	if err != nil {
		return err
	}

	err = p.configureHostToServiceConnectivity()
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

	err = p.ovsClient.SetBridgeDataPathType(name, utilsTypes.NetDev)
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
	err := p.ovsClient.SetPortType(port, utilsTypes.Patch)
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
	return p.ovsClient.SetPortType(uplink, utilsTypes.DPDK)
}

// configurePodToPodOnDifferentNodeConnectivity configures a VTEP interface and the ovn-encap-ip external ID so that
// traffic going through the geneve tunnels can function as expected.
func (p *DPUCNIProvisioner) configurePodToPodOnDifferentNodeConnectivity(uplinkPort string) error {
	// TODO: Assign IP and bring interface up
	vtep0 := "vtep0"
	err := p.ovsClient.AddPort(brOVN, vtep0)
	if err != nil {
		return err
	}
	err = p.ovsClient.SetPortType(vtep0, utilsTypes.Internal)
	if err != nil {
		return err
	}
	// TODO: IP of vtep0 must match this one
	err = p.ovsClient.SetOVNEncapIP(net.ParseIP("192.168.1.1"))
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
	err = p.ovsClient.SetPortType(pfRep, utilsTypes.DPDK)
	if err != nil {
		return err
	}
	err = p.ovsClient.SetBridgeHostToServicePort(brEx, pfRep)
	if err != nil {
		return err
	}
	// TODO: This must match the MAC address of the PF representor on the host
	mac, err := net.ParseMAC("00:00:00:00:00:01")
	if err != nil {
		return err
	}
	return p.ovsClient.SetBridgeMAC(brEx, mac)
}

// configureOVNManagementVF configures the VF that is going to be used by OVN Kubernetes for the management (ovn-k8s-mp0)
//
//nolint:unused
func (p *DPUCNIProvisioner) configureOVNManagementVF() error { panic("unimplemented") }

// configureVFs renames the existing VFs to map the fake environment we have on the host
//
//nolint:unused
func (p *DPUCNIProvisioner) configureVFs() error { panic("unimplemented") }

// cleanUpBridges removes all the relevant bridges. Errors are not checked, deletion is best effort.
//
//nolint:unused
func (p *DPUCNIProvisioner) cleanUpBridges() {
	panic("unimplemented")
}

// exposeOVSDBOverTCP reconfigures OVS to expose ovs-db via TCP
//
//nolint:unused
func (p *DPUCNIProvisioner) exposeOVSDBOverTCP() error {
	panic("unimplemented")
}
