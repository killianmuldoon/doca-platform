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
	// DPUKind is the kind of the DPU object
	DPUKind = "DPU"
)

// DPUGroupVersionKind is the GroupVersionKind of the DPU object
var DPUGroupVersionKind = GroupVersion.WithKind(DPUKind)

// DPUPhase describes current state of DPU.
// Only one of the following state may be specified.
// Default is Initializing.
// +kubebuilder:validation:Enum="Initializing";"Node Effect";"Pending";"DMSDeployment";"OS Installing";"DPU Cluster Config";"Host Network Configuration";"Ready";"Error";"Deleting";"Rebooting"
type DPUPhase string

// These are the valid statuses of DPU.
const (
	DPUFinalizer = "provisioning.dpu.nvidia.com/dpu-protection"

	// DPU CR is created by DPUSet.
	DPUInitializing DPUPhase = "Initializing"
	// In DPUNodeEffect state, the controller will handle the node effect provided by the user.
	DPUNodeEffect DPUPhase = "Node Effect"
	// In this state, the controller will check whether BFB is ready.
	DPUPending DPUPhase = "Pending"
	// In DPUDMSDeployment state, the controller will create DMS pod and proxy pod.
	DPUDMSDeployment DPUPhase = "DMSDeployment"
	// In DPUOSInstalling state, the controller will call DMS gNOI interface to do dpu provisioning.
	DPUOSInstalling DPUPhase = "OS Installing"
	// In DPUClusterConfig state, The controller will verify DPU joined successfully to Kamaji cluster.
	DPUClusterConfig DPUPhase = "DPU Cluster Config"
	// Setup host network
	DPUHostNetworkConfiguration DPUPhase = "Host Network Configuration"
	// DPUReady means the DPU is ready to use.
	DPUReady DPUPhase = "Ready"
	// DPUError means error occurred.
	DPUError DPUPhase = "Error"
	// DPUDeleting means the DPU CR will be deleted, controller will do some cleanup works.
	DPUDeleting DPUPhase = "Deleting"
	// DPURebooting means the host of DPU is rebooting.
	DPURebooting DPUPhase = "Rebooting"
)

type DPUConditionType string

const (
	DPUCondInitialized      DPUConditionType = "Initialized"
	DPUCondBFBReady         DPUConditionType = "BFBReady"
	DPUCondNodeEffectReady  DPUConditionType = "NodeEffectReady"
	DPUCondDMSRunning       DPUConditionType = "DMSRunning"
	DPUCondOSInstalled      DPUConditionType = "OSInstalled"
	DPUCondRebooted         DPUConditionType = "Rebooted"
	DPUCondHostNetworkReady DPUConditionType = "HostNetworkReady"
	DPUCondReady            DPUConditionType = "Ready"
)

func (ct DPUConditionType) String() string {
	return string(ct)
}

type K8sCluster struct {
	// +optional
	Name string `json:"name"`
	// +optional
	NameSpace string `json:"namespace"`
	// +optional
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`
}

// DPUSpec defines the desired state of DPU
type DPUSpec struct {
	// Specifies Node this DPU belongs to
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="Value is immutable"
	// +required
	NodeName string `json:"nodeName"`

	// Specifies name of the bfb CR to use for this DPU
	// +required
	BFB string `json:"bfb"`

	// The PCI device related DPU
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="Value is immutable"
	// +optional
	PCIAddress string `json:"pciAddress,omitempty"`

	// Specifies how changes to the DPU should affect the Node
	// +kubebuilder:default={drain: {automaticNodeReboot: true}}
	// +optional
	NodeEffect *NodeEffect `json:"nodeEffect,omitempty"`

	// Specifies details on the K8S cluster to join
	// +required
	Cluster K8sCluster `json:"cluster"`

	// DPUFlavor is the name of the DPUFlavor that will be used to deploy the DPU.
	// +optional
	DPUFlavor string `json:"dpuFlavor,omitempty"`

	// Specifies if the DPU controller should automatically reboot the node on upgrades,
	// this field is intended for advanced cases that don’t use draining but want to reboot the host based with custom logic
	// +optional
	AutomaticNodeReboot bool `json:"automaticNodeReboot,omitempty"`
}

// DPUStatus defines the observed state of DPU
type DPUStatus struct {
	// The current state of DPU.
	// +kubebuilder:default=Initializing
	// +required
	Phase DPUPhase `json:"phase"`

	// +optional
	Conditions []metav1.Condition `json:"conditions"`

	// bfb version of this DPU
	// +optional
	BFBVersion string `json:"bfbVersion,omitempty"`

	// pci device information of this DPU
	// +optional
	PCIDevice string `json:"pciDevice,omitempty"`

	// whether require reset of DPU
	// +optional
	RequiredReset *bool `json:"requiredReset,omitempty"`

	// the firmware information of DPU
	// +optional
	Firmware Firmware `json:"firmware,omitempty"`
}

type Firmware struct {
	BMC  string `json:"bmc,omitempty"`
	NIC  string `json:"nic,omitempty"`
	UEFI string `json:"uefi,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DPU is the Schema for the dpus API
type DPU struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec DPUSpec `json:"spec,omitempty"`

	// +kubebuilder:default={phase: Initializing}
	// +optional
	Status DPUStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DPUList contains a list of DPU
type DPUList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPU `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPU{}, &DPUList{})
}
