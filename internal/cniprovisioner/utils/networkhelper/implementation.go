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

package networkhelper

import (
	"errors"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
)

type networkHelper struct{}

// New creates a NetworkHelper
func newNetworkHelper() NetworkHelper {
	return &networkHelper{}
}

// SetLinkUp sets the administrative state of a link to "up"
func (n *networkHelper) SetLinkUp(link string) error {
	l, err := netlink.LinkByName(link)
	if err != nil {
		return fmt.Errorf("netlink.LinkByName() failed: %w", err)
	}
	err = netlink.LinkSetUp(l)
	if err != nil {
		return fmt.Errorf("netlink.LinkSetUp() failed: %w", err)
	}
	return nil
}

// SetLinkDown sets the administrative state of a link to "down"
func (n *networkHelper) SetLinkDown(link string) error {
	l, err := netlink.LinkByName(link)
	if err != nil {
		return fmt.Errorf("netlink.LinkByName() failed: %w", err)
	}
	err = netlink.LinkSetDown(l)
	if err != nil {
		return fmt.Errorf("netlink.LinkSetDown() failed: %w", err)
	}
	return nil
}

// RenameLink renames a link to the given name
func (n *networkHelper) RenameLink(link string, newName string) error {
	l, err := netlink.LinkByName(link)
	if err != nil {
		return fmt.Errorf("netlink.LinkByName() failed: %w", err)
	}
	err = netlink.LinkSetName(l, newName)
	if err != nil {
		return fmt.Errorf("netlink.LinkSetName() failed: %w", err)
	}
	return nil
}

// SetLinkIPAddress sets the IP address of a link
func (n *networkHelper) SetLinkIPAddress(link string, ipNet *net.IPNet) error {
	if ipNet == nil {
		return errors.New("ipNet is empty, can't set link IP")
	}
	l, err := netlink.LinkByName(link)
	if err != nil {
		return fmt.Errorf("netlink.LinkByName() failed: %w", err)
	}
	ip, err := netlink.ParseAddr(ipNet.String())
	if err != nil {
		return fmt.Errorf("netlink.ParseAddr() failed: %w", err)
	}
	err = netlink.AddrAdd(l, ip)
	if err != nil {
		return fmt.Errorf("netlink.AddrAdd() failed: %w", err)
	}
	return nil
}

// DeleteLinkIPAddress deletes the given IP of a link
func (n *networkHelper) DeleteLinkIPAddress(link string, ipNet *net.IPNet) error {
	if ipNet == nil {
		return errors.New("ipNet is empty, can't delete link IP")
	}
	l, err := netlink.LinkByName(link)
	if err != nil {
		return fmt.Errorf("netlink.LinkByName() failed: %w", err)
	}
	ip, err := netlink.ParseAddr(ipNet.String())
	if err != nil {
		return fmt.Errorf("netlink.ParseAddr() failed: %w", err)
	}
	err = netlink.AddrDel(l, ip)
	if err != nil {
		return fmt.Errorf("netlink.AddrDel() failed: %w", err)
	}
	return nil
}

// DeleteNeighbour deletes a neighbour
func (n *networkHelper) DeleteNeighbour(ip net.IP, device string) error {
	if ip == nil {
		return errors.New("ip is empty, can't delete neighbour")
	}
	l, err := netlink.LinkByName(device)
	if err != nil {
		return fmt.Errorf("netlink.LinkByName() failed: %w", err)
	}
	neigh := &netlink.Neigh{
		LinkIndex: l.Attrs().Index,
		IP:        ip,
	}
	err = netlink.NeighDel(neigh)
	if err != nil {
		return fmt.Errorf("netlink.NeighDel() failed: %w", err)
	}
	return nil
}

// DeleteRoute deletes a route
func (n *networkHelper) DeleteRoute(network *net.IPNet, gateway net.IP, device string) error {
	if network == nil {
		return errors.New("network is empty, can't delete route")
	}
	l, err := netlink.LinkByName(device)
	if err != nil {
		return fmt.Errorf("netlink.LinkByName() failed: %w", err)
	}
	r := &netlink.Route{
		Dst:       network,
		Gw:        gateway,
		LinkIndex: l.Attrs().Index,
	}
	err = netlink.RouteDel(r)
	if err != nil {
		return fmt.Errorf("netlink.RouteDel() failed: %w", err)
	}
	return nil
}

// AddRoute adds a route
func (n *networkHelper) AddRoute(network *net.IPNet, gateway net.IP, device string) error {
	if network == nil {
		return errors.New("network is empty, can't add route")
	}
	l, err := netlink.LinkByName(device)
	if err != nil {
		return fmt.Errorf("netlink.LinkByName() failed: %w", err)
	}
	r := &netlink.Route{
		Dst:       network,
		Gw:        gateway,
		LinkIndex: l.Attrs().Index,
	}
	err = netlink.RouteAdd(r)
	if err != nil {
		return fmt.Errorf("netlink.RouteAdd() failed: %w", err)
	}
	return nil
}

// AddDummyLink adds a dummy link
func (n *networkHelper) AddDummyLink(link string) error {
	l := netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{
			Name: link,
		},
	}
	err := netlink.LinkAdd(&l)
	if err != nil {
		return fmt.Errorf("netlink.LinkAdd() failed: %w", err)
	}
	return nil
}

// DummyLinkExists checks if a dummy link exists
func (n *networkHelper) DummyLinkExists(link string) (bool, error) {
	l, err := netlink.LinkByName(link)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return false, nil
		}
		return false, fmt.Errorf("netlink.LinkByName() failed: %w", err)
	}

	d := netlink.Dummy{}
	if l.Type() != d.Type() {
		return false, fmt.Errorf("link %s exists but is not of type dummy: type=%s", link, l.Type())
	}

	return true, nil
}

// GetPFRepMacAddress returns the MAC address of the PF Representor provided as input. When you run this function in
// the DPU, it will give you the MAC address of the PF Representor on the host.
func (n *networkHelper) GetPFRepMACAddress(device string) (net.HardwareAddr, error) {
	ports, err := netlink.DevLinkGetAllPortList()
	if err != nil {
		return nil, fmt.Errorf("netlink.DevLinkGetAllPortList() failed: %w", err)
	}

	for _, p := range ports {
		if p.PortFlavour == nl.DEVLINK_PORT_FLAVOUR_PCI_PF && p.Fn != nil {
			return p.Fn.HwAddr, nil
		}
	}
	return nil, fmt.Errorf("MAC Address not found for PF Representor %s", device)
}
