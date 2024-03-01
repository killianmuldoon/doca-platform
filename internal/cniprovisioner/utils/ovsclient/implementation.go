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

package ovsclient

import (
	"fmt"
	"net"
	"os/exec"
)

const ovsVsctl = "ovs-vsctl"

type ovsClient struct {
	ovsVsctlPath string
}

// New creates an OVSClient and returns an error if the OVS util binaries can't be found.
func newOvsClient() (OVSClient, error) {
	var err error
	c := &ovsClient{}
	c.ovsVsctlPath, err = exec.LookPath(ovsVsctl)
	if err != nil {
		return nil, err
	}
	return c, err
}

func (c *ovsClient) runOVSVsctl(args ...string) error {
	cmd := exec.Command(c.ovsVsctlPath, args...)
	return cmd.Run()
}

// BridgeExists checks if a bridge exists
func (c *ovsClient) BridgeExists(name string) (bool, error) {
	err := c.runOVSVsctl("br-exists", name)
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// https://github.com/openvswitch/ovs/blob/166ee41d282c506d100bc2185d60af277121b55b/utilities/ovs-vsctl.8.in#L203-L206
			if exiterr.ExitCode() == 2 {
				return false, nil
			}
		}
		return false, err
	}

	return true, nil
}

// AddBridge adds a bridge
func (c *ovsClient) AddBridge(name string) error {
	return c.runOVSVsctl("add-br", name)
}

// DeleteBridge deletes a bridge
func (c *ovsClient) DeleteBridge(name string) error {
	return c.runOVSVsctl("del-br", name)
}

// SetBridgeDataPathType sets the datapath type of a bridge
func (c *ovsClient) SetBridgeDataPathType(bridge string, bridgeType BridgeDataPathType) error {
	return c.runOVSVsctl("set", "bridge", bridge, fmt.Sprintf("type=%s", bridgeType))
}

// SetBridgeMAC sets the MAC address for the bridge interface
func (c *ovsClient) SetBridgeMAC(bridge string, mac net.HardwareAddr) error {
	return c.runOVSVsctl("set", "bridge", bridge, fmt.Sprintf("other-config:hwaddr=%s", mac.String()))
}

// SetBridgeUplink sets the bridge-uplink external ID of the bridge. It overrides if already exists.
func (c *ovsClient) SetBridgeUplinkPort(bridge string, port string) error {
	return c.runOVSVsctl("br-set-external-id", bridge, "bridge-uplink", port)
}

// SetBridgeHostToServicePort sets the host-to-service external ID of the bridge. It overrides if already exists.
func (c *ovsClient) SetBridgeHostToServicePort(bridge string, port string) error {
	return c.runOVSVsctl("br-set-external-id", bridge, "host-to-service-interface", port)
}

// SetBridgeController sets the controller for a bridge
func (c *ovsClient) SetBridgeController(bridge string, controller string) error {
	return c.runOVSVsctl("set-controller", bridge, controller)
}

// AddPort adds a port to a bridge
func (c *ovsClient) AddPort(bridge string, port string) error {
	return c.runOVSVsctl("add-port", bridge, port)
}

// SetPortType adds a port to a bridge
func (c *ovsClient) SetPortType(port string, portType PortType) error {
	return c.runOVSVsctl("set", "interface", port, fmt.Sprintf("type=%s", portType))
}

// SetPatchPortPeer sets the peer for a patch port
func (c *ovsClient) SetPatchPortPeer(port string, peer string) error {
	return c.runOVSVsctl("set", "interface", port, fmt.Sprintf("options:peer=%s", peer))
}

// SetOVNEncapIP sets the ovn-encap-ip external ID in the Open_vSwitch table in OVS
func (c *ovsClient) SetOVNEncapIP(ip net.IP) error {
	return c.runOVSVsctl("set", "Open_vSwitch", ".", fmt.Sprintf("external_ids:ovn-encap-ip=%s", ip.String()))
}

// SetDOCAInit sets the doca-init other_config in the Open_vSwitch table in OVS
func (c *ovsClient) SetDOCAInit(enable bool) error {
	return c.runOVSVsctl("set", "Open_vSwitch", ".", fmt.Sprintf("other_config:doca-init=%t", enable))
}
