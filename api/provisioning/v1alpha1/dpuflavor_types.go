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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
}

type DPUFlavorGrub struct {
	KernelParameters []string `json:"kernelParameters"`
}

type DPUFLavorSysctl struct {
	Parameters []string `json:"parameters"`
}

type DPUFlavorNVConfig struct {
	// +optional
	Device     *string  `json:"device"`
	Parameters []string `json:"parameters"`
	// +optional
	HostPowerCycleRequired *bool `json:"hostPowerCycleRequired,omitempty"`
}

type DPUFlavorOVS struct {
	RawConfigScript string `json:"rawConfigScript"`
}

// +kubebuilder:validation:Enum=override;append
type DPUFlavorFileOp string

const (
	FileOverride DPUFlavorFileOp = "override"
	FileAppend   DPUFlavorFileOp = "append"
)

type ConfigFile struct {
	Path string `json:"path"`
	// +optional
	Operation   DPUFlavorFileOp `json:"operation,omitempty"`
	Raw         string          `json:"raw"`
	Permissions string          `json:"permissions"`
}

type ContainerdConfig struct {
	RegistryEndpoint string `json:"registryEndpoint"`
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
