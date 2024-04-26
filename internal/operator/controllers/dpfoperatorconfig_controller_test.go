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
	"fmt"
	"os"
	"strings"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"
	dpucniprovisionerconfig "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/dpu/config"
	hostcniprovisionerconfig "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/host/config"
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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//nolint:dupl
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

			dpfOperatorConfig := getMinimalDPFOperatorConfig(testNS.Name)
			Expect(testClient.Create(ctx, dpfOperatorConfig)).To(Succeed())
			// DPF Operator creates objects when reconciling the DPFOperatorConfig and we need to ensure that on
			// deletion of these objects there is no DPFOperatorConfig in the cluster to trigger recreation of those.
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpfOperatorConfig)

			Eventually(func(g Gomega) []string {
				gotConfig := &operatorv1.DPFOperatorConfig{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpfOperatorConfig), gotConfig)).To(Succeed())
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
		var nodeWorker1 *corev1.Node
		var nodeWorker2 *corev1.Node
		var nodeControlPlane1 *corev1.Node

		BeforeEach(func() {
			By("Creating the namespace")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			cleanupObjects = []client.Object{}

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
				workerNodeLabel: "",
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
				workerNodeLabel: "",
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
			dpfOperatorConfig := getMinimalDPFOperatorConfig(testNS.Name)
			Expect(testClient.Create(ctx, dpfOperatorConfig)).To(Succeed())
			// DPF Operator creates objects when reconciling the DPFOperatorConfig and we need to ensure that on
			// deletion of these objects there is no DPFOperatorConfig in the cluster to trigger recreation of those.
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpfOperatorConfig)

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
						g.Expect(node.Annotations).NotTo(HaveKey(ovnKubernetesNodeChassisIDAnnotation))
					}
				}
				g.Expect(workerCounter).To(Equal(2))
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
				g.Expect(testClient.List(ctx, got)).To(Succeed())
				workerCounter := 0
				for _, node := range got.Items {
					if _, ok := node.Labels[workerNodeLabel]; ok {
						workerCounter++
						g.Expect(node.Labels).To(HaveKey(networkPreconfigurationReadyNodeLabel))
					}
				}
				g.Expect(workerCounter).To(Equal(2))
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
			}).WithTimeout(30 * time.Second).Should(Succeed())
		})

		It("should not deploy the CNI provisioners if original OVN Kubernetes pods are still running", func() {
			got := &appsv1.DaemonSet{}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(ovnKubernetesDaemonSet), got)).To(Succeed())
			got.Status.NumberMisscheduled = 5
			got.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DaemonSet"))
			got.ManagedFields = nil
			Expect(testClient.Status().Patch(ctx, got, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			dpfOperatorConfig := getMinimalDPFOperatorConfig(testNS.Name)
			Expect(testClient.Create(ctx, dpfOperatorConfig)).To(Succeed())
			// DPF Operator creates objects when reconciling the DPFOperatorConfig and we need to ensure that on
			// deletion of these objects there is no DPFOperatorConfig in the cluster to trigger recreation of those.
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpfOperatorConfig)

			Consistently(func(g Gomega) {
				_, _ = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(dpfOperatorConfig)})
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
				workerNodeLabel:                       "",
				networkPreconfigurationReadyNodeLabel: "",
			})
			nodeWorker2.SetAnnotations(map[string]string{
				ovnKubernetesNodeChassisIDAnnotation: "worker-2",
			})
			nodeWorker2.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
			nodeWorker2.ManagedFields = nil
			Expect(testClient.Patch(ctx, nodeWorker2, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			dpfOperatorConfig := getMinimalDPFOperatorConfig(testNS.Name)
			Expect(testClient.Create(ctx, dpfOperatorConfig)).To(Succeed())
			// DPF Operator creates objects when reconciling the DPFOperatorConfig and we need to ensure that on
			// deletion of these objects there is no DPFOperatorConfig in the cluster to trigger recreation of those.
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpfOperatorConfig)

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
				dpfOperatorConfig := getMinimalDPFOperatorConfig("")
				dpfOperatorConfig.Spec.HostNetworkConfiguration.HostPF0VF0 = "enp75s0f0v0"
				out, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, dpfOperatorConfig, "some-image")
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

						Expect(c.Env).To(ContainElement(corev1.EnvVar{
							Name:  "OVNKUBE_NODE_MGMT_PORT_NETDEV",
							Value: "enp75s0f0v0",
						}))
						Expect(c.Image).To(Equal("some-image"))
					}
					if c.Name == "ovn-controller" {
						Expect(c.Image).To(Equal("some-image"))
					}
				}
			})
			It("should error out when label app in selector is not found", func() {
				originalDaemonset.Spec.Selector.MatchLabels = nil
				_, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, getMinimalDPFOperatorConfig(""), "")
				Expect(err).To(HaveOccurred())
			})
			It("should error out when label app in pod template is not found", func() {
				originalDaemonset.Spec.Template.Labels = nil
				_, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, getMinimalDPFOperatorConfig(""), "")
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
				_, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, getMinimalDPFOperatorConfig(""), "")
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
				_, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, getMinimalDPFOperatorConfig(""), "")
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
				_, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, getMinimalDPFOperatorConfig(""), "")
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
				_, err := generateCustomOVNKubernetesDaemonSet(&originalDaemonset, getMinimalDPFOperatorConfig(""), "")
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
				dpfOperatorConfig := getMinimalDPFOperatorConfig("dpf-operator-system")
				dpfOperatorConfig.Name = "dpfoperatorconfig"
				dpfOperatorConfig.Spec.HostNetworkConfiguration.HostPF0 = "ens27f0np0"
				dpfOperatorConfig.Spec.HostNetworkConfiguration.Hosts = []operatorv1.Host{
					{
						HostClusterNodeName: "ocp-node-1",
						DPUClusterNodeName:  "dpu-node-1",
						HostIP:              "10.0.96.10/24",
						DPUIP:               "10.0.96.20/24",
						Gateway:             "10.0.96.254",
					},
					{
						HostClusterNodeName: "ocp-node-2",
						DPUClusterNodeName:  "dpu-node-2",
						HostIP:              "10.0.97.10/24",
						DPUIP:               "10.0.97.20/24",
						Gateway:             "10.0.97.254",
					},
				}

				out, err := generateCustomOVNKubernetesEntrypointConfigMap(&originalConfigMap, dpfOperatorConfig)
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
				_, err := generateCustomOVNKubernetesEntrypointConfigMap(&originalConfigMap, getMinimalDPFOperatorConfig(""))
				Expect(err).To(HaveOccurred())
			})
		})
		Context("When checking generateDPUCNIProvisionerObjects()", func() {
			It("should pass the DPFOperatorConfig settings to correct configmap", func() {
				dpfOperatorConfig := getMinimalDPFOperatorConfig("")
				dpfOperatorConfig.Spec.HostNetworkConfiguration.Hosts = []operatorv1.Host{
					{
						DPUClusterNodeName: "dpu-node-1",
						DPUIP:              "10.0.96.20/24",
						Gateway:            "10.0.96.254",
					},
					{
						DPUClusterNodeName: "dpu-node-2",
						DPUIP:              "10.0.97.20/24",
						Gateway:            "10.0.97.254",
					},
				}

				dpfOperatorConfig.Spec.HostNetworkConfiguration.CIDR = "10.0.96.0/20"

				clusterConfigContent, err := os.ReadFile("testdata/original/cluster-config-v1-configmap.yaml")
				Expect(err).ToNot(HaveOccurred())
				clusterConfigUnstructured, err := utils.BytesToUnstructured(clusterConfigContent)
				Expect(err).ToNot(HaveOccurred())
				Expect(clusterConfigUnstructured).To(HaveLen(1))
				var openshiftClusterConfigMap corev1.ConfigMap
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(clusterConfigUnstructured[0].Object, &openshiftClusterConfigMap)
				Expect(err).ToNot(HaveOccurred())

				expectedDPUCNIProvisionerConfig := dpucniprovisionerconfig.DPUCNIProvisionerConfig{
					PerNodeConfig: map[string]dpucniprovisionerconfig.PerNodeConfig{
						"dpu-node-1": {
							VTEPIP:  "10.0.96.20/24",
							Gateway: "10.0.96.254",
						},
						"dpu-node-2": {
							VTEPIP:  "10.0.97.20/24",
							Gateway: "10.0.97.254",
						},
					},
					VTEPCIDR: "10.0.96.0/20",
					HostCIDR: "10.0.110.0/24",
				}

				objects, err := generateDPUCNIProvisionerObjects(dpfOperatorConfig, &openshiftClusterConfigMap)
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
			})
			It("should error out when provisioner configmap is not found", func() {
				By("Copying the dpuCNIProvisionerManifestContent global variable")
				var dpuCNIProvisionerManifestContentCopy []byte
				copy(dpuCNIProvisionerManifestContentCopy, dpuCNIProvisionerManifestContent)

				dpfOperatorConfig := getMinimalDPFOperatorConfig("")

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
				_, err = generateDPUCNIProvisionerObjects(dpfOperatorConfig, &openshiftClusterConfigMap)
				Expect(err).To(HaveOccurred())

				By("Reverting dpuCNIProvisionerManifestContent global variable to the original value")
				copy(dpuCNIProvisionerManifestContent, dpuCNIProvisionerManifestContentCopy)
			})
		})
		Context("When checking generateHostCNIProvisionerObjects()", func() {
			It("should pass the DPFOperatorConfig settings to correct configmap", func() {
				dpfOperatorConfig := getMinimalDPFOperatorConfig("")
				dpfOperatorConfig.Spec.HostNetworkConfiguration.Hosts = []operatorv1.Host{
					{
						HostClusterNodeName: "ocp-node-1",
						HostIP:              "192.168.1.10/24",
					},
					{
						HostClusterNodeName: "ocp-node-2",
						HostIP:              "192.168.1.20/24",
					},
				}

				expectedHostCNIProvisionerConfig := hostcniprovisionerconfig.HostCNIProvisionerConfig{
					PFIPs: map[string]string{
						"ocp-node-1": "192.168.1.10/24",
						"ocp-node-2": "192.168.1.20/24",
					},
				}

				objects, err := generateHostCNIProvisionerObjects(dpfOperatorConfig)
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
				var hostCNIProvisionerManifestContentCopy []byte
				copy(hostCNIProvisionerManifestContentCopy, hostCNIProvisionerManifestContent)

				dpfOperatorConfig := getMinimalDPFOperatorConfig("")

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
				_, err := generateHostCNIProvisionerObjects(dpfOperatorConfig)
				Expect(err).To(HaveOccurred())

				By("Reverting hostCNIProvisionerManifestContent global variable to the original value")
				copy(hostCNIProvisionerManifestContent, hostCNIProvisionerManifestContentCopy)
			})
		})
	})
})

