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
func (in *IPAM) DeepCopyInto(out *IPAM) {
	*out = *in
	out.Reference = in.Reference
	if in.MatchLabels != nil {
		in, out := &in.MatchLabels, &out.MatchLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
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
func (in *ObjectRef) DeepCopyInto(out *ObjectRef) {
	*out = *in
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
	in.Service.DeepCopyInto(&out.Service)
	in.ServiceInterface.DeepCopyInto(&out.ServiceInterface)
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
func (in *Service) DeepCopyInto(out *Service) {
	*out = *in
	out.Reference = in.Reference
	if in.MatchLabels != nil {
		in, out := &in.MatchLabels, &out.MatchLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.IPAM.DeepCopyInto(&out.IPAM)
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
func (in *ServiceIfc) DeepCopyInto(out *ServiceIfc) {
	*out = *in
	out.Reference = in.Reference
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
