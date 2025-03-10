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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BFB) DeepCopyInto(out *BFB) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BFB.
func (in *BFB) DeepCopy() *BFB {
	if in == nil {
		return nil
	}
	out := new(BFB)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BFB) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BFBList) DeepCopyInto(out *BFBList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]BFB, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BFBList.
func (in *BFBList) DeepCopy() *BFBList {
	if in == nil {
		return nil
	}
	out := new(BFBList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BFBList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BFBReference) DeepCopyInto(out *BFBReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BFBReference.
func (in *BFBReference) DeepCopy() *BFBReference {
	if in == nil {
		return nil
	}
	out := new(BFBReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BFBSpec) DeepCopyInto(out *BFBSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BFBSpec.
func (in *BFBSpec) DeepCopy() *BFBSpec {
	if in == nil {
		return nil
	}
	out := new(BFBSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BFBStatus) DeepCopyInto(out *BFBStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BFBStatus.
func (in *BFBStatus) DeepCopy() *BFBStatus {
	if in == nil {
		return nil
	}
	out := new(BFBStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterEndpointSpec) DeepCopyInto(out *ClusterEndpointSpec) {
	*out = *in
	if in.Keepalived != nil {
		in, out := &in.Keepalived, &out.Keepalived
		*out = new(KeepalivedSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterEndpointSpec.
func (in *ClusterEndpointSpec) DeepCopy() *ClusterEndpointSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterEndpointSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSpec) DeepCopyInto(out *ClusterSpec) {
	*out = *in
	if in.NodeLabels != nil {
		in, out := &in.NodeLabels, &out.NodeLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSpec.
func (in *ClusterSpec) DeepCopy() *ClusterSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConfigFile) DeepCopyInto(out *ConfigFile) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigFile.
func (in *ConfigFile) DeepCopy() *ConfigFile {
	if in == nil {
		return nil
	}
	out := new(ConfigFile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContainerdConfig) DeepCopyInto(out *ContainerdConfig) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContainerdConfig.
func (in *ContainerdConfig) DeepCopy() *ContainerdConfig {
	if in == nil {
		return nil
	}
	out := new(ContainerdConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPU) DeepCopyInto(out *DPU) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPU.
func (in *DPU) DeepCopy() *DPU {
	if in == nil {
		return nil
	}
	out := new(DPU)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPU) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUCluster) DeepCopyInto(out *DPUCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUCluster.
func (in *DPUCluster) DeepCopy() *DPUCluster {
	if in == nil {
		return nil
	}
	out := new(DPUCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUClusterList) DeepCopyInto(out *DPUClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DPUCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUClusterList.
func (in *DPUClusterList) DeepCopy() *DPUClusterList {
	if in == nil {
		return nil
	}
	out := new(DPUClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUClusterSpec) DeepCopyInto(out *DPUClusterSpec) {
	*out = *in
	if in.ClusterEndpoint != nil {
		in, out := &in.ClusterEndpoint, &out.ClusterEndpoint
		*out = new(ClusterEndpointSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUClusterSpec.
func (in *DPUClusterSpec) DeepCopy() *DPUClusterSpec {
	if in == nil {
		return nil
	}
	out := new(DPUClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUClusterStatus) DeepCopyInto(out *DPUClusterStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUClusterStatus.
func (in *DPUClusterStatus) DeepCopy() *DPUClusterStatus {
	if in == nil {
		return nil
	}
	out := new(DPUClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUFLavorSysctl) DeepCopyInto(out *DPUFLavorSysctl) {
	*out = *in
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUFLavorSysctl.
func (in *DPUFLavorSysctl) DeepCopy() *DPUFLavorSysctl {
	if in == nil {
		return nil
	}
	out := new(DPUFLavorSysctl)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUFlavor) DeepCopyInto(out *DPUFlavor) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUFlavor.
func (in *DPUFlavor) DeepCopy() *DPUFlavor {
	if in == nil {
		return nil
	}
	out := new(DPUFlavor)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUFlavor) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUFlavorGrub) DeepCopyInto(out *DPUFlavorGrub) {
	*out = *in
	if in.KernelParameters != nil {
		in, out := &in.KernelParameters, &out.KernelParameters
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUFlavorGrub.
func (in *DPUFlavorGrub) DeepCopy() *DPUFlavorGrub {
	if in == nil {
		return nil
	}
	out := new(DPUFlavorGrub)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUFlavorList) DeepCopyInto(out *DPUFlavorList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DPUFlavor, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUFlavorList.
func (in *DPUFlavorList) DeepCopy() *DPUFlavorList {
	if in == nil {
		return nil
	}
	out := new(DPUFlavorList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUFlavorList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUFlavorNVConfig) DeepCopyInto(out *DPUFlavorNVConfig) {
	*out = *in
	if in.Device != nil {
		in, out := &in.Device, &out.Device
		*out = new(string)
		**out = **in
	}
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.HostPowerCycleRequired != nil {
		in, out := &in.HostPowerCycleRequired, &out.HostPowerCycleRequired
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUFlavorNVConfig.
func (in *DPUFlavorNVConfig) DeepCopy() *DPUFlavorNVConfig {
	if in == nil {
		return nil
	}
	out := new(DPUFlavorNVConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUFlavorOVS) DeepCopyInto(out *DPUFlavorOVS) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUFlavorOVS.
func (in *DPUFlavorOVS) DeepCopy() *DPUFlavorOVS {
	if in == nil {
		return nil
	}
	out := new(DPUFlavorOVS)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUFlavorSpec) DeepCopyInto(out *DPUFlavorSpec) {
	*out = *in
	in.Grub.DeepCopyInto(&out.Grub)
	in.Sysctl.DeepCopyInto(&out.Sysctl)
	if in.NVConfig != nil {
		in, out := &in.NVConfig, &out.NVConfig
		*out = make([]DPUFlavorNVConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.OVS = in.OVS
	if in.BFCfgParameters != nil {
		in, out := &in.BFCfgParameters, &out.BFCfgParameters
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ConfigFiles != nil {
		in, out := &in.ConfigFiles, &out.ConfigFiles
		*out = make([]ConfigFile, len(*in))
		copy(*out, *in)
	}
	out.ContainerdConfig = in.ContainerdConfig
	if in.DPUResources != nil {
		in, out := &in.DPUResources, &out.DPUResources
		*out = make(corev1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	if in.SystemReservedResources != nil {
		in, out := &in.SystemReservedResources, &out.SystemReservedResources
		*out = make(corev1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUFlavorSpec.
func (in *DPUFlavorSpec) DeepCopy() *DPUFlavorSpec {
	if in == nil {
		return nil
	}
	out := new(DPUFlavorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUList) DeepCopyInto(out *DPUList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DPU, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUList.
func (in *DPUList) DeepCopy() *DPUList {
	if in == nil {
		return nil
	}
	out := new(DPUList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUSet) DeepCopyInto(out *DPUSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUSet.
func (in *DPUSet) DeepCopy() *DPUSet {
	if in == nil {
		return nil
	}
	out := new(DPUSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUSetList) DeepCopyInto(out *DPUSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DPUSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUSetList.
func (in *DPUSetList) DeepCopy() *DPUSetList {
	if in == nil {
		return nil
	}
	out := new(DPUSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUSetSpec) DeepCopyInto(out *DPUSetSpec) {
	*out = *in
	if in.Strategy != nil {
		in, out := &in.Strategy, &out.Strategy
		*out = new(DPUSetStrategy)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.DPUSelector != nil {
		in, out := &in.DPUSelector, &out.DPUSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.DPUTemplate.DeepCopyInto(&out.DPUTemplate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUSetSpec.
func (in *DPUSetSpec) DeepCopy() *DPUSetSpec {
	if in == nil {
		return nil
	}
	out := new(DPUSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUSetStatus) DeepCopyInto(out *DPUSetStatus) {
	*out = *in
	if in.DPUStatistics != nil {
		in, out := &in.DPUStatistics, &out.DPUStatistics
		*out = make(map[DPUPhase]int, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUSetStatus.
func (in *DPUSetStatus) DeepCopy() *DPUSetStatus {
	if in == nil {
		return nil
	}
	out := new(DPUSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUSetStrategy) DeepCopyInto(out *DPUSetStrategy) {
	*out = *in
	if in.RollingUpdate != nil {
		in, out := &in.RollingUpdate, &out.RollingUpdate
		*out = new(RollingUpdateDPU)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUSetStrategy.
func (in *DPUSetStrategy) DeepCopy() *DPUSetStrategy {
	if in == nil {
		return nil
	}
	out := new(DPUSetStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUSpec) DeepCopyInto(out *DPUSpec) {
	*out = *in
	if in.NodeEffect != nil {
		in, out := &in.NodeEffect, &out.NodeEffect
		*out = new(NodeEffect)
		(*in).DeepCopyInto(*out)
	}
	in.Cluster.DeepCopyInto(&out.Cluster)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUSpec.
func (in *DPUSpec) DeepCopy() *DPUSpec {
	if in == nil {
		return nil
	}
	out := new(DPUSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUStatus) DeepCopyInto(out *DPUStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.RequiredReset != nil {
		in, out := &in.RequiredReset, &out.RequiredReset
		*out = new(bool)
		**out = **in
	}
	out.Firmware = in.Firmware
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUStatus.
func (in *DPUStatus) DeepCopy() *DPUStatus {
	if in == nil {
		return nil
	}
	out := new(DPUStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUTemplate) DeepCopyInto(out *DPUTemplate) {
	*out = *in
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUTemplate.
func (in *DPUTemplate) DeepCopy() *DPUTemplate {
	if in == nil {
		return nil
	}
	out := new(DPUTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUTemplateSpec) DeepCopyInto(out *DPUTemplateSpec) {
	*out = *in
	out.BFB = in.BFB
	if in.NodeEffect != nil {
		in, out := &in.NodeEffect, &out.NodeEffect
		*out = new(NodeEffect)
		(*in).DeepCopyInto(*out)
	}
	if in.Cluster != nil {
		in, out := &in.Cluster, &out.Cluster
		*out = new(ClusterSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.AutomaticNodeReboot != nil {
		in, out := &in.AutomaticNodeReboot, &out.AutomaticNodeReboot
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUTemplateSpec.
func (in *DPUTemplateSpec) DeepCopy() *DPUTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(DPUTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Drain) DeepCopyInto(out *Drain) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Drain.
func (in *Drain) DeepCopy() *Drain {
	if in == nil {
		return nil
	}
	out := new(Drain)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Firmware) DeepCopyInto(out *Firmware) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Firmware.
func (in *Firmware) DeepCopy() *Firmware {
	if in == nil {
		return nil
	}
	out := new(Firmware)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *K8sCluster) DeepCopyInto(out *K8sCluster) {
	*out = *in
	if in.NodeLabels != nil {
		in, out := &in.NodeLabels, &out.NodeLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new K8sCluster.
func (in *K8sCluster) DeepCopy() *K8sCluster {
	if in == nil {
		return nil
	}
	out := new(K8sCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KeepalivedSpec) DeepCopyInto(out *KeepalivedSpec) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KeepalivedSpec.
func (in *KeepalivedSpec) DeepCopy() *KeepalivedSpec {
	if in == nil {
		return nil
	}
	out := new(KeepalivedSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeEffect) DeepCopyInto(out *NodeEffect) {
	*out = *in
	if in.Taint != nil {
		in, out := &in.Taint, &out.Taint
		*out = new(corev1.Taint)
		(*in).DeepCopyInto(*out)
	}
	if in.CustomLabel != nil {
		in, out := &in.CustomLabel, &out.CustomLabel
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Drain != nil {
		in, out := &in.Drain, &out.Drain
		*out = new(Drain)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeEffect.
func (in *NodeEffect) DeepCopy() *NodeEffect {
	if in == nil {
		return nil
	}
	out := new(NodeEffect)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RollingUpdateDPU) DeepCopyInto(out *RollingUpdateDPU) {
	*out = *in
	if in.MaxUnavailable != nil {
		in, out := &in.MaxUnavailable, &out.MaxUnavailable
		*out = new(intstr.IntOrString)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RollingUpdateDPU.
func (in *RollingUpdateDPU) DeepCopy() *RollingUpdateDPU {
	if in == nil {
		return nil
	}
	out := new(RollingUpdateDPU)
	in.DeepCopyInto(out)
	return out
}
