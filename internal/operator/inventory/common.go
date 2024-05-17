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
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	namespaceKind                      = "Namespace"
	customResourceDefinitionKind       = "CustomResourceDefinition"
	clusterRoleKind                    = "ClusterRole"
	roleBindingKind                    = "RoleBinding"
	clusterRoleBindingKind             = "ClusterRoleBinding"
	mutatingWebhookConfigurationKind   = "MutatingWebhookConfiguration"
	validatingWebhookConfigurationKind = "ValidatingWebhookConfiguration"
	deploymentKind                     = "Deployment"
	certificateKind                    = "Certificate"
	roleKind                           = "Role"
	serviceAccountKind                 = "ServiceAccount"
)

func setNamespace(namespace string, objects []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	output := make([]unstructured.Unstructured, len(objects))
	for i, obj := range objects {
		obj.SetNamespace(namespace)
		switch obj.GetKind() {
		case roleBindingKind:
			roleBinding := &rbacv1.RoleBinding{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), roleBinding); err != nil {
				return nil, fmt.Errorf("error while converting object to RoleBinding: %w", err)
			}
			for i := range roleBinding.Subjects {
				roleBinding.Subjects[i].Namespace = namespace
			}
			uns, err := runtime.DefaultUnstructuredConverter.ToUnstructured(roleBinding)
			if err != nil {
				return nil, fmt.Errorf("error while converting object to unstructured: %w", err)
			}
			obj = unstructured.Unstructured{Object: uns}
		case clusterRoleBindingKind:
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), clusterRoleBinding); err != nil {
				return nil, fmt.Errorf("error while converting object to ClusterRoleBinding: %w", err)
			}
			for i := range clusterRoleBinding.Subjects {
				clusterRoleBinding.Subjects[i].Namespace = namespace
			}
			uns, err := runtime.DefaultUnstructuredConverter.ToUnstructured(clusterRoleBinding)
			if err != nil {
				return nil, fmt.Errorf("error while converting object to unstructured: %w", err)
			}
			obj = unstructured.Unstructured{Object: uns}

		case mutatingWebhookConfigurationKind:
			mutatingWebhookConfiguration := &admissionregistrationv1.MutatingWebhookConfiguration{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), mutatingWebhookConfiguration); err != nil {
				return nil, fmt.Errorf("error while converting object to MutatingWebhookConfiguration: %w", err)
			}
			for i := range mutatingWebhookConfiguration.Webhooks {
				mutatingWebhookConfiguration.Webhooks[i].ClientConfig.Service.Namespace = namespace
			}
			uns, err := runtime.DefaultUnstructuredConverter.ToUnstructured(mutatingWebhookConfiguration)
			if err != nil {
				return nil, fmt.Errorf("error while converting object to unstructured: %w", err)
			}
			obj = unstructured.Unstructured{Object: uns}
			obj = replaceCertManagerAnnotationNamespace(obj, namespace)

		case validatingWebhookConfigurationKind:
			validatingWebhookConfiguration := &admissionregistrationv1.ValidatingWebhookConfiguration{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), validatingWebhookConfiguration); err != nil {
				return nil, fmt.Errorf("error while converting object to ValidatingWebhookConfiguration: %w", err)
			}
			for i := range validatingWebhookConfiguration.Webhooks {
				validatingWebhookConfiguration.Webhooks[i].ClientConfig.Service.Namespace = namespace
			}
			uns, err := runtime.DefaultUnstructuredConverter.ToUnstructured(validatingWebhookConfiguration)
			if err != nil {
				return nil, fmt.Errorf("error while converting object to unstructured: %w", err)
			}
			obj = unstructured.Unstructured{Object: uns}
			obj = replaceCertManagerAnnotationNamespace(obj, namespace)

		case certificateKind:
			certs, ok, err := unstructured.NestedSlice(obj.UnstructuredContent(), "spec", "dnsNames")
			if err != nil {
				return nil, fmt.Errorf("error while setting namespace values in Certificate: %w", err)
			}
			if !ok {
				continue
			}
			for i := range certs {
				s, ok := certs[i].(string)
				if !ok {
					return nil, fmt.Errorf("error while setting namespace values in Certificate %s/%s", obj.GetNamespace(), obj.GetName())
				}
				// services take the form ${SERVICE_NAME}.${SERVICE_NAMESPACE}.${SERVICE_DOMAIN}.svc
				parts := strings.Split(s, ".")
				if len(parts) < 3 {
					return nil, fmt.Errorf("error while setting namespace values in Certificate %s/%s", obj.GetNamespace(), obj.GetName())
				}
				// Set the second part as the namespace and reset the string and field.
				parts[1] = namespace
				certs[i] = strings.Join(parts, ".")
			}
			if err = unstructured.SetNestedField(obj.UnstructuredContent(), certs, "spec", "dnsNames"); err != nil {
				return nil, fmt.Errorf("error while setting namespace values in Certificate: %w", err)
			}
		}
		output[i] = obj
	}
	return output, nil
}

func replaceCertManagerAnnotationNamespace(obj unstructured.Unstructured, namespace string) unstructured.Unstructured {
	annotations := obj.GetAnnotations()
	if annotations != nil {
		value, ok := annotations["cert-manager.io/inject-ca-from"]
		if ok {
			parts := strings.Split(value, "/")
			if len(parts) == 2 {
				parts[0] = namespace
			}
			annotations["cert-manager.io/inject-ca-from"] = strings.Join(parts, "/")
			obj.SetAnnotations(annotations)
		}
	}
	return obj
}
