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

package inventory

import (
	"fmt"

	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Component = &dpuServiceControllerObjects{}

// dpuServiceControllerObjects contains Kubernetes objects to be created by the DPUService controller.
type dpuServiceControllerObjects struct {
	data               []byte
	Deployment         *appsv1.Deployment
	ServiceAccount     *corev1.ServiceAccount
	Role               *rbacv1.Role
	ClusterRole        *rbacv1.ClusterRole
	ClusterRoleBinding *rbacv1.ClusterRoleBinding
	RoleBinding        *rbacv1.RoleBinding
}

func (d *dpuServiceControllerObjects) Name() string {
	return "DPUServiceController"
}

// Parse returns typed objects for the DPUService controller deployment.
func (d *dpuServiceControllerObjects) Parse() error {
	if d.data == nil {
		return fmt.Errorf("dpuServiceControllerObjects.data can not be empty")
	}
	dpuServiceObjects, err := utils.BytesToUnstructured(d.data)
	if err != nil {
		return fmt.Errorf("error while converting DPUService  manifests to objects: %w", err)
	}
	for _, obj := range dpuServiceObjects {
		switch obj.GetObjectKind().GroupVersionKind().Kind {
		case "Deployment":
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &d.Deployment); err != nil {
				return fmt.Errorf("error while converting DPUService to objects: %w", err)
			}
		case "ServiceAccount":
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &d.ServiceAccount); err != nil {
				return fmt.Errorf("error while converting DPUService to objects: %w", err)
			}
		case "Role":
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &d.Role); err != nil {
				return fmt.Errorf("error while converting DPUService to objects: %w", err)
			}
		case "RoleBinding":
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &d.RoleBinding); err != nil {
				return fmt.Errorf("error while converting DPUService to objects: %w", err)
			}
		case "ClusterRole":
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &d.ClusterRole); err != nil {
				return fmt.Errorf("error while converting DPUService to objects: %w", err)
			}
		case "ClusterRoleBinding":
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &d.ClusterRoleBinding); err != nil {
				return fmt.Errorf("error while converting DPUService to objects: %w", err)
			}
			// Namespace and CustomResourceDefinition are dropped as they are handled elsewhere.
		case "Namespace":
			// Drop namespace
		case "CustomResourceDefinition":
			// Drop CustomResourceDefinition
		default:
			// Error is any unexpected type is found in the manifest.
			return fmt.Errorf("unrecognized kind in DPUService objects: %s", obj.GetObjectKind().GroupVersionKind().Kind)
		}
	}
	return d.validate()
}

// validate asserts the dpuServiceControllerObjects are as expected.
func (d *dpuServiceControllerObjects) validate() error {
	// Check that each and every field expected is set.
	if d.Role == nil {
		return fmt.Errorf("error parsing DPUService objects: Role not found")
	}
	if d.ClusterRole == nil {
		return fmt.Errorf("error parsing DPUService objects: ClusterRole not found")
	}
	if d.ClusterRoleBinding == nil {
		return fmt.Errorf("error parsing DPUService objects: ClusterRoleBinding not found")
	}
	if d.Deployment == nil {
		return fmt.Errorf("error parsing DPUService objects: Deployment not found")
	}
	if d.ServiceAccount == nil {
		return fmt.Errorf("error parsing DPUService objects:  ServiceAccount not found")
	}
	if d.RoleBinding == nil {
		return fmt.Errorf("error parsing DPUService objects: RoleBinding not found")
	}
	return nil
}

// GenerateManifests returns all objects as a list.
func (d *dpuServiceControllerObjects) GenerateManifests(variables Variables) ([]client.Object, error) {
	d.setNamespace(variables.Namespace)
	out := []client.Object{}
	out = append(out,
		d.Deployment.DeepCopy(),
		d.ServiceAccount.DeepCopy(),
		d.Role.DeepCopy(),
		d.RoleBinding.DeepCopy(),
		d.ClusterRole.DeepCopy(),
		d.ClusterRoleBinding.DeepCopy(),
	)
	return out, nil
}

// SetNamespace sets all Namespaces in the dpuServiceControllerObjects to the passed string.
func (d *dpuServiceControllerObjects) setNamespace(namespace string) {
	d.Deployment.SetNamespace(namespace)
	d.ServiceAccount.SetNamespace(namespace)
	d.Role.SetNamespace(namespace)
	d.RoleBinding.SetNamespace(namespace)

	// Namespace is also defined in the RoleBinding and ClusterRoleBinding subjects.
	for i := range d.RoleBinding.Subjects {
		d.RoleBinding.Subjects[i].Namespace = namespace
	}
	for i := range d.ClusterRoleBinding.Subjects {
		d.ClusterRoleBinding.Subjects[i].Namespace = namespace
	}
}
