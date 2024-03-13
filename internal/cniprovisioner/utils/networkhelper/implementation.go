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

package networkhelper

import (
	"errors"
	"net"

	"github.com/vishvananda/netlink"
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
		return err
	}
	return netlink.LinkSetUp(l)
}

// SetLinkDown sets the administrative state of a link to "down"
func (n *networkHelper) SetLinkDown(link string) error {
	l, err := netlink.LinkByName(link)
	if err != nil {
		return err
	}
	return netlink.LinkSetDown(l)
}

// RenameLink renames a link to the given name
func (n *networkHelper) RenameLink(link string, newName string) error {
	l, err := netlink.LinkByName(link)
	if err != nil {
		return err
	}
	return netlink.LinkSetName(l, newName)
}

// SetLinkIPAddress sets the IP address of a link
func (n *networkHelper) SetLinkIPAddress(link string, ipNet *net.IPNet) error {
	if ipNet == nil {
		return errors.New("ipNet is empty, can't set link IP")
	}
	l, err := netlink.LinkByName(link)
	if err != nil {
		return err
	}
	ip, err := netlink.ParseAddr(ipNet.String())
	if err != nil {
		return err
	}

	return netlink.AddrAdd(l, ip)
}

// DeleteLinkIPAddress deletes the given IP of a link
func (n *networkHelper) DeleteLinkIPAddress(link string, ipNet *net.IPNet) error {
	if ipNet == nil {
		return errors.New("ipNet is empty, can't delete link IP")
	}
	l, err := netlink.LinkByName(link)
	if err != nil {
		return err
	}
	ip, err := netlink.ParseAddr(ipNet.String())
	if err != nil {
		return err
	}

	return netlink.AddrDel(l, ip)
}

// DeleteNeighbour deletes a neighbour
func (n *networkHelper) DeleteNeighbour(ip net.IP, device string) error {
	if ip == nil {
		return errors.New("ip is empty, can't delete neighbour")
	}
	l, err := netlink.LinkByName(device)
	if err != nil {
		return err
	}
	neigh := &netlink.Neigh{
		LinkIndex: l.Attrs().Index,
		IP:        ip,
	}
	return netlink.NeighDel(neigh)
}

// DeleteRoute deletes a route
func (n *networkHelper) DeleteRoute(network *net.IPNet, gateway net.IP, device string) error {
	if network == nil {
		return errors.New("network is empty, can't delete route")
	}
	l, err := netlink.LinkByName(device)
	if err != nil {
		return err
	}
	r := &netlink.Route{
		Dst:       network,
		Gw:        gateway,
		LinkIndex: l.Attrs().Index,
	}

	return netlink.RouteDel(r)
}
