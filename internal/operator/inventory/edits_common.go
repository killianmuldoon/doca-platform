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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NamespaceEdit sets namespace for object
func NamespaceEdit(namespace string) UnstructuredEdit {
	return func(un *unstructured.Unstructured) error {
		un.SetNamespace(namespace)
		// special handling
		switch ObjectKind(un.GetKind()) {
		case RoleBindingKind:
			roleBinding := &rbacv1.RoleBinding{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.UnstructuredContent(), roleBinding); err != nil {
				return fmt.Errorf("error while converting object to RoleBinding: %w", err)
			}
			for i := range roleBinding.Subjects {
				roleBinding.Subjects[i].Namespace = namespace
			}
			uns, err := runtime.DefaultUnstructuredConverter.ToUnstructured(roleBinding)
			if err != nil {
				return fmt.Errorf("error while converting object to unstructured: %w", err)
			}
			un.Object = uns

		case ClusterRoleBindingKind:
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.UnstructuredContent(), clusterRoleBinding); err != nil {
				return fmt.Errorf("error while converting object to ClusterRoleBinding: %w", err)
			}
			for i := range clusterRoleBinding.Subjects {
				clusterRoleBinding.Subjects[i].Namespace = namespace
			}
			uns, err := runtime.DefaultUnstructuredConverter.ToUnstructured(clusterRoleBinding)
			if err != nil {
				return fmt.Errorf("error while converting object to unstructured: %w", err)
			}
			un.Object = uns

		case MutatingWebhookConfigurationKind:
			mutatingWebhookConfiguration := &admissionregistrationv1.MutatingWebhookConfiguration{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.UnstructuredContent(), mutatingWebhookConfiguration); err != nil {
				return fmt.Errorf("error while converting object to MutatingWebhookConfiguration: %w", err)
			}
			for i := range mutatingWebhookConfiguration.Webhooks {
				mutatingWebhookConfiguration.Webhooks[i].ClientConfig.Service.Namespace = namespace
			}
			uns, err := runtime.DefaultUnstructuredConverter.ToUnstructured(mutatingWebhookConfiguration)
			if err != nil {
				return fmt.Errorf("error while converting object to unstructured: %w", err)
			}
			un.Object = uns
			handleCertManagerAnnotationNamespace(un, namespace)

		case ValidatingWebhookConfigurationKind:
			validatingWebhookConfiguration := &admissionregistrationv1.ValidatingWebhookConfiguration{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(un.UnstructuredContent(), validatingWebhookConfiguration); err != nil {
				return fmt.Errorf("error while converting object to ValidatingWebhookConfiguration: %w", err)
			}
			for i := range validatingWebhookConfiguration.Webhooks {
				validatingWebhookConfiguration.Webhooks[i].ClientConfig.Service.Namespace = namespace
			}
			uns, err := runtime.DefaultUnstructuredConverter.ToUnstructured(validatingWebhookConfiguration)
			if err != nil {
				return fmt.Errorf("error while converting object to unstructured: %w", err)
			}
			un.Object = uns
			handleCertManagerAnnotationNamespace(un, namespace)

		case CertificateKind:
			certs, ok, err := unstructured.NestedSlice(un.UnstructuredContent(), "spec", "dnsNames")
			if err != nil {
				return fmt.Errorf("error while setting namespace values in Certificate: %w", err)
			}
			if !ok {
				break
			}
			for i := range certs {
				s, ok := certs[i].(string)
				if !ok {
					return fmt.Errorf("error while setting namespace values in Certificate %s/%s", un.GetNamespace(), un.GetName())
				}
				// services take the form ${SERVICE_NAME}.${SERVICE_NAMESPACE}.${SERVICE_DOMAIN}.svc
				parts := strings.Split(s, ".")
				if len(parts) < 3 {
					return fmt.Errorf("error while setting namespace values in Certificate %s/%s", un.GetNamespace(), un.GetName())
				}
				// Set the second part as the namespace and reset the string and field.
				parts[1] = namespace
				certs[i] = strings.Join(parts, ".")
			}
			if err = unstructured.SetNestedField(un.UnstructuredContent(), certs, "spec", "dnsNames"); err != nil {
				return fmt.Errorf("error while setting namespace values in Certificate: %w", err)
			}
		}

		return nil
	}
}

// handleCertManagerAnnotationNamespace updates cert-manager annotations in-place
func handleCertManagerAnnotationNamespace(obj *unstructured.Unstructured, namespace string) {
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
}

// ImagePullSecretsEditForDeploymentEdit sets pullSecrets for Deployment object
func ImagePullSecretsEditForDeploymentEdit(pullSecrets ...string) StructuredEdit {
	return func(obj client.Object) error {
		deployment, ok := obj.(*appsv1.Deployment)
		if !ok {
			return fmt.Errorf("unexpected object %s. expected Deployment", obj.GetObjectKind().GroupVersionKind())
		}

		// replace pull secrets with provided input
		deployment.Spec.Template.Spec.ImagePullSecrets = localObjRefsFromStrings(pullSecrets...)
		return nil
	}
}

// NodeAffinityForDeploymentEdit sets NodeAffinity for Deployment objs
func NodeAffinityForDeploymentEdit(nodeAffinity *corev1.NodeAffinity) StructuredEdit {
	return func(obj client.Object) error {
		deployment, ok := obj.(*appsv1.Deployment)
		if !ok {
			return fmt.Errorf("unexpected object %s. expected Deployment", obj.GetObjectKind().GroupVersionKind())
		}

		if deployment.Spec.Template.Spec.Affinity == nil {
			deployment.Spec.Template.Spec.Affinity = &corev1.Affinity{}
		}
		deployment.Spec.Template.Spec.Affinity.NodeAffinity = nodeAffinity
		return nil
	}
}

// NodeAffinityForStatefulSetEdit sets NodeAffinity for Deployment objs
func NodeAffinityForStatefulSetEdit(nodeAffinity *corev1.NodeAffinity) StructuredEdit {
	return func(obj client.Object) error {
		sts, ok := obj.(*appsv1.StatefulSet)
		if !ok {
			return fmt.Errorf("unexpected object %s. expected Deployment", obj.GetObjectKind().GroupVersionKind())
		}

		if sts.Spec.Template.Spec.Affinity == nil {
			sts.Spec.Template.Spec.Affinity = &corev1.Affinity{}
		}
		sts.Spec.Template.Spec.Affinity.NodeAffinity = nodeAffinity
		return nil
	}
}
