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
	"encoding/json"
	"fmt"
	"time"

	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane/kubeconfig"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane/metadata"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/utils"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
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
			Expect(testClient.Delete(ctx, testNS)).To(Succeed())
			Expect(testutils.CleanupAndWait(ctx, testClient, cleanupObjects...)).To(Succeed())
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
			cleanupObjects = append(cleanupObjects, dpfOperatorConfig)

			Eventually(func(g Gomega) []string {
				gotConfig := &operatorv1.DPFOperatorConfig{}
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpfOperatorConfig), gotConfig)).To(Succeed())
				return gotConfig.Finalizers
			}).WithTimeout(30 * time.Second).Should(ConsistOf([]string{operatorv1.DPFOperatorConfigFinalizer}))
		})
	})

	Context("When checking the custom OVN Kubernetes deployment flow", func() {
		var testNS *corev1.Namespace
		var cleanupObjects []client.Object
		var clusterVersionDeployment *appsv1.Deployment
		var networkOperatorDeployment *appsv1.Deployment
		var nodeIdentityWebhookConfiguration *admissionregistrationv1.ValidatingWebhookConfiguration
		var ovnKubernetesDaemonSet *appsv1.DaemonSet
		var dpfCluster controlplane.DPFCluster

		BeforeEach(func() {
			By("Creating the namespace")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			cleanupObjects = []client.Object{}

			// TODO: Remove that one when we decide where the Host CNI Provisioner should be deployed
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dpf-operator-system"}}
			Expect(testutils.CreateResourceIfNotExist(ctx, testClient, ns)).To(Succeed())

			By("Adding 2 worker nodes in the cluster")
			node1 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
			Expect(testClient.Create(ctx, node1)).To(Succeed())
			cleanupObjects = append(cleanupObjects, node1)
			node2 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}}
			Expect(testClient.Create(ctx, node2)).To(Succeed())
			cleanupObjects = append(cleanupObjects, node2)

			By("Creating the prerequisite OpenShift environment")
			ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: clusterVersionOperatorNamespace}}
			Expect(testutils.CreateResourceIfNotExist(ctx, testClient, ns)).To(Succeed())
			clusterVersionDeployment = getDefaultDeployment(clusterVersionOperatorDeploymentName, clusterVersionOperatorNamespace)
			Expect(testClient.Create(ctx, clusterVersionDeployment)).To(Succeed())
			cleanupObjects = append(cleanupObjects, clusterVersionDeployment)

			ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: networkOperatorNamespace}}
			Expect(testutils.CreateResourceIfNotExist(ctx, testClient, ns)).To(Succeed())
			networkOperatorDeployment = getDefaultDeployment(networkOperatorDeploymentName, networkOperatorNamespace)
			Expect(testClient.Create(ctx, networkOperatorDeployment)).To(Succeed())
			cleanupObjects = append(cleanupObjects, networkOperatorDeployment)

			nodeIdentityWebhookConfiguration = &admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeIdentityWebhookConfigurationName,
				},
			}
			Expect(testClient.Create(ctx, nodeIdentityWebhookConfiguration)).To(Succeed())
			cleanupObjects = append(cleanupObjects, nodeIdentityWebhookConfiguration)

			ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ovnKubernetesNamespace}}
			Expect(testutils.CreateResourceIfNotExist(ctx, testClient, ns)).To(Succeed())
			ovnKubernetesDaemonSet = getDefaultDaemonset(ovnKubernetesDaemonsetName, ovnKubernetesNamespace)
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
			Expect(testClient.Create(ctx, ovnKubernetesDaemonSet)).To(Succeed())
			cleanupObjects = append(cleanupObjects, ovnKubernetesDaemonSet)

			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node1), node1)).To(Succeed())
			node1.SetLabels(map[string]string{
				workerNodeLabel: "",
			})
			node1.SetAnnotations(map[string]string{
				ovnKubernetesNodeChassisIDAnnotation: "node1",
			})
			node1.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
			node1.ManagedFields = nil
			Expect(testClient.Patch(ctx, node1, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node2), node2)).To(Succeed())
			node2.SetLabels(map[string]string{
				ovnKubernetesNodeChassisIDAnnotation: "node2",
			})
			node2.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
			node2.ManagedFields = nil
			Expect(testClient.Patch(ctx, node2, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			By("Faking GetDPFClusters to use the envtest cluster instead of a separate one")
			dpfCluster = controlplane.DPFCluster{Name: "envtest", Namespace: testNS.Name}
			kamajiSecret := getFakeKamajiClusterSecretFromEnvtest(dpfCluster)
			Expect(testClient.Create(ctx, kamajiSecret)).To(Succeed())
			cleanupObjects = append(cleanupObjects, kamajiSecret)
		})
		AfterEach(func() {
			By("Cleaning up the created resources")
			hostCNIProvisionerObjects, err := utils.BytesToUnstructured(hostCNIProvisionerManifestContent)
			Expect(err).ToNot(HaveOccurred())
			for _, o := range hostCNIProvisionerObjects {
				key := client.ObjectKeyFromObject(o)
				err := testClient.Get(ctx, key, o)
				if err != nil {
					if !apierrors.IsNotFound(err) {
						Expect(err).ToNot(HaveOccurred())
					}
					continue
				}
				cleanupObjects = append(cleanupObjects, o)
			}
			dpuCNIProvisionerObjects, err := utils.BytesToUnstructured(dpuCNIProvisionerManifestContent)
			Expect(err).ToNot(HaveOccurred())
			for _, o := range dpuCNIProvisionerObjects {
				key := client.ObjectKeyFromObject(o)
				err := testClient.Get(ctx, key, o)
				if err != nil {
					if !apierrors.IsNotFound(err) {
						Expect(err).ToNot(HaveOccurred())
					}
					continue
				}
				cleanupObjects = append(cleanupObjects, o)
			}
			Expect(testClient.Delete(ctx, testNS)).To(Succeed())
			Expect(testutils.CleanupAndWait(ctx, testClient, cleanupObjects...)).To(Succeed())
		})
		It("should successfully deploy the custom OVN Kubernetes", func() {
			dpfOperatorConfig := &operatorv1.DPFOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config",
					Namespace: testNS.Name,
				},
			}
			Expect(testClient.Create(ctx, dpfOperatorConfig)).To(Succeed())
			cleanupObjects = append(cleanupObjects, dpfOperatorConfig)

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

			Eventually(func(g Gomega) {
				got := &corev1.NodeList{}
				g.Expect(testClient.List(ctx, got)).To(Succeed())
				workerCounter := 0
				for _, node := range got.Items {
					if _, ok := node.Labels[workerNodeLabel]; ok {
						workerCounter++
						g.Expect(node.Labels).NotTo(HaveKey(ovnKubernetesNodeChassisIDAnnotation))
					}
				}
				g.Expect(workerCounter).To(Equal(1))
			}).WithTimeout(30 * time.Second).Should(Succeed())

			Eventually(func(g Gomega) {
				got := &appsv1.DaemonSet{}
				key := client.ObjectKey{Namespace: "dpf-operator-system", Name: "host-cni-provisioner"}
				g.Expect(testClient.Get(ctx, key, got)).To(Succeed())
			}).WithTimeout(30 * time.Second).Should(Succeed())

			Eventually(func(g Gomega) {
				got := &appsv1.DaemonSet{}
				c, err := dpfCluster.NewClient(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				key := client.ObjectKey{Namespace: "dpf-operator-system", Name: "dpu-cni-provisioner"}
				g.Expect(c.Get(ctx, key, got)).To(Succeed())
			}).WithTimeout(30 * time.Second).Should(Succeed())
		})

		It("should not deploy the CNI provisioners if original OVN Kubernetes pods are still running", func() {
			got := &appsv1.DaemonSet{}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(ovnKubernetesDaemonSet), got)).To(Succeed())
			got.Status.NumberMisscheduled = 5
			got.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DaemonSet"))
			got.ManagedFields = nil
			Expect(testClient.Status().Patch(ctx, got, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			dpfOperatorConfig := &operatorv1.DPFOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config",
					Namespace: testNS.Name,
				},
			}
			Expect(testClient.Create(ctx, dpfOperatorConfig)).To(Succeed())
			cleanupObjects = append(cleanupObjects, dpfOperatorConfig)

			Consistently(func(g Gomega) {
				_, _ = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(dpfOperatorConfig)})
				got := &appsv1.DaemonSet{}
				key := client.ObjectKey{Namespace: "dpf-operator-system", Name: "host-cni-provisioner"}
				g.Expect(testClient.Get(ctx, key, got)).To(HaveOccurred())
				key = client.ObjectKey{Namespace: "dpf-operator-system", Name: "dpu-cni-provisioner"}
				g.Expect(testClient.Get(ctx, key, got)).To(HaveOccurred())
			}).WithTimeout(5 * time.Second).Should(Succeed())
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

