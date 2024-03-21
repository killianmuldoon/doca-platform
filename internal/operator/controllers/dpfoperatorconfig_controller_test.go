/*
Copyright 2024.

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

package controller

import (
	"time"

	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DPFOperatorConfig Controller", func() {
	Context("When reconciling a resource", func() {
		var testNS *corev1.Namespace
		var cleanupObjects []client.Object
		BeforeEach(func() {
			By("Creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			cleanupObjects = []client.Object{}
		})
		AfterEach(func() {
			By("Cleaning up the Namespace")
			for _, obj := range cleanupObjects {
				Expect(testClient.Delete(ctx, obj)).To(Succeed())
			}
			Expect(testClient.Delete(ctx, testNS)).To(Succeed())
		})
		It("should successfully reconcile the DPFOperatorConfig", func() {
			By("Reconciling the created resource")

			dpfOperatorConfig := &operatorv1.DPFOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config",
					Namespace: testNS.Name,
				},
			}
			Expect(testClient.Create(ctx, dpfOperatorConfig)).To(Succeed())

			Eventually(func(g Gomega) []string {
				gotConfig := &operatorv1.DPFOperatorConfig{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpfOperatorConfig), gotConfig)).To(Succeed())
				return gotConfig.Finalizers
			}).WithTimeout(30 * time.Second).Should(ConsistOf([]string{operatorv1.DPFOperatorConfigFinalizer}))
		})
		It("should successfully deploy the custom OVN Kubernetes", func() {
			By("Creating the prerequisite environment")
			clusterVersionDeployment := getDefaultDeployment(clusterVersionOperatorDeploymentName, clusterVersionOperatorNamespace)
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: clusterVersionOperatorNamespace}}
			Expect(testClient.Create(ctx, ns)).To(Succeed())
			cleanupObjects = append(cleanupObjects, ns)
			Expect(testClient.Create(ctx, clusterVersionDeployment)).To(Succeed())

			networkOperatorDeployment := getDefaultDeployment(networkOperatorDeploymentName, networkOperatorNamespace)
			ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: networkOperatorNamespace}}
			Expect(testClient.Create(ctx, ns)).To(Succeed())
			cleanupObjects = append(cleanupObjects, ns)
			Expect(testClient.Create(ctx, networkOperatorDeployment)).To(Succeed())

			nodeIdentityWebhookConfiguration := &admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeIdentityWebhookConfigurationName,
				},
			}
			Expect(testClient.Create(ctx, nodeIdentityWebhookConfiguration)).To(Succeed())

			ovnKubernetesDaemonSet := getDefaultDaemonset(ovnKubernetesDaemonsetName, ovnKubernetesNamespace)
			ovnKubernetesDaemonSet.Spec.Template.Spec.Affinity = &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "some-key",
										Operator: corev1.NodeSelectorOpDoesNotExist,
									},
								},
							},
						},
					},
				},
			}
			ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ovnKubernetesNamespace}}
			Expect(testClient.Create(ctx, ns)).To(Succeed())
			cleanupObjects = append(cleanupObjects, ns)
			Expect(testClient.Create(ctx, ovnKubernetesDaemonSet)).To(Succeed())

			By("Reconciling the created resource")
			dpfOperatorConfig := &operatorv1.DPFOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config",
					Namespace: testNS.Name,
				},
			}
			Expect(testClient.Create(ctx, dpfOperatorConfig)).To(Succeed())

			Eventually(func(g Gomega) *int32 {
				got := &appsv1.Deployment{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(clusterVersionDeployment), got)).To(Succeed())
				return got.Spec.Replicas
			}).WithTimeout(30 * time.Second).Should(Equal(ptr.To[int32](0)))

			Eventually(func(g Gomega) *int32 {
				got := &appsv1.Deployment{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(networkOperatorDeployment), got)).To(Succeed())
				return got.Spec.Replicas
			}).WithTimeout(30 * time.Second).Should(Equal(ptr.To[int32](0)))

			Eventually(func(g Gomega) bool {
				got := &admissionregistrationv1.ValidatingWebhookConfiguration{}
				return apierrors.IsNotFound(testClient.Get(ctx, client.ObjectKeyFromObject(nodeIdentityWebhookConfiguration), got))
			}).WithTimeout(30 * time.Second).Should(BeTrue())

			Eventually(func(g Gomega) *corev1.Affinity {
				got := &appsv1.DaemonSet{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(ovnKubernetesDaemonSet), got)).To(Succeed())
				return got.Spec.Template.Spec.Affinity
			}).WithTimeout(30 * time.Second).Should(Equal(&corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      controlPlaneNodeLabel,
										Operator: corev1.NodeSelectorOpExists,
									},
								},
							},
						},
					},
				},
			},
			))
		})
	})
})

func getDefaultDeployment(name string, namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
			},
		},
	}
}

func getDefaultDaemonset(name string, namespace string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
						},
					},
				},
			},
		},
	}
}
