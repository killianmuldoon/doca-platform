/*
Copyright 2024.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DPFOperatorConfigSpec defines the desired state of DPFOperatorConfig
type DPFOperatorConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of DPFOperatorConfig. Edit dpfoperatorconfig_types.go to remove/update
	Foo string `json:"foo,omitempty"`
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