var _ = Describe("DPFOperator controller settings", func() {
	Context("When setting up the controller", func() {
		It("Should restrict reconciliation to a specific namespace and name when ConfigSingletonNamespaceName is set", func() {
			s := scheme.Scheme
			Expect(operatorv1.AddToScheme(scheme.Scheme)).To(Succeed())
			singletonReconciler := &DPFOperatorConfigReconciler{
				Client: testClient,
				Scheme: s,
				Settings: &DPFOperatorConfigReconcilerSettings{
					ConfigSingletonNamespaceName: &types.NamespacedName{
						Namespace: "one-namespace",
						Name:      "one-name",
					},
				},
			}
			_, err := singletonReconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "one-namespace",
					Name:      "one-name",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = singletonReconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "different-namespace",
					Name:      "different-name",
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("only one object"))
		})
		It("Should allow reconciliation in any namespace and name when ConfigSingletonNamespaceName is unset", func() {
			s := scheme.Scheme
			Expect(operatorv1.AddToScheme(scheme.Scheme)).To(Succeed())
			unrestrictedReconciler := &DPFOperatorConfigReconciler{
				Client: testClient,
				Scheme: s,
				Settings: &DPFOperatorConfigReconcilerSettings{
					ConfigSingletonNamespaceName: nil,
				},
			}
			_, err := unrestrictedReconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "one-namespace",
					Name:      "one-name",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = unrestrictedReconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "different-namespace",
					Name:      "different-name",
				},
			})
			Expect(err).NotTo(HaveOccurred())
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

// getMinimalDPFOperatorConfig returns a minimal DPFOperatorConfig
func getMinimalDPFOperatorConfig(namespace string) *operatorv1.DPFOperatorConfig {
	return &operatorv1.DPFOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: namespace,
		},
		Spec: operatorv1.DPFOperatorConfigSpec{
			HostNetworkConfiguration: operatorv1.HostNetworkConfiguration{
				Hosts: []operatorv1.Host{},
			},
			ProvisioningConfiguration: operatorv1.ProvisioningConfiguration{
				BFBPVCName:      "foo-pvc",
				ImagePullSecret: "foo-image-pull-secret",
			},
		},
	}
}

