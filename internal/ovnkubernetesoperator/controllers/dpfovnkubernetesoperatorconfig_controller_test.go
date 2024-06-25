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

package controller

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	ovnkubernetesoperatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/ovnkubernetesoperator/v1alpha1"
	dpucniprovisionerconfig "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/dpu/config"
	hostcniprovisionerconfig "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/host/config"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/utils"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//nolint:goconst
var _ = Describe("DPFOVNKubernetesOperatorConfig Controller", func() {
	Context("When checking the reconciliation of the Network Injector prerequisites", func() {
		Context("With webhook enabled", func() {
			var currentWebhookEnabled bool
			var testNS *corev1.Namespace
			BeforeEach(func() {
				currentWebhookEnabled = reconciler.Settings.WebhookEnabled
				reconciler.Settings.WebhookEnabled = true
				testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
				Expect(testClient.Create(ctx, testNS)).To(Succeed())
			})
			AfterEach(func() {
				reconciler.Settings.WebhookEnabled = currentWebhookEnabled
			})
			It("creates a NetworkAttachmentDefinition", func() {
				config := getMinimalDPFOVNKubernetesOperatorConfig(testNS.Name)
				config.Spec.VFResourceName = "some-resource"
				Expect(testClient.Create(ctx, config)).To(Succeed())
				// DPF Operator creates objects when reconciling the DPFOVNKubernetesOperatorConfig and we need to ensure that on
				// deletion of these objects there is no DPFOVNKubernetesOperatorConfig in the cluster to trigger recreation of those.
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, config)

				Eventually(func(g Gomega) {
					netAttachDef := &unstructured.Unstructured{}
					netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
						Group:   "k8s.cni.cncf.io",
						Version: "v1",
						Kind:    "NetworkAttachmentDefinition",
					})
					key := client.ObjectKey{Namespace: config.Namespace, Name: "dpf-ovn-kubernetes"}
					g.Expect(testClient.Get(ctx, key, netAttachDef)).To(Succeed())
					g.Expect(netAttachDef.GetAnnotations()).To(HaveKeyWithValue("k8s.v1.cni.cncf.io/resourceName", "some-resource"))
					spec, found, err := unstructured.NestedStringMap(netAttachDef.Object, "spec")
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(found).To(BeTrue())
					g.Expect(spec).To(BeComparableTo(
						map[string]string{"config": `{"cniVersion":"0.3.1","name":"ovn-kubernetes","type":"ovn-k8s-cni-overlay","ipam":{},"dns":{}}`},
					))
					DeferCleanup(testutils.CleanupAndWait, ctx, testClient, netAttachDef)
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("doesn't create a NetworkAttachmentDefinition if VFResourceName is missing from the DPFOVNKubernetesOperatorConfig", func() {
				config := getMinimalDPFOVNKubernetesOperatorConfig(testNS.Name)
				Expect(testClient.Create(ctx, config)).To(Succeed())
				// DPF Operator creates objects when reconciling the DPFOVNKubernetesOperatorConfig and we need to ensure that on
				// deletion of these objects there is no DPFOVNKubernetesOperatorConfig in the cluster to trigger recreation of those.
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, config)
				Consistently(func(g Gomega) {
					netAttachDef := &unstructured.Unstructured{}
					netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
						Group:   "k8s.cni.cncf.io",
						Version: "v1",
						Kind:    "NetworkAttachmentDefinition",
					})
					key := client.ObjectKey{Namespace: config.Namespace, Name: "dpf-ovn-kubernetes"}
					g.Expect(apierrors.IsNotFound(testClient.Get(ctx, key, netAttachDef))).To(BeTrue())
				}).WithTimeout(5 * time.Second).Should(Succeed())
			})
		})
		Context("With webhook disabled", func() {
			var currentWebhookEnabled bool
			var testNS *corev1.Namespace
			BeforeEach(func() {
				currentWebhookEnabled = reconciler.Settings.WebhookEnabled
				reconciler.Settings.WebhookEnabled = false
				testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
				Expect(testClient.Create(ctx, testNS)).To(Succeed())
			})
			AfterEach(func() {
				reconciler.Settings.WebhookEnabled = currentWebhookEnabled
			})
			It("doesn't create a NetworkAttachmentDefinition", func() {
				config := getMinimalDPFOVNKubernetesOperatorConfig(testNS.Name)
				Expect(testClient.Create(ctx, config)).To(Succeed())
				// DPF Operator creates objects when reconciling the DPFOVNKubernetesOperatorConfig and we need to ensure that on
				// deletion of these objects there is no DPFOVNKubernetesOperatorConfig in the cluster to trigger recreation of those.
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, config)
				Consistently(func(g Gomega) {
					netAttachDef := &unstructured.Unstructured{}
					netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
						Group:   "k8s.cni.cncf.io",
						Version: "v1",
						Kind:    "NetworkAttachmentDefinition",
					})
					key := client.ObjectKey{Namespace: config.Namespace, Name: "dpf-ovn-kubernetes"}
					g.Expect(apierrors.IsNotFound(testClient.Get(ctx, key, netAttachDef))).To(BeTrue())
				}).WithTimeout(5 * time.Second).Should(Succeed())
			})
		})
	})
	Context("When checking the custom OVN Kubernetes deployment flow", func() {
		var testNS *corev1.Namespace
		var clusterVersionCR *unstructured.Unstructured
		var networkOperatorDeployment *appsv1.Deployment
		var nodeIdentityWebhookConfiguration *admissionregistrationv1.ValidatingWebhookConfiguration
		var ovnKubernetesDaemonSet *appsv1.DaemonSet
		var dpfCluster controlplane.DPFCluster
		var nodeWorker1 *corev1.Node
		var nodeWorker2 *corev1.Node
		var nodeControlPlane1 *corev1.Node

		BeforeEach(func() {
			By("Creating the namespace")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			cleanupObjects := []client.Object{}

			// TODO: Remove that one when we decide where the Host CNI Provisioner should be deployed
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dpf-operator-system"}}
			Expect(testutils.CreateResourceIfNotExist(ctx, testClient, ns)).To(Succeed())

			By("Adding 2 worker nodes in the cluster")
			nodeWorker1 = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}}
			Expect(testClient.Create(ctx, nodeWorker1)).To(Succeed())
			cleanupObjects = append(cleanupObjects, nodeWorker1)
			nodeWorker2 = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-2"}}
			Expect(testClient.Create(ctx, nodeWorker2)).To(Succeed())
			cleanupObjects = append(cleanupObjects, nodeWorker2)
			nodeControlPlane1 = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "control-plane-1"}}
			Expect(testClient.Create(ctx, nodeControlPlane1)).To(Succeed())
			cleanupObjects = append(cleanupObjects, nodeControlPlane1)

			By("Creating the prerequisite OpenShift environment")
			clusterVersionCR = getDefaultClusterVersion(clusterVersionCRName)
			Expect(testClient.Create(ctx, clusterVersionCR)).To(Succeed())
			cleanupObjects = append(cleanupObjects, clusterVersionCR)

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

			ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: clusterConfigNamespace}}
			Expect(testutils.CreateResourceIfNotExist(ctx, testClient, ns)).To(Succeed())
			clusterConfigConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterConfigConfigMapName,
					Namespace: clusterConfigNamespace,
				},
				Data: map[string]string{
					"install-config": `
networking:
  machineNetwork:
  - cidr: 192.168.1.0/24`,
				},
			}
			Expect(testClient.Create(ctx, clusterConfigConfigMap)).To(Succeed())
			cleanupObjects = append(cleanupObjects, clusterConfigConfigMap)

			// Mocked Worker Node
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(nodeWorker1), nodeWorker1)).To(Succeed())
			nodeWorker1.SetLabels(map[string]string{
				dpuEnabledNodeLabelKey: dpuEnabledNodeLabelValue,
			})
			nodeWorker1.SetAnnotations(map[string]string{
				ovnKubernetesNodeChassisIDAnnotation: "worker-1",
			})
			nodeWorker1.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
			nodeWorker1.ManagedFields = nil
			Expect(testClient.Patch(ctx, nodeWorker1, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			// Mocked Worker Node
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(nodeWorker2), nodeWorker2)).To(Succeed())
			nodeWorker2.SetLabels(map[string]string{
				dpuEnabledNodeLabelKey: dpuEnabledNodeLabelValue,
			})
			nodeWorker2.SetAnnotations(map[string]string{
				ovnKubernetesNodeChassisIDAnnotation: "worker-2",
			})
			nodeWorker2.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
			nodeWorker2.ManagedFields = nil
			Expect(testClient.Patch(ctx, nodeWorker2, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			// Mocked Control Plane Node
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(nodeControlPlane1), nodeControlPlane1)).To(Succeed())
			nodeControlPlane1.SetLabels(map[string]string{})
			nodeControlPlane1.SetAnnotations(map[string]string{
				ovnKubernetesNodeChassisIDAnnotation: "control-plane-1",
			})
			nodeControlPlane1.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
			nodeControlPlane1.ManagedFields = nil
			Expect(testClient.Patch(ctx, nodeControlPlane1, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			var ovnKubernetesManifests []*unstructured.Unstructured
			content, err := os.ReadFile("testdata/original/ovnkubernetes-daemonset.yaml")
			Expect(err).ToNot(HaveOccurred())
			manifests, err := utils.BytesToUnstructured(content)
			Expect(err).ToNot(HaveOccurred())
			ovnKubernetesManifests = append(ovnKubernetesManifests, manifests...)
			content, err = os.ReadFile("testdata/original/ovnkubernetes-configmap.yaml")
			Expect(err).ToNot(HaveOccurred())
			manifests, err = utils.BytesToUnstructured(content)
			Expect(err).ToNot(HaveOccurred())
			ovnKubernetesManifests = append(ovnKubernetesManifests, manifests...)
			content, err = os.ReadFile("testdata/original/ovnkubernetes-entrypointconfigmap.yaml")
			Expect(err).ToNot(HaveOccurred())
			manifests, err = utils.BytesToUnstructured(content)
			Expect(err).ToNot(HaveOccurred())
			ovnKubernetesManifests = append(ovnKubernetesManifests, manifests...)
			Expect(reconcileUnstructuredObjects(ctx, testClient, ovnKubernetesManifests)).To(Succeed())
			for _, o := range ovnKubernetesManifests {
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

			By("Faking GetDPFClusters to use the envtest cluster instead of a separate one")
			dpfCluster = controlplane.DPFCluster{Name: "envtest", Namespace: testNS.Name}
			kamajiSecret, err := testutils.GetFakeKamajiClusterSecretFromEnvtest(dpfCluster, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(testClient.Create(ctx, kamajiSecret)).To(Succeed())
			cleanupObjects = append(cleanupObjects, kamajiSecret)

			By("Creating image pull secrets")
			secretOne := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret-1", Namespace: testNS.Name}}
			secretTwo := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret-2", Namespace: testNS.Name}}
			Expect(testClient.Create(ctx, secretOne)).To(Succeed())
			cleanupObjects = append(cleanupObjects, secretOne)
			Expect(testClient.Create(ctx, secretTwo)).To(Succeed())
			cleanupObjects = append(cleanupObjects, secretTwo)

			// We explicitly use `DeferCleanup` instead of `AfterEach` to ensure that we can have some order when
			// cleaning up the objects. In particular, we need to delete DPF Operator Config before all the rest of
			// the objects to ensure that we don't hit race conditions.
			// According to upstream, mixing DeferCleanup (which is used in the `It` nodes) and AfterEach can lead to
			// race conditions that we also faced in the past:
			// * https://github.com/onsi/ginkgo/issues/1360#issuecomment-1949181004
			// * https://github.com/onsi/ginkgo/issues/1022#issuecomment-1224732382
			DeferCleanup(func() {
				By("Cleaning up the resources created by the DPF Operator")
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
				// We don't use the CleanupAndWait function here because of a race condition related to the missing
				// finalizer. In particular, there is a race condition with a reconciliation that is in progress and the
				// config CR is removed. In that case, the CleanupAndWait that is called at the end of each ginkgo spec
				// that creates a config CR exits very fast and this DeferCleanup we are in proceeds with the deletion
				// of the rest of the objects. In the meantime, the controller installs those objects again. If we had
				// a finalizer, that CleanupAndWait would exit after the CR has been removed which means that there is
				// no ongoing reconciliation that modifies objects (i.e. calls reconcileCustomOVNKubernetesDeployment())
				// happening on removal of the object.
				Eventually(func(g Gomega) {
					g.Consistently(func(g Gomega) {
						for _, o := range cleanupObjects {
							g.Expect(client.IgnoreNotFound(testClient.Delete(ctx, o))).To(Succeed())
							g.Expect(apierrors.IsNotFound(testClient.Get(ctx, client.ObjectKeyFromObject(o), o))).To(BeTrue())
						}
					}).WithTimeout(5 * time.Second).Should(Succeed())
				}).WithTimeout(180 * time.Second).Should(Succeed())
			})
		})

		It("should successfully deploy the custom OVN Kubernetes", func() {
			operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig(testNS.Name)
			operatorConfig.Spec.ImagePullSecrets = []string{"secret-1", "secret-2"}
			Expect(testClient.Create(ctx, operatorConfig)).To(Succeed())
			// DPF Operator creates objects when reconciling the DPFOVNKubernetesOperatorConfig and we need to ensure that on
			// deletion of these objects there is no DPFOVNKubernetesOperatorConfig in the cluster to trigger recreation of those.
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, operatorConfig)

			Eventually(func(g Gomega) []interface{} {
				got := &unstructured.Unstructured{}
				got.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "config.openshift.io",
					Version: "v1",
					Kind:    "ClusterVersion",
				})
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(clusterVersionCR), got)).To(Succeed())

				overrides, found, err := unstructured.NestedSlice(got.Object, "spec", "overrides")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())
				return overrides
			}).WithTimeout(3 * time.Second).Should(BeComparableTo([]interface{}{
				map[string]interface{}{
					"kind":      "Deployment",
					"group":     "apps",
					"name":      networkOperatorDeploymentName,
					"namespace": networkOperatorNamespace,
					"unmanaged": true,
				},
			}))

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
										Key:      dpuEnabledNodeLabelKey,
										Operator: corev1.NodeSelectorOpNotIn,
										Values:   []string{dpuEnabledNodeLabelValue},
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
				g.Expect(testClient.List(ctx, got, client.MatchingLabels{
					dpuEnabledNodeLabelKey: dpuEnabledNodeLabelValue,
				})).To(Succeed())
				g.Expect(got.Items).To(HaveLen(2))
				for _, node := range got.Items {
					g.Expect(node.Annotations).NotTo(HaveKey(ovnKubernetesNodeChassisIDAnnotation))
				}
			}).WithTimeout(30 * time.Second).Should(Succeed())

			Eventually(func(g Gomega) {
				got := &appsv1.DaemonSet{}
				c, err := dpfCluster.NewClient(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				key := client.ObjectKey{Namespace: "dpf-operator-system", Name: "dpu-cni-provisioner"}
				g.Expect(c.Get(ctx, key, got)).To(Succeed())
			}).WithTimeout(30 * time.Second).Should(Succeed())

			Eventually(func(g Gomega) {
				got := &appsv1.DaemonSet{}
				key := client.ObjectKey{Namespace: "dpf-operator-system", Name: "host-cni-provisioner"}
				g.Expect(testClient.Get(ctx, key, got)).To(Succeed())
			}).WithTimeout(30 * time.Second).Should(Succeed())

			By("Turning the CNI Provisioners to ready")
			got := &appsv1.DaemonSet{}
			Expect(testClient.Get(ctx, client.ObjectKey{Namespace: "dpf-operator-system", Name: "host-cni-provisioner"}, got)).To(Succeed())
			got.Status.NumberReady = 1
			got.Status.DesiredNumberScheduled = 1
			got.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DaemonSet"))
			got.ManagedFields = nil
			Expect(testClient.Status().Patch(ctx, got, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			got = &appsv1.DaemonSet{}
			c, err := dpfCluster.NewClient(ctx, testClient)
			Expect(err).ToNot(HaveOccurred())
			Expect(c.Get(ctx, client.ObjectKey{Namespace: "dpf-operator-system", Name: "dpu-cni-provisioner"}, got)).To(Succeed())
			got.Status.NumberReady = 1
			got.Status.DesiredNumberScheduled = 1
			got.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DaemonSet"))
			got.ManagedFields = nil
			Expect(c.Status().Patch(ctx, got, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			Eventually(func(g Gomega) {
				got := &corev1.NodeList{}
				g.Expect(testClient.List(ctx, got, client.MatchingLabels{
					dpuEnabledNodeLabelKey: dpuEnabledNodeLabelValue,
				})).To(Succeed())
				g.Expect(got.Items).To(HaveLen(2))
				for _, node := range got.Items {
					g.Expect(node.Labels).To(HaveKey(networkPreconfigurationReadyNodeLabel))
				}
			}).WithTimeout(30 * time.Second).Should(Succeed())

			By("Checking the deployment of the custom OVN Kubernetes")
			Eventually(func(g Gomega) {
				gotDaemonSet := &appsv1.DaemonSet{}
				key := client.ObjectKey{Namespace: "openshift-ovn-kubernetes", Name: "ovnkube-node-dpf"}
				g.Expect(testClient.Get(ctx, key, gotDaemonSet)).To(Succeed())
				gotConfigMap := &corev1.ConfigMap{}
				key = client.ObjectKey{Namespace: "openshift-ovn-kubernetes", Name: "ovnkube-config-dpf"}
				g.Expect(testClient.Get(ctx, key, gotConfigMap)).To(Succeed())
				gotConfigMap = &corev1.ConfigMap{}
				key = client.ObjectKey{Namespace: "openshift-ovn-kubernetes", Name: "ovnkube-script-lib-dpf"}
				g.Expect(testClient.Get(ctx, key, gotConfigMap)).To(Succeed())
				gotDaemonSet = &appsv1.DaemonSet{}
				key = client.ObjectKey{Namespace: "openshift-ovn-kubernetes", Name: "ovnkube-node"}
				g.Expect(testClient.Get(ctx, key, gotDaemonSet)).To(Succeed())
				g.Expect(gotDaemonSet.Spec.Template.Spec.Containers).To(ContainElement(
					And(
						HaveField("Name", "ovnkube-controller"),
						HaveField("Image", reconciler.Settings.CustomOVNKubernetesNonDPUImage)),
				))
				g.Expect(gotDaemonSet.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf([]corev1.LocalObjectReference{
					{Name: "secret-1"},
					{Name: "secret-2"},
				}))
			}).WithTimeout(30 * time.Second).Should(Succeed())
		})

		It("should not deploy the CNI provisioners if original OVN Kubernetes pods are still running", func() {
			got := &appsv1.DaemonSet{}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(ovnKubernetesDaemonSet), got)).To(Succeed())
			got.Status.NumberMisscheduled = 5
			got.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DaemonSet"))
			got.ManagedFields = nil
			Expect(testClient.Status().Patch(ctx, got, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig(testNS.Name)
			Expect(testClient.Create(ctx, operatorConfig)).To(Succeed())
			// DPF Operator creates objects when reconciling the DPFOVNKubernetesOperatorConfig and we need to ensure that on
			// deletion of these objects there is no DPFOVNKubernetesOperatorConfig in the cluster to trigger recreation of those.
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, operatorConfig)

			Consistently(func(g Gomega) {
				_, _ = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(operatorConfig)})
				got := &appsv1.DaemonSet{}
				key := client.ObjectKey{Namespace: "dpf-operator-system", Name: "host-cni-provisioner"}
				g.Expect(testClient.Get(ctx, key, got)).To(HaveOccurred())
				key = client.ObjectKey{Namespace: "dpf-operator-system", Name: "dpu-cni-provisioner"}
				g.Expect(testClient.Get(ctx, key, got)).To(HaveOccurred())
			}).WithTimeout(5 * time.Second).Should(Succeed())
		})

		// TODO: Consider replacing with unit test when extracting the node labeling into its own function
		It("should not cleanup node chassis id annotation if node network preconfiguration is done", func() {
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(nodeWorker2), nodeWorker2)).To(Succeed())
			nodeWorker2.SetLabels(map[string]string{
				dpuEnabledNodeLabelKey:                dpuEnabledNodeLabelValue,
				networkPreconfigurationReadyNodeLabel: "",
			})
			nodeWorker2.SetAnnotations(map[string]string{
				ovnKubernetesNodeChassisIDAnnotation: "worker-2",
			})
			nodeWorker2.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
			nodeWorker2.ManagedFields = nil
			Expect(testClient.Patch(ctx, nodeWorker2, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig(testNS.Name)
			Expect(testClient.Create(ctx, operatorConfig)).To(Succeed())
			// DPF Operator creates objects when reconciling the DPFOVNKubernetesOperatorConfig and we need to ensure that on
			// deletion of these objects there is no DPFOVNKubernetesOperatorConfig in the cluster to trigger recreation of those.
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, operatorConfig)

			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(nodeWorker2), nodeWorker2)).To(Succeed())
				g.Expect(nodeWorker2.Annotations).To(HaveKey(ovnKubernetesNodeChassisIDAnnotation))
			}).WithTimeout(5 * time.Second).Should(Succeed())

		})
	})
	Context("When checking the output of custom OVN generation functions", func() {
		Context("When checking generateCustomOVNKubernetesDaemonSet()", func() {
			var originalDaemonset appsv1.DaemonSet
			BeforeEach(func() {
				content, err := os.ReadFile("testdata/original/ovnkubernetes-daemonset.yaml")
				Expect(err).ToNot(HaveOccurred())
				manifests, err := utils.BytesToUnstructured(content)
				Expect(err).ToNot(HaveOccurred())
				Expect(manifests).To(HaveLen(1))
				originalDaemonset = appsv1.DaemonSet{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifests[0].Object, &originalDaemonset)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should generate correct object when all expected fields are there", func() {
				operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig("")
				operatorConfig.Spec.ImagePullSecrets = []string{"secret-1", "secret-2"}
				out, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, operatorConfig, "some-image")
				Expect(err).ToNot(HaveOccurred())

				Expect(out.Name).To(Equal("ovnkube-node-dpf"))
				Expect(out.Namespace).To(Equal("openshift-ovn-kubernetes"))
				Expect(out.Spec.Selector.MatchLabels["app"]).To(Equal("ovnkube-node-dpf"))
				Expect(out.Spec.Template.Labels["app"]).To(Equal("ovnkube-node-dpf"))
				Expect(out.Spec.Template.Spec.Affinity).To(Equal(&corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      "dpf.nvidia.com/network-preconfig-ready",
											Operator: corev1.NodeSelectorOpExists,
										},
									},
								},
							},
						},
					},
				},
				))

				Expect(out.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
					Name: "fake-sys",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/dpf/sys",
							Type: ptr.To[corev1.HostPathType](corev1.HostPathDirectory),
						},
					},
				}))

				containers := out.Spec.Template.Spec.Containers
				Expect(containers).To(ContainElement(HaveField("Name", "ovnkube-controller")))
				Expect(containers).To(ContainElement(HaveField("Name", "ovn-controller")))
				for _, c := range containers {
					if c.Name == "ovnkube-controller" {
						Expect(c.VolumeMounts).To(ContainElement(corev1.VolumeMount{
							Name:      "fake-sys",
							MountPath: "/var/dpf/sys",
						}))

						Expect(c.Image).To(Equal("some-image"))
					}
					if c.Name == "ovn-controller" {
						Expect(c.Image).To(Equal("some-image"))
					}
				}
				Expect(out.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf([]corev1.LocalObjectReference{
					{Name: "secret-1"},
					{Name: "secret-2"},
				}))
			})
			It("should error out when label app in selector is not found", func() {
				originalDaemonset.Spec.Selector.MatchLabels = nil
				_, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, getMinimalDPFOVNKubernetesOperatorConfig(""), "")
				Expect(err).To(HaveOccurred())
			})
			It("should error out when label app in pod template is not found", func() {
				originalDaemonset.Spec.Template.Labels = nil
				_, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, getMinimalDPFOVNKubernetesOperatorConfig(""), "")
				Expect(err).To(HaveOccurred())
			})
			It("should error out when OVN Kube Controller container is not found", func() {
				for i, c := range originalDaemonset.Spec.Template.Spec.Containers {
					if c.Name != "ovnkube-controller" {
						continue
					}
					originalDaemonset.Spec.Template.Spec.Containers[i].Name = "other-ovnkube-controller"
					break
				}
				_, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, getMinimalDPFOVNKubernetesOperatorConfig(""), "")
				Expect(err).To(HaveOccurred())
			})
			It("should error out when OVN Controller container is not found", func() {
				for i, c := range originalDaemonset.Spec.Template.Spec.Containers {
					if c.Name != "ovn-controller" {
						continue
					}
					originalDaemonset.Spec.Template.Spec.Containers[i].Name = "other-ovnkube-controller"
					break
				}
				_, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, getMinimalDPFOVNKubernetesOperatorConfig(""), "")
				Expect(err).To(HaveOccurred())
			})
			It("should error out when volume related to configmap ovnkube-config is not found", func() {
				for i, v := range originalDaemonset.Spec.Template.Spec.Volumes {
					if v.Name != "ovnkube-config" {
						continue
					}
					originalDaemonset.Spec.Template.Spec.Volumes[i].Name = "other-ovnkube-config"
					break
				}
				_, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, getMinimalDPFOVNKubernetesOperatorConfig(""), "")
				Expect(err).To(HaveOccurred())
			})
			It("should error out when volume related to configmap ovnkube-script-lib is not found", func() {
				for i, v := range originalDaemonset.Spec.Template.Spec.Volumes {
					if v.Name != "ovnkube-script-lib" {
						continue
					}
					originalDaemonset.Spec.Template.Spec.Volumes[i].Name = "other-ovnkube-script-lib"
					break
				}
				_, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, getMinimalDPFOVNKubernetesOperatorConfig(""), "")
				Expect(err).To(HaveOccurred())
			})
		})
		Context("When checking generateCustomOVNKubernetesConfigMap()", func() {
			var originalConfigMap corev1.ConfigMap
			BeforeEach(func() {
				content, err := os.ReadFile("testdata/original/ovnkubernetes-configmap.yaml")
				Expect(err).ToNot(HaveOccurred())
				manifests, err := utils.BytesToUnstructured(content)
				Expect(err).ToNot(HaveOccurred())
				Expect(manifests).To(HaveLen(1))
				originalConfigMap = corev1.ConfigMap{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifests[0].Object, &originalConfigMap)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should generate correct object", func() {
				out, err := generateCustomOVNKubernetesConfigMap(&originalConfigMap)
				Expect(err).ToNot(HaveOccurred())

				content, err := os.ReadFile("testdata/expected/ovnkubernetes-configmap.yaml")
				Expect(err).ToNot(HaveOccurred())
				manifests, err := utils.BytesToUnstructured(content)
				Expect(err).ToNot(HaveOccurred())
				Expect(manifests).To(HaveLen(1))
				expectedConfigMap := corev1.ConfigMap{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifests[0].Object, &expectedConfigMap)
				Expect(err).ToNot(HaveOccurred())

				Expect(out.Name).To(Equal("ovnkube-config-dpf"))
				Expect(out.Namespace).To(Equal("openshift-ovn-kubernetes"))
				Expect(out.Data).To(BeComparableTo(expectedConfigMap.Data))
			})
			It("should error out when relevant key is not found in configmap", func() {
				delete(originalConfigMap.Data, ovnKubernetesConfigMapDataKey)
				_, err := generateCustomOVNKubernetesConfigMap(&originalConfigMap)
				Expect(err).To(HaveOccurred())
			})
			It("should error out when gateway section is not found in toml", func() {
				originalConfigMap.Data[ovnKubernetesConfigMapDataKey] = strings.ReplaceAll(originalConfigMap.Data[ovnKubernetesConfigMapDataKey], "[gateway]", "[somesection]")
				_, err := generateCustomOVNKubernetesConfigMap(&originalConfigMap)
				Expect(err).To(HaveOccurred())
			})
			It("should error out when default section is not found in toml", func() {
				originalConfigMap.Data[ovnKubernetesConfigMapDataKey] = strings.ReplaceAll(originalConfigMap.Data[ovnKubernetesConfigMapDataKey], "[default]", "[somesection]")
				_, err := generateCustomOVNKubernetesConfigMap(&originalConfigMap)
				Expect(err).To(HaveOccurred())
			})
		})
		Context("When checking generateCustomOVNKubernetesEntrypointConfigMap()", func() {
			var originalConfigMap corev1.ConfigMap
			BeforeEach(func() {
				content, err := os.ReadFile("testdata/original/ovnkubernetes-entrypointconfigmap.yaml")
				Expect(err).ToNot(HaveOccurred())
				manifests, err := utils.BytesToUnstructured(content)
				Expect(err).ToNot(HaveOccurred())
				Expect(manifests).To(HaveLen(1))
				originalConfigMap = corev1.ConfigMap{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifests[0].Object, &originalConfigMap)
				Expect(err).ToNot(HaveOccurred())
			})
			It("should generate correct object", func() {
				operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig("dpf-operator-system")
				operatorConfig.Spec.HostPF0 = "ens27f0np0"
				operatorConfig.Spec.Hosts = []ovnkubernetesoperatorv1.Host{
					{
						HostClusterNodeName: "ocp-node-1",
						HostIP:              "10.0.96.10/24",
						DPUIP:               "10.0.96.20/24",
						Gateway:             "10.0.96.254",
					},
					{
						HostClusterNodeName: "ocp-node-2",
						HostIP:              "10.0.97.10/24",
						DPUIP:               "10.0.97.20/24",
						Gateway:             "10.0.97.254",
					},
					// We don't expect this node to be included in the config because the node doesn't have the expected
					// label
					{
						HostClusterNodeName: "ocp-node-3",
						HostIP:              "10.0.98.10/24",
						DPUIP:               "10.0.98.20/24",
						Gateway:             "10.0.98.254",
					},
					// We don't expect this node to be included in the config because it doesn't exist in the cluster
					{
						HostClusterNodeName: "ocp-node-4",
						HostIP:              "10.0.99.10/24",
						DPUIP:               "10.0.99.20/24",
						Gateway:             "10.0.99.254",
					},
				}

				hostNodes := []corev1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "ocp-node-1", Labels: map[string]string{"feature.node.kubernetes.io/dpu.features-dpu-pf-name": "ens1f0np0"}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "ocp-node-2", Labels: map[string]string{"feature.node.kubernetes.io/dpu.features-dpu-pf-name": "ens27f0np0"}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "ocp-node-3"}},
				}

				out, err := generateCustomOVNKubernetesEntrypointConfigMap(&originalConfigMap, operatorConfig, hostNodes)
				Expect(err).ToNot(HaveOccurred())

				content, err := os.ReadFile("testdata/expected/ovnkubernetes-entrypointconfigmap.yaml")
				Expect(err).ToNot(HaveOccurred())
				manifests, err := utils.BytesToUnstructured(content)
				Expect(err).ToNot(HaveOccurred())
				Expect(manifests).To(HaveLen(1))
				expectedConfigMap := corev1.ConfigMap{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifests[0].Object, &expectedConfigMap)
				Expect(err).ToNot(HaveOccurred())

				Expect(out.Name).To(Equal("ovnkube-script-lib-dpf"))
				Expect(out.Namespace).To(Equal("openshift-ovn-kubernetes"))
				Expect(out.Data).To(BeComparableTo(expectedConfigMap.Data))
				Expect(out.Immutable).To(BeComparableTo(expectedConfigMap.Immutable))
				Expect(out.BinaryData).To(BeComparableTo(expectedConfigMap.BinaryData))
			})
			It("should error out when relevant key is not found in configmap", func() {
				delete(originalConfigMap.Data, ovnKubernetesEntrypointConfigMapScriptKey)
				_, err := generateCustomOVNKubernetesEntrypointConfigMap(&originalConfigMap, getMinimalDPFOVNKubernetesOperatorConfig(""), []corev1.Node{})
				Expect(err).To(HaveOccurred())
			})
		})
		Context("When checking generateDPUCNIProvisionerObjects()", func() {
			DescribeTable("should pass the DPFOVNKubernetesOperatorConfig settings to correct configmap", func(
				operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig,
				nodes []corev1.Node,
				expectedDPUCNIProvisionerConfig dpucniprovisionerconfig.DPUCNIProvisionerConfig,
			) {
				clusterConfigContent, err := os.ReadFile("testdata/original/cluster-config-v1-configmap.yaml")
				Expect(err).ToNot(HaveOccurred())
				clusterConfigUnstructured, err := utils.BytesToUnstructured(clusterConfigContent)
				Expect(err).ToNot(HaveOccurred())
				Expect(clusterConfigUnstructured).To(HaveLen(1))
				var openshiftClusterConfigMap corev1.ConfigMap
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(clusterConfigUnstructured[0].Object, &openshiftClusterConfigMap)
				Expect(err).ToNot(HaveOccurred())

				objects, err := generateDPUCNIProvisionerObjects(operatorConfig, &openshiftClusterConfigMap, nodes)
				Expect(err).ToNot(HaveOccurred())

				rawObjects, err := utils.BytesToUnstructured(dpuCNIProvisionerManifestContent)
				Expect(err).ToNot(HaveOccurred())

				Expect(objects).To(HaveLen(len(rawObjects)))

				var found bool
				for _, o := range objects {
					if !(o.GetKind() == "ConfigMap" && o.GetName() == "dpu-cni-provisioner") {
						continue
					}
					var configMap corev1.ConfigMap
					err = runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, &configMap)
					Expect(err).ToNot(HaveOccurred())
					Expect(configMap.Data).To(HaveKey("config.yaml"))

					var outDPUCNIProvisionerConfig dpucniprovisionerconfig.DPUCNIProvisionerConfig
					Expect(json.Unmarshal([]byte(configMap.Data["config.yaml"]), &outDPUCNIProvisionerConfig)).To(Succeed())
					Expect(outDPUCNIProvisionerConfig).To(BeComparableTo(expectedDPUCNIProvisionerConfig))
					found = true
				}
				Expect(found).To(BeTrue())

			},
				Entry("normal case", func() *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig {
					operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig("")
					operatorConfig.Spec.Hosts = []ovnkubernetesoperatorv1.Host{
						{
							HostClusterNodeName: "worker1",
							DPUIP:               "10.0.96.20/24",
							Gateway:             "10.0.96.254",
						},
						{
							HostClusterNodeName: "worker2",
							DPUIP:               "10.0.97.20/24",
							Gateway:             "10.0.97.254",
						},
					}
					operatorConfig.Spec.CIDR = "10.0.96.0/20"
					return operatorConfig
				}(),
					[]corev1.Node{
						{ObjectMeta: metav1.ObjectMeta{Name: "worker1", Labels: map[string]string{"feature.node.kubernetes.io/dpu.features-dpu-pciAddress": "0000-90-00"}}},
						{ObjectMeta: metav1.ObjectMeta{Name: "worker2", Labels: map[string]string{"feature.node.kubernetes.io/dpu.features-dpu-pciAddress": "0000-30-00"}}},
					},
					dpucniprovisionerconfig.DPUCNIProvisionerConfig{
						PerNodeConfig: map[string]dpucniprovisionerconfig.PerNodeConfig{
							"worker1-0000-90-00": {
								VTEPIP:  "10.0.96.20/24",
								Gateway: "10.0.96.254",
							},
							"worker2-0000-30-00": {
								VTEPIP:  "10.0.97.20/24",
								Gateway: "10.0.97.254",
							},
						},
						VTEPCIDR: "10.0.96.0/20",
						HostCIDR: "10.0.110.0/24",
					}),
				Entry("node doesn't have pci label", func() *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig {
					operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig("")
					operatorConfig.Spec.Hosts = []ovnkubernetesoperatorv1.Host{
						{
							HostClusterNodeName: "worker1",
							DPUIP:               "10.0.96.20/24",
							Gateway:             "10.0.96.254",
						},
						{
							HostClusterNodeName: "worker2",
							DPUIP:               "10.0.97.20/24",
							Gateway:             "10.0.97.254",
						},
					}
					operatorConfig.Spec.CIDR = "10.0.96.0/20"
					return operatorConfig
				}(),
					[]corev1.Node{
						{ObjectMeta: metav1.ObjectMeta{Name: "worker1", Labels: map[string]string{"feature.node.kubernetes.io/dpu.features-dpu-pciAddress": "0000-90-00"}}},
						{ObjectMeta: metav1.ObjectMeta{Name: "worker2"}},
					},
					dpucniprovisionerconfig.DPUCNIProvisionerConfig{
						PerNodeConfig: map[string]dpucniprovisionerconfig.PerNodeConfig{
							"worker1-0000-90-00": {
								VTEPIP:  "10.0.96.20/24",
								Gateway: "10.0.96.254",
							},
						},
						VTEPCIDR: "10.0.96.0/20",
						HostCIDR: "10.0.110.0/24",
					}),
				Entry("node not mentioned in the DPFOVNKubernetesOperatorConfig", func() *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig {
					operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig("")
					operatorConfig.Spec.Hosts = []ovnkubernetesoperatorv1.Host{
						{
							HostClusterNodeName: "worker1",
							DPUIP:               "10.0.96.20/24",
							Gateway:             "10.0.96.254",
						},
					}
					operatorConfig.Spec.CIDR = "10.0.96.0/20"
					return operatorConfig
				}(),
					[]corev1.Node{
						{ObjectMeta: metav1.ObjectMeta{Name: "worker1", Labels: map[string]string{"feature.node.kubernetes.io/dpu.features-dpu-pciAddress": "0000-90-00"}}},
						{ObjectMeta: metav1.ObjectMeta{Name: "worker2", Labels: map[string]string{"feature.node.kubernetes.io/dpu.features-dpu-pciAddress": "0000-30-00"}}},
					},
					dpucniprovisionerconfig.DPUCNIProvisionerConfig{
						PerNodeConfig: map[string]dpucniprovisionerconfig.PerNodeConfig{
							"worker1-0000-90-00": {
								VTEPIP:  "10.0.96.20/24",
								Gateway: "10.0.96.254",
							},
						},
						VTEPCIDR: "10.0.96.0/20",
						HostCIDR: "10.0.110.0/24",
					}),
			)
			It("should error out when provisioner configmap is not found", func() {
				By("Copying the dpuCNIProvisionerManifestContent global variable")
				dpuCNIProvisionerManifestContentLength := len(dpuCNIProvisionerManifestContent)
				dpuCNIProvisionerManifestContentCopy := make([]byte, dpuCNIProvisionerManifestContentLength)
				copy(dpuCNIProvisionerManifestContentCopy, dpuCNIProvisionerManifestContent)

				operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig("")

				dpuCNIProvisionerManifestContent = []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
        `)

				clusterConfigContent, err := os.ReadFile("testdata/original/cluster-config-v1-configmap.yaml")
				Expect(err).ToNot(HaveOccurred())
				clusterConfigUnstructured, err := utils.BytesToUnstructured(clusterConfigContent)
				Expect(err).ToNot(HaveOccurred())
				Expect(clusterConfigUnstructured).To(HaveLen(1))
				var openshiftClusterConfigMap corev1.ConfigMap
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(clusterConfigUnstructured[0].Object, &openshiftClusterConfigMap)
				Expect(err).ToNot(HaveOccurred())

				By("Running the test against the mocked environment")
				_, err = generateDPUCNIProvisionerObjects(operatorConfig, &openshiftClusterConfigMap, nil)
				Expect(err).To(HaveOccurred())

				By("Reverting dpuCNIProvisionerManifestContent global variable to the original value")
				dpuCNIProvisionerManifestContent = make([]byte, dpuCNIProvisionerManifestContentLength)
				copy(dpuCNIProvisionerManifestContent, dpuCNIProvisionerManifestContentCopy)
			})
			It("should pass imagePullSecrets specified in DPFOVNKubernetesOperatorConfig to the DaemonSet", func() {
				operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig("")
				operatorConfig.Spec.ImagePullSecrets = []string{"secret-1", "secret-2"}

				clusterConfigContent, err := os.ReadFile("testdata/original/cluster-config-v1-configmap.yaml")
				Expect(err).ToNot(HaveOccurred())
				clusterConfigUnstructured, err := utils.BytesToUnstructured(clusterConfigContent)
				Expect(err).ToNot(HaveOccurred())
				Expect(clusterConfigUnstructured).To(HaveLen(1))
				var openshiftClusterConfigMap corev1.ConfigMap
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(clusterConfigUnstructured[0].Object, &openshiftClusterConfigMap)
				Expect(err).ToNot(HaveOccurred())

				objects, err := generateDPUCNIProvisionerObjects(operatorConfig, &openshiftClusterConfigMap, nil)
				Expect(err).ToNot(HaveOccurred())

				rawObjects, err := utils.BytesToUnstructured(dpuCNIProvisionerManifestContent)
				Expect(err).ToNot(HaveOccurred())

				Expect(objects).To(HaveLen(len(rawObjects)))

				var found bool
				for _, o := range objects {
					if !(o.GetKind() == "DaemonSet" && o.GetName() == "dpu-cni-provisioner") {
						continue
					}
					var daemonset appsv1.DaemonSet
					err = runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, &daemonset)
					Expect(err).ToNot(HaveOccurred())
					Expect(daemonset.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf([]corev1.LocalObjectReference{
						{Name: "secret-1"},
						{Name: "secret-2"},
					}))

					found = true
				}
				Expect(found).To(BeTrue())
			})
		})
		Context("When checking generateHostCNIProvisionerObjects()", func() {
			It("should pass the DPFOVNKubernetesOperatorConfig settings to correct configmap", func() {
				operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig("")
				operatorConfig.Spec.Hosts = []ovnkubernetesoperatorv1.Host{
					{
						HostClusterNodeName: "ocp-node-1",
						HostIP:              "192.168.1.10/24",
					},
					{
						HostClusterNodeName: "ocp-node-2",
						HostIP:              "192.168.1.20/24",
					},
				}
				operatorConfig.Spec.HostPF0 = "ens27f0np0"

				expectedHostCNIProvisionerConfig := hostcniprovisionerconfig.HostCNIProvisionerConfig{
					PFIPs: map[string]string{
						"ocp-node-1": "192.168.1.10/24",
						"ocp-node-2": "192.168.1.20/24",
					},
					HostPF0: "ens27f0np0",
				}

				objects, err := generateHostCNIProvisionerObjects(operatorConfig)
				Expect(err).ToNot(HaveOccurred())

				rawObjects, err := utils.BytesToUnstructured(hostCNIProvisionerManifestContent)
				Expect(err).ToNot(HaveOccurred())

				Expect(objects).To(HaveLen(len(rawObjects)))

				var found bool
				for _, o := range objects {
					if !(o.GetKind() == "ConfigMap" && o.GetName() == "host-cni-provisioner") {
						continue
					}
					var configMap corev1.ConfigMap
					err = runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, &configMap)
					Expect(err).ToNot(HaveOccurred())
					Expect(configMap.Data).To(HaveKey("config.yaml"))

					var outHostCNIProvisionerConfig hostcniprovisionerconfig.HostCNIProvisionerConfig
					Expect(json.Unmarshal([]byte(configMap.Data["config.yaml"]), &outHostCNIProvisionerConfig)).To(Succeed())
					Expect(outHostCNIProvisionerConfig).To(BeComparableTo(expectedHostCNIProvisionerConfig))
					found = true
				}
				Expect(found).To(BeTrue())
			})
			It("should error out when configmap is not found", func() {
				By("Copying the hostCNIProvisionerManifestContent global variable")
				hostCNIProvisionerManifestContentLength := len(hostCNIProvisionerManifestContent)
				hostCNIProvisionerManifestContentCopy := make([]byte, hostCNIProvisionerManifestContentLength)
				copy(hostCNIProvisionerManifestContentCopy, hostCNIProvisionerManifestContent)

				operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig("")

				hostCNIProvisionerManifestContent = []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
        `)

				By("Running the test against the mocked environment")
				_, err := generateHostCNIProvisionerObjects(operatorConfig)
				Expect(err).To(HaveOccurred())

				By("Reverting hostCNIProvisionerManifestContent global variable to the original value")
				hostCNIProvisionerManifestContent = make([]byte, hostCNIProvisionerManifestContentLength)
				copy(hostCNIProvisionerManifestContent, hostCNIProvisionerManifestContentCopy)
			})
			It("should pass imagePullSecrets specified in DPFOVNKubernetesOperatorConfig to the DaemonSet", func() {
				operatorConfig := getMinimalDPFOVNKubernetesOperatorConfig("")
				operatorConfig.Spec.ImagePullSecrets = []string{"secret-1", "secret-2"}

				objects, err := generateHostCNIProvisionerObjects(operatorConfig)
				Expect(err).ToNot(HaveOccurred())

				rawObjects, err := utils.BytesToUnstructured(hostCNIProvisionerManifestContent)
				Expect(err).ToNot(HaveOccurred())

				Expect(objects).To(HaveLen(len(rawObjects)))

				var found bool
				for _, o := range objects {
					if !(o.GetKind() == "DaemonSet" && o.GetName() == "host-cni-provisioner") {
						continue
					}
					var daemonset appsv1.DaemonSet
					err = runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, &daemonset)
					Expect(err).ToNot(HaveOccurred())
					Expect(daemonset.Spec.Template.Spec.ImagePullSecrets).To(ConsistOf([]corev1.LocalObjectReference{{Name: "secret-1"}, {Name: "secret-2"}}))
					found = true
				}
				Expect(found).To(BeTrue())
			})
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

func getDefaultClusterVersion(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "config.openshift.io/v1",
			"kind":       "ClusterVersion",
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"clusterID": "9a3d8fa9-773a-4d99-a80a-5ddac0956d95",
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

// getMinimalDPFOVNKubernetesOperatorConfig returns a minimal DPFOVNKubernetesOperatorConfig
func getMinimalDPFOVNKubernetesOperatorConfig(namespace string) *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig {
	return &ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: namespace,
		},
		Spec: ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfigSpec{
			Hosts: []ovnkubernetesoperatorv1.Host{},
		},
	}
}
