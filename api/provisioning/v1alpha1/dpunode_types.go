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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DPUNodeKind is the kind of the DPUNode object
	DPUNodeKind = "DPUNode"
)

// DPUNodeGroupVersionKind is the GroupVersionKind of the DPUNode object
var DPUNodeGroupVersionKind = GroupVersion.WithKind(DPUNodeKind)

type DPUNodeInstallInterfaceType string

// List of valid Install Interface types
const (
	DPUNodeInstallInterfaceGNOI    DPUNodeInstallInterfaceType = "gNOI"
	DPUNodeInstallIntrefaceRedfish DPUNodeInstallInterfaceType = "redfish"
)

type DPUNodeConditionType string

// List of valid condition types
const (
	// DPUNodeConditionReady means the DPU is ready.
	DPUNodeConditionReady DPUNodeConditionType = "Ready"
	// DPUNodeConditionInvalidDPUDetails means the DPU details provided are invalid.
	DPUNodeConditionInvalidDPUDetails DPUNodeConditionType = "InvalidDPUDetails"
	// DPUNodeConditionRebootInProgress means the DPUNode is in the process of rebooting.
	DPUNodeConditionRebootInProgress DPUNodeConditionType = "DPUNodeRebootInProgress"
	// DPUNodeConditionDPUUpdateInProgress means the DPU is in the process of being updated.
	DPUNodeConditionDPUUpdateInProgress DPUNodeConditionType = "DPUUpdateInProgress"
)

type DPUNodeRebootMethod string

// List of valid condition types
const (
	DPUNodeRebootMethodExternal     DPUNodeRebootMethod = "external"
	DPUNodeRebootMethodDMS          DPUNodeRebootMethod = "DMS"
	DPUNodeRebootMethodCustomScript DPUNodeRebootMethod = "custom_script"
)

func (ct DPUNodeConditionType) String() string {
	return string(ct)
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:metadata:annotations=helm.sh/resource-policy=keep

// DPUNode is the Schema for the dpunodes API
type DPUNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DPUNodeSpec   `json:"spec,omitempty"`
	Status DPUNodeStatus `json:"status,omitempty"`
}

// DPUNodeSpec defines the desired state of DPUNode
type DPUNodeSpec struct {
	// Defines the method for rebooting the host.
	// One of the following options can be chosen for this field:
	//    - "external": Reboot the host via an external means, not controlled by the
	//      DPU controller.
	//    - "custom_script": Reboot the host by executing a custom script.
	//    - "DMS": Use the DPU's DMS interface to reboot the host.
	// +kubebuilder:validation:Enum=external;custom_script;DMS
	// +kubebuilder:default=external
	// +required
	NodeRebootMethod string `json:"nodeRebootMethod"`

	// The IP address and port where the DMS is exposed. Only applicable if dpuInstallInterface is set to gNOI.
	// +optional
	NodeDMSAddress *DMSAddress `json:"nodeDMSAddress,omitempty"`

	// A map containing names of each DPUDevice attached to the node.
	// +optional
	DPUs map[string]bool `json:"dpus,omitempty"`
}

// DMSAddress represents the IP and Port configuration for DMS.
type DMSAddress struct {
	// IP address in IPv4 format.
	// +kubebuilder:validation:Format=ipv4
	IP string `json:"ip"`

	// Port number.
	// +kubebuilder:validation:Minimum=1
	Port uint16 `json:"port"`
}

// DPUNodeStatus defines the observed state of DPUNode
type DPUNodeStatus struct {
	// Conditions represent the latest available observations of an object's state.
	// +kubebuilder:validation:Type=array
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// The name of the interface which will be used to install the bfb image, can be one of gNOI,redfish
	// +kubebuilder:validation:Enum=gNOI;redfish
	// +required
	DPUInstallInterface string `json:"dpuInstallInterface,omitempty"`
	// The name of the Kubernetes Node object that this DPUNode represents.
	// This field is optional and only relevant if the x86 host is part of the DPF Kubernetes cluster.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="KubeNodeRef is immutable"
	// +optional
	KubeNodeRef *string `json:"kubeNodeRef,omitempty"`
}

// +kubebuilder:object:root=true

// DPUNodeList contains a list of DPUNode
type DPUNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPUNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPUNode{}, &DPUNodeList{})
}
