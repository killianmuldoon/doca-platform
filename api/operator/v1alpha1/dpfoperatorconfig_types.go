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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	DPFOperatorConfigFinalizer         = "dpf.nvidia.com/dpfoperatorconfig"
	DPFOperatorConfigNameLabelKey      = "dpf.nvidia.com/dpfoperatorconfig-name"
	DPFOperatorConfigNamespaceLabelKey = "dpf.nvidia.com/dpfoperatorconfig-namespace"
)

// DPFOperatorConfigSpec defines the desired state of DPFOperatorConfig
type DPFOperatorConfigSpec struct {
	HostNetworkConfiguration HostNetworkConfiguration `json:"hostNetworkConfiguration"`
}

// HostNetworkConfiguration holds network related configuration required to create a functional host network.
type HostNetworkConfiguration struct {
	// CIDR is the CIDR that is used by the administrator to calculate the IPs defined in HostIPs and DPUIPs
	// TODO: Add validator in validating webhook to ensure all the IPs below are part of this CIDR
	CIDR string `json:"cidr"`
	// HostIPs represents the IPs that will be assigned to the PF Representor on each Host. Key is the Node name on the
	// Host cluster.
	// TODO: Add validator in validating webhook to ensure string is actually net.IPNet
	HostIPs map[string]string `json:"hostIPs"`
	// DPUConfiguration holds configuration fields that are needed to properly setup the DPU to enable a functional
	// host network. Key is the Node name on the DPU cluster.
	DPUConfiguration map[string]DPUConfiguration `json:"dpu"`
}

// DPUConfiguration is a struct that holds the DPU network related configuration that is required for the host network
// to work.
type DPUConfiguration struct {
	// IP represents the IP that will be assigned to the VTEP interface of the DPU.
	// TODO: Add validator in validating webhook to ensure string is actually net.IPNet
	IP string `json:"ip"`
	// Gateway is the gateway that should be added on the routes related to OVN Kubernetes traffic.
	// TODO: Add validator in validating webhook to ensure string is actually net.IP
	Gateway string `json:"gateway"`
}

// DPFOperatorConfigStatus defines the observed state of DPFOperatorConfig
type DPFOperatorConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DPFOperatorConfig is the Schema for the dpfoperatorconfigs API
type DPFOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DPFOperatorConfigSpec   `json:"spec,omitempty"`
	Status DPFOperatorConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DPFOperatorConfigList contains a list of DPFOperatorConfig
type DPFOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPFOperatorConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPFOperatorConfig{}, &DPFOperatorConfigList{})
}
