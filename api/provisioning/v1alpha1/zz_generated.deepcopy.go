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
	"k8s.io/apimachinery/pkg/runtime"
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
	if in.DPUDeploymentResources != nil {
		in, out := &in.DPUDeploymentResources, &out.DPUDeploymentResources
		*out = make(corev1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	if in.ResourceRequirements != nil {
		in, out := &in.ResourceRequirements, &out.ResourceRequirements
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
func (in *DPUSpec) DeepCopyInto(out *DPUSpec) {
	*out = *in
	out.BFB = in.BFB
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
func (in *Dpu) DeepCopyInto(out *Dpu) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Dpu.
func (in *Dpu) DeepCopy() *Dpu {
	if in == nil {
		return nil
	}
	out := new(Dpu)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Dpu) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DpuList) DeepCopyInto(out *DpuList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Dpu, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DpuList.
func (in *DpuList) DeepCopy() *DpuList {
	if in == nil {
		return nil
	}
	out := new(DpuList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DpuList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DpuSet) DeepCopyInto(out *DpuSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DpuSet.
func (in *DpuSet) DeepCopy() *DpuSet {
	if in == nil {
		return nil
	}
	out := new(DpuSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DpuSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DpuSetList) DeepCopyInto(out *DpuSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DpuSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DpuSetList.
func (in *DpuSetList) DeepCopy() *DpuSetList {
	if in == nil {
		return nil
	}
	out := new(DpuSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DpuSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DpuSetSpec) DeepCopyInto(out *DpuSetSpec) {
	*out = *in
	if in.Strategy != nil {
		in, out := &in.Strategy, &out.Strategy
		*out = new(DpuSetStrategy)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.DpuSelector != nil {
		in, out := &in.DpuSelector, &out.DpuSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.DpuTemplate.DeepCopyInto(&out.DpuTemplate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DpuSetSpec.
func (in *DpuSetSpec) DeepCopy() *DpuSetSpec {
	if in == nil {
		return nil
	}
	out := new(DpuSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DpuSetStatus) DeepCopyInto(out *DpuSetStatus) {
	*out = *in
	if in.Dpustatistics != nil {
		in, out := &in.Dpustatistics, &out.Dpustatistics
		*out = make(map[DpuPhase]int, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DpuSetStatus.
func (in *DpuSetStatus) DeepCopy() *DpuSetStatus {
	if in == nil {
		return nil
	}
	out := new(DpuSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DpuSetStrategy) DeepCopyInto(out *DpuSetStrategy) {
	*out = *in
	if in.RollingUpdate != nil {
		in, out := &in.RollingUpdate, &out.RollingUpdate
		*out = new(RollingUpdateDpu)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DpuSetStrategy.
func (in *DpuSetStrategy) DeepCopy() *DpuSetStrategy {
	if in == nil {
		return nil
	}
	out := new(DpuSetStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DpuSpec) DeepCopyInto(out *DpuSpec) {
	*out = *in
	if in.NodeEffect != nil {
		in, out := &in.NodeEffect, &out.NodeEffect
		*out = new(NodeEffect)
		(*in).DeepCopyInto(*out)
	}
	in.Cluster.DeepCopyInto(&out.Cluster)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DpuSpec.
func (in *DpuSpec) DeepCopy() *DpuSpec {
	if in == nil {
		return nil
	}
	out := new(DpuSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DpuStatus) DeepCopyInto(out *DpuStatus) {
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

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DpuStatus.
func (in *DpuStatus) DeepCopy() *DpuStatus {
	if in == nil {
		return nil
	}
	out := new(DpuStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DpuTemplate) DeepCopyInto(out *DpuTemplate) {
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

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DpuTemplate.
func (in *DpuTemplate) DeepCopy() *DpuTemplate {
	if in == nil {
		return nil
	}
	out := new(DpuTemplate)
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
func (in *RollingUpdateDpu) DeepCopyInto(out *RollingUpdateDpu) {
	*out = *in
	if in.MaxUnavailable != nil {
		in, out := &in.MaxUnavailable, &out.MaxUnavailable
		*out = new(intstr.IntOrString)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RollingUpdateDpu.
func (in *RollingUpdateDpu) DeepCopy() *RollingUpdateDpu {
	if in == nil {
		return nil
	}
	out := new(RollingUpdateDpu)
	in.DeepCopyInto(out)
	return out
}
