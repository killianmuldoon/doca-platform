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

// BfbPhase is a label for the condition of a DPU at the current time.
// +enum
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

// BfbSpec defines the desired state of Bfb
type BfbSpec struct {
	FileName string `json:"file_name,omitempty"`
	URL      string `json:"url"`
	BFCFG    string `json:"bf_cfg,omitempty"`
}

// BfbStatus defines the observed state of Bfb
type BfbStatus struct {
	Phase BfbPhase `json:"phase,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Bfb is the Schema for the bfbs API
type Bfb struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BfbSpec   `json:"spec,omitempty"`
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
