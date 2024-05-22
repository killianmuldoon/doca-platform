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

var (
	DPFOperatorConfigFinalizer         = "dpf.nvidia.com/dpfoperatorconfig"
	DPFOperatorConfigNameLabelKey      = "dpf.nvidia.com/dpfoperatorconfig-name"
	DPFOperatorConfigNamespaceLabelKey = "dpf.nvidia.com/dpfoperatorconfig-namespace"
	// TODO: Ensure this label is added on all objects created by DPF.
	DPFComponentLabelKey = "dpf.nvidia.com/component"
)

// Overrides exposes a set of fields which impact the recommended behaviour of the DPF Operator
type Overrides struct {
	// DisableSystemComponents is a list of system components that will not be deployed.
	// +optional
	DisableSystemComponents []string `json:"disableSystemComponents,omitempty"`
	// DisableOVNKubernetesReconcile disables the reconciliation of OVNKubernetes in Openshift.
	// +optional
	DisableOVNKubernetesReconcile bool `json:"disableOVNKubernetesReconcile,omitempty"`
	// Paused disables all reconciliation of the DPFOperatorConfig when set to true.
	// +optional
	Paused bool `json:"paused,omitempty"`
}

// DPFOperatorConfigSpec defines the desired state of DPFOperatorConfig
type DPFOperatorConfigSpec struct {
	HostNetworkConfiguration  HostNetworkConfiguration  `json:"hostNetworkConfiguration"`
	ProvisioningConfiguration ProvisioningConfiguration `json:"provisioningConfiguration,omitempty"`
	// +optional
	Overrides *Overrides `json:"overrides,omitempty"`
	// List of secret names which are used to pull images for DPF system components and DPUServices.
	// These secrets must be in the same namespace as the DPF Operator Config and should be created before the config is created.
	// System reconciliation will not proceed until these secrets are available.
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`
}

// HostNetworkConfiguration holds network related configuration required to create a functional host network.
type HostNetworkConfiguration struct {
	// CIDR is the CIDR that is used by the administrator to calculate the IPs defined in HostIPs and DPUIPs
	// TODO: Add validator in validating webhook to ensure all the IPs below are part of this CIDR
	CIDR string `json:"cidr"`
	// Hosts is a list of items that reflects the configuration for each particular host. By host here we mean a physical
	// host that contains a DPU. An entry for each host that we want to configure DPF for should be added in that list
	// together with the relevant configuration needed.
	Hosts []Host `json:"hosts"`
	// HostPF0 is the name of the PF0 on the host. It needs to be the same across all the worker nodes.
	HostPF0 string `json:"hostPF0"`
	// HostPF0VF0 is the name of the first VF of the PF0 on the host. It needs to be the same across all the worker
	// nodes.
	HostPF0VF0 string `json:"hostPF0VF0"`
}

// Host represents a host with a DPU where DPF operator is going to be installed to and we expect to have host network
// working.
type Host struct {
	// HostClusterNodeName is the name of the node in the host cluster (OpenShift).
	HostClusterNodeName string `json:"hostClusterNodeName"`
	// DPUClusterNodeName is the name of the node in the DPU cluster (Kamaji).
	DPUClusterNodeName string `json:"dpuClusterNodeName"`
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

// ProvisioningConfiguration defines dpf-provisioning-controller related configurations
type ProvisioningConfiguration struct {
	// BFBPersistentVolumeClaimName is the name of the PersistentVolumeClaim used by dpf-provisioning-controller
	BFBPersistentVolumeClaimName string `json:"bfbPVCName"`
	ImagePullSecret              string `json:"imagePullSecret"`
	// DHCPServerAddress is the address of the DHCP server that allocates IP for DPU
	DHCPServerAddress string `json:"dhcpServerAddress"`
}

// DPFOperatorConfigStatus defines the observed state of DPFOperatorConfig
type DPFOperatorConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
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
