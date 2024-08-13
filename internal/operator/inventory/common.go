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
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectKind represents different Kind of Kubernetes Objects
type ObjectKind string

const (
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

// deploymentReadyCheck can be used to check for readiness of an inventory Component that has exactly one Kubernetes Deployment of interest.
// The Component returns and error when the number of ReadyReplicas is zero or below the current number of Replicas.
func deploymentReadyCheck(ctx context.Context, c client.Client, namespace string, objects []*unstructured.Unstructured) error {
	deployment := &appsv1.Deployment{}
	found := false
	for _, obj := range objects {
		if obj.GetKind() == string(DeploymentKind) {
			err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: obj.GetName()}, deployment)
			if err != nil {
				return err
			}
			found = true
			break
		}
	}
	if !found {
		return errors.New("deployment not found in objects")
	}

	// Consider the deployment not ready if it has no replicas.
	if deployment.Status.Replicas == 0 {
		return fmt.Errorf("Deployment %s/%s has no replicas", deployment.GetNamespace(), deployment.GetName())
	}
	if deployment.Status.ReadyReplicas != deployment.Status.Replicas {
		return fmt.Errorf("Deployment %s/%s has %d readyReplicas, want %d",
			deployment.GetNamespace(), deployment.GetName(), deployment.Status.ReadyReplicas, deployment.Status.Replicas)
	}
	return nil
}
