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
	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DPUDeploymentFinalizer = "dpf.nvidia.com/dpudeployment"
	DPUDeploymentKind      = "DPUDeployment"
)

var DPUDeploymentGroupVersionKind = GroupVersion.WithKind(DPUDeploymentKind)

// Status related variables
const (
	ConditionPreReqsReady               conditions.ConditionType = "PrerequisitesReady"
	ConditionResourceFittingReady       conditions.ConditionType = "ResourceFittingReady"
	ConditionDPUSetsReconciled          conditions.ConditionType = "DpuSetsReconciled"
	ConditionDPUSetsReady               conditions.ConditionType = "DpuSetsReady"
	ConditionDPUServicesReconciled      conditions.ConditionType = "DPUServicesReconciled"
	ConditionDPUServicesReady           conditions.ConditionType = "DPUServicesReady"
	ConditionDPUServiceChainsReconciled conditions.ConditionType = "DPUServiceChainsReconciled"
	ConditionDPUServiceChainsReady      conditions.ConditionType = "DPUServiceChainsReady"
)

var (
	DPUDeploymentConditions = []conditions.ConditionType{
		conditions.TypeReady,
		ConditionPreReqsReady,
		ConditionResourceFittingReady,
		ConditionDPUSetsReconciled,
		ConditionDPUSetsReady,
		ConditionDPUServicesReconciled,
		ConditionDPUServicesReady,
		ConditionDPUServiceChainsReconciled,
		ConditionDPUServiceChainsReady,
	}
)

var _ conditions.GetSet = &DPUDeployment{}

func (c *DPUDeployment) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

func (c *DPUDeployment) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DPUDeployment is the Schema for the dpudeployments API. This object connects DPUServices with specific BFBs and
// DPUServiceChains.
type DPUDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DPUDeploymentSpec   `json:"spec,omitempty"`
	Status DPUDeploymentStatus `json:"status,omitempty"`
}

// DPUDeploymentSpec defines the desired state of DPUDeployment
type DPUDeploymentSpec struct {
	// DPUs contains the DPU related configuration
	DPUs DPUs `json:"dpus"`
	// Services contains the Service related configuration. The key is the deploymentServiceName and the value is its
	// configuration. All underlying objects must specify the same deploymentServiceName in order to be able to be consumed by the
	// DPUDeployment.
	Services map[string]DPUDeploymentServiceConfiguration `json:"services"`
	// ServiceChains contains the configuration related to the DPUServiceChains that the DPUDeployment creates.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=50
	ServiceChains []sfcv1.Switch `json:"serviceChains"`
	// Strategy contains configuration related to the rolling update that the DPUDeployment can do whenever a DPUService
	// or a DPUSet related setting has changed.
	// +optional
	Strategy Strategy `json:"strategy,omitempty"`
}

// DPUs contains the DPU related configuration
type DPUs struct {
	// BFB is the name of the BFB object to be used in this DPUDeployment. It must be in the same namespace as the
	// DPUDeployment.
	BFB string `json:"bfb"`
	// Flavor is the name of the DPUFlavor object to be used in this DPUDeployment. It must be in the same namespace as
	// the DPUDeployment.
	Flavor string `json:"flavor"`
	// DPUSets contains configuration for each DPUSet that is going to be created by the DPUDeployment
	// +optional
	DPUSets []DPUSet `json:"dpuSets,omitempty"`
}

// DPUSet contains configuration for the DPUSet to be created by the DPUDeployment
type DPUSet struct {
	// NodeSelector defines the nodes that the DPUSet should target
	// +optional
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`
	// DPUSelector defines the DPUs that the DPUSet should target
	// TODO: Revisit if this one is needed at all or we can use the nodeSelector directly. If it's not the case, drop
	// this field and the field in the DPUSet. Based on the current implementation, it looks like we could remove it.
	// +optional
	DPUSelector map[string]string `json:"dpuSelector,omitempty"`
	// DPUAnnotations is the annotations to be added to the DPU object created by the DPUSet.
	// +optional
	DPUAnnotations map[string]string `json:"dpuAnnotations,omitempty"`
}

// DPUDeploymentServiceConfiguration describes the configuration of a particular Service
type DPUDeploymentServiceConfiguration struct {
	// ServiceTemplate is the name of the DPUServiceTemplate object to be used for this Service. It must be in the same
	// namespace as the DPUDeployment.
	ServiceTemplate string `json:"serviceTemplate"`
	// ServiceConfiguration is the name of the DPUServiceConfiguration object to be used for this Service. It must be
	// in the same namespace as the DPUDeployment.
	ServiceConfiguration string `json:"serviceConfiguration"`
}

// Strategy contains configuration related to the rolling update that the DPUDeployment can do whenever a DPUService
// or a DPUSet related setting has changed.
type Strategy struct {
	// RollingUpdate specifies the parameters related to the rolling update
	RollingUpdate appsv1.RollingUpdateDaemonSet `json:"rollingUpdate"`
}

// DPUDeploymentStatus defines the observed state of DPUDeployment
type DPUDeploymentStatus struct {
	// Conditions reflect the status of the object
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ObservedGeneration records the Generation observed on the object the last time it was patched.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true

// DPUDeploymentList contains a list of DPUDeployment
type DPUDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPUDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPUDeployment{}, &DPUDeploymentList{})
}
