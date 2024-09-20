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

package ovsclient

import (
	"bytes"
	"errors"
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
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error running ovs-vsctl command with args %v failed: err=%w stderr=%s", args, err, stderr.String())
	}
	return nil
}

// BridgeExists checks if a bridge exists
func (c *ovsClient) BridgeExists(name string) (bool, error) {
	err := c.runOVSVsctl("br-exists", name)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// https://github.com/openvswitch/ovs/blob/166ee41d282c506d100bc2185d60af277121b55b/utilities/ovs-vsctl.8.in#L203-L206
			if exitErr.ExitCode() == 2 {
				return false, nil
			}
		}
		return false, err
	}

	return true, nil
}

// AddBridgeIfNotExists adds a bridge if it doesn't exist
func (c *ovsClient) AddBridgeIfNotExists(name string) error {
	return c.runOVSVsctl("--may-exist", "add-br", name)
}

// DeleteBridgeIfExists deletes a bridge if it exists
func (c *ovsClient) DeleteBridgeIfExists(name string) error {
	return c.runOVSVsctl("--if-exists", "del-br", name)
}

// SetBridgeDataPathType sets the datapath type of a bridge
func (c *ovsClient) SetBridgeDataPathType(bridge string, bridgeType BridgeDataPathType) error {
	return c.runOVSVsctl("set", "bridge", bridge, fmt.Sprintf("datapath_type=%s", bridgeType))
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

// AddPortIfNotExists adds a port to a bridge if it doesn't exist
func (c *ovsClient) AddPortIfNotExists(bridge string, port string) error {
	return c.runOVSVsctl("--may-exist", "add-port", bridge, port)
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

// SetKubernetesHostNodeName sets the host-k8s-nodename external ID in the Open_vSwitch table in OVS
func (c *ovsClient) SetKubernetesHostNodeName(name string) error {
	return c.runOVSVsctl("set", "Open_vSwitch", ".", fmt.Sprintf("external_ids:host-k8s-nodename=%s", name))
}
