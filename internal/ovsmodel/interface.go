/*
COPYRIGHT 2024 NVIDIA

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

// Code generated by "libovsdb.modelgen"
// DO NOT EDIT.

package ovsmodel

const InterfaceTable = "Interface"

type (
	InterfaceAdminState       = string
	InterfaceCFMRemoteOpstate = string
	InterfaceDuplex           = string
	InterfaceLinkState        = string
)

var (
	InterfaceAdminStateDown       InterfaceAdminState       = "down"
	InterfaceAdminStateUp         InterfaceAdminState       = "up"
	InterfaceCFMRemoteOpstateDown InterfaceCFMRemoteOpstate = "down"
	InterfaceCFMRemoteOpstateUp   InterfaceCFMRemoteOpstate = "up"
	InterfaceDuplexFull           InterfaceDuplex           = "full"
	InterfaceDuplexHalf           InterfaceDuplex           = "half"
	InterfaceLinkStateDown        InterfaceLinkState        = "down"
	InterfaceLinkStateUp          InterfaceLinkState        = "up"
)

// Interface defines an object in Interface table
type Interface struct {
	UUID                      string                     `ovsdb:"_uuid"`
	AdminState                *InterfaceAdminState       `ovsdb:"admin_state"`
	BFD                       map[string]string          `ovsdb:"bfd"`
	BFDStatus                 map[string]string          `ovsdb:"bfd_status"`
	CFMFault                  *bool                      `ovsdb:"cfm_fault"`
	CFMFaultStatus            []string                   `ovsdb:"cfm_fault_status"`
	CFMFlapCount              *int                       `ovsdb:"cfm_flap_count"`
	CFMHealth                 *int                       `ovsdb:"cfm_health"`
	CFMMpid                   *int                       `ovsdb:"cfm_mpid"`
	CFMRemoteMpids            []int                      `ovsdb:"cfm_remote_mpids"`
	CFMRemoteOpstate          *InterfaceCFMRemoteOpstate `ovsdb:"cfm_remote_opstate"`
	Duplex                    *InterfaceDuplex           `ovsdb:"duplex"`
	Error                     *string                    `ovsdb:"error"`
	ExternalIDs               map[string]string          `ovsdb:"external_ids"`
	Ifindex                   *int                       `ovsdb:"ifindex"`
	IngressPolicingBurst      int                        `ovsdb:"ingress_policing_burst"`
	IngressPolicingKpktsBurst int                        `ovsdb:"ingress_policing_kpkts_burst"`
	IngressPolicingKpktsRate  int                        `ovsdb:"ingress_policing_kpkts_rate"`
	IngressPolicingRate       int                        `ovsdb:"ingress_policing_rate"`
	LACPCurrent               *bool                      `ovsdb:"lacp_current"`
	LinkResets                *int                       `ovsdb:"link_resets"`
	LinkSpeed                 *int                       `ovsdb:"link_speed"`
	LinkState                 *InterfaceLinkState        `ovsdb:"link_state"`
	LLDP                      map[string]string          `ovsdb:"lldp"`
	MAC                       *string                    `ovsdb:"mac"`
	MACInUse                  *string                    `ovsdb:"mac_in_use"`
	MTU                       *int                       `ovsdb:"mtu"`
	MTURequest                *int                       `ovsdb:"mtu_request"`
	Name                      string                     `ovsdb:"name"`
	Ofport                    *int                       `ovsdb:"ofport"`
	OfportRequest             *int                       `ovsdb:"ofport_request"`
	Options                   map[string]string          `ovsdb:"options"`
	OtherConfig               map[string]string          `ovsdb:"other_config"`
	Statistics                map[string]int             `ovsdb:"statistics"`
	Status                    map[string]string          `ovsdb:"status"`
	Type                      string                     `ovsdb:"type"`
}
