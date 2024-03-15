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
	"net"
)

// NetworkHelper is a helper that can be used to configure networking related settings on the system.

//go:generate ../../../../hack/tools/bin/mockgen -copyright_file ../../../../hack/boilerplate.go.txt -destination mock/networkhelper.go -source types.go
type NetworkHelper interface {
	// SetLinkUp sets the administrative state of a link to "up"
	SetLinkUp(link string) error
	// SetLinkDown sets the administrative state of a link to "down"
	SetLinkDown(link string) error
	// RenameLink renames a link to the given name
	RenameLink(link string, newName string) error
	// SetLinkIPAddress sets the IP address of a link
	SetLinkIPAddress(link string, ipNet *net.IPNet) error
	// DeleteLinkIPAddress deletes the given IP of a link
	DeleteLinkIPAddress(link string, ipNet *net.IPNet) error
	// DeleteNeighbour deletes a neighbour
	DeleteNeighbour(ip net.IP, device string) error
	// DeleteRoute deletes a route
	DeleteRoute(network *net.IPNet, gateway net.IP, device string) error
	// AddDummyLink adds a dummy link
	AddDummyLink(link string) error
	// DummyLinkExists checks if a dummy link exists
	DummyLinkExists(link string) (bool, error)
}
