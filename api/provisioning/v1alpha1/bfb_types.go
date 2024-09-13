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

const (
	// BfbKind is the kind of the Bfb object
	BfbKind = "Bfb"
)

// BfbGroupVersionKind is the GroupVersionKind of the Bfb object
var BfbGroupVersionKind = GroupVersion.WithKind(BfbKind)

// BfbPhase describes current state of Bfb CR.
// Only one of the following state may be specified.
// Default is Initializing.
// +kubebuilder:validation:Enum=Initializing;Downloading;Ready;Deleting;Error
type BfbPhase string

// These are the valid statuses of Bfb.
const (
	BFBFinalizer = "provisioning.dpf.nvidia.com/bfb-protection"

	// Bfb CR is created
	BfbInitializing BfbPhase = "Initializing"
	// Downloading BFB file
	BfbDownloading BfbPhase = "Downloading"
	// Finished downloading BFB file, ready for DPU to use
	BfbReady BfbPhase = "Ready"
	// Delete BFB
	BfbDeleting BfbPhase = "Deleting"
	// Error happens during BFB downloading
	BfbError BfbPhase = "Error"
)

// BfbSpec defines the content of Bfb
// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="Value is immutable"
type BfbSpec struct {
	// Specifies bfb file name on the volume or
	// use CRD name in case it is omitted.
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9\_\-\.]+\.bfb$`
	// +optional
	FileName string `json:"file_name,omitempty"`

	// The url of the bfb image to download.
	// +kubebuilder:validation:Pattern=`^(http|https)://.+\.bfb$`
	// +required
	URL string `json:"url"`
}

// BfbStatus defines the observed state of Bfb
type BfbStatus struct {
	// The current state of Bfb.
	// +kubebuilder:default=Initializing
	// +required
	Phase BfbPhase `json:"phase"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Bfb is the Schema for the bfbs API
type Bfb struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec BfbSpec `json:"spec,omitempty"`

	// +kubebuilder:default={phase: Initializing}
	// +optional
	Status BfbStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BfbList contains a list of Bfb
type BfbList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bfb `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Bfb{}, &BfbList{})
}
