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

// ComponentConfig defines the shared config for all components deployed by the DPF Operator.
// +kubebuilder:object:generate=false
type ComponentConfig interface {
	Name() string
	Disabled() bool
}

// ImageComponentConfig is the shared config for helm components.
//
// +kubebuilder:object:generate=false
type ImageComponentConfig interface {
	GetImage() *string
}

// HelmComponentConfig is the shared config for helm components.
//
// +kubebuilder:object:generate=false
type HelmComponentConfig interface {
	GetHelmChart() *string
}

type ProvisioningControllerConfiguration struct {
	// Disable ensures the component is not deployed when set to true.
	// +optional
	Disable *bool `json:"disable"`

	//Image overrides the container image used by the Provisioning controller
	Image *string `json:"image,omitempty"`

	// BFBPersistentVolumeClaimName is the name of the PersistentVolumeClaim used by dpf-provisioning-controller
	// +kubebuilder:validation:MinLength=1
	BFBPersistentVolumeClaimName string `json:"bfbPVCName"`

	// DMSTimeout is the max time in seconds within which a DMS API must respond, 0 is unlimited
	// +kubebuilder:validation:Minimum=1
	// +optional
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

func (c ProvisioningControllerConfiguration) GetImage() *string {
	return c.Image
}

type DPUServiceControllerConfiguration struct {
	// Disable ensures the component is not deployed when set to true.
	// +optional
	Disable *bool `json:"disable,omitempty"`

	// Image overrides the container image used by the DPUService controller
	// +optional
	Image *string `json:"image,omitempty"`
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

func (c *DPUServiceControllerConfiguration) GetImage() *string {
	return c.Image
}

type HostedControlPlaneManagerConfiguration struct {
	// Disable ensures the component is not deployed when set to true.
	// +optional
	Disable *bool `json:"disable,omitempty"`

	// Image overrides the container image used by the HostedControlPlaneManager.
	Image *string `json:"image,omitempty"`
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

func (c *HostedControlPlaneManagerConfiguration) GetImage() *string {
	return c.Image
}

type StaticControlPlaneManagerConfiguration struct {
	// Disable ensures the component is not deployed when set to true.
	// +optional
	Disable *bool `json:"disable,omitempty"`

	// Image is the container image used by the StaticControlPlaneManager
	// +optional
	Image *string `json:"image,omitempty"`
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

func (c *StaticControlPlaneManagerConfiguration) GetImage() *string {
	return c.Image
}

type ServiceSetControllerConfiguration struct {
	// Disable ensures the component is not deployed when set to true.
	// +optional
	Disable *bool `json:"disable,omitempty"`

	// Image overrides the container image used by the ServiceSetController
	// +optional
	Image *string `json:"image,omitempty"`

	// HelmChart overrides the helm chart used by the ServiceSet controller.
	// +optional
	HelmChart *string `json:"helmChart,omitempty"`
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

func (c *ServiceSetControllerConfiguration) GetImage() *string {
	return c.Image
}

func (c *ServiceSetControllerConfiguration) GetHelmChart() *string {
	return c.HelmChart
}

type FlannelConfiguration struct {
	// Disable ensures the component is not deployed when set to true.
	// +optional
	Disable *bool `json:"disable,omitempty"`

	// HelmChart overrides the helm chart used by the Flannel
	// +optional
	HelmChart *string `json:"helmChart,omitempty"`
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

func (c *FlannelConfiguration) GetHelmChart() *string {
	return c.HelmChart
}

type MultusConfiguration struct {
	// Disable ensures the component is not deployed when set to true.
	// +optional
	Disable *bool `json:"disable,omitempty"`

	// Image overrides the container image used by Multus
	// +optional
	Image *string `json:"image,omitempty"`

	// HelmChart overrides the helm chart used by Multus
	// +optional
	HelmChart *string `json:"helmChart,omitempty"`
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

func (c *MultusConfiguration) GetImage() *string {
	return c.Image
}

func (c *MultusConfiguration) GetHelmChart() *string {
	return c.HelmChart
}

type NVIPAMConfiguration struct {
	// Disable ensures the component is not deployed when set to true.
	// +optional
	Disable *bool `json:"disable,omitempty"`

	// Image overrides the container image used by NVIPAM
	// +optional
	Image *string `json:"image,omitempty"`

	// HelmChart overrides the helm chart used by NVIPAM
	// +optional
	HelmChart *string `json:"helmChart,omitempty"`
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

func (c *NVIPAMConfiguration) GetImage() *string {
	return c.Image
}

func (c *NVIPAMConfiguration) GetHelmChart() *string {
	return c.HelmChart
}

type SRIOVDevicePluginConfiguration struct {
	// Disable ensures the component is not deployed when set to true.
	// +optional
	Disable *bool `json:"disable,omitempty"`

	// Image overrides the container image used by the SRIOV Device Plugin
	// +optional
	Image *string `json:"image,omitempty"`

	// HelmChart overrides the helm chart used by the SRIOV Device Plugin
	// +optional
	HelmChart *string `json:"helmChart,omitempty"`
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

func (c *SRIOVDevicePluginConfiguration) GetImage() *string {
	return c.Image
}

func (c *SRIOVDevicePluginConfiguration) GetHelmChart() *string {
	return c.HelmChart
}

type OVSCNIConfiguration struct {
	// Disable ensures the component is not deployed when set to true.
	// +optional
	Disable *bool `json:"disable,omitempty"`

	// Image overrides the container image used by the OVS CNI
	// +optional
	Image *string `json:"image,omitempty"`

	// HelmChart overrides the helm chart used by the OVS CNI
	// +optional
	HelmChart *string `json:"helmChart,omitempty"`
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

func (c *OVSCNIConfiguration) GetImage() *string {
	return c.Image
}

func (c *OVSCNIConfiguration) GetHelmChart() *string {
	return c.HelmChart
}

type SFCControllerConfiguration struct {
	// Disable ensures the component is not deployed when set to true.
	// +optional
	Disable *bool `json:"disable,omitempty"`

	// Image overrides the container image used by the SFC Controller
	// +optional
	Image *string `json:"image,omitempty"`

	// HelmChart overrides the helm chart used by the SFC Controller
	// +optional
	HelmChart *string `json:"helmChart,omitempty"`
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

func (c *SFCControllerConfiguration) GetImage() *string {
	return c.Image
}

func (c *SFCControllerConfiguration) GetHelmChart() *string {
	return c.HelmChart
}
