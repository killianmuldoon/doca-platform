/*
COPYRIGHT 2024 NVIDIA

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

package controller //nolint:dupl

import (
	"context"
	"time"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	resourceName = "test-resource"
	defaultNS    = "default"
)

//nolint:dupl
var _ = Describe("ServiceChainSet Controller", func() {
	Context("When reconciling a resource", func() {
		var cleanupObjects []client.Object
		BeforeEach(func() {
			cleanupObjects = []client.Object{}
		})
		AfterEach(func() {
			By("Cleaning up the objects")
			Expect(testutils.CleanupAndWait(ctx, testClient, cleanupObjects...)).To(Succeed())
		})
		It("should successfully reconcile the ServiceChainSet without Node Selector", func() {
			By("Create ServiceChainSet, without Node Selector")
			cleanupObjects = append(cleanupObjects, createServiceChainSet(ctx, nil))
			By("Verify ServiceChain not created, no nodes")
			Consistently(func(g Gomega) {
				serviceChainList := &sfcv1.ServiceChainList{}
				err := testClient.List(ctx, serviceChainList)
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceChainList.Items).To(BeEmpty())
			}).WithTimeout(20 * time.Second).Should(Succeed())

			By("Create 3 nodes")
			labels := map[string]string{"role": "firewall"}
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node1", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node2", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node3", make(map[string]string)))

			By("Reconciling the created resource, 3 nodes")
			Eventually(func(g Gomega) {
				assertServiceChainList(ctx, g, 3, &cleanupObjects, getTestServiceChainSpec())
			}, timeout*30, interval).Should(Succeed())

		})
		It("should successfully reconcile the ServiceChainSet with Node Selector", func() {
			By("creating ServiceChainSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createServiceChainSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}))

			By("Create 3 nodes")
			labels := map[string]string{"role": "firewall"}
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node1", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node2", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node3", make(map[string]string)))

			By("Reconciling the created resource, 3 nodes, 2 matches")
			Eventually(func(g Gomega) {
				assertServiceChainList(ctx, g, 2, &cleanupObjects, getTestServiceChainSpec())
			}, timeout*30, interval).Should(Succeed())
		})
		It("should successfully reconcile the ServiceChainSet with Node Selector and remove Service Chain", func() {
			By("creating ServiceChainSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createServiceChainSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}))

			By("Create 3 nodes")
			labels := map[string]string{"role": "firewall"}
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node1", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node2", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node3", labels))

			By("Reconciling the created resource, 3 nodes, 3 matches")
			Eventually(func(g Gomega) {
				assertServiceChainList(ctx, g, 3, &cleanupObjects, getTestServiceChainSpec())
			}, timeout*30, interval).Should(Succeed())

			By("Update Node-3 label to not be selected")
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node3"}}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).NotTo(HaveOccurred())
			node.Labels = make(map[string]string)
			Expect(testClient.Update(ctx, node)).NotTo(HaveOccurred())

			By("Reconciling the created resource, 3 nodes, 2 matching")
			Eventually(func(g Gomega) {
				assertServiceChainList(ctx, g, 2, &cleanupObjects, getTestServiceChainSpec())
			}, timeout*30, interval).Should(Succeed())
		})
		It("should successfully reconcile the ServiceChainSet after update", func() {
			By("creating ServiceChainSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createServiceChainSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}))

			By("Create 3 nodes")
			labels := map[string]string{"role": "firewall"}
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node1", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node2", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node3", make(map[string]string)))

			By("Reconciling the created resource, 3 nodes, 2 matches")
			Eventually(func(g Gomega) {
				assertServiceChainList(ctx, g, 2, &cleanupObjects, getTestServiceChainSpec())
			}, timeout*30, interval).Should(Succeed())

			By("Update ServiceChainSet Spec")
			scs := &sfcv1.ServiceChainSet{ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: defaultNS}}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
			updatedSpec := &sfcv1.ServiceChainSpec{
				Switches: []sfcv1.Switch{
					{
						Ports: []sfcv1.Port{
							{
								Service: &sfcv1.Service{
									InterfaceName: "head-iface",
								},
								ServiceInterface: &sfcv1.ServiceIfc{
									Reference: &sfcv1.ObjectRef{
										Name: "p0",
									},
								},
							},
						},
					},
				},
			}
			scs.Spec.Template.Spec = *updatedSpec
			Expect(testClient.Update(ctx, scs)).NotTo(HaveOccurred())
			By("Reconciling the updated resource")
			Eventually(func(g Gomega) {
				assertServiceChainList(ctx, g, 2, &cleanupObjects, updatedSpec)
			}, timeout*30, interval).Should(Succeed())
		})
	})
})

func createServiceChainSet(ctx context.Context, labelSelector *metav1.LabelSelector) *sfcv1.ServiceChainSet {
	scs := &sfcv1.ServiceChainSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: defaultNS,
		},
		Spec: sfcv1.ServiceChainSetSpec{
			NodeSelector: labelSelector,
			Template: sfcv1.ServiceChainSpecTemplate{
				Spec: *getTestServiceChainSpec(),
				ObjectMeta: sfcv1.ObjectMeta{
					Labels: testutils.GetTestLabels(),
				},
			},
		},
	}
	Expect(testClient.Create(ctx, scs)).NotTo(HaveOccurred())
	return scs
}

func getTestServiceChainSpec() *sfcv1.ServiceChainSpec {
	return &sfcv1.ServiceChainSpec{
		Switches: []sfcv1.Switch{
			{
				Ports: []sfcv1.Port{
					{
						Service: &sfcv1.Service{
							InterfaceName: "head-iface",
						},
						ServiceInterface: &sfcv1.ServiceIfc{
							Reference: &sfcv1.ObjectRef{
								Name: "p0",
							},
						},
					},
				},
			},
		},
	}
}

func createNode(ctx context.Context, name string, labels map[string]string) *corev1.Node {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
	Expect(testClient.Create(ctx, node)).NotTo(HaveOccurred())
	return node
}

func assertServiceChainList(ctx context.Context, g Gomega, nodeCount int, cleanupObjects *[]client.Object,
	testSpec *sfcv1.ServiceChainSpec) {
	serviceChainList := &sfcv1.ServiceChainList{}
	g.ExpectWithOffset(1, testClient.List(ctx, serviceChainList)).NotTo(HaveOccurred())
	g.ExpectWithOffset(1, serviceChainList.Items).To(HaveLen(nodeCount))

	nodeMap := make(map[string]bool)
	for _, sc := range serviceChainList.Items {
		serviceChain := sc
		*cleanupObjects = append(*cleanupObjects, &serviceChain)
		assertServiceChain(g, &sc, testSpec)
		nodeMap[sc.Spec.Node] = true
	}
	g.ExpectWithOffset(1, nodeMap).To(HaveLen(nodeCount))
}

func assertServiceChain(g Gomega, sc *sfcv1.ServiceChain, testSpec *sfcv1.ServiceChainSpec) {
	specCopy := testSpec.DeepCopy()
	node := sc.Spec.Node
	specCopy.Node = node
	specCopy.Switches[0].Ports[0].ServiceInterface.Reference.Name = specCopy.Switches[0].Ports[0].ServiceInterface.Reference.Name + "-" + node
	g.ExpectWithOffset(2, sc.Spec).To(Equal(*specCopy))
	g.ExpectWithOffset(2, node).NotTo(BeEmpty())
	g.ExpectWithOffset(2, sc.Name).To(Equal(resourceName + "-" + node))
	g.ExpectWithOffset(2, sc.Labels[ServiceChainSetNameLabel]).To(Equal(resourceName))
	g.ExpectWithOffset(2, sc.Labels[ServiceChainSetNamespaceLabel]).To(Equal(defaultNS))
	g.ExpectWithOffset(2, sc.OwnerReferences).To(HaveLen(1))
	for k, v := range testutils.GetTestLabels() {
		g.ExpectWithOffset(2, sc.Labels[k]).To(Equal(v))
	}
}
