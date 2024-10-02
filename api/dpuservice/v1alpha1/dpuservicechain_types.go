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

//nolint:dupl
package v1alpha1

import (
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DPUServiceChainFinalizer = "svc.dpf.nvidia.com/dpuservicechain"
	DPUServiceChainKind      = "DPUServiceChain"
	DPUServiceChainListKind  = "DPUServiceChainList"
)

var DPUServiceChainGroupVersionKind = GroupVersion.WithKind(DPUServiceChainKind)

// Status related variables
const (
	ConditionServiceChainSetReconciled conditions.ConditionType = "ServiceChainSetReconciled"
	ConditionServiceChainSetReady      conditions.ConditionType = "ServiceChainSetReady"
)

var (
	DPUServiceChainConditions = []conditions.ConditionType{
		conditions.TypeReady,
		ConditionServiceChainSetReconciled,
		ConditionServiceChainSetReady,
	}
)

var _ conditions.GetSet = &DPUServiceChain{}

func (c *DPUServiceChain) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

func (c *DPUServiceChain) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// DPUServiceChainSpec defines the desired state of DPUServiceChainSpec
type DPUServiceChainSpec struct {
	// Select the Clusters with specific labels, ServiceChainSet CRs will be created only for these Clusters
	ClusterSelector *metav1.LabelSelector       `json:"clusterSelector,omitempty"`
	Template        ServiceChainSetSpecTemplate `json:"template"`
}

type ServiceChainSetSpecTemplate struct {
	Spec       ServiceChainSetSpec `json:"spec"`
	ObjectMeta `json:"metadata,omitempty"`
}

// DPUServiceChainStatus defines the observed state of DPUServiceChain
type DPUServiceChainStatus struct {
	// Conditions reflect the status of the object
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ObservedGeneration records the Generation observed on the object the last time it was patched.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DPUServiceChain is the Schema for the DPUServiceChain API
type DPUServiceChain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DPUServiceChainSpec   `json:"spec,omitempty"`
	Status DPUServiceChainStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DPUServiceChainList contains a list of DPUServiceChain
type DPUServiceChainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPUServiceChain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPUServiceChain{}, &DPUServiceChainList{})
}
