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
	corev1 "k8s.io/api/core/v1"
)

// ObjectKind represends different Kind of Kubernetes Objects
type ObjectKind string

const (
	// ObjectKind
	NamespaceKind                      ObjectKind = "Namespace"
	CustomResourceDefinitionKind       ObjectKind = "CustomResourceDefinition"
	ClusterRoleKind                    ObjectKind = "ClusterRole"
	RoleBindingKind                    ObjectKind = "RoleBinding"
	ClusterRoleBindingKind             ObjectKind = "ClusterRoleBinding"
	MutatingWebhookConfigurationKind   ObjectKind = "MutatingWebhookConfiguration"
	ValidatingWebhookConfigurationKind ObjectKind = "ValidatingWebhookConfiguration"
	DeploymentKind                     ObjectKind = "Deployment"
	StatefulSetKind                    ObjectKind = "StatefulSet"
	CertificateKind                    ObjectKind = "Certificate"
	IssuerKind                         ObjectKind = "Issuer"
	RoleKind                           ObjectKind = "Role"
	ServiceAccountKind                 ObjectKind = "ServiceAccount"
	ServiceKind                        ObjectKind = "Service"
	DPUServiceKind                     ObjectKind = "DPUService"

	kubernetesNodeRoleMaster       = "node-role.kubernetes.io/master"
	kubernetesNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
)

var (
	controlPlaneNodeAffinity = corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      kubernetesNodeRoleMaster,
						Operator: corev1.NodeSelectorOpExists,
					},
					},
				},
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      kubernetesNodeRoleControlPlane,
							Operator: corev1.NodeSelectorOpExists,
						},
					},
				},
			},
		},
	}
)

func localObjRefsFromStrings(names ...string) []corev1.LocalObjectReference {
	localObjectRefs := make([]corev1.LocalObjectReference, 0, len(names))
	for _, name := range names {
		localObjectRefs = append(localObjectRefs, corev1.LocalObjectReference{Name: name})
	}
	return localObjectRefs
}
