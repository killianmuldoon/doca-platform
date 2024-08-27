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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DPUServiceTemplate is the Schema for the dpuservicetemplates API. This object is intended to be used in
// conjunction with a DPUDeployment object. This object is the template from which the DPUService will be created. It
// contains configuration options related to resources required by the service to be deployed. The rest of the
// configuration options must be defined in a DPUServiceConfiguration object.
type DPUServiceTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Spec is immutable"
	Spec   DPUServiceTemplateSpec   `json:"spec,omitempty"`
	Status DPUServiceTemplateStatus `json:"status,omitempty"`
}

// DPUServiceTemplateSpec defines the desired state of DPUServiceTemplate
type DPUServiceTemplateSpec struct {
	// Service is the name of the DPU service this configuration refers to. It must match .spec.service of a
	// DPUServiceConfiguration object and one of the keys in .spec.services of a DPUDeployment object.
	Service string `json:"service"`
	// ServiceDaemonSet contains settings related to the underlying DaemonSet that is part of the Helm chart
	// +optional
	ServiceDaemonSet DPUServiceTemplateServiceDaemonSetValues `json:"serviceDaemonSet,omitempty"`
	// ResourceRequirements contains the overall resources required by this particular service to run on a single node
	// +optional
	ResourceRequirements corev1.ResourceList `json:"resourceRequirements,omitempty"`
}

// DPUServiceTemplateServiceDaemonSet reflects the Helm related configuration
type DPUServiceTemplateServiceDaemonSetValues struct {
	// Resources specifies the resource limits and requests for the ServiceDaemonSet.
	// +optional
	Resources corev1.ResourceList `json:"resources,omitempty"`
}

// DPUServiceTemplateStatus defines the observed state of DPUServiceTemplate
type DPUServiceTemplateStatus struct{}

//+kubebuilder:object:root=true

// DPUServiceTemplateList contains a list of DPUServiceTemplate
type DPUServiceTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPUServiceTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPUServiceTemplate{}, &DPUServiceTemplateList{})
}
