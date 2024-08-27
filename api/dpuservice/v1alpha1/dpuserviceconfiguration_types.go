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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DPUServiceConfiguration is the Schema for the dpuserviceconfigurations API. This object is intended to be used in
// conjunction with a DPUDeployment object. This object is the template from which the DPUService will be created. It
// contains all configuration options from the user to be provided to the service itself via the helm chart values.
// This object doesn't allow configuration of nodeSelector and resources in purpose as these are delegated to the
// DPUDeployment and DPUServiceTemplate accordingly.
type DPUServiceConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DPUServiceConfigurationSpec   `json:"spec,omitempty"`
	Status DPUServiceConfigurationStatus `json:"status,omitempty"`
}

// DPUServiceConfigurationSpec defines the desired state of DPUServiceConfiguration
type DPUServiceConfigurationSpec struct {
	// Service is the name of the DPU service this configuration refers to. It must match .spec.service of a
	// DPUServiceTemplate object and one of the keys in .spec.services of a DPUDeployment object.
	Service string `json:"service"`
	// ServiceConfiguration contains fields that are configured on the generated DPUService.
	ServiceConfiguration ServiceConfiguration `json:"serviceConfiguration"`
}

// ServiceConfiguration contains fields that are configured on the generated DPUService.
type ServiceConfiguration struct {
	// HelmChart reflects the Helm related configuration
	HelmChart DPUServiceConfigurationHelmChart `json:"helmChart"`
	// ServiceDaemonSet contains settings related to the underlying DaemonSet that is part of the Helm chart
	// +optional
	ServiceDaemonSet DPUServiceConfigurationServiceDaemonSetValues `json:"serviceDaemonSet,omitempty"`
	// TODO: Add nodeEffect
	// DeployInCluster indicates if the DPUService Helm Chart will be deployed on the Host cluster. Default to false.
	// +optional
	DeployInCluster *bool `json:"deployInCluster,omitempty"`
}

// DPUServiceConfigurationHelmChart reflects the helm related configuration
type DPUServiceConfigurationHelmChart struct {
	// Source specifies information about the Helm chart
	Source ApplicationSource `json:"source"`
	// Values specifies Helm values to be passed to Helm template, defined as a map. This takes precedence over Values.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Values *runtime.RawExtension `json:"values,omitempty"`
}

// DPUServiceConfigurationServiceDaemonSet reflects the Helm related configuration
type DPUServiceConfigurationServiceDaemonSetValues struct {
	// Labels specifies labels which are added to the ServiceDaemonSet.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations specifies annotations which are added to the ServiceDaemonSet.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// UpdateStrategy specifies the DeaemonSet update strategy for the ServiceDaemonset.
	// +optional
	UpdateStrategy *appsv1.DaemonSetUpdateStrategy `json:"updateStrategy,omitempty"`
}

// DPUServiceConfigurationStatus defines the observed state of DPUServiceConfiguration
type DPUServiceConfigurationStatus struct{}

//+kubebuilder:object:root=true

// DPUServiceConfigurationList contains a list of DPUServiceConfiguration
type DPUServiceConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPUServiceConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPUServiceConfiguration{}, &DPUServiceConfigurationList{})
}
