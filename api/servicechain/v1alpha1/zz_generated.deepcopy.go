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
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceChain) DeepCopyInto(out *DPUServiceChain) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceChain.
func (in *DPUServiceChain) DeepCopy() *DPUServiceChain {
	if in == nil {
		return nil
	}
	out := new(DPUServiceChain)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUServiceChain) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceChainList) DeepCopyInto(out *DPUServiceChainList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DPUServiceChain, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceChainList.
func (in *DPUServiceChainList) DeepCopy() *DPUServiceChainList {
	if in == nil {
		return nil
	}
	out := new(DPUServiceChainList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUServiceChainList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceChainSpec) DeepCopyInto(out *DPUServiceChainSpec) {
	*out = *in
	if in.ClusterSelector != nil {
		in, out := &in.ClusterSelector, &out.ClusterSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceChainSpec.
func (in *DPUServiceChainSpec) DeepCopy() *DPUServiceChainSpec {
	if in == nil {
		return nil
	}
	out := new(DPUServiceChainSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceChainStatus) DeepCopyInto(out *DPUServiceChainStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceChainStatus.
func (in *DPUServiceChainStatus) DeepCopy() *DPUServiceChainStatus {
	if in == nil {
		return nil
	}
	out := new(DPUServiceChainStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceIPAM) DeepCopyInto(out *DPUServiceIPAM) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceIPAM.
func (in *DPUServiceIPAM) DeepCopy() *DPUServiceIPAM {
	if in == nil {
		return nil
	}
	out := new(DPUServiceIPAM)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUServiceIPAM) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceIPAMList) DeepCopyInto(out *DPUServiceIPAMList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DPUServiceIPAM, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceIPAMList.
func (in *DPUServiceIPAMList) DeepCopy() *DPUServiceIPAMList {
	if in == nil {
		return nil
	}
	out := new(DPUServiceIPAMList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUServiceIPAMList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceIPAMSpec) DeepCopyInto(out *DPUServiceIPAMSpec) {
	*out = *in
	if in.IPV4Network != nil {
		in, out := &in.IPV4Network, &out.IPV4Network
		*out = new(IPV4Network)
		(*in).DeepCopyInto(*out)
	}
	if in.IPV4Subnet != nil {
		in, out := &in.IPV4Subnet, &out.IPV4Subnet
		*out = new(IPV4Subnet)
		(*in).DeepCopyInto(*out)
	}
	if in.ClusterSelector != nil {
		in, out := &in.ClusterSelector, &out.ClusterSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = new(corev1.NodeSelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceIPAMSpec.
func (in *DPUServiceIPAMSpec) DeepCopy() *DPUServiceIPAMSpec {
	if in == nil {
		return nil
	}
	out := new(DPUServiceIPAMSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceIPAMStatus) DeepCopyInto(out *DPUServiceIPAMStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceIPAMStatus.
func (in *DPUServiceIPAMStatus) DeepCopy() *DPUServiceIPAMStatus {
	if in == nil {
		return nil
	}
	out := new(DPUServiceIPAMStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceInterface) DeepCopyInto(out *DPUServiceInterface) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceInterface.
func (in *DPUServiceInterface) DeepCopy() *DPUServiceInterface {
	if in == nil {
		return nil
	}
	out := new(DPUServiceInterface)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUServiceInterface) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceInterfaceList) DeepCopyInto(out *DPUServiceInterfaceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DPUServiceInterface, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceInterfaceList.
func (in *DPUServiceInterfaceList) DeepCopy() *DPUServiceInterfaceList {
	if in == nil {
		return nil
	}
	out := new(DPUServiceInterfaceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUServiceInterfaceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceInterfaceSpec) DeepCopyInto(out *DPUServiceInterfaceSpec) {
	*out = *in
	if in.ClusterSelector != nil {
		in, out := &in.ClusterSelector, &out.ClusterSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceInterfaceSpec.
func (in *DPUServiceInterfaceSpec) DeepCopy() *DPUServiceInterfaceSpec {
	if in == nil {
		return nil
	}
	out := new(DPUServiceInterfaceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceInterfaceStatus) DeepCopyInto(out *DPUServiceInterfaceStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceInterfaceStatus.
func (in *DPUServiceInterfaceStatus) DeepCopy() *DPUServiceInterfaceStatus {
	if in == nil {
		return nil
	}
	out := new(DPUServiceInterfaceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IPAM) DeepCopyInto(out *IPAM) {
	*out = *in
	if in.Reference != nil {
		in, out := &in.Reference, &out.Reference
		*out = new(ObjectRef)
		(*in).DeepCopyInto(*out)
	}
	if in.MatchLabels != nil {
		in, out := &in.MatchLabels, &out.MatchLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.DefaultGateway != nil {
		in, out := &in.DefaultGateway, &out.DefaultGateway
		*out = new(bool)
		**out = **in
	}
	if in.SetDefaultRoute != nil {
		in, out := &in.SetDefaultRoute, &out.SetDefaultRoute
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IPAM.
func (in *IPAM) DeepCopy() *IPAM {
	if in == nil {
		return nil
	}
	out := new(IPAM)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IPV4Network) DeepCopyInto(out *IPV4Network) {
	*out = *in
	if in.Exclusions != nil {
		in, out := &in.Exclusions, &out.Exclusions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Allocations != nil {
		in, out := &in.Allocations, &out.Allocations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Routes != nil {
		in, out := &in.Routes, &out.Routes
		*out = make([]Route, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IPV4Network.
func (in *IPV4Network) DeepCopy() *IPV4Network {
	if in == nil {
		return nil
	}
	out := new(IPV4Network)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IPV4Subnet) DeepCopyInto(out *IPV4Subnet) {
	*out = *in
	if in.Routes != nil {
		in, out := &in.Routes, &out.Routes
		*out = make([]Route, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IPV4Subnet.
func (in *IPV4Subnet) DeepCopy() *IPV4Subnet {
	if in == nil {
		return nil
	}
	out := new(IPV4Subnet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ObjectMeta) DeepCopyInto(out *ObjectMeta) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ObjectMeta.
func (in *ObjectMeta) DeepCopy() *ObjectMeta {
	if in == nil {
		return nil
	}
	out := new(ObjectMeta)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ObjectRef) DeepCopyInto(out *ObjectRef) {
	*out = *in
	if in.Namespace != nil {
		in, out := &in.Namespace, &out.Namespace
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ObjectRef.
func (in *ObjectRef) DeepCopy() *ObjectRef {
	if in == nil {
		return nil
	}
	out := new(ObjectRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PF) DeepCopyInto(out *PF) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PF.
func (in *PF) DeepCopy() *PF {
	if in == nil {
		return nil
	}
	out := new(PF)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Port) DeepCopyInto(out *Port) {
	*out = *in
	if in.Service != nil {
		in, out := &in.Service, &out.Service
		*out = new(Service)
		(*in).DeepCopyInto(*out)
	}
	if in.ServiceInterface != nil {
		in, out := &in.ServiceInterface, &out.ServiceInterface
		*out = new(ServiceIfc)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Port.
func (in *Port) DeepCopy() *Port {
	if in == nil {
		return nil
	}
	out := new(Port)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Route) DeepCopyInto(out *Route) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Route.
func (in *Route) DeepCopy() *Route {
	if in == nil {
		return nil
	}
	out := new(Route)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Service) DeepCopyInto(out *Service) {
	*out = *in
	if in.Reference != nil {
		in, out := &in.Reference, &out.Reference
		*out = new(ObjectRef)
		(*in).DeepCopyInto(*out)
	}
	if in.MatchLabels != nil {
		in, out := &in.MatchLabels, &out.MatchLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.IPAM != nil {
		in, out := &in.IPAM, &out.IPAM
		*out = new(IPAM)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Service.
func (in *Service) DeepCopy() *Service {
	if in == nil {
		return nil
	}
	out := new(Service)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceChain) DeepCopyInto(out *ServiceChain) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceChain.
func (in *ServiceChain) DeepCopy() *ServiceChain {
	if in == nil {
		return nil
	}
	out := new(ServiceChain)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceChain) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceChainList) DeepCopyInto(out *ServiceChainList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ServiceChain, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceChainList.
func (in *ServiceChainList) DeepCopy() *ServiceChainList {
	if in == nil {
		return nil
	}
	out := new(ServiceChainList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceChainList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceChainSet) DeepCopyInto(out *ServiceChainSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceChainSet.
func (in *ServiceChainSet) DeepCopy() *ServiceChainSet {
	if in == nil {
		return nil
	}
	out := new(ServiceChainSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceChainSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceChainSetList) DeepCopyInto(out *ServiceChainSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ServiceChainSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceChainSetList.
func (in *ServiceChainSetList) DeepCopy() *ServiceChainSetList {
	if in == nil {
		return nil
	}
	out := new(ServiceChainSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceChainSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceChainSetSpec) DeepCopyInto(out *ServiceChainSetSpec) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceChainSetSpec.
func (in *ServiceChainSetSpec) DeepCopy() *ServiceChainSetSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceChainSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceChainSetSpecTemplate) DeepCopyInto(out *ServiceChainSetSpecTemplate) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceChainSetSpecTemplate.
func (in *ServiceChainSetSpecTemplate) DeepCopy() *ServiceChainSetSpecTemplate {
	if in == nil {
		return nil
	}
	out := new(ServiceChainSetSpecTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceChainSetStatus) DeepCopyInto(out *ServiceChainSetStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceChainSetStatus.
func (in *ServiceChainSetStatus) DeepCopy() *ServiceChainSetStatus {
	if in == nil {
		return nil
	}
	out := new(ServiceChainSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceChainSpec) DeepCopyInto(out *ServiceChainSpec) {
	*out = *in
	if in.Node != nil {
		in, out := &in.Node, &out.Node
		*out = new(string)
		**out = **in
	}
	if in.Switches != nil {
		in, out := &in.Switches, &out.Switches
		*out = make([]Switch, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceChainSpec.
func (in *ServiceChainSpec) DeepCopy() *ServiceChainSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceChainSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceChainSpecTemplate) DeepCopyInto(out *ServiceChainSpecTemplate) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceChainSpecTemplate.
func (in *ServiceChainSpecTemplate) DeepCopy() *ServiceChainSpecTemplate {
	if in == nil {
		return nil
	}
	out := new(ServiceChainSpecTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceChainStatus) DeepCopyInto(out *ServiceChainStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceChainStatus.
func (in *ServiceChainStatus) DeepCopy() *ServiceChainStatus {
	if in == nil {
		return nil
	}
	out := new(ServiceChainStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceDef) DeepCopyInto(out *ServiceDef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceDef.
func (in *ServiceDef) DeepCopy() *ServiceDef {
	if in == nil {
		return nil
	}
	out := new(ServiceDef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceIfc) DeepCopyInto(out *ServiceIfc) {
	*out = *in
	if in.Reference != nil {
		in, out := &in.Reference, &out.Reference
		*out = new(ObjectRef)
		(*in).DeepCopyInto(*out)
	}
	if in.MatchLabels != nil {
		in, out := &in.MatchLabels, &out.MatchLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceIfc.
func (in *ServiceIfc) DeepCopy() *ServiceIfc {
	if in == nil {
		return nil
	}
	out := new(ServiceIfc)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceInterface) DeepCopyInto(out *ServiceInterface) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceInterface.
func (in *ServiceInterface) DeepCopy() *ServiceInterface {
	if in == nil {
		return nil
	}
	out := new(ServiceInterface)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceInterface) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceInterfaceList) DeepCopyInto(out *ServiceInterfaceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ServiceInterface, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceInterfaceList.
func (in *ServiceInterfaceList) DeepCopy() *ServiceInterfaceList {
	if in == nil {
		return nil
	}
	out := new(ServiceInterfaceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceInterfaceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceInterfaceSet) DeepCopyInto(out *ServiceInterfaceSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceInterfaceSet.
func (in *ServiceInterfaceSet) DeepCopy() *ServiceInterfaceSet {
	if in == nil {
		return nil
	}
	out := new(ServiceInterfaceSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceInterfaceSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceInterfaceSetList) DeepCopyInto(out *ServiceInterfaceSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ServiceInterfaceSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceInterfaceSetList.
func (in *ServiceInterfaceSetList) DeepCopy() *ServiceInterfaceSetList {
	if in == nil {
		return nil
	}
	out := new(ServiceInterfaceSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceInterfaceSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceInterfaceSetSpec) DeepCopyInto(out *ServiceInterfaceSetSpec) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceInterfaceSetSpec.
func (in *ServiceInterfaceSetSpec) DeepCopy() *ServiceInterfaceSetSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceInterfaceSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceInterfaceSetSpecTemplate) DeepCopyInto(out *ServiceInterfaceSetSpecTemplate) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceInterfaceSetSpecTemplate.
func (in *ServiceInterfaceSetSpecTemplate) DeepCopy() *ServiceInterfaceSetSpecTemplate {
	if in == nil {
		return nil
	}
	out := new(ServiceInterfaceSetSpecTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceInterfaceSetStatus) DeepCopyInto(out *ServiceInterfaceSetStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceInterfaceSetStatus.
func (in *ServiceInterfaceSetStatus) DeepCopy() *ServiceInterfaceSetStatus {
	if in == nil {
		return nil
	}
	out := new(ServiceInterfaceSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceInterfaceSpec) DeepCopyInto(out *ServiceInterfaceSpec) {
	*out = *in
	if in.Node != nil {
		in, out := &in.Node, &out.Node
		*out = new(string)
		**out = **in
	}
	if in.InterfaceName != nil {
		in, out := &in.InterfaceName, &out.InterfaceName
		*out = new(string)
		**out = **in
	}
	if in.Vlan != nil {
		in, out := &in.Vlan, &out.Vlan
		*out = new(VLAN)
		**out = **in
	}
	if in.VF != nil {
		in, out := &in.VF, &out.VF
		*out = new(VF)
		**out = **in
	}
	if in.PF != nil {
		in, out := &in.PF, &out.PF
		*out = new(PF)
		**out = **in
	}
	if in.Service != nil {
		in, out := &in.Service, &out.Service
		*out = new(ServiceDef)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceInterfaceSpec.
func (in *ServiceInterfaceSpec) DeepCopy() *ServiceInterfaceSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceInterfaceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceInterfaceSpecTemplate) DeepCopyInto(out *ServiceInterfaceSpecTemplate) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceInterfaceSpecTemplate.
func (in *ServiceInterfaceSpecTemplate) DeepCopy() *ServiceInterfaceSpecTemplate {
	if in == nil {
		return nil
	}
	out := new(ServiceInterfaceSpecTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceInterfaceStatus) DeepCopyInto(out *ServiceInterfaceStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceInterfaceStatus.
func (in *ServiceInterfaceStatus) DeepCopy() *ServiceInterfaceStatus {
	if in == nil {
		return nil
	}
	out := new(ServiceInterfaceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Switch) DeepCopyInto(out *Switch) {
	*out = *in
	if in.Ports != nil {
		in, out := &in.Ports, &out.Ports
		*out = make([]Port, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Switch.
func (in *Switch) DeepCopy() *Switch {
	if in == nil {
		return nil
	}
	out := new(Switch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VF) DeepCopyInto(out *VF) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VF.
func (in *VF) DeepCopy() *VF {
	if in == nil {
		return nil
	}
	out := new(VF)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VLAN) DeepCopyInto(out *VLAN) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VLAN.
func (in *VLAN) DeepCopy() *VLAN {
	if in == nil {
		return nil
	}
	out := new(VLAN)
	in.DeepCopyInto(out)
	return out
}
