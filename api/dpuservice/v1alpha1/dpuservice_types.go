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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	DPUServiceFinalizer         = "dpf.nvidia.com/dpuservice"
	DPUServiceNameLabelKey      = "dpf.nvidia.com/dpuservice-name"
	DPUServiceNamespaceLabelKey = "dpf.nvidia.com/dpuservice-namespace"
	// DPFImagePullSecretLabelKey marks a secret as being an ImagePullSecret used by DPF which should be mirrored to DPUClusters.
	DPFImagePullSecretLabelKey = "dpf.nvidia.com/image-pull-secret"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DPUService is the Schema for the dpuservices API
type DPUService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DPUServiceSpec   `json:"spec,omitempty"`
	Status DPUServiceStatus `json:"status,omitempty"`
}

// DPUServiceSpec defines the desired state of DPUService
type DPUServiceSpec struct {
	Source ApplicationSource `json:"source"`
	// +optional
	ServiceID *string `json:"serviceID,omitempty"`
	// Values specifies Helm values to be passed to helm template, defined as a map. This takes precedence over Values.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Values *runtime.RawExtension `json:"values,omitempty"`
	// +optional
	ServiceDaemonSet *ServiceDaemonSetValues `json:"serviceDaemonSet,omitempty"`
}

type ApplicationSource struct {
	// TODO: This currently allows deploying a helm chart either from a helm repo. This should be validated as such.
	// RepoURL is the URL to the repository that contains the application helm chart.
	RepoURL string `json:"repoURL"`
	// Path is the location of the chart inside the repo.
	// +optional
	Path string `json:"path"`
	// Version is a semver tag for the Chart's version.
	Version string `json:"version"`
	// Chart is the name of the helm chart.
	// +optional
	Chart string `json:"chart"`
	// ReleaseName is the name to give to the release generate from the DPUService.
	// +optional
	ReleaseName string `json:"releaseName,omitempty"`
}

type ServiceDaemonSetValues struct {
	// NodeSelector specifies which Nodes to deploy the ServiceDaemonSet to.
	NodeSelector *corev1.NodeSelector `json:"nodeSelector,omitempty"`
	// Resources specifies the resource limits and requests for the ServiceDaemonSet.
	Resources corev1.ResourceList `json:"resources,omitempty"`
	// UpdateStrategy specifies the DeaemonSet update strategy for the ServiceDaemonset.
	UpdateStrategy *appsv1.DaemonSetUpdateStrategy `json:"updateStrategy,omitempty"`
	// Labels specifies labels which are added to the ServiceDaemonSet.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations specifies annotations which are added to the ServiceDaemonSet.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// DPUServiceStatus defines the observed state of DPUService
type DPUServiceStatus struct{}

//+kubebuilder:object:root=true

// DPUServiceList contains a list of DPUService
type DPUServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPUService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPUService{}, &DPUServiceList{})
}
