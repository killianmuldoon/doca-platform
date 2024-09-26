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
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ImagePullSecretsReconciledCondition conditions.ConditionType = "ImagePullSecretsReconciled"
	SystemComponentsReconciledCondition conditions.ConditionType = "SystemComponentsReconciled"
	SystemComponentsReadyCondition      conditions.ConditionType = "SystemComponentsReady"
)

var (
	Conditions = []conditions.ConditionType{
		conditions.TypeReady,
		ImagePullSecretsReconciledCondition,
		SystemComponentsReconciledCondition,
		SystemComponentsReadyCondition,
	}
)

var (
	DPFOperatorConfigFinalizer = "dpf.nvidia.com/dpfoperatorconfig"
	// DPFComponentLabelKey is added on all objects created by the DPF Operator.
	DPFComponentLabelKey = "dpf.nvidia.com/component"
)

// Overrides exposes a set of fields which impact the recommended behaviour of the DPF Operator
type Overrides struct {
	// Paused disables all reconciliation of the DPFOperatorConfig when set to true.
	// +optional
	Paused *bool `json:"paused,omitempty"`
}

// DPFOperatorConfigSpec defines the desired state of DPFOperatorConfig
type DPFOperatorConfigSpec struct {
	Overrides *Overrides `json:"overrides,omitempty"`
	// DPUServiceController is the configuration for the DPUServiceController
	// +optional
	DPUServiceController *DPUServiceControllerConfiguration `json:"dpuServiceController,omitempty"`
	// ProvisioningController is the configuration for the ProvisioningController
	ProvisioningController ProvisioningControllerConfiguration `json:"provisioningController"`
	// ServiceSetController is the configuration for the ServiceSetController
	// +optional
	ServiceSetController *ServiceSetControllerConfiguration `json:"serviceSetController,omitempty"`
	// Multus is the configuration for Multus
	// +optional
	Multus *MultusConfiguration `json:"multus,omitempty"`
	// SRIOVDevicePlugin is the configuration for the SRIOVDevicePlugin
	// +optional
	SRIOVDevicePlugin *SRIOVDevicePluginConfiguration `json:"sriovDevicePlugin,omitempty"`
	// Flannel is the configuration for Flannel
	// +optional
	Flannel *FlannelConfiguration `json:"flannel,omitempty"`
	// OVSCNI is the configuration for OVSCNI
	// +optional
	OVSCNI *OVSCNIConfiguration `json:"ovsCNI,omitempty"`
	// NVIPAM is the configuration for NVIPAM
	// +optional
	NVIPAM *NVIPAMConfiguration `json:"nvipam,omitempty"`
	// SFCController is the configuration for the SFCController
	// +optional
	SFCController *SFCControllerConfiguration `json:"sfcController,omitempty"`
	// HostedControlPlaneManager is the configuration for the HostedControlPlaneManager
	// +optional
	HostedControlPlaneManager *HostedControlPlaneManagerConfiguration `json:"hostedControlPlaneManager,omitempty"`
	// StaticControlPlaneManager is the configuration for the StaticControlPlaneManager
	// +optional
	StaticControlPlaneManager *StaticControlPlaneManagerConfiguration `json:"staticControlPlaneManager,omitempty"`

	// List of secret names which are used to pull images for DPF system components and DPUServices.
	// These secrets must be in the same namespace as the DPF Operator Config and should be created before the config is created.
	// System reconciliation will not proceed until these secrets are available.
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`
}

// DPFOperatorConfigStatus defines the observed state of DPFOperatorConfig
type DPFOperatorConfigStatus struct {
	// Conditions exposes the current state of the OperatorConfig.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ObservedGeneration records the Generation observed on the object the last time it was patched.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DPFOperatorConfig is the Schema for the dpfoperatorconfigs API
type DPFOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DPFOperatorConfigSpec   `json:"spec,omitempty"`
	Status DPFOperatorConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DPFOperatorConfigList contains a list of DPFOperatorConfig
type DPFOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPFOperatorConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPFOperatorConfig{}, &DPFOperatorConfigList{})
}

func (c *DPFOperatorConfig) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}
func (c *DPFOperatorConfig) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}
