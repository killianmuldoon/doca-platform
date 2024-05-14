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

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func setNamespace(namespace string, objects []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	output := make([]unstructured.Unstructured, len(objects))
	for i, obj := range objects {
		obj.SetNamespace(namespace)
		switch obj.GetKind() {
		case "RoleBinding":
			roleBinding := &rbacv1.RoleBinding{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), roleBinding); err != nil {
				return nil, fmt.Errorf("error while converting object to RoleBinding: %w", err)
			}
			for _, subject := range roleBinding.Subjects {
				subject.Namespace = namespace
			}
		case "ClusterRoleBinding":
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), clusterRoleBinding); err != nil {
				return nil, fmt.Errorf("error while converting object to ClusterRoleBinding: %w", err)
			}
			for _, subject := range clusterRoleBinding.Subjects {
				subject.Namespace = namespace
			}
		case "MutatingWebhookConfiguration":
			mutatingWebhookConfiguration := &admissionregistrationv1.MutatingWebhookConfiguration{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), mutatingWebhookConfiguration); err != nil {
				return nil, fmt.Errorf("error while converting object to MutatingWebhookConfiguration: %w", err)
			}
			for _, webhook := range mutatingWebhookConfiguration.Webhooks {
				webhook.ClientConfig.Service.Namespace = namespace
			}
		case "ValidatingWebhookConfiguration":
			validatingWebhookConfiguration := &admissionregistrationv1.ValidatingWebhookConfiguration{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), validatingWebhookConfiguration); err != nil {
				return nil, fmt.Errorf("error while converting object to ValidatingWebhookConfiguration: %w", err)
			}
			for _, webhook := range validatingWebhookConfiguration.Webhooks {
				webhook.ClientConfig.Service.Namespace = namespace
			}
		}
		output[i] = obj
	}
	return output, nil
}
