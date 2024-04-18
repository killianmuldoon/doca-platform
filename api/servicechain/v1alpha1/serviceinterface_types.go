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
type ServiceInterfaceSpec struct {

	// Node where this interface exists
	Node string `json:"node,omitempty"`
	// +kubebuilder:validation:Enum={"vlan", "physical", "pf", "vf", "ovn"}
	// The interface type ("vlan", "physical", "pf", "vf", "ovn")
	InterfaceType string `json:"interfaceType"`
	// The interface name
	InterfaceName string `json:"interfaceName,omitempty"`
	// The VLAN definition
	Vlan *VLAN `json:"vlan,omitempty"`
	// The VF definition
	VF *VF `json:"vf,omitempty"`
	// The PF definition
	PF *PF `json:"pf,omitempty"`
}

type VLAN struct {
	VlanID             int    `json:"vlanID"`
	ParentInterfaceRef string `json:"parentInterfaceRef"`
}

type VF struct {
	VFID               int    `json:"vfID"`
	PFID               int    `json:"pfID"`
	ParentInterfaceRef string `json:"parentInterfaceRef"`
}

type PF struct {
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
