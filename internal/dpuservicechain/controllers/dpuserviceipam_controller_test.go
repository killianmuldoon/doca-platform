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

package controllers

import (
	"time"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"
	nvipamv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/nvipam/api/v1alpha1"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//nolint:goconst
var _ = Describe("DPUServiceIPAM Controller", func() {
	Context("When reconciling a resource", func() {
		var testNS *corev1.Namespace
		BeforeEach(func() {
			By("Creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			DeferCleanup(testClient.Delete, ctx, testNS)
		})
		It("should successfully reconcile the DPUServiceIPAM", func() {
			By("Reconciling the created resource")
			dpuServiceIPAM := getMinimalDPUServiceIPAM(testNS.Name)
			Expect(testClient.Create(ctx, dpuServiceIPAM)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceIPAM)

			By("checking that finalizer is added")
			Eventually(func(g Gomega) []string {
				got := &sfcv1.DPUServiceIPAM{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuServiceIPAM), got)).To(Succeed())
				return got.Finalizers
			}).WithTimeout(10 * time.Second).Should(ConsistOf([]string{sfcv1.DPUServiceIPAMFinalizer}))
		})
	})
	Context("When checking the behavior on the DPU cluster ", func() {
		var testNS *corev1.Namespace
		var dpfClusterClient client.Client
		BeforeEach(func() {
			By("Creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			DeferCleanup(testClient.Delete, ctx, testNS)

			By("Adding fake kamaji cluster")
			dpfCluster := controlplane.DPFCluster{Name: "envtest", Namespace: testNS.Name}
			kamajiSecret, err := testutils.GetFakeKamajiClusterSecretFromEnvtest(dpfCluster, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(testClient.Create(ctx, kamajiSecret)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, kamajiSecret)
			dpfClusterClient, err = dpfCluster.NewClient(ctx, testClient)
			Expect(err).ToNot(HaveOccurred())
		})
		It("should reconcile NVIPAM IPPool in DPU cluster when ipv4Subnet is set", func() {
			By("Creating the DPUServiceIPAM resource")
			dpuServiceIPAM := getMinimalDPUServiceIPAM(testNS.Name)
			dpuServiceIPAM.Name = "pool-1"
			dpuServiceIPAM.Spec.IPV4Subnet = &sfcv1.IPV4Subnet{
				Subnet:         "192.168.0.0/20",
				Gateway:        "192.168.0.1",
				PerNodeIPCount: 256,
			}
			dpuServiceIPAM.Spec.NodeSelector = &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "some-key",
								Operator: corev1.NodeSelectorOpExists,
							},
						},
					},
				},
			}

			Expect(testClient.Create(ctx, dpuServiceIPAM)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceIPAM)

			Eventually(func(g Gomega) {
				got := &nvipamv1.IPPool{}
				g.Expect(dpfClusterClient.Get(ctx, client.ObjectKey{Namespace: testNS.Name, Name: "pool-1"}, got)).To(Succeed())
				g.Expect(got.Labels).To(HaveKeyWithValue("dpf.nvidia.com/dpuserviceipam-name", "pool-1"))
				g.Expect(got.Labels).To(HaveKeyWithValue("dpf.nvidia.com/dpuserviceipam-namespace", testNS.Name))
				g.Expect(got.Spec.Subnet).To(Equal("192.168.0.0/20"))
				g.Expect(got.Spec.PerNodeBlockSize).To(Equal(256))
				g.Expect(got.Spec.Gateway).To(Equal("192.168.0.1"))
				g.Expect(got.Spec.NodeSelector).To(BeComparableTo(&corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "some-key",
									Operator: corev1.NodeSelectorOpExists,
								},
							},
						},
					},
				}))
			}).WithTimeout(10 * time.Second).Should(Succeed())

			By("Removing the DPUServiceIPAM resource")
			Expect(testClient.Delete(ctx, dpuServiceIPAM)).To(Succeed())

			Eventually(func(g Gomega) {
				got := &nvipamv1.IPPoolList{}
				g.Expect(dpfClusterClient.List(ctx, got)).To(Succeed())
				g.Expect(got.Items).To(BeEmpty())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})
		It("should delete only the NVIPAM IPPool related to the DPUServiceIPAM that is deleted", func() {
			By("Creating 2 DPUServiceIPAM resources")
			dpuServiceIPAMOne := getMinimalDPUServiceIPAM(testNS.Name)
			dpuServiceIPAMOne.Name = "resource-1"
			dpuServiceIPAMOne.Spec.IPV4Subnet = &sfcv1.IPV4Subnet{
				Subnet:         "192.168.0.0/20",
				Gateway:        "192.168.0.1",
				PerNodeIPCount: 256,
			}
			Expect(testClient.Create(ctx, dpuServiceIPAMOne)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceIPAMOne)

			dpuServiceIPAMTwo := getMinimalDPUServiceIPAM(testNS.Name)
			dpuServiceIPAMTwo.Name = "resource-2"
			dpuServiceIPAMTwo.Spec.IPV4Subnet = &sfcv1.IPV4Subnet{
				Subnet:         "192.168.32.0/20",
				Gateway:        "192.168.32.1",
				PerNodeIPCount: 256,
			}
			Expect(testClient.Create(ctx, dpuServiceIPAMTwo)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceIPAMTwo)

			By("Removing the DPUServiceIPAM resource")
			Expect(testClient.Delete(ctx, dpuServiceIPAMOne)).To(Succeed())

			Eventually(func(g Gomega) {
				got := &nvipamv1.IPPoolList{}
				g.Expect(dpfClusterClient.List(ctx, got)).To(Succeed())
				g.Expect(got.Items).To(ConsistOf(
					HaveField("ObjectMeta.Name", "resource-2"),
				))
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})
		It("should reconcile NVIPAM CIDRPool in DPU cluster when ipv4Network is set", func() {
			By("Creating the DPUServiceIPAM resource")
			dpuServiceIPAM := getMinimalDPUServiceIPAM(testNS.Name)
			dpuServiceIPAM.Name = "pool-1"
			dpuServiceIPAM.Spec.IPV4Network = &sfcv1.IPV4Network{
				Network:      "192.168.0.0/20",
				GatewayIndex: 1,
				PrefixSize:   24,
				Exclusions:   []string{"192.168.0.1", "192.168.0.2"},
				Allocations: map[string]string{
					"node-1": "192.168.1.0/24",
					"node-2": "192.168.2.0/24",
				},
			}
			dpuServiceIPAM.Spec.NodeSelector = &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "some-key",
								Operator: corev1.NodeSelectorOpExists,
							},
						},
					},
				},
			}

			Expect(testClient.Create(ctx, dpuServiceIPAM)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceIPAM)

			Eventually(func(g Gomega) {
				got := &nvipamv1.CIDRPool{}
				g.Expect(dpfClusterClient.Get(ctx, client.ObjectKey{Namespace: testNS.Name, Name: "pool-1"}, got)).To(Succeed())
				g.Expect(got.Labels).To(HaveKeyWithValue("dpf.nvidia.com/dpuserviceipam-name", "pool-1"))
				g.Expect(got.Labels).To(HaveKeyWithValue("dpf.nvidia.com/dpuserviceipam-namespace", testNS.Name))
				g.Expect(got.Spec.CIDR).To(Equal("192.168.0.0/20"))
				g.Expect(got.Spec.PerNodeNetworkPrefix).To(Equal(uint(24)))
				g.Expect(got.Spec.GatewayIndex).To(Equal(ptr.To[uint](1)))
				g.Expect(got.Spec.Exclusions).To(ConsistOf([]nvipamv1.ExcludeRange{
					{StartIP: "192.168.0.1", EndIP: "192.168.0.1"},
					{StartIP: "192.168.0.2", EndIP: "192.168.0.2"},
				}))
				g.Expect(got.Spec.StaticAllocations).To(ConsistOf([]nvipamv1.CIDRPoolStaticAllocation{
					{NodeName: "node-1", Prefix: "192.168.1.0/24"},
					{NodeName: "node-2", Prefix: "192.168.2.0/24"},
				}))
				g.Expect(got.Spec.NodeSelector).To(BeComparableTo(&corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "some-key",
									Operator: corev1.NodeSelectorOpExists,
								},
							},
						},
					},
				}))
			}).WithTimeout(10 * time.Second).Should(Succeed())

			By("Removing the DPUServiceIPAM resource")
			Expect(testClient.Delete(ctx, dpuServiceIPAM)).To(Succeed())

			Eventually(func(g Gomega) {
				got := &nvipamv1.CIDRPoolList{}
				g.Expect(dpfClusterClient.List(ctx, got)).To(Succeed())
				g.Expect(got.Items).To(BeEmpty())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})
		It("should remove NVIPAM IPPool in DPU cluster when ipv4Subnet is updated to unset and ipv4Network is set", func() {
			By("Creating the DPUServiceIPAM resource")
			dpuServiceIPAM := getMinimalDPUServiceIPAM(testNS.Name)
			dpuServiceIPAM.Name = "pool-1"
			dpuServiceIPAM.Spec.IPV4Subnet = &sfcv1.IPV4Subnet{
				Subnet:         "192.168.0.0/20",
				Gateway:        "192.168.0.1",
				PerNodeIPCount: 256,
			}

			dpuServiceIPAM.SetManagedFields(nil)
			dpuServiceIPAM.SetGroupVersionKind(sfcv1.DPUServiceIPAMGroupVersionKind)
			// FieldOwner must be the same as the controller so that we can set a field to nil later
			Expect(testClient.Patch(ctx, dpuServiceIPAM, client.Apply, client.ForceOwnership, client.FieldOwner(dpuServiceIPAMControllerName))).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceIPAM)

			Eventually(func(g Gomega) {
				gotCIDRPool := &nvipamv1.CIDRPool{}
				g.Expect(apierrors.IsNotFound(dpfClusterClient.Get(ctx, client.ObjectKey{Namespace: testNS.Name, Name: "pool-1"}, gotCIDRPool))).To(BeTrue())
				gotIPPool := &nvipamv1.IPPool{}
				g.Expect(dpfClusterClient.Get(ctx, client.ObjectKey{Namespace: testNS.Name, Name: "pool-1"}, gotIPPool)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())

			By("Updating the spec to unset ipv4Subnet and set ipv4Network")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuServiceIPAM), dpuServiceIPAM)).To(Succeed())
				dpuServiceIPAM.Spec.IPV4Subnet = nil
				dpuServiceIPAM.Spec.IPV4Network = &sfcv1.IPV4Network{
					Network:      "192.168.0.0/20",
					GatewayIndex: 1,
					PrefixSize:   24,
				}
				dpuServiceIPAM.SetManagedFields(nil)
				dpuServiceIPAM.SetGroupVersionKind(sfcv1.DPUServiceIPAMGroupVersionKind)
				// FieldOwner must be the same as the controller because we modify a field that the controller owns (see
				// defer in the main reconcile function)
				g.Expect(testClient.Patch(ctx, dpuServiceIPAM, client.Apply, client.FieldOwner(dpuServiceIPAMControllerName))).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())

			Eventually(func(g Gomega) {
				gotIPPool := &nvipamv1.IPPool{}
				g.Expect(apierrors.IsNotFound(dpfClusterClient.Get(ctx, client.ObjectKey{Namespace: testNS.Name, Name: "pool-1"}, gotIPPool))).To(BeTrue())
				gotCIDRPool := &nvipamv1.CIDRPool{}
				g.Expect(dpfClusterClient.Get(ctx, client.ObjectKey{Namespace: testNS.Name, Name: "pool-1"}, gotCIDRPool)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})
		It("should remove NVIPAM CIDRPool in DPU cluster when ipv4Network is updated to unset and ipv4Subnet is set", func() {
			By("Creating the DPUServiceIPAM resource")
			dpuServiceIPAM := getMinimalDPUServiceIPAM(testNS.Name)
			dpuServiceIPAM.Name = "pool-1"
			dpuServiceIPAM.Spec.IPV4Network = &sfcv1.IPV4Network{
				Network:      "192.168.0.0/20",
				GatewayIndex: 1,
				PrefixSize:   24,
			}

			dpuServiceIPAM.SetManagedFields(nil)
			dpuServiceIPAM.SetGroupVersionKind(sfcv1.DPUServiceIPAMGroupVersionKind)
			// FieldOwner must be the same as the controlller so that we can set a field to nil later
			Expect(testClient.Patch(ctx, dpuServiceIPAM, client.Apply, client.ForceOwnership, client.FieldOwner(dpuServiceIPAMControllerName))).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceIPAM)

			Eventually(func(g Gomega) {
				gotIPPool := &nvipamv1.IPPool{}
				g.Expect(apierrors.IsNotFound(dpfClusterClient.Get(ctx, client.ObjectKey{Namespace: testNS.Name, Name: "pool-1"}, gotIPPool))).To(BeTrue())
				gotCIDRPool := &nvipamv1.CIDRPool{}
				g.Expect(dpfClusterClient.Get(ctx, client.ObjectKey{Namespace: testNS.Name, Name: "pool-1"}, gotCIDRPool)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())

			By("Updating the spec to unset ipv4Network and set ipv4Subnet")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuServiceIPAM), dpuServiceIPAM)).To(Succeed())
				dpuServiceIPAM.Spec.IPV4Network = nil
				dpuServiceIPAM.Spec.IPV4Subnet = &sfcv1.IPV4Subnet{
					Subnet:         "192.168.0.0/20",
					Gateway:        "192.168.0.1",
					PerNodeIPCount: 256,
				}

				dpuServiceIPAM.SetManagedFields(nil)
				dpuServiceIPAM.SetGroupVersionKind(sfcv1.DPUServiceIPAMGroupVersionKind)
				// FieldOwner must be the same as the controller because we modify a field that the controller owns (see
				// defer in the main reconcile function)
				g.Expect(testClient.Patch(ctx, dpuServiceIPAM, client.Apply, client.FieldOwner(dpuServiceIPAMControllerName))).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())

			Eventually(func(g Gomega) {
				gotCIDRPool := &nvipamv1.CIDRPool{}
				g.Expect(apierrors.IsNotFound(dpfClusterClient.Get(ctx, client.ObjectKey{Namespace: testNS.Name, Name: "pool-1"}, gotCIDRPool))).To(BeTrue())
				gotIPPool := &nvipamv1.IPPool{}
				g.Expect(dpfClusterClient.Get(ctx, client.ObjectKey{Namespace: testNS.Name, Name: "pool-1"}, gotIPPool)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})
	})
})

func getMinimalDPUServiceIPAM(namespace string) *sfcv1.DPUServiceIPAM {
	return &sfcv1.DPUServiceIPAM{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dpuserviceipam",
			Namespace: namespace,
		},
	}
}
