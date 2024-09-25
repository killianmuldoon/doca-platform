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

type ClusterType string

const (
	StaticCluster ClusterType = "Static"
	NVidiaCluster ClusterType = "Nvidia"
)

type ConditionType string

const (
	ConditionCreated ConditionType = "Created"
	ConditionReady   ConditionType = "Ready"
)

type ClusterPhase string

const (
	PhasePending  ClusterPhase = "Pending"
	PhaseCreating ClusterPhase = "Creating"
	PhaseReady    ClusterPhase = "Ready"
	PhaseNotReady ClusterPhase = "NotReady"
	PhaseFailed   ClusterPhase = "Failed"
)

const (
	FinalizerCleanUp         = "provisioning.dpf.nvidia.com/cluster-manager-clean-up"
	FinalizerInternalCleanUp = "provisioning.dpf.nvidia.com/cluster-manager-internal-clean-up"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DPUClusterSpec defines the desired state of DPUCluster
type DPUClusterSpec struct {
	//+kubebuilder:validation:Pattern="nvidia|static|[^/]+/.*"
	Type string `json:"type"`

	//+kubebuilder:validation:XValidation:rule="self > 0 && self <= 1000",message="maxNode must be in range (0, 1000]"
	// MaxNodes is the max amount of node in the cluster
	MaxNodes int `json:"maxNodes"`

	// Version is the K8s control-plane version of the cluster
	Version string `json:"version"`

	//+kubebuilder:validation:XValidation:rule="oldSelf==\"\"||self==oldSelf",message="kubeconfig is immutable"
	//+optional
	// Kubeconfig is the secret that contains the admin kubeconfig
	Kubeconfig string `json:"kubeconfig"`

	//+optional
	// ClusterEndpoint contains configurations of the cluster entry point
	ClusterEndpoint *ClusterEndpointSpec `json:"clusterEndpoint,omitempty"`
}

// DPUClusterStatus defines the observed state of DPUCluster
type DPUClusterStatus struct {
	//+kubebuilder:validation:Enum=Pending;Creating;Ready;NotReady;Failed
	//+kubebuilder:default="Pending"
	Phase ClusterPhase `json:"phase"`

	//+optional
	Conditions []metav1.Condition `json:"conditions"`
}

type ClusterEndpointSpec struct {
	// Keepalived configures the keepalived that will be deployed for the cluster control-plane
	//+optional
	Keepalived *KeepalivedSpec `json:"keepalived,omitempty"`
}

type KeepalivedSpec struct {
	// VIP is the virtual IP owned by the keepalived instances
	VIP string `json:"vip"`

	// NodeSelector specifies the nodes that keepalived instances should be deployed
	//+optional
	NodeSelector map[string]string `json:"nodeSelector"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type",description="type of the cluster"
//+kubebuilder:printcolumn:name="MaxNodes",type="integer",JSONPath=".spec.maxNodes",description="max amount of nodes"
//+kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version",description="Kubernetes control-plane version"
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="status of the cluster"

// DPUCluster is the Schema for the dpuclusters API
type DPUCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	//+required
	Spec   DPUClusterSpec   `json:"spec,omitempty"`
	Status DPUClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DPUClusterList contains a list of DPUCluster
type DPUClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPUCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPUCluster{}, &DPUClusterList{})
}
