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

const (
	ProvisioningControllerName    = "provisioning-controller"
	DPUServiceControllerName      = "dpuservice-controller"
	ServiceSetControllerName      = "servicechainset-controller"
	FlannelName                   = "flannel"
	MultusName                    = "multus"
	SRIOVDevicePluginName         = "sriov-device-plugin"
	OVSCNIName                    = "ovs-cni"
	NVIPAMName                    = "nvidia-k8s-ipam"
	SFCControllerName             = "sfc-controller"
	HostedControlPlaneManagerName = "hosted-control-plane"
	StaticControlPlaneManagerName = "static-control-plane"
)

func (c *DPFOperatorConfig) ComponentConfigs() []ComponentConfig {
	out := []ComponentConfig{}

	out = append(out, c.Spec.ProvisioningController)

	if c.Spec.DPUServiceController != nil {
		out = append(out, c.Spec.DPUServiceController)
	}
	if c.Spec.Flannel != nil {
		out = append(out, c.Spec.Flannel)
	}
	if c.Spec.Multus != nil {
		out = append(out, c.Spec.Multus)
	}
	if c.Spec.SRIOVDevicePlugin != nil {
		out = append(out, c.Spec.SRIOVDevicePlugin)
	}
	if c.Spec.NVIPAM != nil {
		out = append(out, c.Spec.NVIPAM)
	}
	if c.Spec.SFCController != nil {
		out = append(out, c.Spec.SFCController)
	}
	if c.Spec.OVSCNI != nil {
		out = append(out, c.Spec.OVSCNI)
	}
	if c.Spec.ServiceSetController != nil {
		out = append(out, c.Spec.ServiceSetController)
	}
	if c.Spec.HostedControlPlaneManager != nil {
		out = append(out, c.Spec.HostedControlPlaneManager)
	}
	if c.Spec.StaticControlPlaneManager != nil {
		out = append(out, c.Spec.StaticControlPlaneManager)
	}
	return out
}

// +kubebuilder:object:generate=false
type ComponentConfig interface {
	Name() string
	Disabled() bool
}

// +kubebuilder:object:generate=false
type HelmComponentConfig interface {
	HelmChart() string
}

// +kubebuilder:object:generate=false
type ImageComponentConfig interface {
	Image() string
}

type ProvisioningControllerConfiguration struct {
	// +optional
	Disable *bool `json:"disable"`
	// BFBPersistentVolumeClaimName is the name of the PersistentVolumeClaim used by dpf-provisioning-controller
	// +kubebuilder:validation:MinLength=1
	BFBPersistentVolumeClaimName string `json:"bfbPVCName"`
	// DMSTimeout is the max time in seconds within which a DMS API must respond, 0 is unlimited
	// +kubebuilder:validation:Minimum=1
	DMSTimeout *int `json:"dmsTimeout,omitempty"`
}

func (c ProvisioningControllerConfiguration) Name() string {
	return ProvisioningControllerName
}

func (c ProvisioningControllerConfiguration) Disabled() bool {
	if c.Disable == nil {
		return false
	}
	return *c.Disable
}

type DPUServiceControllerConfiguration struct {
	// +optional
	Disable *bool `json:"disable"`
}

func (c *DPUServiceControllerConfiguration) Name() string {
	return DPUServiceControllerName
}

func (c *DPUServiceControllerConfiguration) Disabled() bool {
	if c.Disable == nil {
		return false
	}
	return *c.Disable
}

type ServiceSetControllerConfiguration struct {
	// +optional
	Disable *bool `json:"disable"`
}

func (c *ServiceSetControllerConfiguration) Name() string {
	return ServiceSetControllerName
}

func (c *ServiceSetControllerConfiguration) Disabled() bool {
	if c.Disable == nil {
		return false
	}
	return *c.Disable
}

type FlannelConfiguration struct {
	// +optional
	Disable *bool `json:"disable"`
}

func (c *FlannelConfiguration) Name() string {
	return FlannelName
}

func (c *FlannelConfiguration) Disabled() bool {
	if c.Disable == nil {
		return false
	}
	return *c.Disable
}

type MultusConfiguration struct {
	// +optional
	Disable *bool `json:"disable"`
}

func (c *MultusConfiguration) Name() string {
	return MultusName
}

func (c *MultusConfiguration) Disabled() bool {
	if c.Disable == nil {
		return false
	}
	return *c.Disable
}

type NVIPAMConfiguration struct {
	// +optional
	Disable *bool `json:"disable"`
}

func (c *NVIPAMConfiguration) Name() string {
	return NVIPAMName
}

func (c *NVIPAMConfiguration) Disabled() bool {
	if c.Disable == nil {
		return false
	}
	return *c.Disable
}

type SRIOVDevicePluginConfiguration struct {
	// +optional
	Disable *bool `json:"disable"`
}

func (c *SRIOVDevicePluginConfiguration) Name() string {
	return SRIOVDevicePluginName
}

func (c *SRIOVDevicePluginConfiguration) Disabled() bool {
	if c.Disable == nil {
		return false
	}
	return *c.Disable
}

type OVSCNIConfiguration struct {
	// +optional
	Disable *bool `json:"disable"`
}

func (c *OVSCNIConfiguration) Name() string {
	return OVSCNIName
}

func (c *OVSCNIConfiguration) Disabled() bool {
	if c.Disable == nil {
		return false
	}
	return *c.Disable
}

type SFCControllerConfiguration struct {
	// +optional
	Disable *bool `json:"disable"`
}

func (c *SFCControllerConfiguration) Name() string {
	return SFCControllerName
}

func (c *SFCControllerConfiguration) Disabled() bool {
	if c.Disable == nil {
		return false
	}
	return *c.Disable
}

type HostedControlPlaneManagerConfiguration struct {
	// +optional
	Disable *bool `json:"disable"`
}

func (c *HostedControlPlaneManagerConfiguration) Name() string {
	return HostedControlPlaneManagerName
}

func (c *HostedControlPlaneManagerConfiguration) Disabled() bool {
	if c.Disable == nil {
		return false
	}
	return *c.Disable
}

type StaticControlPlaneManagerConfiguration struct {
	// +optional
	Disable *bool `json:"disable"`
}

func (c *StaticControlPlaneManagerConfiguration) Name() string {
	return StaticControlPlaneManagerName
}

func (c *StaticControlPlaneManagerConfiguration) Disabled() bool {
	if c.Disable == nil {
		return false
	}
	return *c.Disable
}
