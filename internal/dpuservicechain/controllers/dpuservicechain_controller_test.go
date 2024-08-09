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
	"sync"
	"time"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
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
			cleanupObjects = append(cleanupObjects, createDPUServiceChain(ctx, dscResourceName, testNS, nil))
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
			cleanupObjects = append(cleanupObjects, createDPUServiceChain(ctx, dscResourceName, testNS, nil))
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
	Context("When checking the status transitions", func() {
		var testNS *corev1.Namespace
		var dpuServiceChain *sfcv1.DPUServiceChain
		var kamajiSecret *corev1.Secret
		var dpfClusterClient client.Client
		type event struct {
			oldObj *unstructured.Unstructured
			newObj *unstructured.Unstructured
		}
		var updateEvents chan event
		var deleteEvents chan *unstructured.Unstructured

		BeforeEach(func() {
			By("Creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			DeferCleanup(testClient.Delete, ctx, testNS)

			By("Adding fake kamaji cluster")
			dpfCluster := controlplane.DPFCluster{Name: "envtest", Namespace: testNS.Name}
			var err error
			kamajiSecret, err = testutils.GetFakeKamajiClusterSecretFromEnvtest(dpfCluster, cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(testClient.Create(ctx, kamajiSecret)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, kamajiSecret)
			dpfClusterClient, err = dpfCluster.NewClient(ctx, testClient)
			Expect(err).ToNot(HaveOccurred())

			By("Creating the informer infrastructure for DPUServiceChain")
			var wg sync.WaitGroup
			stopCh := make(chan struct{})
			updateEvents = make(chan event, 100)
			deleteEvents = make(chan *unstructured.Unstructured, 100)
			DeferCleanup(func() {
				close(stopCh)
				wg.Wait()
				close(updateEvents)
				close(deleteEvents)
			})

			dc, err := dynamic.NewForConfig(cfg)
			Expect(err).ToNot(HaveOccurred())
			informer := dynamicinformer.
				NewFilteredDynamicSharedInformerFactory(dc, 0, testNS.Name, nil).
				ForResource(schema.GroupVersionResource{
					Group:    sfcv1.DPUServiceChainGroupVersionKind.Group,
					Version:  sfcv1.DPUServiceChainGroupVersionKind.Version,
					Resource: "dpuservicechains",
				}).Informer()

			handlers := cache.ResourceEventHandlerFuncs{
				UpdateFunc: func(oldObj, obj interface{}) {
					ou := oldObj.(*unstructured.Unstructured)
					nu := obj.(*unstructured.Unstructured)
					updateEvents <- event{
						oldObj: ou,
						newObj: nu,
					}
				},
				DeleteFunc: func(obj interface{}) {
					o := obj.(*unstructured.Unstructured)
					deleteEvents <- o
				},
			}
			_, err = informer.AddEventHandler(handlers)
			Expect(err).ToNot(HaveOccurred())

			wg.Add(1)
			go func() {
				informer.Run(stopCh)
				defer wg.Done()
			}()

			By("Creating a DPUServiceChain")
			dpuServiceChain = createDPUServiceChain(ctx, "chain", testNS.Name, nil)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceChain)
		})
		It("DPUServiceChain has condition ServiceChainSetReconciled with Pending Reason at start of the reconciliation loop", func() {
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &event{}
				g.Eventually(updateEvents).Should(Receive(ev))
				oldObj := &sfcv1.DPUServiceChain{}
				newObj := &sfcv1.DPUServiceChain{}
				g.Expect(testClient.Scheme().Convert(ev.oldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(testClient.Scheme().Convert(ev.newObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Conditions).To(BeEmpty())
				g.Expect(newObj.Status.Conditions).ToNot(BeEmpty())
				return newObj.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", string(conditions.TypeReady)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
				And(
					HaveField("Type", string(sfcv1.ConditionServiceChainSetReconciled)),
					HaveField("Status", metav1.ConditionUnknown),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
			))
		})
		It("DPUServiceChain has condition ServiceChainSetReconciled with Success Reason at end of successful reconciliation loop", func() {
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &event{}
				g.Eventually(updateEvents).Should(Receive(ev))
				oldObj := &sfcv1.DPUServiceChain{}
				newObj := &sfcv1.DPUServiceChain{}
				g.Expect(testClient.Scheme().Convert(ev.oldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(testClient.Scheme().Convert(ev.newObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", string(sfcv1.ConditionServiceChainSetReconciled)),
						HaveField("Status", metav1.ConditionUnknown),
						HaveField("Reason", string(conditions.ReasonPending)),
					),
				))
				return newObj.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", string(conditions.TypeReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(sfcv1.ConditionServiceChainSetReconciled)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
			))
		})
		It("DPUServiceChain has condition ServiceChainSetReconciled with Error Reason at the end of a reconciliation loop that failed", func() {
			By("Breaking the kamaji cluster secret to produce an error")
			// Taking ownership of the labels
			clusterName := kamajiSecret.Labels["kamaji.clastix.io/name"]
			kamajiSecret.Labels["kamaji.clastix.io/name"] = "some2"
			kamajiSecret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
			kamajiSecret.SetManagedFields(nil)
			Expect(testClient.Patch(ctx, kamajiSecret, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
			delete(kamajiSecret.Labels, "kamaji.clastix.io/name")
			kamajiSecret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
			kamajiSecret.SetManagedFields(nil)
			Expect(testClient.Patch(ctx, kamajiSecret, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			DeferCleanup(func() {
				By("Reverting the kamaji cluster secret to ensure DPUServiceChain deletion can be done")
				kamajiSecret.Labels["kamaji.clastix.io/name"] = clusterName
				kamajiSecret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
				kamajiSecret.SetManagedFields(nil)
				Expect(testClient.Patch(ctx, kamajiSecret, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
			})

			By("Checking condition")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &event{}
				g.Eventually(updateEvents).Should(Receive(ev))
				oldObj := &sfcv1.DPUServiceChain{}
				newObj := &sfcv1.DPUServiceChain{}
				g.Expect(testClient.Scheme().Convert(ev.oldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(testClient.Scheme().Convert(ev.newObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", string(sfcv1.ConditionServiceChainSetReconciled)),
						HaveField("Status", metav1.ConditionUnknown),
						HaveField("Reason", string(conditions.ReasonPending)),
					),
				))
				return newObj.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", string(conditions.TypeReady)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
				And(
					HaveField("Type", string(sfcv1.ConditionServiceChainSetReconciled)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonError)),
				),
			))

		})
		It("DPUServiceChain has condition ServiceChainSetReconciled with AwaitingDeletion Reason when there are still objects in the DPUCluster", func() {
			By("Ensuring that the DPUServiceChain has been reconciled successfully")
			Eventually(func(g Gomega) []metav1.Condition {
				got := &sfcv1.DPUServiceChain{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuServiceChain), got)).To(Succeed())
				return got.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ContainElement(
				And(
					HaveField("Type", string(sfcv1.ConditionServiceChainSetReconciled)),
					HaveField("Status", metav1.ConditionTrue),
				),
			))

			By("Adding finalizer to the underlying object")
			gotChainSet := &sfcv1.ServiceChainSet{}
			Eventually(dpfClusterClient.Get).WithArguments(ctx, client.ObjectKey{Namespace: testNS.Name, Name: "chain"}, gotChainSet).Should(Succeed())
			gotChainSet.SetFinalizers([]string{"test.dpf.nvidia.com/test"})
			gotChainSet.SetGroupVersionKind(sfcv1.ServiceChainSetGroupVersionKind)
			gotChainSet.SetManagedFields(nil)
			Expect(testClient.Patch(ctx, gotChainSet, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())

			By("Deleting the DPUServiceChain")
			Expect(testClient.Delete(ctx, dpuServiceChain)).To(Succeed())

			By("Checking the deleted condition is added")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &event{}
				g.Eventually(updateEvents).Should(Receive(ev))
				oldObj := &sfcv1.DPUServiceChain{}
				newObj := &sfcv1.DPUServiceChain{}
				g.Expect(testClient.Scheme().Convert(ev.oldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(testClient.Scheme().Convert(ev.newObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", string(sfcv1.ConditionServiceChainSetReconciled)),
					),
				))
				return newObj.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", string(conditions.TypeReady)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonAwaitingDeletion)),
				),
				And(
					HaveField("Type", string(sfcv1.ConditionServiceChainSetReconciled)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonAwaitingDeletion)),
					HaveField("Message", ContainSubstring("1")),
				),
			))

			By("Removing finalizer from the underlying object to ensure deletion")
			gotChainSet = &sfcv1.ServiceChainSet{}
			Eventually(dpfClusterClient.Get).WithArguments(ctx, client.ObjectKey{Namespace: testNS.Name, Name: "chain"}, gotChainSet).Should(Succeed())
			gotChainSet.SetFinalizers([]string{})
			gotChainSet.SetGroupVersionKind(sfcv1.ServiceChainSetGroupVersionKind)
			gotChainSet.SetManagedFields(nil)
			Expect(testClient.Patch(ctx, gotChainSet, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
		})
	})
})

func createDPUServiceChain(ctx context.Context, name string, namespace string, labelSelector *metav1.LabelSelector) *sfcv1.DPUServiceChain {
	dsc := &sfcv1.DPUServiceChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
