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

// DpuPhase describes current state of Dpu.
// Only one of the following state may be specified.
// Default is Initializing.
// +kubebuilder:validation:Enum="Initializing";"Node Effect";"Pending";"DMSDeployment";"OS Installing";"DPU Cluster Config";"Ready";"Error";"Deleting";"Rebooting"
type DpuPhase string

// These are the valid statuses of DPU.
const (
	DPUFinalizer = "provisioning.dpf.nvidia.com/dpu-protection"

	// DPU CR is created by DPUSet.
	DPUInitializing DpuPhase = "Initializing"
	// In DPUNodeEffect state, the controller will handle the node effect provided by the user.
	DPUNodeEffect DpuPhase = "Node Effect"
	// In this state, the controller will check whether BFB is ready.
	DPUPending DpuPhase = "Pending"
	// In DPUDMSDeployment state, the controller will create DMS pod and proxy pod.
	DPUDMSDeployment DpuPhase = "DMSDeployment"
	// In DPUOSInstalling state, the controller will call DMS gNOI interface to do dpu provisioning.
	DPUOSInstalling DpuPhase = "OS Installing"
	// In DPUClusterConfig state, The controller will verify DPU joined successfully to Kamaji cluster.
	DPUClusterConfig DpuPhase = "DPU Cluster Config"
	// Setup host network
	DPUHostNetworkConfiguration DpuPhase = "Host Network Configuration"
	// DPUReady means the DPU is ready to use.
	DPUReady DpuPhase = "Ready"
	// DPUError means error occurred.
	DPUError DpuPhase = "Error"
	// DPUDeleting means the DPU CR will be deleted, controller will do some cleanup works.
	DPUDeleting DpuPhase = "Deleting"
	// DPURebooting means the host of DPU is rebooting.
	DPURebooting DpuPhase = "Rebooting"
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
	Name       string            `json:"name"`
	NameSpace  string            `json:"namespace"`
	NodeLabels map[string]string `json:"node_labels,omitempty"`
}

// DpuSpec defines the desired state of Dpu
type DpuSpec struct {
	// Specifies Node this DPU belongs to
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="Value is immutable"
	// +required
	NodeName string `json:"nodeName"`

	// Specifies name of the bfb CR to use for this DPU
	// +required
	BFB string `json:"bfb"`

	// The PCI device related DPU
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="Value is immutable"
	// +required
	PCIAddress string `json:"pci_address,omitempty"`

	// Specifies how changes to the DPU should affect the Node
	// +kubebuilder:default={drain: true}
	// +optional
	NodeEffect *NodeEffect `json:"nodeEffect,omitempty"`

	// Specifies details on the K8S cluster to join
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="Value is immutable"
	// +required
	Cluster K8sCluster `json:"k8s_cluster"`

	// DPUFlavor is the name of the DPUFlavor that will be used to deploy the DPU.
	// +required
	DPUFlavor string `json:"dpuFlavor"`
}

// DpuStatus defines the observed state of DPU
type DpuStatus struct {
	// high-level summary of where the DPU is in its lifecycle
	// +required
	Phase DpuPhase `json:"phase"`

	// +optional
	Conditions []metav1.Condition `json:"conditions"`

	// bfb version of this DPU
	// +optional
	BFBVersion string `json:"bfb_version,omitempty"`

	// pci device information of this DPU
	// +optional
	PCIDevice string `json:"pci_device,omitempty"`

	// whether require reset of DPU
	// +optional
	RequiredReset *bool `json:"required_reset,omitempty"`

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

// Dpu is the Schema for the dpus API
type Dpu struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DpuSpec   `json:"spec,omitempty"`
	Status DpuStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DpuList contains a list of Dpu
type DpuList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Dpu `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Dpu{}, &DpuList{})
}