func waitForDeployment(ns, name string) *appsv1.Deployment {
	deployment := &appsv1.Deployment{}
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKey{
			Namespace: ns,
			Name:      name},
			deployment)).To(Succeed())
	}).WithTimeout(30 * time.Second).Should(Succeed())
	return deployment
}

var _ = Describe("DPFOperatorConfig Controller", func() {
	Context("controller should create DPF System components", func() {
		var testNS *corev1.Namespace
		BeforeEach(func() {
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
		})
		It("reconciles DPUService controller", func() {
			config := getMinimalDPFOperatorConfig(testNS.Name)
			Expect(testClient.Create(ctx, config)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, config)

			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{
					Namespace: config.Namespace,
					Name:      "dpuservice-controller-manager"},
					deployment)).To(Succeed())
			}).WithTimeout(30 * time.Second).Should(Succeed())
		})

		verifyPVC := func(deployment *appsv1.Deployment, expected string) {
			var bfbPvc *corev1.PersistentVolumeClaimVolumeSource
			for _, vol := range deployment.Spec.Template.Spec.Volumes {
				if vol.Name == "bfb-volume" && vol.PersistentVolumeClaim != nil {
					bfbPvc = vol.PersistentVolumeClaim
					break
				}
			}
			if bfbPvc == nil {
				Fail("no pvc volume found")
			}
			Expect(bfbPvc.ClaimName).To(Equal(expected))
		}

		It("reconciles dpf-provisioning-controller: set bfb PVC", func() {
			config := getMinimalDPFOperatorConfig(testNS.Name)
			config.Spec.ProvisioningConfiguration.BFBPVCName = "foo-pvc"
			config.Spec.ProvisioningConfiguration.ImagePullSecret = "foo-image-pull-secret"
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, config)
			Expect(testClient.Create(ctx, config)).To(Succeed())

			deployment := waitForDeployment("dpf-provisioning", "dpf-provisioning-controller-manager")
			verifyPVC(deployment, "foo-pvc")
		})

		It("reconciles ServiceFunctionChainSet controllers as DPUServices", func() {
			config := getMinimalDPFOperatorConfig(testNS.Name)
			Expect(testClient.Create(ctx, config)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, config)
			Eventually(func(g Gomega) {
				dpuservice := &dpuservicev1.DPUService{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{
					Namespace: config.Namespace,
					Name:      "servicefunctionchainset-controller"},
					dpuservice)).To(Succeed())
			}).WithTimeout(30 * time.Second).Should(Succeed())
		})
	})
})
