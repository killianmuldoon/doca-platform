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

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dscResourceName = "test-dpu-service-chain"
	testNS          = "test-ns"
)

//nolint:dupl
var _ = Describe("ServiceChainSet Controller", func() {
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
		It("should successfully reconcile the DPUServiceChain ", func() {
			By("Create DPUServiceChain")
			cleanupObjects = append(cleanupObjects, createDPUServiceChain(ctx, nil))
			By("Verify ServiceChainSet is created")
			Eventually(func(g Gomega) {
				scs := &sfcv1.ServiceChainSet{ObjectMeta: metav1.ObjectMeta{Name: dscResourceName, Namespace: testNS}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
				cleanupObjects = append(cleanupObjects, scs)
			}, timeout*30, interval).Should(Succeed())
			By("Verify ServiceChainSet")
			scs := &sfcv1.ServiceChainSet{ObjectMeta: metav1.ObjectMeta{Name: dscResourceName, Namespace: testNS}}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
			for k, v := range testutils.GetTestLabels() {
				Expect(scs.Labels[k]).To(Equal(v))
			}
			Expect(scs.Spec).To(BeEquivalentTo(*getTestServiceChainSetSpec(nil)))
			By("Update DPUServiceChain")
			dsc := &sfcv1.DPUServiceChain{ObjectMeta: metav1.ObjectMeta{Name: dscResourceName, Namespace: testNS}}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dsc), dsc)).NotTo(HaveOccurred())
			labelSelector := &metav1.LabelSelector{MatchLabels: map[string]string{"role": "firewall"}}
			updatedSpec := getTestServiceChainSetSpec(labelSelector)
			dsc.Spec.Template.Spec = *updatedSpec
			Expect(testClient.Update(ctx, dsc)).NotTo(HaveOccurred())
			By("Verify ServiceChainSet is updated")
			Eventually(func(g Gomega) {
				scs := &sfcv1.ServiceChainSet{ObjectMeta: metav1.ObjectMeta{Name: dscResourceName, Namespace: testNS}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
				g.Expect(scs.Spec).To(BeEquivalentTo(*getTestServiceChainSetSpec(labelSelector)))
			}, timeout*30, interval).Should(Succeed())
		})
		It("should successfully delete the DPUServiceChain and ServiceChainSet", func() {
			By("Create DPUServiceChain")
			cleanupObjects = append(cleanupObjects, createDPUServiceChain(ctx, nil))
			By("Verify ServiceChainSet is created")
			Eventually(func(g Gomega) {
				scs := &sfcv1.ServiceChainSet{ObjectMeta: metav1.ObjectMeta{Name: dscResourceName, Namespace: testNS}}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
			}, timeout*30, interval).Should(Succeed())
			By("Delete DPUServiceChain")
			dsc := &sfcv1.DPUServiceChain{ObjectMeta: metav1.ObjectMeta{Name: dscResourceName, Namespace: testNS}}
			Expect(testClient.Delete(ctx, dsc)).NotTo(HaveOccurred())
			By("Verify ServiceChainSet is deleted")
			Eventually(func(g Gomega) {
				scs := &sfcv1.ServiceChainSet{ObjectMeta: metav1.ObjectMeta{Name: dscResourceName, Namespace: testNS}}
				err := testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}, timeout*30, interval).Should(Succeed())
			By("Verify DPUServiceChain is deleted")
			Eventually(func(g Gomega) {
				scs := &sfcv1.DPUServiceChain{ObjectMeta: metav1.ObjectMeta{Name: dscResourceName, Namespace: testNS}}
				err := testClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}, timeout*30, interval).Should(Succeed())
		})
	})
})

func createDPUServiceChain(ctx context.Context, labelSelector *metav1.LabelSelector) *sfcv1.DPUServiceChain {
	dsc := &sfcv1.DPUServiceChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dscResourceName,
			Namespace: testNS,
		},
		Spec: sfcv1.DPUServiceChainSpec{
			Template: sfcv1.ServiceChainSetSpecTemplate{
				ObjectMeta: sfcv1.ObjectMeta{
					Labels: testutils.GetTestLabels(),
				},
				Spec: *getTestServiceChainSetSpec(labelSelector),
			},
		},
	}
	Expect(testClient.Create(ctx, dsc)).NotTo(HaveOccurred())
	return dsc
}

func getTestServiceChainSetSpec(labelSelector *metav1.LabelSelector) *sfcv1.ServiceChainSetSpec {
	return &sfcv1.ServiceChainSetSpec{
		NodeSelector: labelSelector,
		Template: sfcv1.ServiceChainSpecTemplate{
			Spec: *getTestServiceChainSpec(),
			ObjectMeta: sfcv1.ObjectMeta{
				Labels: testutils.GetTestLabels(),
			},
		},
	}
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
