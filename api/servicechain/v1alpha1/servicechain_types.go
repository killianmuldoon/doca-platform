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

// ServiceChainSpec defines the desired state of ServiceChain
type ServiceChainSpec struct {
	// Node where this ServiceChain applies to
	// +optional
	Node *string `json:"node,omitempty"`
	// The switches of the ServiceChain, order is significant
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=50
	// +required
	Switches []Switch `json:"switches"`
}

// Switch defines the switch configuration
type Switch struct {
	// Ports of the switch
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=50
	// +required
	Ports []Port `json:"ports"`
}

// Port defines the port configuration
// +kubebuilder:validation:XValidation:rule="(has(self.service) && !has(self.serviceInterface)) || (!has(self.service) && has(self.serviceInterface))", message="either service or serviceInterface must be specified"
type Port struct {
	// +optional
	Service *Service `json:"service,omitempty"`
	// +optional
	ServiceInterface *ServiceIfc `json:"serviceInterface,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="(has(self.reference) && !has(self.matchLabels)) || (!has(self.reference) && has(self.matchLabels))", message="either reference or matchLabels must be specified"
type Service struct {
	// Interface name
	// +required
	InterfaceName string `json:"interface"`
	// TODO: What is this field supposed to be?
	// +optional
	Reference *ObjectRef `json:"reference,omitempty"`
	// MatchLabels is a map of string keys and values that are used to select
	// an object.
	// +kubebuilder:validation:MinProperties=1
	// +kubebuilder:validation:MaxProperties=50
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	// IPAM defines the IPAM configuration
	// +optional
	IPAM *IPAM `json:"ipam,omitempty"`
}

// ServiceIfc defines the service interface configuration
// +kubebuilder:validation:XValidation:rule="(has(self.reference) && !has(self.matchLabels)) || (!has(self.reference) && has(self.matchLabels))", message="either reference or matchLabels must be specified"
type ServiceIfc struct {
	// TODO: What is this field supposed to be?
	// +optional
	Reference *ObjectRef `json:"reference,omitempty"`
	// +kubebuilder:validation:MinProperties=1
	// +kubebuilder:validation:MaxProperties=50
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// IPAM defines the IPAM configuration
// +kubebuilder:validation:XValidation:rule="(has(self.reference) && !has(self.matchLabels)) || (!has(self.reference) && has(self.matchLabels))", message="either reference or matchLabels must be specified"
type IPAM struct {
	// +optional
	Reference *ObjectRef `json:"reference,omitempty"`
	// +kubebuilder:validation:MinProperties=1
	// +kubebuilder:validation:MaxProperties=50
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
	// +optional
	DefaultGateway *bool `json:"defaultGateway,omitempty"`
	// +optional
	SetDefaultRoute *bool `json:"setDefaultRoute,omitempty"`
}

type ObjectRef struct {
	// Namespace of the object
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +optional
	Namespace *string `json:"namespace,omitempty"`
	// Name of the object
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +required
	Name string `json:"name"`
}

// ServiceChainStatus defines the observed state of ServiceChain
type ServiceChainStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ServiceChain is the Schema for the servicechains API
type ServiceChain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceChainSpec   `json:"spec,omitempty"`
	Status ServiceChainStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServiceChainList contains a list of ServiceChain
type ServiceChainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceChain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceChain{}, &ServiceChainList{})
}
