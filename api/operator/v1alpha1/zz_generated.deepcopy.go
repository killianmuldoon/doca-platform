//go:build !ignore_autogenerated

/*
Copyright 2024 NVIDIA.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPFOperatorConfig) DeepCopyInto(out *DPFOperatorConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPFOperatorConfig.
func (in *DPFOperatorConfig) DeepCopy() *DPFOperatorConfig {
	if in == nil {
		return nil
	}
	out := new(DPFOperatorConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPFOperatorConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPFOperatorConfigList) DeepCopyInto(out *DPFOperatorConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DPFOperatorConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPFOperatorConfigList.
func (in *DPFOperatorConfigList) DeepCopy() *DPFOperatorConfigList {
	if in == nil {
		return nil
	}
	out := new(DPFOperatorConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPFOperatorConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPFOperatorConfigSpec) DeepCopyInto(out *DPFOperatorConfigSpec) {
	*out = *in
	if in.Overrides != nil {
		in, out := &in.Overrides, &out.Overrides
		*out = new(Overrides)
		(*in).DeepCopyInto(*out)
	}
	if in.DPUServiceController != nil {
		in, out := &in.DPUServiceController, &out.DPUServiceController
		*out = new(DPUServiceControllerConfiguration)
		(*in).DeepCopyInto(*out)
	}
	in.ProvisioningController.DeepCopyInto(&out.ProvisioningController)
	if in.ServiceSetController != nil {
		in, out := &in.ServiceSetController, &out.ServiceSetController
		*out = new(ServiceSetControllerConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.Multus != nil {
		in, out := &in.Multus, &out.Multus
		*out = new(MultusConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.SRIOVDevicePlugin != nil {
		in, out := &in.SRIOVDevicePlugin, &out.SRIOVDevicePlugin
		*out = new(SRIOVDevicePluginConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.Flannel != nil {
		in, out := &in.Flannel, &out.Flannel
		*out = new(FlannelConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.OVSCNI != nil {
		in, out := &in.OVSCNI, &out.OVSCNI
		*out = new(OVSCNIConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.NVIPAM != nil {
		in, out := &in.NVIPAM, &out.NVIPAM
		*out = new(NVIPAMConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.SFCController != nil {
		in, out := &in.SFCController, &out.SFCController
		*out = new(SFCControllerConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.HostedControlPlaneManager != nil {
		in, out := &in.HostedControlPlaneManager, &out.HostedControlPlaneManager
		*out = new(HostedControlPlaneManagerConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.StaticControlPlaneManager != nil {
		in, out := &in.StaticControlPlaneManager, &out.StaticControlPlaneManager
		*out = new(StaticControlPlaneManagerConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPFOperatorConfigSpec.
func (in *DPFOperatorConfigSpec) DeepCopy() *DPFOperatorConfigSpec {
	if in == nil {
		return nil
	}
	out := new(DPFOperatorConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPFOperatorConfigStatus) DeepCopyInto(out *DPFOperatorConfigStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPFOperatorConfigStatus.
func (in *DPFOperatorConfigStatus) DeepCopy() *DPFOperatorConfigStatus {
	if in == nil {
		return nil
	}
	out := new(DPFOperatorConfigStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceControllerConfiguration) DeepCopyInto(out *DPUServiceControllerConfiguration) {
	*out = *in
	if in.Disable != nil {
		in, out := &in.Disable, &out.Disable
		*out = new(bool)
		**out = **in
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceControllerConfiguration.
func (in *DPUServiceControllerConfiguration) DeepCopy() *DPUServiceControllerConfiguration {
	if in == nil {
		return nil
	}
	out := new(DPUServiceControllerConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FlannelConfiguration) DeepCopyInto(out *FlannelConfiguration) {
	*out = *in
	if in.Disable != nil {
		in, out := &in.Disable, &out.Disable
		*out = new(bool)
		**out = **in
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
	if in.HelmChart != nil {
		in, out := &in.HelmChart, &out.HelmChart
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FlannelConfiguration.
func (in *FlannelConfiguration) DeepCopy() *FlannelConfiguration {
	if in == nil {
		return nil
	}
	out := new(FlannelConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HostedControlPlaneManagerConfiguration) DeepCopyInto(out *HostedControlPlaneManagerConfiguration) {
	*out = *in
	if in.Disable != nil {
		in, out := &in.Disable, &out.Disable
		*out = new(bool)
		**out = **in
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HostedControlPlaneManagerConfiguration.
func (in *HostedControlPlaneManagerConfiguration) DeepCopy() *HostedControlPlaneManagerConfiguration {
	if in == nil {
		return nil
	}
	out := new(HostedControlPlaneManagerConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MultusConfiguration) DeepCopyInto(out *MultusConfiguration) {
	*out = *in
	if in.Disable != nil {
		in, out := &in.Disable, &out.Disable
		*out = new(bool)
		**out = **in
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
	if in.HelmChart != nil {
		in, out := &in.HelmChart, &out.HelmChart
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MultusConfiguration.
func (in *MultusConfiguration) DeepCopy() *MultusConfiguration {
	if in == nil {
		return nil
	}
	out := new(MultusConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NVIPAMConfiguration) DeepCopyInto(out *NVIPAMConfiguration) {
	*out = *in
	if in.Disable != nil {
		in, out := &in.Disable, &out.Disable
		*out = new(bool)
		**out = **in
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
	if in.HelmChart != nil {
		in, out := &in.HelmChart, &out.HelmChart
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NVIPAMConfiguration.
func (in *NVIPAMConfiguration) DeepCopy() *NVIPAMConfiguration {
	if in == nil {
		return nil
	}
	out := new(NVIPAMConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OVSCNIConfiguration) DeepCopyInto(out *OVSCNIConfiguration) {
	*out = *in
	if in.Disable != nil {
		in, out := &in.Disable, &out.Disable
		*out = new(bool)
		**out = **in
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
	if in.HelmChart != nil {
		in, out := &in.HelmChart, &out.HelmChart
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OVSCNIConfiguration.
func (in *OVSCNIConfiguration) DeepCopy() *OVSCNIConfiguration {
	if in == nil {
		return nil
	}
	out := new(OVSCNIConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Overrides) DeepCopyInto(out *Overrides) {
	*out = *in
	if in.Paused != nil {
		in, out := &in.Paused, &out.Paused
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Overrides.
func (in *Overrides) DeepCopy() *Overrides {
	if in == nil {
		return nil
	}
	out := new(Overrides)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProvisioningControllerConfiguration) DeepCopyInto(out *ProvisioningControllerConfiguration) {
	*out = *in
	if in.Disable != nil {
		in, out := &in.Disable, &out.Disable
		*out = new(bool)
		**out = **in
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
	if in.DMSTimeout != nil {
		in, out := &in.DMSTimeout, &out.DMSTimeout
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProvisioningControllerConfiguration.
func (in *ProvisioningControllerConfiguration) DeepCopy() *ProvisioningControllerConfiguration {
	if in == nil {
		return nil
	}
	out := new(ProvisioningControllerConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SFCControllerConfiguration) DeepCopyInto(out *SFCControllerConfiguration) {
	*out = *in
	if in.Disable != nil {
		in, out := &in.Disable, &out.Disable
		*out = new(bool)
		**out = **in
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
	if in.HelmChart != nil {
		in, out := &in.HelmChart, &out.HelmChart
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SFCControllerConfiguration.
func (in *SFCControllerConfiguration) DeepCopy() *SFCControllerConfiguration {
	if in == nil {
		return nil
	}
	out := new(SFCControllerConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SRIOVDevicePluginConfiguration) DeepCopyInto(out *SRIOVDevicePluginConfiguration) {
	*out = *in
	if in.Disable != nil {
		in, out := &in.Disable, &out.Disable
		*out = new(bool)
		**out = **in
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
	if in.HelmChart != nil {
		in, out := &in.HelmChart, &out.HelmChart
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SRIOVDevicePluginConfiguration.
func (in *SRIOVDevicePluginConfiguration) DeepCopy() *SRIOVDevicePluginConfiguration {
	if in == nil {
		return nil
	}
	out := new(SRIOVDevicePluginConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceSetControllerConfiguration) DeepCopyInto(out *ServiceSetControllerConfiguration) {
	*out = *in
	if in.Disable != nil {
		in, out := &in.Disable, &out.Disable
		*out = new(bool)
		**out = **in
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
	if in.HelmChart != nil {
		in, out := &in.HelmChart, &out.HelmChart
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceSetControllerConfiguration.
func (in *ServiceSetControllerConfiguration) DeepCopy() *ServiceSetControllerConfiguration {
	if in == nil {
		return nil
	}
	out := new(ServiceSetControllerConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StaticControlPlaneManagerConfiguration) DeepCopyInto(out *StaticControlPlaneManagerConfiguration) {
	*out = *in
	if in.Disable != nil {
		in, out := &in.Disable, &out.Disable
		*out = new(bool)
		**out = **in
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StaticControlPlaneManagerConfiguration.
func (in *StaticControlPlaneManagerConfiguration) DeepCopy() *StaticControlPlaneManagerConfiguration {
	if in == nil {
		return nil
	}
	out := new(StaticControlPlaneManagerConfiguration)
	in.DeepCopyInto(out)
	return out
}
