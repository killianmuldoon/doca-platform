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
	// DPFClusterSecretLabels are the labels that identify the admin kubeconfig of a DPF cluster.
	DPFClusterSecretLabels = map[string]string{"kamaji.clastix.io/component": "admin-kubeconfig", "kamaji.clastix.io/project": "kamaji"}

	// DPFClusterSecretClusterNameLabelKey is the key of the label linking a DPFClusterSecret to the name of the cluster.
	DPFClusterSecretClusterNameLabelKey = "kamaji.clastix.io/name"
)

// DPFClusterSpec defines the desired state of DPFCluster
type DPFClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of DPFCluster. Edit dpfcluster_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// DPFClusterStatus defines the observed state of DPFCluster
type DPFClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DPFCluster is the Schema for the dpfclusters API
type DPFCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DPFClusterSpec   `json:"spec,omitempty"`
	Status DPFClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DPFClusterList contains a list of DPFCluster
type DPFClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPFCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPFCluster{}, &DPFClusterList{})
}
