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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceInterfaceSpec defines the desired state of ServiceInterface
// +kubebuilder:validation:XValidation:rule="(self.interfaceType == 'vlan' && has(self.vlan)) || (self.interfaceType == 'pf' && has(self.pf)) || (self.interfaceType == 'vf' && has(self.vf))", message="vlan, pf, vf must have corresponding fields"
type ServiceInterfaceSpec struct {
	// Node where this interface exists
	// +optional
	Node *string `json:"node,omitempty"`
	// The interface type ("vlan", "physical", "pf", "vf", "ovn")
	// +kubebuilder:validation:Enum={"vlan", "physical", "pf", "vf", "ovn"}
	// +required
	InterfaceType string `json:"interfaceType"`
	// The interface name
	// +optional
	InterfaceName *string `json:"interfaceName,omitempty"`
	// The VLAN definition
	// +optional
	Vlan *VLAN `json:"vlan,omitempty"`
	// The VF definition
	// +optional
	VF *VF `json:"vf,omitempty"`
	// The PF definition
	// +optional
	PF *PF `json:"pf,omitempty"`
}

// VLAN defines the VLAN configuration
type VLAN struct {
	// The VLAN ID
	// +required
	VlanID int `json:"vlanID"`
	// The parent interface reference
	// TODO: Figure out what this field is supposed to be
	// +required
	ParentInterfaceRef string `json:"parentInterfaceRef"`
}

// VF defines the VF configuration
type VF struct {
	// The VF ID
	// +required
	VFID int `json:"vfID"`
	// The PF ID
	// +required
	PFID int `json:"pfID"`
	// The parent interface reference
	// TODO: Figure out what this field is supposed to be
	// +required
	ParentInterfaceRef string `json:"parentInterfaceRef"`
}

// PF defines the PF configuration
type PF struct {
	// The PF ID
	// +required
	ID int `json:"pfID"`
}

// ServiceInterfaceStatus defines the observed state of ServiceInterface
type ServiceInterfaceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="IfType",type=string,JSONPath=`.spec.interfaceType`
//+kubebuilder:printcolumn:name="IfName",type=string,JSONPath=`.spec.interfaceName`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ServiceInterface is the Schema for the serviceinterfaces API
type ServiceInterface struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceInterfaceSpec   `json:"spec,omitempty"`
	Status ServiceInterfaceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServiceInterfaceList contains a list of ServiceInterface
type ServiceInterfaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceInterface `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceInterface{}, &ServiceInterfaceList{})
}