// getFakeKamajiClusterSecretFromEnvtest creates a kamaji secret using the envtest information to simulate that we have
// a kamaji cluster. In reality, this is the same envtest Kubernetes API.
func getFakeKamajiClusterSecretFromEnvtest(cluster controlplane.DPFCluster) *corev1.Secret {
	adminConfig := &kubeconfig.Type{
		Clusters: []*kubeconfig.ClusterWithName{
			{
				Name: cluster.Name,
				Cluster: kubeconfig.Cluster{
					Server:                   cfg.Host,
					CertificateAuthorityData: cfg.CAData,
				},
			},
		},
		Users: []*kubeconfig.UserWithName{
			{
				Name: "user",
				User: kubeconfig.User{
					ClientKeyData:         cfg.KeyData,
					ClientCertificateData: cfg.CertData,
				},
			},
		},
		Contexts: []*kubeconfig.NamedContext{
			{
				Name: "default",
				Context: kubeconfig.Context{
					Cluster: cluster.Name,
					User:    "user",
				},
			},
		},
		CurrentContext: "default",
	}
	confData, err := json.Marshal(adminConfig)
	Expect(err).To(Not(HaveOccurred()))
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-admin-kubeconfig", cluster.Name),
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				controlplanemeta.DPFClusterSecretClusterNameLabelKey: cluster.Name,
				"kamaji.clastix.io/component":                        "admin-kubeconfig",
				"kamaji.clastix.io/project":                          "kamaji",
			},
		},
		Data: map[string][]byte{
			"admin.conf": confData,
		},
	}
}
