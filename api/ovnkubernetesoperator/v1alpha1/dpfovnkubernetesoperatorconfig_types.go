/*
COPYRIGHT 2024 NVIDIA

Licensed under the Apache License, Version 2.0 (the License);
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an AS IS BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DPFOVNKubernetesOperatorConfigSpec defines the desired state of DPFOVNKubernetesOperatorConfig
type DPFOVNKubernetesOperatorConfigSpec struct {
	// CIDR is the CIDR that is used by the administrator to calculate the IPs defined in HostIPs and DPUIPs
	// TODO: Add validator in validating webhook to ensure all the IPs below are part of this CIDR
	CIDR string `json:"cidr"`
	// Hosts is a list of items that reflects the configuration for each particular host. By host here we mean a physical
	// host that contains a DPU. An entry for each host that we want to configure DPF for should be added in that list
	// together with the relevant configuration needed.
	Hosts []Host `json:"hosts"`

	// List of secret names which are used to pull images for OVN Kubernetes components.
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// NetworkInjectorConfig is the configuration related to the Network Injector Webhook.
	NetworkInjectorConfig `json:",inline"`
}

// Host represents a host with a DPU where DPF operator is going to be installed to and we expect to have host network
// working.
type Host struct {
	// HostClusterNodeName is the name of the node in the host cluster (OpenShift).
	HostClusterNodeName string `json:"hostClusterNodeName"`
	// HostIP is the IP that will be assigned to the PF Representor on each Host.
	// TODO: Add validator in validating webhook to ensure string is actually net.IPNet
	HostIP string `json:"hostIP"`
	// DPUIP is the IP that will be assigned to the VTEP interface of the DPU.
	// TODO: Add validator in validating webhook to ensure string is actually net.IPNet
	DPUIP string `json:"dpuIP"`
	// Gateway is the gateway that will be added on the routes related to OVN Kubernetes traffic.
	// TODO: Add validator in validating webhook to ensure string is actually net.IPNet
	Gateway string `json:"gateway"`
}

// NetworkInjectorConfig is the configuration related to the Network Injector Webhook.
type NetworkInjectorConfig struct {
	// VFResourceName is the name of the Kubernetes resource associated with the VFs that will be injected into
	// workloads.
	VFResourceName string `json:"vfResourceName,omitempty"`
}

// DPFOVNKubernetesOperatorConfigStatus defines the observed state of DPFOVNKubernetesOperatorConfig
type DPFOVNKubernetesOperatorConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DPFOVNKubernetesOperatorConfig is the Schema for the dpfovnkubernetesoperatorconfigs API
type DPFOVNKubernetesOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DPFOVNKubernetesOperatorConfigSpec   `json:"spec,omitempty"`
	Status DPFOVNKubernetesOperatorConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DPFOVNKubernetesOperatorConfigList contains a list of DPFOVNKubernetesOperatorConfig
type DPFOVNKubernetesOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPFOVNKubernetesOperatorConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPFOVNKubernetesOperatorConfig{}, &DPFOVNKubernetesOperatorConfigList{})
}
