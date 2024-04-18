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
	Node string `json:"node,omitempty"`
	// The switches of the ServiceChain, order is significant
	Switches []Switch `json:"switches"`
}

type Switch struct {
	Ports []Port `json:"ports"`
}

type Port struct {
	Service          *Service    `json:"service,omitempty"`
	ServiceInterface *ServiceIfc `json:"serviceInterface,omitempty"`
}

type Service struct {
	// +kubebuilder:validation:Required
	InterfaceName string            `json:"interface"`
	Reference     *ObjectRef        `json:"reference,omitempty"`
	MatchLabels   map[string]string `json:"matchLabels,omitempty"`
	IPAM          *IPAM             `json:"ipam,omitempty"`
}

type ServiceIfc struct {
	Reference   *ObjectRef        `json:"reference,omitempty"`
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

type IPAM struct {
	Reference       *ObjectRef        `json:"reference,omitempty"`
	MatchLabels     map[string]string `json:"matchLabels,omitempty"`
	DefaultGateway  bool              `json:"defaultGateway,omitempty"`
	SetDefaultRoute bool              `json:"setDefaultRoute,omitempty"`
}

type ObjectRef struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
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
