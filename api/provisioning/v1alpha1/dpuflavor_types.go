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
)

const (
	// DPUFlavorKind is the kind of the DPUFlavor object
	DPUFlavorKind = "DPUFlavor"
)

// DPUFlavorGroupVersionKind is the GroupVersionKind of the DPUFlavor object
var DPUFlavorGroupVersionKind = GroupVersion.WithKind(DPUFlavorKind)

// DPUFlavorSpec defines the content of DPUFlavor
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DPUFlavor spec is immutable"
type DPUFlavorSpec struct {
	// +optional
	Grub DPUFlavorGrub `json:"grub,omitempty"`
	// +optional
	Sysctl DPUFLavorSysctl `json:"sysctl,omitempty"`
	// +optional
	NVConfig []DPUFlavorNVConfig `json:"nvconfig,omitempty"`
	// +optional
	OVS DPUFlavorOVS `json:"ovs,omitempty"`
	// +optional
	BFCfgParameters []string `json:"bfcfgParameters,omitempty"`
	// +optional
	ConfigFiles []ConfigFile `json:"configFiles,omitempty"`
	// +optional
	ContainerdConfig ContainerdConfig `json:"containerdConfig,omitempty"`
	// DPUDeploymentResources indicates the resources available for DPUServices to consume after the BFB with this
	// particular flavor and the DPF system components have been installed on a DPU. These resources do not take into
	// account potential resources consumed by other DPUServices. The DPUDeployment Controller takes into account that
	// field to understand if a DPUService can be installed on a given DPU.
	// +optional
	DPUDeploymentResources corev1.ResourceList `json:"dpuDeploymentResources,omitempty"`
	// ResourceRequirements indicates the minimum amount of resources needed for a BFB with that flavor to be installed
	// on a DPU. Using this field, the controller can understand if that flavor can be installed on a particular DPU.
	// +optional
	ResourceRequirements corev1.ResourceList `json:"resourceRequirements,omitempty"`
}

type DPUFlavorGrub struct {
	// +optional
	KernelParameters []string `json:"kernelParameters,omitempty"`
}

type DPUFLavorSysctl struct {
	// +optional
	Parameters []string `json:"parameters,omitempty"`
}

type DPUFlavorNVConfig struct {
	// +optional
	Device *string `json:"device,omitempty"`
	// +optional
	Parameters []string `json:"parameters,omitempty"`
	// +optional
	HostPowerCycleRequired *bool `json:"hostPowerCycleRequired,omitempty"`
}

type DPUFlavorOVS struct {
	// +optional
	RawConfigScript string `json:"rawConfigScript,omitempty"`
}

// +kubebuilder:validation:Enum=override;append
type DPUFlavorFileOp string

const (
	FileOverride DPUFlavorFileOp = "override"
	FileAppend   DPUFlavorFileOp = "append"
)

type ConfigFile struct {
	// +optional
	Path string `json:"path,omitempty"`
	// +optional
	Operation DPUFlavorFileOp `json:"operation,omitempty"`
	// +optional
	Raw string `json:"raw,omitempty"`
	// +optional
	Permissions string `json:"permissions,omitempty"`
}

type ContainerdConfig struct {
	// +optional
	RegistryEndpoint string `json:"registryEndpoint,omitempty"`
}

//+kubebuilder:object:root=true

// DPUFlavor is the Schema for the dpuflavors API
type DPUFlavor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec DPUFlavorSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// DPUFlavorList contains a list of DPUFlavor
type DPUFlavorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPUFlavor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPUFlavor{}, &DPUFlavorList{})
}
