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
	servicechainv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApplicationSource) DeepCopyInto(out *ApplicationSource) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApplicationSource.
func (in *ApplicationSource) DeepCopy() *ApplicationSource {
	if in == nil {
		return nil
	}
	out := new(ApplicationSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUDeployment) DeepCopyInto(out *DPUDeployment) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUDeployment.
func (in *DPUDeployment) DeepCopy() *DPUDeployment {
	if in == nil {
		return nil
	}
	out := new(DPUDeployment)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUDeployment) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUDeploymentList) DeepCopyInto(out *DPUDeploymentList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DPUDeployment, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUDeploymentList.
func (in *DPUDeploymentList) DeepCopy() *DPUDeploymentList {
	if in == nil {
		return nil
	}
	out := new(DPUDeploymentList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUDeploymentList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUDeploymentServiceConfiguration) DeepCopyInto(out *DPUDeploymentServiceConfiguration) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUDeploymentServiceConfiguration.
func (in *DPUDeploymentServiceConfiguration) DeepCopy() *DPUDeploymentServiceConfiguration {
	if in == nil {
		return nil
	}
	out := new(DPUDeploymentServiceConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUDeploymentSpec) DeepCopyInto(out *DPUDeploymentSpec) {
	*out = *in
	in.DPUs.DeepCopyInto(&out.DPUs)
	if in.Services != nil {
		in, out := &in.Services, &out.Services
		*out = make(map[string]DPUDeploymentServiceConfiguration, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.ServiceChains != nil {
		in, out := &in.ServiceChains, &out.ServiceChains
		*out = make([]servicechainv1alpha1.Switch, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Strategy.DeepCopyInto(&out.Strategy)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUDeploymentSpec.
func (in *DPUDeploymentSpec) DeepCopy() *DPUDeploymentSpec {
	if in == nil {
		return nil
	}
	out := new(DPUDeploymentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUDeploymentStatus) DeepCopyInto(out *DPUDeploymentStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUDeploymentStatus.
func (in *DPUDeploymentStatus) DeepCopy() *DPUDeploymentStatus {
	if in == nil {
		return nil
	}
	out := new(DPUDeploymentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUService) DeepCopyInto(out *DPUService) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUService.
func (in *DPUService) DeepCopy() *DPUService {
	if in == nil {
		return nil
	}
	out := new(DPUService)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUService) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceConfiguration) DeepCopyInto(out *DPUServiceConfiguration) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceConfiguration.
func (in *DPUServiceConfiguration) DeepCopy() *DPUServiceConfiguration {
	if in == nil {
		return nil
	}
	out := new(DPUServiceConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUServiceConfiguration) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceConfigurationList) DeepCopyInto(out *DPUServiceConfigurationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DPUServiceConfiguration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceConfigurationList.
func (in *DPUServiceConfigurationList) DeepCopy() *DPUServiceConfigurationList {
	if in == nil {
		return nil
	}
	out := new(DPUServiceConfigurationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUServiceConfigurationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceConfigurationServiceDaemonSetValues) DeepCopyInto(out *DPUServiceConfigurationServiceDaemonSetValues) {
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
	if in.UpdateStrategy != nil {
		in, out := &in.UpdateStrategy, &out.UpdateStrategy
		*out = new(appsv1.DaemonSetUpdateStrategy)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceConfigurationServiceDaemonSetValues.
func (in *DPUServiceConfigurationServiceDaemonSetValues) DeepCopy() *DPUServiceConfigurationServiceDaemonSetValues {
	if in == nil {
		return nil
	}
	out := new(DPUServiceConfigurationServiceDaemonSetValues)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceConfigurationSpec) DeepCopyInto(out *DPUServiceConfigurationSpec) {
	*out = *in
	in.ServiceConfiguration.DeepCopyInto(&out.ServiceConfiguration)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceConfigurationSpec.
func (in *DPUServiceConfigurationSpec) DeepCopy() *DPUServiceConfigurationSpec {
	if in == nil {
		return nil
	}
	out := new(DPUServiceConfigurationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceConfigurationStatus) DeepCopyInto(out *DPUServiceConfigurationStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceConfigurationStatus.
func (in *DPUServiceConfigurationStatus) DeepCopy() *DPUServiceConfigurationStatus {
	if in == nil {
		return nil
	}
	out := new(DPUServiceConfigurationStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceList) DeepCopyInto(out *DPUServiceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DPUService, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceList.
func (in *DPUServiceList) DeepCopy() *DPUServiceList {
	if in == nil {
		return nil
	}
	out := new(DPUServiceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUServiceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceSpec) DeepCopyInto(out *DPUServiceSpec) {
	*out = *in
	in.HelmChart.DeepCopyInto(&out.HelmChart)
	if in.ServiceID != nil {
		in, out := &in.ServiceID, &out.ServiceID
		*out = new(string)
		**out = **in
	}
	if in.ServiceDaemonSet != nil {
		in, out := &in.ServiceDaemonSet, &out.ServiceDaemonSet
		*out = new(ServiceDaemonSetValues)
		(*in).DeepCopyInto(*out)
	}
	if in.DeployInCluster != nil {
		in, out := &in.DeployInCluster, &out.DeployInCluster
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceSpec.
func (in *DPUServiceSpec) DeepCopy() *DPUServiceSpec {
	if in == nil {
		return nil
	}
	out := new(DPUServiceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceStatus) DeepCopyInto(out *DPUServiceStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceStatus.
func (in *DPUServiceStatus) DeepCopy() *DPUServiceStatus {
	if in == nil {
		return nil
	}
	out := new(DPUServiceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceTemplate) DeepCopyInto(out *DPUServiceTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceTemplate.
func (in *DPUServiceTemplate) DeepCopy() *DPUServiceTemplate {
	if in == nil {
		return nil
	}
	out := new(DPUServiceTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUServiceTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceTemplateList) DeepCopyInto(out *DPUServiceTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]DPUServiceTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceTemplateList.
func (in *DPUServiceTemplateList) DeepCopy() *DPUServiceTemplateList {
	if in == nil {
		return nil
	}
	out := new(DPUServiceTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DPUServiceTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceTemplateServiceDaemonSetValues) DeepCopyInto(out *DPUServiceTemplateServiceDaemonSetValues) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make(corev1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceTemplateServiceDaemonSetValues.
func (in *DPUServiceTemplateServiceDaemonSetValues) DeepCopy() *DPUServiceTemplateServiceDaemonSetValues {
	if in == nil {
		return nil
	}
	out := new(DPUServiceTemplateServiceDaemonSetValues)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceTemplateSpec) DeepCopyInto(out *DPUServiceTemplateSpec) {
	*out = *in
	in.ServiceDaemonSet.DeepCopyInto(&out.ServiceDaemonSet)
	if in.ResourceRequirements != nil {
		in, out := &in.ResourceRequirements, &out.ResourceRequirements
		*out = make(corev1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceTemplateSpec.
func (in *DPUServiceTemplateSpec) DeepCopy() *DPUServiceTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(DPUServiceTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUServiceTemplateStatus) DeepCopyInto(out *DPUServiceTemplateStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUServiceTemplateStatus.
func (in *DPUServiceTemplateStatus) DeepCopy() *DPUServiceTemplateStatus {
	if in == nil {
		return nil
	}
	out := new(DPUServiceTemplateStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUSet) DeepCopyInto(out *DPUSet) {
	*out = *in
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
	if in.DPUAnnotations != nil {
		in, out := &in.DPUAnnotations, &out.DPUAnnotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
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

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DPUs) DeepCopyInto(out *DPUs) {
	*out = *in
	if in.DPUSets != nil {
		in, out := &in.DPUSets, &out.DPUSets
		*out = make([]DPUSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DPUs.
func (in *DPUs) DeepCopy() *DPUs {
	if in == nil {
		return nil
	}
	out := new(DPUs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HelmChart) DeepCopyInto(out *HelmChart) {
	*out = *in
	out.Source = in.Source
	if in.Values != nil {
		in, out := &in.Values, &out.Values
		*out = new(runtime.RawExtension)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HelmChart.
func (in *HelmChart) DeepCopy() *HelmChart {
	if in == nil {
		return nil
	}
	out := new(HelmChart)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceConfiguration) DeepCopyInto(out *ServiceConfiguration) {
	*out = *in
	in.HelmChart.DeepCopyInto(&out.HelmChart)
	in.ServiceDaemonSet.DeepCopyInto(&out.ServiceDaemonSet)
	if in.DeployInCluster != nil {
		in, out := &in.DeployInCluster, &out.DeployInCluster
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceConfiguration.
func (in *ServiceConfiguration) DeepCopy() *ServiceConfiguration {
	if in == nil {
		return nil
	}
	out := new(ServiceConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceDaemonSetValues) DeepCopyInto(out *ServiceDaemonSetValues) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = new(corev1.NodeSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make(corev1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	if in.UpdateStrategy != nil {
		in, out := &in.UpdateStrategy, &out.UpdateStrategy
		*out = new(appsv1.DaemonSetUpdateStrategy)
		(*in).DeepCopyInto(*out)
	}
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

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceDaemonSetValues.
func (in *ServiceDaemonSetValues) DeepCopy() *ServiceDaemonSetValues {
	if in == nil {
		return nil
	}
	out := new(ServiceDaemonSetValues)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Strategy) DeepCopyInto(out *Strategy) {
	*out = *in
	in.RollingUpdate.DeepCopyInto(&out.RollingUpdate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Strategy.
func (in *Strategy) DeepCopy() *Strategy {
	if in == nil {
		return nil
	}
	out := new(Strategy)
	in.DeepCopyInto(out)
	return out
}
