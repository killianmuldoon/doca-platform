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

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	svcIfcSetName = "svc-if-set"
)

//nolint:dupl
var _ = Describe("ServiceInterfaceSet Controller", func() {
	Context("When reconciling a resource", func() {
		var cleanupObjects []client.Object
		BeforeEach(func() {
			cleanupObjects = []client.Object{}
		})
		AfterEach(func() {
			By("Cleaning up the objects")
			Expect(testutils.CleanupAndWait(ctx, testClient, cleanupObjects...)).To(Succeed())
		})
		It("should successfully reconcile the ServiceInterfaceSet without Node Selector", func() {
			By("Create ServiceInterfaceSet, without Node Selector")
			cleanupObjects = append(cleanupObjects, createServiceInterfaceSet(ctx, nil))
			By("Verify ServiceInterface not created, no nodes")
			Consistently(func(g Gomega) {
				serviceInterfaceList := &sfcv1.ServiceInterfaceList{}
				err := testClient.List(ctx, serviceInterfaceList)
				Expect(err).NotTo(HaveOccurred())
				Expect(serviceInterfaceList.Items).To(BeEmpty())
			}).WithTimeout(20 * time.Second).Should(Succeed())

			By("Create 3 nodes")
			labels := map[string]string{"role": "firewall"}
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node1", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node2", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node3", make(map[string]string)))

			By("Reconciling the created resource, 3 nodes")
			Eventually(func(g Gomega) {
				assertServiceInterfaceList(ctx, g, 3, &cleanupObjects, getTestServiceInterfaceSpec())
			}, timeout*3, interval).Should(Succeed())
			By("Delete ServiceInterfaceSet Spec")
			Expect(testClient.Delete(ctx, &sfcv1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: svcIfcSetName, Namespace: defaultNS}})).To(Succeed())
		})
		It("should successfully reconcile the ServiceInterfaceSet with Node Selector", func() {
			By("creating ServiceInterfaceSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createServiceInterfaceSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}))

			By("Create 3 nodes")
			labels := map[string]string{"role": "firewall"}
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node1", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node2", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node3", make(map[string]string)))

			By("Reconciling the created resource, 3 nodes, 2 matches")
			Eventually(func(g Gomega) {
				assertServiceInterfaceList(ctx, g, 2, &cleanupObjects, getTestServiceInterfaceSpec())
			}, timeout*30, interval).Should(Succeed())
			By("Delete ServiceInterfaceSet Spec")
			Expect(testClient.Delete(ctx, &sfcv1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: svcIfcSetName, Namespace: defaultNS}})).To(Succeed())
		})
		It("should successfully reconcile the ServiceInterfaceSet with Node Selector and remove Service Interface", func() {
			By("creating ServiceInterfaceSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createServiceInterfaceSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}))

			By("Create 3 nodes")
			labels := map[string]string{"role": "firewall"}
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node1", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node2", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node3", labels))

			By("Reconciling the created resource, 3 nodes, 3 matches")
			Eventually(func(g Gomega) {
				assertServiceInterfaceList(ctx, g, 3, &cleanupObjects, getTestServiceInterfaceSpec())
			}, timeout*30, interval).Should(Succeed())

			By("Update Node-3 label to not be selected")
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node3"}}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).NotTo(HaveOccurred())
			node.Labels = make(map[string]string)
			Expect(testClient.Update(ctx, node)).NotTo(HaveOccurred())

			By("Reconciling the created resource, 3 nodes, 2 matching")
			Eventually(func(g Gomega) {
				assertServiceInterfaceList(ctx, g, 2, &cleanupObjects, getTestServiceInterfaceSpec())
			}, timeout*30, interval).Should(Succeed())
			By("Delete ServiceInterfaceSet Spec")
			Expect(testClient.Delete(ctx, &sfcv1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: svcIfcSetName, Namespace: defaultNS}})).To(Succeed())
		})
		It("should successfully reconcile the ServiceInterfaceSet after update", func() {
			By("creating ServiceInterfaceSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createServiceInterfaceSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}))

			By("Create 3 nodes")
			labels := map[string]string{"role": "firewall"}
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node1", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node2", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node3", make(map[string]string)))

			By("Reconciling the created resource, 3 nodes, 2 matches")
			Eventually(func(g Gomega) {
				assertServiceInterfaceList(ctx, g, 2, &cleanupObjects, getTestServiceInterfaceSpec())
			}, timeout*30, interval).Should(Succeed())

			By("Update ServiceInterfaceSet Spec")
			sis := &sfcv1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: svcIfcSetName, Namespace: defaultNS}}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(sis), sis)).NotTo(HaveOccurred())
			updatedSpec := &sfcv1.ServiceInterfaceSpec{
				InterfaceType: sfcv1.InterfaceTypeVLAN,
				InterfaceName: ptr.To("eth1.100"),
				Vlan: &sfcv1.VLAN{
					VlanID:             100,
					ParentInterfaceRef: "p7",
				},
				VF: &sfcv1.VF{
					VFID:               3,
					PFID:               7,
					ParentInterfaceRef: "p10",
				},
				PF: &sfcv1.PF{
					ID: 8,
				},
			}
			sis.Spec.Template.Spec = *updatedSpec
			Expect(testClient.Update(ctx, sis)).NotTo(HaveOccurred())
			By("Reconciling the updated resource")
			Eventually(func(g Gomega) {
				assertServiceInterfaceList(ctx, g, 2, &cleanupObjects, updatedSpec)
			}, timeout*30, interval).Should(Succeed())
			By("Delete ServiceInterfaceSet Spec")
			Expect(testClient.Delete(ctx, &sfcv1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: svcIfcSetName, Namespace: defaultNS}})).To(Succeed())
		})
		It("should successfully delete the ServiceInterfaceSet", func() {
			By("Creating ServiceInterfaceSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createServiceInterfaceSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}))

			By("Creating 2 nodes")
			labels := map[string]string{"role": "firewall"}
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node1", labels))
			cleanupObjects = append(cleanupObjects, createNode(ctx, "node2", labels))

			By("Reconciling the created resource, 2 nodes, 2 matches")
			Eventually(func(g Gomega) {
				assertServiceInterfaceList(ctx, g, 2, &cleanupObjects, getTestServiceInterfaceSpec())
			}, timeout*30, interval).Should(Succeed())

			By("Deleting ServiceInterfaceSet")
			sis := cleanupObjects[0].(*sfcv1.ServiceInterfaceSet)
			Expect(testClient.Delete(ctx, sis)).NotTo(HaveOccurred())

			By("Verifying ServiceInterfaceSet is deleted")
			Eventually(func(g Gomega) {
				sis := cleanupObjects[0].(*sfcv1.ServiceInterfaceSet)
				err := testClient.Get(ctx, client.ObjectKeyFromObject(sis), sis)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}, timeout*30, interval).Should(Succeed())
		})
	})

	Context("Validating ServiceInterfaceSet creation", func() {
		var cleanupObjects []client.Object
		BeforeEach(func() {
			cleanupObjects = []client.Object{}
		})
		AfterEach(func() {
			By("Cleaning up the objects")
			Expect(testutils.CleanupAndWait(ctx, testClient, cleanupObjects...)).To(Succeed())
		})
		It("should successfully create the ServiceInterfaceSet with vlan interface", func() {
			By("creating ServiceInterfaceSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createTypedServiceInterfaceSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}, sfcv1.InterfaceTypeVLAN))
		})
		It("should successfully create the ServiceInterfaceSet with pf interface", func() {
			By("creating ServiceInterfaceSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createTypedServiceInterfaceSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}, sfcv1.InterfaceTypePF))
		})
		It("should successfully create the ServiceInterfaceSet with vf interface", func() {
			By("creating ServiceInterfaceSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createTypedServiceInterfaceSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}, sfcv1.InterfaceTypeVF))
		})
		It("should successfully create the ServiceInterfaceSet with physical interface", func() {
			By("creating ServiceInterfaceSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createTypedServiceInterfaceSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}, sfcv1.InterfaceTypePhysical))
		})
		It("should successfully create the ServiceInterfaceSet with ovn interface", func() {
			By("creating ServiceInterfaceSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createTypedServiceInterfaceSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}, sfcv1.InterfaceTypeOVN))
		})
		It("should successfully create the ServiceInterfaceSet with service interface", func() {
			By("creating ServiceInterfaceSet, with Node Selector")
			cleanupObjects = append(cleanupObjects, createTypedServiceInterfaceSet(ctx, &metav1.LabelSelector{
				MatchLabels: map[string]string{"role": "firewall"}}, sfcv1.InterfaceTypeService))
		})

		It("should fail to create the ServiceInterfaceSet with missing vlan interface", func() {
			createInvalidTypedServiceInterfaceSet(ctx, &metav1.LabelSelector{MatchLabels: map[string]string{"role": "firewall"}}, sfcv1.InterfaceTypeVLAN)
		})
		It("should fail to create the ServiceInterfaceSet with missing pf interface", func() {
			createInvalidTypedServiceInterfaceSet(ctx, &metav1.LabelSelector{MatchLabels: map[string]string{"role": "firewall"}}, sfcv1.InterfaceTypePF)
		})
		It("should fail to create the ServiceInterfaceSet with missing vf interface", func() {
			createInvalidTypedServiceInterfaceSet(ctx, &metav1.LabelSelector{MatchLabels: map[string]string{"role": "firewall"}}, sfcv1.InterfaceTypeVF)
		})
		It("should fail to create the ServiceInterfaceSet with missing physical interface", func() {
			createInvalidTypedServiceInterfaceSet(ctx, &metav1.LabelSelector{MatchLabels: map[string]string{"role": "firewall"}}, sfcv1.InterfaceTypePhysical)
		})
		It("should fail to create the ServiceInterfaceSet with missing service definition", func() {
			createInvalidTypedServiceInterfaceSet(ctx, &metav1.LabelSelector{MatchLabels: map[string]string{"role": "firewall"}}, sfcv1.InterfaceTypeService)
		})
	})
})

func assertServiceInterfaceList(ctx context.Context, g Gomega, nodeCount int, cleanupObjects *[]client.Object,
	testSpec *sfcv1.ServiceInterfaceSpec) {
	serviceInterfaceList := &sfcv1.ServiceInterfaceList{}
	g.ExpectWithOffset(1, testClient.List(ctx, serviceInterfaceList)).NotTo(HaveOccurred())
	g.ExpectWithOffset(1, serviceInterfaceList.Items).To(HaveLen(nodeCount))

	nodeMap := make(map[string]bool)
	for _, si := range serviceInterfaceList.Items {
		serviceInterface := si
		*cleanupObjects = append(*cleanupObjects, &serviceInterface)
		assertServiceInterface(g, &si, testSpec)
		nodeMap[*si.Spec.Node] = true
	}
	g.ExpectWithOffset(1, nodeMap).To(HaveLen(nodeCount))
}

func assertServiceInterface(g Gomega, sc *sfcv1.ServiceInterface, testSpec *sfcv1.ServiceInterfaceSpec) {
	specCopy := testSpec.DeepCopy()
	node := sc.Spec.Node
	specCopy.Vlan.ParentInterfaceRef = specCopy.Vlan.ParentInterfaceRef + "-" + *node
	specCopy.VF.ParentInterfaceRef = specCopy.VF.ParentInterfaceRef + "-" + *node
	specCopy.Node = node
	g.ExpectWithOffset(2, sc.Spec).To(Equal(*specCopy))
	g.ExpectWithOffset(2, *node).NotTo(BeEmpty())
	g.ExpectWithOffset(2, sc.Name).To(Equal(svcIfcSetName + "-" + *node))
	g.ExpectWithOffset(2, sc.Labels[ServiceInterfaceSetNameLabel]).To(Equal(svcIfcSetName))
	g.ExpectWithOffset(2, sc.Labels[ServiceInterfaceSetNamespaceLabel]).To(Equal(defaultNS))
	g.ExpectWithOffset(2, sc.OwnerReferences).To(HaveLen(1))
	for k, v := range testutils.GetTestLabels() {
		g.ExpectWithOffset(2, sc.Labels[k]).To(Equal(v))
	}
}

func createServiceInterfaceSet(ctx context.Context, labelSelector *metav1.LabelSelector) *sfcv1.ServiceInterfaceSet {
	sis := serviceInterfaceSpec(labelSelector)
	sis.Spec.Template.Spec = *getTestServiceInterfaceSpec()

	Expect(testClient.Create(ctx, sis)).NotTo(HaveOccurred())
	return sis
}

func createTypedServiceInterfaceSet(ctx context.Context, labelSelector *metav1.LabelSelector, typ string) *sfcv1.ServiceInterfaceSet {
	sis := serviceInterfaceSpec(labelSelector)
	sis.Spec.Template.Spec = getTypedTestServiceInterfaceSpec(typ)

	Expect(testClient.Create(ctx, sis)).NotTo(HaveOccurred())
	return sis
}

func createInvalidTypedServiceInterfaceSet(ctx context.Context, labelSelector *metav1.LabelSelector, typ string) {
	sis := serviceInterfaceSpec(labelSelector)
	sis.Spec.Template.Spec = getInvalidTestServiceInterfaceSpec(typ)

	Expect(testClient.Create(ctx, sis)).To(HaveOccurred())
}

func serviceInterfaceSpec(labelSelector *metav1.LabelSelector) *sfcv1.ServiceInterfaceSet {
	sis := &sfcv1.ServiceInterfaceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcIfcSetName,
			Namespace: defaultNS,
		},
		Spec: sfcv1.ServiceInterfaceSetSpec{
			NodeSelector: labelSelector,
			Template: sfcv1.ServiceInterfaceSpecTemplate{
				ObjectMeta: sfcv1.ObjectMeta{
					Labels: testutils.GetTestLabels(),
				},
			},
		},
	}
	return sis
}

func getTestServiceInterfaceSpec() *sfcv1.ServiceInterfaceSpec {
	return &sfcv1.ServiceInterfaceSpec{
		InterfaceType: sfcv1.InterfaceTypeVF,
		InterfaceName: ptr.To("enp33s0f0np0v0"),
		Vlan: &sfcv1.VLAN{
			VlanID:             102,
			ParentInterfaceRef: "p0",
		},
		VF: &sfcv1.VF{
			VFID:               0,
			PFID:               1,
			ParentInterfaceRef: "p0",
		},
		PF: &sfcv1.PF{
			ID: 3,
		},
	}
}

func getTypedTestServiceInterfaceSpec(typ string) sfcv1.ServiceInterfaceSpec {
	sfc := sfcv1.ServiceInterfaceSpec{}
	switch typ {
	case sfcv1.InterfaceTypeVLAN:
		sfc.InterfaceType = sfcv1.InterfaceTypeVLAN
		sfc.InterfaceName = ptr.To("eth1.100")
		sfc.Vlan = &sfcv1.VLAN{
			VlanID:             102,
			ParentInterfaceRef: "p0",
		}
	case sfcv1.InterfaceTypePF:
		sfc.InterfaceType = sfcv1.InterfaceTypePF
		sfc.PF = &sfcv1.PF{
			ID: 3,
		}
	case sfcv1.InterfaceTypeVF:
		sfc.InterfaceType = sfcv1.InterfaceTypeVF
		sfc.VF = &sfcv1.VF{
			VFID:               0,
			PFID:               1,
			ParentInterfaceRef: "p0",
		}
	case sfcv1.InterfaceTypePhysical:
		sfc.InterfaceType = sfcv1.InterfaceTypePhysical
		sfc.InterfaceName = ptr.To("enp33s0f0np0v0")
	case sfcv1.InterfaceTypeOVN:
		sfc.InterfaceType = sfcv1.InterfaceTypeOVN
	case sfcv1.InterfaceTypeService:
		sfc.InterfaceType = sfcv1.InterfaceTypeService
		sfc.InterfaceName = ptr.To("net1")
		sfc.Service = &sfcv1.ServiceDef{
			ServiceID:   "awsome-firewall",
			NetworkName: "mybrsfc",
		}
	}

	return sfc
}

func getInvalidTestServiceInterfaceSpec(typ string) sfcv1.ServiceInterfaceSpec {
	sfc := getTypedTestServiceInterfaceSpec(typ)
	switch typ {
	case sfcv1.InterfaceTypeVLAN:
		sfc.Vlan = nil
	case sfcv1.InterfaceTypePF:
		sfc.PF = nil
	case sfcv1.InterfaceTypeVF:
		sfc.VF = nil
	case sfcv1.InterfaceTypePhysical:
		sfc.InterfaceName = nil
	case sfcv1.InterfaceTypeService:
		sfc.Service = nil
	}
	return sfc
}
