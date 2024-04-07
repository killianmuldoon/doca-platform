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

// ServiceChainSetSpec defines the desired state of ServiceChainSet
type ServiceChainSetSpec struct {
	// Select the Nodes with specific labels, ServiceChain CRs will be created only for these Nodes
	NodeSelector *metav1.LabelSelector    `json:"nodeSelector,omitempty"`
	Template     ServiceChainSpecTemplate `json:"template"`
}

type ServiceChainSpecTemplate struct {
	Spec              ServiceChainSpec `json:"spec"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// ServiceChainSetStatus defines the observed state of ServiceChainSet
type ServiceChainSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ServiceChainSet is the Schema for the servicechainsets API
type ServiceChainSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceChainSetSpec   `json:"spec,omitempty"`
	Status ServiceChainSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServiceChainSetList contains a list of ServiceChainSet
type ServiceChainSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceChainSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ServiceChainSet{}, &ServiceChainSetList{})
}
