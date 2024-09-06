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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// DpuSetKind is the kind of the DpuSet object
	DpuSetKind = "DpuSet"
)

// DpuSetGroupVersionKind is the GroupVersionKind of the DpuSet object
var DpuSetGroupVersionKind = GroupVersion.WithKind(DpuSetKind)

// StrategyType describes strategy to use to reprovision existing DPUs.
// Default is "Recreate".
// +kubebuilder:validation:Enum=Recreate;RollingUpdate
type StrategyType string

const (
	DpuSetFinalizer = "provisioning.dpf.nvidia.com/dpuset-protection"

	// Delete all the existing DPUs before creating new ones.
	RecreateStrategyType StrategyType = "Recreate"

	// Gradually scale down the old DPUs and scale up the new one.
	RollingUpdateStrategyType StrategyType = "RollingUpdate"
)

type DpuSetStrategy struct {
	// Can be "Recreate" or "RollingUpdate".
	// +kubebuilder:default=Recreate
	// +optional
	Type StrategyType `json:"type,omitempty"`

	// Rolling update config params. Present only if StrategyType = RollingUpdate.
	// +optional
	RollingUpdate *RollingUpdateDpu `json:"rollingUpdate,omitempty"`
}

// Spec to control the desired behavior of rolling update.
type RollingUpdateDpu struct {
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

type BFBSpec struct {
	BFBName string `json:"bfb,omitempty"`
}

type DPUSpec struct {
	Bfb        BFBSpec     `json:"BFB,omitempty"`
	NodeEffect *NodeEffect `json:"nodeEffect,omitempty"`
	Cluster    K8sCluster  `json:"k8s_cluster"`
	DPUFlavor  string      `json:"dpuFlavor"`
	// Specifies if the DPU controller should automatically reboot the node on upgrades,
	// this field is intended for advanced cases that don’t use draining but want to reboot the host based with custom logic
	// +optional
	AutomaticNodeReboot bool `json:"automaticNodeReboot,omitempty"`
}
type DpuTemplate struct {
	Annotations map[string]string `json:"annotations,omitempty"`
	Spec        DPUSpec           `json:"spec,omitempty"`
}

type NodeEffect struct {
	Taint       *corev1.Taint     `json:"taint,omitempty"`
	NoEffect    bool              `json:"no_effect,omitempty"`
	CustomLabel map[string]string `json:"custom_label,omitempty"`
	Drain       *Drain            `json:"drain,omitempty"`
}

type Drain struct {
	// +optional
	AutomaticNodeReboot bool `json:"automaticNodeReboot,omitempty"`
}

// DpuSetSpec defines the desired state of DpuSet
type DpuSetSpec struct {
	// The rolling update strategy to use to updating existing Dpus with new ones.
	// +optional
	Strategy *DpuSetStrategy `json:"strategy,omitempty"`

	// Select the Nodes with specific labels
	// +optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`

	// Select the DPU with specific labels
	// +optional
	DpuSelector map[string]string `json:"dpuSelector,omitempty"`

	// Object that describes the DPU that will be created if insufficient replicas are detected
	// +optional
	DpuTemplate DpuTemplate `json:"dpuTemplate,omitempty"`
}

// DpuSetStatus defines the observed state of DpuSet
type DpuSetStatus struct {
	// +optional
	Dpustatistics map[DpuPhase]int `json:"dpuStatistics,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DpuSet is the Schema for the dpusets API
type DpuSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DpuSetSpec   `json:"spec,omitempty"`
	Status DpuSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DpuSetList contains a list of DpuSet
type DpuSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DpuSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DpuSet{}, &DpuSetList{})
}
