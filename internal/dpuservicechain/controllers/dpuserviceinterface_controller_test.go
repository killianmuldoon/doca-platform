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

package controllers //nolint:dupl

import (
	"context"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dsiResourceName = "test-dpu-service-ifc"
)

//nolint:dupl
var _ = Describe("ServiceInterfaceSet Controller", func() {
	Context("When reconciling a resource", func() {
		var cleanupObjects []client.Object
		BeforeEach(func() {
			cleanupObjects = []client.Object{}
			By("Faking GetDPFClusters to use the envtest cluster instead of a separate one")
			dpfCluster := controlplane.DPFCluster{Name: "envtest", Namespace: "default"}
			kamajiSecret, err := testutils.GetFakeKamajiClusterSecretFromEnvtest(dpfCluster, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(testClient.Create(ctx, kamajiSecret)).To(Succeed())
			cleanupObjects = append(cleanupObjects, kamajiSecret)
		})
		AfterEach(func() {
			By("Cleaning up the objects")
			Expect(testutils.CleanupAndWait(ctx, testClient, cleanupObjects...)).To(Succeed())
		})
		It("should successfully reconcile the DPUServiceInterface ", func() {
			By("Create DPUServiceInterface")
			cleanupObjects = append(cleanupObjects, createDPUServiceInterface(ctx, nil))
			By("Verify ServiceInterfaceSet is created")
			Eventually(func(g Gomega) {
				scs := &sfcv1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: dsiResourceName, Namespace: testNS}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
				cleanupObjects = append(cleanupObjects, scs)
			}, timeout*30, interval).Should(Succeed())
			By("Verify ServiceInterfaceSet")
			scs := &sfcv1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: dsiResourceName, Namespace: testNS}}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
			for k, v := range testutils.GetTestLabels() {
				Expect(scs.Labels[k]).To(Equal(v))
			}
			Expect(scs.Spec).To(BeEquivalentTo(*getTestServiceInterfaceSetSpec(nil)))
			By("Update DPUServiceInterface")
			dsc := &sfcv1.DPUServiceInterface{ObjectMeta: metav1.ObjectMeta{Name: dsiResourceName, Namespace: testNS}}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dsc), dsc)).NotTo(HaveOccurred())
			labelSelector := &metav1.LabelSelector{MatchLabels: map[string]string{"role": "firewall"}}
			updatedSpec := getTestServiceInterfaceSetSpec(labelSelector)
			dsc.Spec.Template.Spec = *updatedSpec
			Expect(testClient.Update(ctx, dsc)).NotTo(HaveOccurred())
			By("Verify ServiceInterfaceSet is updated")
			Eventually(func(g Gomega) {
				scs := &sfcv1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: dsiResourceName, Namespace: testNS}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
				g.Expect(scs.Spec).To(BeEquivalentTo(*getTestServiceInterfaceSetSpec(labelSelector)))
			}, timeout*30, interval).Should(Succeed())
		})
		It("should successfully delete the DPUServiceInterface and ServiceInterfaceSet", func() {
			By("Create DPUServiceInterface")
			cleanupObjects = append(cleanupObjects, createDPUServiceInterface(ctx, nil))
			By("Verify ServiceInterfaceSet is created")
			Eventually(func(g Gomega) {
				scs := &sfcv1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: dsiResourceName, Namespace: testNS}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
			}, timeout*30, interval).Should(Succeed())
			By("Delete DPUServiceInterface")
			dsc := &sfcv1.DPUServiceInterface{ObjectMeta: metav1.ObjectMeta{Name: dsiResourceName, Namespace: testNS}}
			Expect(testClient.Delete(ctx, dsc)).NotTo(HaveOccurred())
			By("Verify ServiceInterfaceSet is deleted")
			Eventually(func(g Gomega) {
				scs := &sfcv1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: dsiResourceName, Namespace: testNS}}
				err := testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}, timeout*30, interval).Should(Succeed())
			By("Verify DPUServiceInterface is deleted")
			Eventually(func(g Gomega) {
				scs := &sfcv1.DPUServiceInterface{ObjectMeta: metav1.ObjectMeta{Name: dsiResourceName, Namespace: testNS}}
				err := testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}, timeout*30, interval).Should(Succeed())
		})
	})
})

func createDPUServiceInterface(ctx context.Context, labelSelector *metav1.LabelSelector) *sfcv1.DPUServiceInterface {
	dsc := &sfcv1.DPUServiceInterface{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsiResourceName,
			Namespace: testNS,
		},
		Spec: sfcv1.DPUServiceInterfaceSpec{
			Template: sfcv1.ServiceInterfaceSetSpecTemplate{
				ObjectMeta: sfcv1.ObjectMeta{
					Labels: testutils.GetTestLabels(),
				},
				Spec: *getTestServiceInterfaceSetSpec(labelSelector),
			},
		},
	}
	Expect(testClient.Create(ctx, dsc)).NotTo(HaveOccurred())
	return dsc
}

func getTestServiceInterfaceSpec() *sfcv1.ServiceInterfaceSpec {
	return &sfcv1.ServiceInterfaceSpec{
		InterfaceType: "vf",
		InterfaceName: "enp33s0f0np0v0",
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

func getTestServiceInterfaceSetSpec(labelSelector *metav1.LabelSelector) *sfcv1.ServiceInterfaceSetSpec {
	return &sfcv1.ServiceInterfaceSetSpec{
		NodeSelector: labelSelector,
		Template: sfcv1.ServiceInterfaceSpecTemplate{
			Spec: *getTestServiceInterfaceSpec(),
			ObjectMeta: sfcv1.ObjectMeta{
				Labels: testutils.GetTestLabels(),
			},
		},
	}
}
