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

const CTTimeoutPolicyTable = "CT_Timeout_Policy"

type (
	CTTimeoutPolicyTimeouts = string
)

var (
	CTTimeoutPolicyTimeoutsICMPFirst      CTTimeoutPolicyTimeouts = "icmp_first"
	CTTimeoutPolicyTimeoutsICMPReply      CTTimeoutPolicyTimeouts = "icmp_reply"
	CTTimeoutPolicyTimeoutsTCPClose       CTTimeoutPolicyTimeouts = "tcp_close"
	CTTimeoutPolicyTimeoutsTCPCloseWait   CTTimeoutPolicyTimeouts = "tcp_close_wait"
	CTTimeoutPolicyTimeoutsTCPEstablished CTTimeoutPolicyTimeouts = "tcp_established"
	CTTimeoutPolicyTimeoutsTCPFinWait     CTTimeoutPolicyTimeouts = "tcp_fin_wait"
	CTTimeoutPolicyTimeoutsTCPLastAck     CTTimeoutPolicyTimeouts = "tcp_last_ack"
	CTTimeoutPolicyTimeoutsTCPRetransmit  CTTimeoutPolicyTimeouts = "tcp_retransmit"
	CTTimeoutPolicyTimeoutsTCPSynRecv     CTTimeoutPolicyTimeouts = "tcp_syn_recv"
	CTTimeoutPolicyTimeoutsTCPSynSent     CTTimeoutPolicyTimeouts = "tcp_syn_sent"
	CTTimeoutPolicyTimeoutsTCPSynSent2    CTTimeoutPolicyTimeouts = "tcp_syn_sent2"
	CTTimeoutPolicyTimeoutsTCPTimeWait    CTTimeoutPolicyTimeouts = "tcp_time_wait"
	CTTimeoutPolicyTimeoutsTCPUnack       CTTimeoutPolicyTimeouts = "tcp_unack"
	CTTimeoutPolicyTimeoutsUDPFirst       CTTimeoutPolicyTimeouts = "udp_first"
	CTTimeoutPolicyTimeoutsUDPMultiple    CTTimeoutPolicyTimeouts = "udp_multiple"
	CTTimeoutPolicyTimeoutsUDPSingle      CTTimeoutPolicyTimeouts = "udp_single"
)

// CTTimeoutPolicy defines an object in CT_Timeout_Policy table
type CTTimeoutPolicy struct {
	UUID        string            `ovsdb:"_uuid"`
	ExternalIDs map[string]string `ovsdb:"external_ids"`
	Timeouts    map[string]int    `ovsdb:"timeouts"`
}
