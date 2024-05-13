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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DPUServiceIPAMSpec defines the desired state of DPUServiceIPAM
type DPUServiceIPAMSpec struct {
	// IPV4CIDRs is the configuration related to splitting a CIDR into subnets per node, each with their own gateway.
	// TODO: Revisit and validate that slice is needed instead of single object.
	// TODO: Add validation that only one of ipv4CIDRs and ipv4Subnets are configured.
	IPV4CIDRs []IPV4CIDR `json:"ipv4CIDRs,omitempty"`
	// IPV4Subnets is the configuration related to splitting a subnet into blocks per node. In this setup, there is a
	// single gateway.
	// TODO: Revisit and validate that slice is needed instead of single object.
	// TODO: Add validation that only one of ipv4CIDRs and ipv4Subnets are configured.
	IPV4Subnets []IPV4Subnet `json:"ipv4Subnets,omitempty"`

	// ClusterSelector determines in which clusters the DPUServiceIPAM controller should apply the configuration.
	ClusterSelector *metav1.LabelSelector `json:"clusterSelector,omitempty"`
	// NodeSelector determines in which DPU nodes the DPUServiceIPAM controller should apply the configuration.
	NodeSelector *corev1.NodeSelector `json:"nodeSelector,omitempty"`
}

// IPV4CIDR describes the configuration relevant to splitting a CIDR into subnet per node (i.e. different gateway and
// broadcast IP per node).
type IPV4CIDR struct {
	// Name is the name of the subnet. This name is used to reference the subnet in ServiceChain.
	Name string `json:"name"`
	// CIDR is the CIDR from which subnets should be created per node.
	// TODO: Validate that input is a valid subnet
	CIDR string `json:"cidr"`
	// GatewayIndex determines which IP in the subnet extracted from the CIDR should be the gateway IP.
	GatewayIndex int `json:"gatewayIndex"`
	// PrefixSize is the size of the subnet that should be allocated per node.
	// TODO: Validate that value fits the CIDR
	PrefixSize int `json:"blockSize"`
	// Exclusions is a list of subnets that should be excluded when splitting the CIDR into subnets per node.
	// TODO: Validate values are part of the CIDR
	Exclusions []string `json:"exclusions,omitempty"`
	// Allocations describes the subnets that should be assigned in each DPU node.
	// TODO: Validate value is part of the CIDR defined above
	Allocations map[string]string `json:"allocations,omitempty"`
}

// IPV4Subnet describes the configuration relevant to splitting a subnet to a subnet block per node (i.e. same gateway
// and broadcast IP across all nodes).
type IPV4Subnet struct {
	// Name is the name of the subnet. This name is used to reference the subnet in ServiceChain.
	Name string `json:"name"`
	// Subnet is the subnet that should be allocated.
	// TODO: Validate that input is a valid subnet
	Subnet string `json:"subnet"`
	// Gateway is the IP in the subnet that should be the gateway of the subnet.
	// TODO: Validate that IP is part of subnet
	Gateway string `json:"gateway"`
	// BlockSize is the size of the block that should be allocated per node. The value should be power of 2.
	// TODO: Validate that  value is power of 2 to enable the CIDR notation in the allocations
	BlockSize int `json:"blockSize"`
	// Allocations describe the blocks that should be assigned in each DPU node.
	// TODO: Validate value is part of the subnet defined above
	Allocations map[string]string `json:"allocations,omitempty"`
}

// DPUServiceIPAMStatus defines the observed state of DPUServiceIPAM
type DPUServiceIPAMStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DPUServiceIPAM is the Schema for the dpuserviceipams API
type DPUServiceIPAM struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DPUServiceIPAMSpec   `json:"spec,omitempty"`
	Status DPUServiceIPAMStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DPUServiceIPAMList contains a list of DPUServiceIPAM
type DPUServiceIPAMList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPUServiceIPAM `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPUServiceIPAM{}, &DPUServiceIPAMList{})
}
