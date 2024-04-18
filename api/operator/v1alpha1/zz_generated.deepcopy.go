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
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPFOperatorConfig) DeepCopyInto(out *DPFOperatorConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
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
	in.HostNetworkConfiguration.DeepCopyInto(&out.HostNetworkConfiguration)
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
func (in *DPUConfiguration) DeepCopyInto(out *DPUConfiguration) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUConfiguration.
func (in *DPUConfiguration) DeepCopy() *DPUConfiguration {
	if in == nil {
		return nil
	}
	out := new(DPUConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HostNetworkConfiguration) DeepCopyInto(out *HostNetworkConfiguration) {
	*out = *in
	if in.HostIPs != nil {
		in, out := &in.HostIPs, &out.HostIPs
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.DPUConfiguration != nil {
		in, out := &in.DPUConfiguration, &out.DPUConfiguration
		*out = make(map[string]DPUConfiguration, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HostNetworkConfiguration.
func (in *HostNetworkConfiguration) DeepCopy() *HostNetworkConfiguration {
	if in == nil {
		return nil
	}
	out := new(HostNetworkConfiguration)
	in.DeepCopyInto(out)
	return out
}
