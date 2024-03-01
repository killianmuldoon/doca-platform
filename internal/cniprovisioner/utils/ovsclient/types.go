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
	"net"
)

// OVSClient is a client that can be used to do specific actions on OVS.
//
//go:generate ../../../../hack/tools/bin/mockgen -copyright_file ../../../../hack/boilerplate.go.txt -destination mock/ovsclient.go -source types.go
type OVSClient interface {
	// BridgeExists checks if a bridge exists
	BridgeExists(name string) (bool, error)
	// AddBridge adds a bridge
	AddBridge(name string) error
	// DeleteBridgeIfExists deletes a bridge if it exists
	DeleteBridgeIfExists(name string) error
	// SetBridgeDataPathType sets the datapath type of a bridge
	SetBridgeDataPathType(bridge string, bridgeType BridgeDataPathType) error
	// SetBridgeMAC sets the MAC address for the bridge interface
	SetBridgeMAC(bridge string, mac net.HardwareAddr) error
	// SetBridgeUplink sets the bridge-uplink external ID of the bridge. It overrides if already exists.
	SetBridgeUplinkPort(bridge string, port string) error
	// SetBridgeHostToServicePort sets the host-to-service external ID of the bridge. It overrides if already exists.
	SetBridgeHostToServicePort(bridge string, port string) error
	// SetBridgeController sets the controller for a bridge
	SetBridgeController(bridge string, controller string) error

	// AddPort adds a port to a bridge
	AddPort(bridge string, port string) error
	// SetPortType sets the type of a port
	SetPortType(port string, portType PortType) error
	// SetPatchPortPeer sets the peer for a patch port
	SetPatchPortPeer(port string, peer string) error

	// SetOVNEncapIP sets the ovn-encap-ip external ID in the Open_vSwitch table in OVS
	SetOVNEncapIP(ip net.IP) error
	// SetDOCAInit sets the doca-init other_config in the Open_vSwitch table in OVS. Requires OVS daemon restart.
	SetDOCAInit(enable bool) error
}

// BridgeDataPathType represents the various datapath types a bridge can be configured with
type BridgeDataPathType string

const (
	NetDev BridgeDataPathType = "netdev"
)

// PortType represents the various types a port can be configured with
type PortType string

const (
	DPDK     PortType = "dpdk"
	Internal PortType = "internal"
	Patch    PortType = "patch"
)
