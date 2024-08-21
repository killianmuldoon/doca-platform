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

// ServiceInterfaceSetKind is the kind of the ServiceInterfaceSet
const ServiceInterfaceSetKind = "ServiceInterfaceSet"

var (
	// ServiceInterfaceSetGroupVersionKind is the GroupVersionKind of the ServiceInterfaceSet
	ServiceInterfaceSetGroupVersionKind = GroupVersion.WithKind(ServiceInterfaceSetKind)
)

// ServiceInterfaceSetSpec defines the desired state of ServiceInterfaceSet
type ServiceInterfaceSetSpec struct {
	// Select the Nodes with specific labels, ServiceInterface CRs will be
	// created only for these Nodes
	// +optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`
	// Template holds the template for the erviceInterfaceSpec
	// +required
	Template ServiceInterfaceSpecTemplate `json:"template"`
}

// ServiceInterfaceSpecTemplate defines the template from which ServiceInterfaceSpecs
// are created
type ServiceInterfaceSpecTemplate struct {
	// ServiceInterfaceSpec is the spec for the ServiceInterfaceSpec
	// +required
	Spec ServiceInterfaceSpec `json:"spec"`
	// ObjectMeta holds metadata like labels and annotations.
	// +optional
	ObjectMeta `json:"metadata,omitempty"`
}

// ServiceInterfaceSetStatus defines the observed state of ServiceInterfaceSet
type ServiceInterfaceSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="IfType",type=string,JSONPath=`.spec.template.spec.interfaceType`
//+kubebuilder:printcolumn:name="IfName",type=string,JSONPath=`.spec.template.spec.interfaceName`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ServiceInterfaceSet is the Schema for the serviceinterfacesets API
type ServiceInterfaceSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceInterfaceSetSpec   `json:"spec,omitempty"`
	Status ServiceInterfaceSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServiceInterfaceSetList contains a list of ServiceInterfaceSet
type ServiceInterfaceSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceInterfaceSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceInterfaceSet{}, &ServiceInterfaceSetList{})
}
