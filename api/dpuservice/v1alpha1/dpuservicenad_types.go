/*
Copyright 2025 NVIDIA

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
	"github.com/nvidia/doca-platform/internal/conditions"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Status related variables
const (
	ConditionDPUNADObjectReconciled conditions.ConditionType = "DPUNADObjectReconciled"
	ConditionDPUNADObjectReady      conditions.ConditionType = "DPUNADObjectReady"
)

// status conditions on DPUServiceNAD crd object
var (
	DPUServiceNADConditions = []conditions.ConditionType{
		conditions.TypeReady,
		ConditionDPUNADObjectReconciled,
		ConditionDPUNADObjectReady,
	}
)

var _ conditions.GetSet = &DPUServiceNAD{}

func (c *DPUServiceNAD) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

func (c *DPUServiceNAD) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// DPUServiceNADSpec defines the desired state of DPUServiceNAD.
type DPUServiceNADSpec struct {
	ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:Enum={"vf", "sf", "veth"}
	// +required
	ResourceType string `json:"resourceType"`
	// +optional
	Bridge string `json:"bridge,omitempty"`
	// +optional
	MTU int `json:"mtu,omitempty"`
	// +optional
	IPAM bool `json:"ipam,omitempty"`
}

// DPUServiceNADStatus defines the observed state of DPUServiceNAD.
type DPUServiceNADStatus struct {
	// Conditions reflect the status of the object
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:metadata:annotations=helm.sh/resource-policy=keep

// DPUServiceNAD is the Schema for the dpuservicenads API.
type DPUServiceNAD struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DPUServiceNADSpec   `json:"spec,omitempty"`
	Status DPUServiceNADStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DPUServiceNADList contains a list of DPUServiceNAD.
type DPUServiceNADList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPUServiceNAD `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPUServiceNAD{}, &DPUServiceNADList{})
}
