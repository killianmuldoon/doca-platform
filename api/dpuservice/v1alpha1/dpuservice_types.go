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
	"strings"

	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	DPUServiceKind              = "DPUService"
	DPUServiceListKind          = "DPUServiceList"
	DPUServiceFinalizer         = "dpf.nvidia.com/dpuservice"
	DPUServiceNameLabelKey      = "dpf.nvidia.com/dpuservice-name"
	DPUServiceNamespaceLabelKey = "dpf.nvidia.com/dpuservice-namespace"

	// DPFImagePullSecretLabelKey marks a secret as being an ImagePullSecret used by DPF which should be mirrored to DPUClusters.
	DPFImagePullSecretLabelKey = "dpf.nvidia.com/image-pull-secret"
)

var DPUServiceGroupVersionKind = GroupVersion.WithKind(DPUServiceKind)

const (
	ConditionApplicationPrereqsReconciled conditions.ConditionType = "ApplicationPrereqsReconciled"
	ConditionApplicationsReconciled       conditions.ConditionType = "ApplicationsReconciled"
	ConditionApplicationsReady            conditions.ConditionType = "ApplicationsReady"
)

var (
	Conditions = []conditions.ConditionType{
		conditions.TypeReady,
		ConditionApplicationPrereqsReconciled,
		ConditionApplicationsReconciled,
		ConditionApplicationsReady,
	}
)

var _ conditions.GetSet = &DPUService{}

func (c *DPUService) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

func (c *DPUService) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

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
	// HelmChart reflects the Helm related configuration
	HelmChart HelmChart `json:"helmChart"`
	// +optional
	ServiceID *string `json:"serviceID,omitempty"`
	// +optional
	ServiceDaemonSet *ServiceDaemonSetValues `json:"serviceDaemonSet,omitempty"`
	// DeployInCluster indicates if the DPUService Helm Chart will be deployed on the Host cluster. Default to false.
	// +optional
	DeployInCluster *bool `json:"deployInCluster,omitempty"`
}

// HelmChart reflects the helm related configuration
type HelmChart struct {
	// Source specifies information about the Helm chart
	Source ApplicationSource `json:"source"`
	// Values specifies Helm values to be passed to Helm template, defined as a map. This takes precedence over Values.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Values *runtime.RawExtension `json:"values,omitempty"`
}

type ApplicationSource struct {
	// RepoURL specifies the URL to the repository that contains the application Helm chart.
	// The URL must begin with either 'oci://' or 'https://', ensuring it points to a valid
	// OCI registry or a web-based repository.
	// +kubebuilder:validation:Pattern=`^(oci://|https://).+$`
	RepoURL string `json:"repoURL"`
	// Path is the location of the chart inside the repo.
	// +optional
	Path string `json:"path"`
	// Version is a semver tag for the Chart's version.
	// +kubebuilder:validation:MinLength=1
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
	// UpdateStrategy specifies the DeaemonSet update strategy for the ServiceDaemonset.
	UpdateStrategy *appsv1.DaemonSetUpdateStrategy `json:"updateStrategy,omitempty"`
	// Labels specifies labels which are added to the ServiceDaemonSet.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations specifies annotations which are added to the ServiceDaemonSet.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// DPUServiceStatus defines the observed state of DPUService
type DPUServiceStatus struct {
	// Conditions defines current service state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ObservedGeneration records the Generation observed on the object the last time it was patched.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

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

func (a *ApplicationSource) GetArgoRepoURL() string {
	return strings.TrimPrefix(a.RepoURL, "oci://")
}
