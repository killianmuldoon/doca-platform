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
	"fmt"
	"slices"
	"strings"
	"time"

	dpuservicev1 "github.com/nvidia/doca-platform/api/dpuservice/v1alpha1"
	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/conditions"
	testutils "github.com/nvidia/doca-platform/test/utils"
	"github.com/nvidia/doca-platform/test/utils/informer"

	"github.com/fluxcd/pkg/runtime/patch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

//nolint:goconst
var _ = Describe("DPUDeployment Controller", func() {
	defaultPauseDPUServiceReconciler := pauseDPUServiceReconciler
	defaultPauseDPUServiceTemplateReconciler := pauseDPUServiceTemplateReconciler
	defaultDPUDeploymentReconcileDeleteRequeueDuration := reconcileRequeueDuration
	BeforeEach(func() {
		DeferCleanup(func() {
			pauseDPUServiceReconciler = defaultPauseDPUServiceReconciler
			pauseDPUServiceTemplateReconciler = defaultPauseDPUServiceTemplateReconciler
			reconcileRequeueDuration = defaultDPUDeploymentReconcileDeleteRequeueDuration
		})

		// These are modified to speed up the testing suite and also simplify the deletion logic
		pauseDPUServiceReconciler = true
		reconcileRequeueDuration = 1 * time.Second
		pauseDPUServiceTemplateReconciler = true
	})
	Context("When reconciling a resource", func() {
		var testNS *corev1.Namespace
		BeforeEach(func() {
			By("Creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			DeferCleanup(testClient.Delete, ctx, testNS)
		})
		It("should successfully reconcile the DPUDeployment", func() {
			By("reconciling the created resource")
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			By("checking that finalizer is added")
			Eventually(func(g Gomega) []string {
				got := &dpuservicev1.DPUDeployment{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuDeployment), got)).To(Succeed())
				return got.Finalizers
			}).WithTimeout(30 * time.Second).Should(ConsistOf([]string{dpuservicev1.DPUDeploymentFinalizer}))

			By("checking that the resource can be deleted (finalizer is removed)")
			Expect(testutils.CleanupAndWait(ctx, testClient, dpuDeployment)).To(Succeed())
		})
		It("should not create DPUSet, DPUService, DPUServiceChain and DPUServiceInterface if any of the dependencies does not exist", func() {
			By("reconciling the created resource")
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			By("checking that no object is created")
			Consistently(func(g Gomega) {
				gotDPUSetList := &provisioningv1.DPUSetList{}
				g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
				g.Expect(gotDPUSetList.Items).To(BeEmpty())

				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				g.Expect(gotDPUServiceList.Items).To(BeEmpty())

				gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
				g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
				g.Expect(gotDPUServiceChainList.Items).To(BeEmpty())

				gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
				g.Expect(gotDPUServiceInterfaceList.Items).To(BeEmpty())
			}).WithTimeout(5 * time.Second).Should(Succeed())
		})
		It("should cleanup child objects on delete", func() {
			By("Creating the dependencies")
			bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

			dpuFlavor := getMinimalDPUFlavor(testNS.Name)
			Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

			dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
			Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

			dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

			DeferCleanup(cleanDPUDeploymentDerivatives, testNS.Name)

			By("creating the dpudeployment")
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			dpuDeployment.Spec.DPUs.DPUSets = []dpuservicev1.DPUSet{
				{
					NameSuffix: "dpuset1",
					NodeSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nodekey1": "nodevalue1",
						},
					},
					DPUSelector: map[string]string{
						"dpukey1": "dpuvalue1",
					},
					DPUAnnotations: map[string]string{
						"annotationkey1": "annotationvalue1",
					},
				},
			}
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			By("checking that dependencies are marked")
			Eventually(func(g Gomega) {
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(bfb),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(dpuFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(dpuServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(dpuServiceTemplate),
				} {
					g.Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					g.Expect(obj.GetFinalizers()).To(ContainElement(dpuservicev1.DPUDeploymentFinalizer), fmt.Sprintf("%T", obj))
					g.Expect(obj.GetLabels()).To(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
				}
			}).WithTimeout(5 * time.Second).Should(Succeed())

			By("checking that objects are created")
			Eventually(func(g Gomega) {
				gotDPUSetList := &provisioningv1.DPUSetList{}
				g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
				g.Expect(gotDPUSetList.Items).To(HaveLen(1))

				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				g.Expect(gotDPUServiceList.Items).To(HaveLen(1))

				gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
				g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
				g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))

				gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
				g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(1))
			}).WithTimeout(5 * time.Second).Should(Succeed())

			By("deleting the resource")
			Expect(testClient.Delete(ctx, dpuDeployment)).To(Succeed())

			By("checking that the child resources are removed")
			Eventually(func(g Gomega) {
				gotDPUSetList := &provisioningv1.DPUSetList{}
				g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
				g.Expect(gotDPUSetList.Items).To(BeEmpty())

				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				g.Expect(gotDPUServiceList.Items).To(BeEmpty())

				gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
				g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
				g.Expect(gotDPUServiceChainList.Items).To(BeEmpty())

				gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
				g.Expect(gotDPUServiceInterfaceList.Items).To(BeEmpty())
			}).WithTimeout(30 * time.Second).Should(Succeed())

			By("checking that the dependencies are released")
			Eventually(func(g Gomega) {
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(bfb),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(dpuFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(dpuServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(dpuServiceTemplate),
				} {
					g.Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					g.Expect(obj.GetFinalizers()).ToNot(ContainElement(dpuservicev1.DPUDeploymentFinalizer), fmt.Sprintf("%T", obj))
					g.Expect(obj.GetLabels()).ToNot(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
				}
			}).WithTimeout(5 * time.Second).Should(Succeed())
		})
		It("should not delete the DPUSets until the rest of the child objects are deleted", func() {
			By("Creating the dependencies")
			bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

			dpuFlavor := getMinimalDPUFlavor(testNS.Name)
			Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

			dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
			Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

			dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

			DeferCleanup(cleanDPUDeploymentDerivatives, testNS.Name)

			By("creating the dpudeployment")
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			dpuDeployment.Spec.DPUs.DPUSets = []dpuservicev1.DPUSet{
				{
					NameSuffix: "dpuset1",
					NodeSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nodekey1": "nodevalue1",
						},
					},
					DPUSelector: map[string]string{
						"dpukey1": "dpuvalue1",
					},
					DPUAnnotations: map[string]string{
						"annotationkey1": "annotationvalue1",
					},
				},
			}
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			objs := make(map[client.Object]interface{})
			By("checking that objects are created")
			Eventually(func(g Gomega) {
				gotDPUSetList := &provisioningv1.DPUSetList{}
				g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
				g.Expect(gotDPUSetList.Items).To(HaveLen(1))

				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				g.Expect(gotDPUServiceList.Items).To(HaveLen(1))
				objs[&gotDPUServiceList.Items[0]] = struct{}{}

				gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
				g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
				g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))
				objs[&gotDPUServiceChainList.Items[0]] = struct{}{}

				gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
				g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(1))
				objs[&gotDPUServiceInterfaceList.Items[0]] = struct{}{}
			}).WithTimeout(5 * time.Second).Should(Succeed())

			By("Patching the objects with a fake finalizer to prevent deletion")
			DeferCleanup(func() {
				By("Cleaning up the finalizers so that objects can be deleted")
				for obj := range objs {
					Expect(client.IgnoreNotFound(testClient.Patch(ctx, obj, client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"finalizers":[]}}`))))).To(Succeed())
				}
			})
			gotDPUServiceList := &dpuservicev1.DPUServiceList{}
			Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())

			gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
			Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())

			gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
			Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
			for obj, gvk := range map[client.Object]schema.GroupVersionKind{
				&gotDPUServiceList.Items[0]:          dpuservicev1.DPUServiceGroupVersionKind,
				&gotDPUServiceChainList.Items[0]:     dpuservicev1.DPUServiceChainGroupVersionKind,
				&gotDPUServiceInterfaceList.Items[0]: dpuservicev1.DPUServiceInterfaceGroupVersionKind,
			} {
				finalizers := obj.GetFinalizers()
				finalizers = append(finalizers, "test.io/some-finalizer")
				obj.SetFinalizers(finalizers)
				obj.GetObjectKind().SetGroupVersionKind(gvk)
				obj.SetManagedFields(nil)
				Expect(testClient.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
			}

			By("deleting the resource")
			Expect(testClient.Delete(ctx, dpuDeployment)).To(Succeed())

			By("checking that all child objects but the DPUSets have deletion timestamp")
			Eventually(func(g Gomega) {
				gotDPUSetList := &provisioningv1.DPUSetList{}
				g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
				g.Expect(gotDPUSetList.Items).To(HaveLen(1))
				g.Expect(gotDPUSetList.Items[0].DeletionTimestamp).To(BeNil())

				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				g.Expect(gotDPUServiceList.Items).To(HaveLen(1))
				g.Expect(gotDPUServiceList.Items[0].DeletionTimestamp).ToNot(BeNil())

				gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
				g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
				g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))
				g.Expect(gotDPUServiceChainList.Items[0].DeletionTimestamp).ToNot(BeNil())

				gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
				g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(1))
				g.Expect(gotDPUServiceInterfaceList.Items[0].DeletionTimestamp).ToNot(BeNil())
			}).WithTimeout(30 * time.Second).Should(Succeed())

			By("Removing the fake finalizer from the objects")
			gotDPUServiceList = &dpuservicev1.DPUServiceList{}
			Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())

			gotDPUServiceChainList = &dpuservicev1.DPUServiceChainList{}
			Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())

			gotDPUServiceInterfaceList = &dpuservicev1.DPUServiceInterfaceList{}
			Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
			for obj, gvk := range map[client.Object]schema.GroupVersionKind{
				&gotDPUServiceList.Items[0]:          dpuservicev1.DPUServiceGroupVersionKind,
				&gotDPUServiceChainList.Items[0]:     dpuservicev1.DPUServiceChainGroupVersionKind,
				&gotDPUServiceInterfaceList.Items[0]: dpuservicev1.DPUServiceInterfaceGroupVersionKind,
			} {
				obj.SetFinalizers([]string{})
				obj.GetObjectKind().SetGroupVersionKind(gvk)
				obj.SetManagedFields(nil)
				Expect(testClient.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
			}

			By("checking that the child resources are removed")
			Eventually(func(g Gomega) {
				gotDPUSetList := &provisioningv1.DPUSetList{}
				g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
				g.Expect(gotDPUSetList.Items).To(BeEmpty())

				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				g.Expect(gotDPUServiceList.Items).To(BeEmpty())

				gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
				g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
				g.Expect(gotDPUServiceChainList.Items).To(BeEmpty())

				gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
				g.Expect(gotDPUServiceInterfaceList.Items).To(BeEmpty())
			}).WithTimeout(30 * time.Second).Should(Succeed())
		})
	})
	Context("When reconciling multiple resources", func() {
		var testNS *corev1.Namespace
		BeforeEach(func() {
			By("Creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			DeferCleanup(testClient.Delete, ctx, testNS)
		})
		It("should cleanup child objects on delete", func() {
			By("Creating the dependencies")
			bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

			dpuFlavor := getMinimalDPUFlavor(testNS.Name)
			Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

			dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
			Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

			dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

			DeferCleanup(cleanDPUDeploymentDerivatives, testNS.Name)

			By("creating the dpudeployments")
			dpusets := []dpuservicev1.DPUSet{
				{
					NameSuffix: "dpuset1",
					NodeSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nodekey1": "nodevalue1",
						},
					},
					DPUSelector: map[string]string{
						"dpukey1": "dpuvalue1",
					},
					DPUAnnotations: map[string]string{
						"annotationkey1": "annotationvalue1",
					},
				},
			}

			dpuDeployment1 := getMinimalDPUDeployment(testNS.Name)
			dpuDeployment1.Name = "dpudeployment1"
			dpuDeployment1.Spec.DPUs.DPUSets = dpusets
			Expect(testClient.Create(ctx, dpuDeployment1)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment1)

			dpuDeployment2 := getMinimalDPUDeployment(testNS.Name)
			dpuDeployment2.Name = "dpudeployment2"
			dpuDeployment2.Spec.DPUs.DPUSets = dpusets
			Expect(testClient.Create(ctx, dpuDeployment2)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment1)

			By("checking that dependencies are marked")
			Eventually(func(g Gomega) {
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(bfb),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(dpuFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(dpuServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(dpuServiceTemplate),
				} {
					g.Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					g.Expect(obj.GetFinalizers()).To(ContainElement(dpuservicev1.DPUDeploymentFinalizer), fmt.Sprintf("%T", obj))
					g.Expect(obj.GetLabels()).To(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment1)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
					g.Expect(obj.GetLabels()).To(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment2)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
				}
			}).WithTimeout(30 * time.Second).Should(Succeed())

			By("Deleting all the dependencies")
			Expect(testClient.Delete(ctx, bfb)).To(Succeed())
			Expect(testClient.Delete(ctx, dpuFlavor)).To(Succeed())
			Expect(testClient.Delete(ctx, dpuServiceConfiguration)).To(Succeed())
			Expect(testClient.Delete(ctx, dpuServiceTemplate)).To(Succeed())

			By("Deleting the first dpudeployment")
			Expect(testClient.Delete(ctx, dpuDeployment1)).To(Succeed())

			By("checking that dependencies are marked only for the second dpudeployment and finalizer still in place")
			Eventually(func(g Gomega) {
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(bfb),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(dpuFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(dpuServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(dpuServiceTemplate),
				} {
					g.Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					g.Expect(obj.GetFinalizers()).To(ContainElement(dpuservicev1.DPUDeploymentFinalizer), fmt.Sprintf("%T", obj))
					g.Expect(obj.GetLabels()).ToNot(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment1)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
					g.Expect(obj.GetLabels()).To(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment2)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
				}
			}).WithTimeout(30 * time.Second).Should(Succeed())

			By("checking that the dependencies still exist")
			Consistently(func(g Gomega) {
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(bfb),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(dpuFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(dpuServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(dpuServiceTemplate),
				} {
					g.Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
				}
			}).WithTimeout(5 * time.Second).Should(Succeed())

			By("Deleting the second dpudeployment and verify that this is the time the finalizer is removed")
			Expect(testClient.Delete(ctx, dpuDeployment2)).To(Succeed())

			By("checking that the dependencies are removed")
			Eventually(func(g Gomega) {
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(bfb),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(dpuFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(dpuServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(dpuServiceTemplate),
				} {
					g.Expect(apierrors.IsNotFound(testClient.Get(ctx, key, obj))).To(BeTrue(), fmt.Sprintf("%T", obj))
				}
			}).WithTimeout(30 * time.Second).Should(Succeed())
		})
	})
	Context("When unit testing individual functions", func() {
		var testNS *corev1.Namespace
		BeforeEach(func() {
			By("Creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			DeferCleanup(testClient.Delete, ctx, testNS)
		})
		Context("When checking getDependencies()", func() {
			It("should return the correct object", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				By("Creating the dependencies")
				bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

				dpuFlavor := getMinimalDPUFlavor(testNS.Name)
				Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				bfb.SetGroupVersionKind(schema.EmptyObjectKind.GroupVersionKind())
				dpuServiceTemplate.SetGroupVersionKind(schema.EmptyObjectKind.GroupVersionKind())

				By("Checking the output of the function")
				deps, err := getDependencies(ctx, testClient, dpuDeployment)
				Expect(err).ToNot(HaveOccurred())
				Expect(deps).To(BeComparableTo(&dpuDeploymentDependencies{
					DPUFlavor: dpuFlavor,
					BFB:       bfb,
					DPUServiceConfigurations: map[string]*dpuservicev1.DPUServiceConfiguration{
						"someservice": dpuServiceConfiguration,
					},
					DPUServiceTemplates: map[string]*dpuservicev1.DPUServiceTemplate{
						"someservice": dpuServiceTemplate,
					},
				}))
			})
			It("should error if a dependency doesn't exist", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				By("Checking the output of the function")
				_, err := getDependencies(ctx, testClient, dpuDeployment)
				Expect(err).To(HaveOccurred())
			})
			It("should error if bfb is not ready", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				By("Creating the dependencies")
				bfb := getMinimalBFB("somebfb", testNS.Name)
				Expect(testClient.Create(ctx, bfb)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

				dpuFlavor := getMinimalDPUFlavor(testNS.Name)
				Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				By("Checking the output of the function")
				_, err := getDependencies(ctx, testClient, dpuDeployment)
				Expect(err).To(HaveOccurred())
			})
			It("should error if a DPUServiceConfiguration doesn't match DPUDeployment service", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				By("Creating the dependencies")
				bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

				dpuFlavor := getMinimalDPUFlavor(testNS.Name)
				Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				dpuServiceConfiguration.Spec.DeploymentServiceName = "wrong-service"
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				By("Checking the output of the function")
				_, err := getDependencies(ctx, testClient, dpuDeployment)
				Expect(err).To(HaveOccurred())
			})
			It("should error if a DPUServiceTemplate doesn't match DPUDeployment service", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				By("Creating the dependencies")
				bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

				dpuFlavor := getMinimalDPUFlavor(testNS.Name)
				Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := getMinimalDPUServiceTemplate(testNS.Name)
				dpuServiceTemplate.Spec.DeploymentServiceName = "wrong-service"
				Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
				patchDPUServiceTemplateWithStatus(dpuServiceTemplate)

				By("Checking the output of the function")
				_, err := getDependencies(ctx, testClient, dpuDeployment)
				Expect(err).To(HaveOccurred())
			})
			It("should error if a DPUServiceTemplate is not ready", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				By("Creating the dependencies")
				bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

				dpuFlavor := getMinimalDPUFlavor(testNS.Name)
				Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := getMinimalDPUServiceTemplate(testNS.Name)
				Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				By("Checking the output of the function")
				_, err := getDependencies(ctx, testClient, dpuDeployment)
				Expect(err).To(HaveOccurred())
			})
		})
		Context("When checking updateDependencies()", func() {
			var (
				dpuDeployment                *dpuservicev1.DPUDeployment
				bfb                          *provisioningv1.BFB
				extraBFB                     *provisioningv1.BFB
				dpuFlavor                    *provisioningv1.DPUFlavor
				extraDPUFlavor               *provisioningv1.DPUFlavor
				dpuServiceConfiguration      *dpuservicev1.DPUServiceConfiguration
				extraDPUServiceConfiguration *dpuservicev1.DPUServiceConfiguration
				dpuServiceTemplate           *dpuservicev1.DPUServiceTemplate
				extraDPUServiceTemplate      *dpuservicev1.DPUServiceTemplate
				objGVK                       map[client.Object]schema.GroupVersionKind
			)
			BeforeEach(func() {
				dpuDeployment = getMinimalDPUDeployment(testNS.Name)
				By("Creating the dependencies")
				bfb = createMinimalBFBWithStatus("somebfb", testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

				extraBFB = createMinimalBFBWithStatus("extra-bfb", testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, extraBFB)

				dpuFlavor = getMinimalDPUFlavor(testNS.Name)
				Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

				extraDPUFlavor = getMinimalDPUFlavor(testNS.Name)
				extraDPUFlavor.Name = "extra-dpuflavor"
				Expect(testClient.Create(ctx, extraDPUFlavor)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, extraDPUFlavor)

				dpuServiceConfiguration = getMinimalDPUServiceConfiguration(testNS.Name)
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				extraDPUServiceConfiguration = getMinimalDPUServiceConfiguration(testNS.Name)
				extraDPUServiceConfiguration.Name = "extra-dpuserviceconfiguration"
				Expect(testClient.Create(ctx, extraDPUServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, extraDPUServiceConfiguration)

				dpuServiceTemplate = createMinimalDPUServiceTemplateWithStatus(testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				extraDPUServiceTemplate = getMinimalDPUServiceTemplate(testNS.Name)
				extraDPUServiceTemplate.Name = "extra-dpuservicetemplate"
				Expect(testClient.Create(ctx, extraDPUServiceTemplate)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, extraDPUServiceTemplate)
				patchDPUServiceTemplateWithStatus(extraDPUServiceTemplate)

				objGVK = map[client.Object]schema.GroupVersionKind{
					bfb:                          provisioningv1.BFBGroupVersionKind,
					extraBFB:                     provisioningv1.BFBGroupVersionKind,
					dpuFlavor:                    provisioningv1.DPUFlavorGroupVersionKind,
					extraDPUFlavor:               provisioningv1.DPUFlavorGroupVersionKind,
					dpuServiceConfiguration:      dpuservicev1.DPUServiceConfigurationGroupVersionKind,
					extraDPUServiceConfiguration: dpuservicev1.DPUServiceConfigurationGroupVersionKind,
					dpuServiceTemplate:           dpuservicev1.DPUServiceTemplateGroupVersionKind,
					extraDPUServiceTemplate:      dpuservicev1.DPUServiceTemplateGroupVersionKind,
				}
				DeferCleanup(func() {
					By("Cleaning up the finalizers so that objects can be deleted")
					for obj := range objGVK {
						Expect(testClient.Patch(ctx, obj, client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"finalizers":[]}}`)))).To(Succeed())
					}
				})
			})
			It("should mark only the current dependencies", func() {
				By("Constructing the dependencies object")
				deps, err := getDependencies(ctx, testClient, dpuDeployment)
				Expect(err).ToNot(HaveOccurred())

				By("Updating the dependencies")
				Expect(updateDependencies(ctx, testClient, dpuDeployment, deps)).To(Succeed())

				By("Checking the current dependencies after update")
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(bfb),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(dpuFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(dpuServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(dpuServiceTemplate),
				} {
					Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					Expect(obj.GetFinalizers()).To(ContainElement(dpuservicev1.DPUDeploymentFinalizer), fmt.Sprintf("%T", obj))
					Expect(obj.GetLabels()).To(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
				}

				By("Checking the rest of the objects after update")
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(extraBFB),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(extraDPUFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(extraDPUServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(extraDPUServiceTemplate),
				} {
					Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					Expect(obj.GetFinalizers()).ToNot(ContainElement(dpuservicev1.DPUDeploymentFinalizer), fmt.Sprintf("%T", obj))
					Expect(obj.GetLabels()).ToNot(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
				}
			})
			It("should clean only the stale dependencies", func() {
				By("Constructing the dependencies object")
				deps, err := getDependencies(ctx, testClient, dpuDeployment)
				Expect(err).ToNot(HaveOccurred())

				By("Updating the dependencies")
				Expect(updateDependencies(ctx, testClient, dpuDeployment, deps)).To(Succeed())

				By("Checking the current dependencies after update")
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(bfb),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(dpuFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(dpuServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(dpuServiceTemplate),
				} {
					Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					Expect(obj.GetFinalizers()).To(ContainElement(dpuservicev1.DPUDeploymentFinalizer), fmt.Sprintf("%T", obj))
					Expect(obj.GetLabels()).To(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
				}

				By("Checking the rest of the objects after update")
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(extraBFB),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(extraDPUFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(extraDPUServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(extraDPUServiceTemplate),
				} {
					Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					Expect(obj.GetFinalizers()).ToNot(ContainElement(dpuservicev1.DPUDeploymentFinalizer), fmt.Sprintf("%T", obj))
					Expect(obj.GetLabels()).ToNot(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
				}
				By("Updating the DPUDeployment deps")
				svc := dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      extraDPUServiceTemplate.Name,
					ServiceConfiguration: extraDPUServiceConfiguration.Name,
				}
				dpuDeployment.Spec.Services["someservice"] = svc
				dpuDeployment.Spec.DPUs.BFB = extraBFB.Name
				dpuDeployment.Spec.DPUs.Flavor = extraDPUFlavor.Name

				By("Constructing the dependencies object")
				deps, err = getDependencies(ctx, testClient, dpuDeployment)
				Expect(err).ToNot(HaveOccurred())

				By("Updating the dependencies")
				Expect(updateDependencies(ctx, testClient, dpuDeployment, deps)).To(Succeed())

				By("Checking the current dependencies after update")
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(extraBFB),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(extraDPUFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(extraDPUServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(extraDPUServiceTemplate),
				} {
					Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					Expect(obj.GetFinalizers()).To(ContainElement(dpuservicev1.DPUDeploymentFinalizer), fmt.Sprintf("%T", obj))
					Expect(obj.GetLabels()).To(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
				}

				By("Checking the rest of the objects after update")
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(bfb),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(dpuFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(dpuServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(dpuServiceTemplate),
				} {
					Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					Expect(obj.GetFinalizers()).ToNot(ContainElement(dpuservicev1.DPUDeploymentFinalizer), fmt.Sprintf("%T", obj))
					Expect(obj.GetLabels()).ToNot(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
				}
			})
			It("should be able to mark and clean stale dependencies that other controller have applied finalizers and labels to", func() {
				By("Service side applying the dependencies with finalizers and labels")
				for obj, gvk := range objGVK {
					obj.SetFinalizers([]string{"test.io/some-finalizer"})
					obj.SetLabels(map[string]string{"some": "label"})
					obj.GetObjectKind().SetGroupVersionKind(gvk)
					obj.SetManagedFields(nil)
					Expect(testClient.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
				}

				By("Constructing the dependencies object")
				deps, err := getDependencies(ctx, testClient, dpuDeployment)
				Expect(err).ToNot(HaveOccurred())

				By("Updating the dependencies")
				Expect(updateDependencies(ctx, testClient, dpuDeployment, deps)).To(Succeed())

				By("Checking the current dependencies after update")
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(bfb),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(dpuFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(dpuServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(dpuServiceTemplate),
				} {
					Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					Expect(obj.GetFinalizers()).To(ContainElements(dpuservicev1.DPUDeploymentFinalizer, "test.io/some-finalizer"), fmt.Sprintf("%T", obj))
					Expect(obj.GetLabels()).To(And(
						HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment)), dependentDPUDeploymentLabelValue),
						HaveKeyWithValue("some", "label"),
					), fmt.Sprintf("%T", obj))
				}

				By("Checking the rest of the objects after update")
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(extraBFB),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(extraDPUFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(extraDPUServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(extraDPUServiceTemplate),
				} {
					Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					Expect(obj.GetFinalizers()).ToNot(ContainElement(dpuservicev1.DPUDeploymentFinalizer), fmt.Sprintf("%T", obj))
					Expect(obj.GetFinalizers()).To(ContainElement("test.io/some-finalizer"), fmt.Sprintf("%T", obj))
					Expect(obj.GetLabels()).ToNot(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
					Expect(obj.GetLabels()).To(HaveKeyWithValue("some", "label"), fmt.Sprintf("%T", obj))
				}

				By("Service side applying the dependencies again with finalizers and labels")
				for obj, gvk := range objGVK {
					Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					controllerutil.AddFinalizer(obj, "test.io/some-finalizer")
					labels := obj.GetLabels()
					labels["some"] = "label"
					obj.SetLabels(labels)
					obj.GetObjectKind().SetGroupVersionKind(gvk)
					obj.SetManagedFields(nil)
					Expect(testClient.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
				}

				By("Updating the DPUDeployment deps")
				svc := dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      extraDPUServiceTemplate.Name,
					ServiceConfiguration: extraDPUServiceConfiguration.Name,
				}
				dpuDeployment.Spec.Services["someservice"] = svc
				dpuDeployment.Spec.DPUs.BFB = extraBFB.Name
				dpuDeployment.Spec.DPUs.Flavor = extraDPUFlavor.Name

				By("Constructing the dependencies object")
				deps, err = getDependencies(ctx, testClient, dpuDeployment)
				Expect(err).ToNot(HaveOccurred())

				By("Updating the dependencies")
				Expect(updateDependencies(ctx, testClient, dpuDeployment, deps)).To(Succeed())

				By("Checking the current dependencies after update")
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(extraBFB),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(extraDPUFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(extraDPUServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(extraDPUServiceTemplate),
				} {
					Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					Expect(obj.GetFinalizers()).To(ContainElements(dpuservicev1.DPUDeploymentFinalizer, "test.io/some-finalizer"), fmt.Sprintf("%T", obj))
					Expect(obj.GetLabels()).To(And(
						HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment)), dependentDPUDeploymentLabelValue),
						HaveKeyWithValue("some", "label"),
					), fmt.Sprintf("%T", obj))
				}

				By("Checking the rest of the objects after update")
				for obj, key := range map[client.Object]client.ObjectKey{
					&provisioningv1.BFB{}:                   client.ObjectKeyFromObject(bfb),
					&provisioningv1.DPUFlavor{}:             client.ObjectKeyFromObject(dpuFlavor),
					&dpuservicev1.DPUServiceConfiguration{}: client.ObjectKeyFromObject(dpuServiceConfiguration),
					&dpuservicev1.DPUServiceTemplate{}:      client.ObjectKeyFromObject(dpuServiceTemplate),
				} {
					Expect(testClient.Get(ctx, key, obj)).To(Succeed(), fmt.Sprintf("%T", obj))
					Expect(obj.GetFinalizers()).ToNot(ContainElement(dpuservicev1.DPUDeploymentFinalizer), fmt.Sprintf("%T", obj))
					Expect(obj.GetFinalizers()).To(ContainElement("test.io/some-finalizer"), fmt.Sprintf("%T", obj))
					Expect(obj.GetLabels()).ToNot(HaveKeyWithValue(getDependentDPUDeploymentLabelKey(client.ObjectKeyFromObject(dpuDeployment)), dependentDPUDeploymentLabelValue), fmt.Sprintf("%T", obj))
					Expect(obj.GetLabels()).To(HaveKeyWithValue("some", "label"), fmt.Sprintf("%T", obj))
				}
			})
		})
		Context("When checking reconcileDPUSets()", func() {
			var (
				initialDPUSetSettings []dpuservicev1.DPUSet
				expectedDPUSetSpecs   []provisioningv1.DPUSetSpec
				bfb                   *provisioningv1.BFB
				dpuFlavor             *provisioningv1.DPUFlavor
			)
			BeforeEach(func() {
				dpuFlavor = getMinimalDPUFlavor(testNS.Name)
				initialDPUSetSettings = []dpuservicev1.DPUSet{
					{
						NameSuffix: "dpuset1",
						NodeSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"nodekey1": "nodevalue1",
							},
						},
						DPUSelector: map[string]string{
							"dpukey1": "dpuvalue1",
						},
						DPUAnnotations: map[string]string{
							"annotationkey1": "annotationvalue1",
						},
					},
					{
						NameSuffix: "dpuset2",
						NodeSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"nodekey2": "nodevalue2",
							},
						},
						DPUSelector: map[string]string{
							"dpukey2": "dpuvalue2",
						},
						DPUAnnotations: map[string]string{
							"annotationkey2": "annotationvalue2",
						},
					},
				}

				expectedDPUSetSpecs = []provisioningv1.DPUSetSpec{
					{
						NodeSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"nodekey1": "nodevalue1",
							},
						},
						DPUSelector: map[string]string{
							"dpukey1": "dpuvalue1",
						},
						Strategy: &provisioningv1.DPUSetStrategy{
							Type: provisioningv1.RollingUpdateStrategyType,
						},
						DPUTemplate: provisioningv1.DPUTemplate{
							Annotations: map[string]string{
								"annotationkey1": "annotationvalue1",
							},
							Spec: provisioningv1.DPUTemplateSpec{
								BFB: provisioningv1.BFBReference{
									Name: "somebfb",
								},
								DPUFlavor: "someflavor",
								NodeEffect: &provisioningv1.NodeEffect{
									Drain: ptr.To(true),
								},
								AutomaticNodeReboot: ptr.To(true),
							},
						},
					},
					{
						NodeSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"nodekey2": "nodevalue2",
							},
						},
						DPUSelector: map[string]string{
							"dpukey2": "dpuvalue2",
						},
						Strategy: &provisioningv1.DPUSetStrategy{
							Type: provisioningv1.RollingUpdateStrategyType,
						},
						DPUTemplate: provisioningv1.DPUTemplate{
							Annotations: map[string]string{
								"annotationkey2": "annotationvalue2",
							},
							Spec: provisioningv1.DPUTemplateSpec{
								BFB: provisioningv1.BFBReference{
									Name: "somebfb",
								},
								DPUFlavor: "someflavor",
								NodeEffect: &provisioningv1.NodeEffect{
									Drain: ptr.To(true),
								},
								AutomaticNodeReboot: ptr.To(true),
							},
						},
					},
				}

				By("Creating the dependencies")
				bfb = createMinimalBFBWithStatus("somebfb", testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

				Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
				DeferCleanup(cleanDPUDeploymentDerivatives, testNS.Name)
			})
			It("should create the correct DPUSets", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("retrieving the DPUServiceChain and DPUService")
				var dpuServiceChain *dpuservicev1.DPUServiceChain
				Eventually(func(g Gomega) {
					dpuServiceChainList := getDPUServiceChainList()
					g.Expect(dpuServiceChainList.Items).To(HaveLen(1))
					dpuServiceChain = &dpuServiceChainList.Items[0]
					g.Expect(dpuServiceChain).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				var dpuService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					dpuService = &dpuServiceList.Items[0]
					g.Expect(dpuService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				for i := range expectedDPUSetSpecs {
					expectedDPUSetSpecs[i].DPUTemplate.Spec.Cluster = &provisioningv1.ClusterSpec{
						NodeLabels: map[string]string{
							"svc.dpu.nvidia.com/dpuservicechain-version":        dpuServiceChain.Name,
							"svc.dpu.nvidia.com/dpuservice-someservice-version": dpuService.Name,
							dpuservicev1.ParentDPUDeploymentNameLabel:           fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name),
						},
					}
				}

				By("checking that correct DPUSets are created")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]provisioningv1.DPUSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should update the existing DPUSets on update of the .spec.dpus in the DPUDeployment", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)

				By("retrieving the DPUServiceChain and DPUService")
				var dpuServiceChain *dpuservicev1.DPUServiceChain
				Eventually(func(g Gomega) {
					dpuServiceChainList := getDPUServiceChainList()
					g.Expect(dpuServiceChainList.Items).To(HaveLen(1))
					dpuServiceChain = &dpuServiceChainList.Items[0]
					g.Expect(dpuServiceChain).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					gotDPUService = &dpuServiceList.Items[0]
					g.Expect(gotDPUService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				for i := range expectedDPUSetSpecs {
					expectedDPUSetSpecs[i].DPUTemplate.Spec.Cluster = &provisioningv1.ClusterSpec{
						NodeLabels: map[string]string{
							"svc.dpu.nvidia.com/dpuservicechain-version":        dpuServiceChain.Name,
							"svc.dpu.nvidia.com/dpuservice-someservice-version": gotDPUService.Name,
							dpuservicev1.ParentDPUDeploymentNameLabel:           fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name),
						},
					}
				}

				By("waiting for the initial DPUSets to be applied")
				firstDPUSetUIDs := make([]types.UID, 0, 2)
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
					for _, dpuSet := range gotDPUSetList.Items {
						firstDPUSetUIDs = append(firstDPUSetUIDs, dpuSet.UID)
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUDeployment object and checking the outcome")
				dpuDeployment.Spec.DPUs.DPUSets[1].DPUAnnotations["newkey"] = "newvalue"
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())
				By("checking that correct DPUSets are created")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						// Validate that the same object is updated
						g.Expect(firstDPUSetUIDs).To(ContainElement(dpuSet.UID))

						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]provisioningv1.DPUSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					expectedDPUSetSpecs[1].DPUTemplate.Annotations["newkey"] = "newvalue"
					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should keep the existing DPUSets labels on update of a DPUServiceConfiguration", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("retrieving the DPUServiceChain and DPUService")
				var gotDPUServiceChain *dpuservicev1.DPUServiceChain
				Eventually(func(g Gomega) {
					dpuServiceChainList := getDPUServiceChainList()
					g.Expect(dpuServiceChainList.Items).To(HaveLen(1))
					gotDPUServiceChain = &dpuServiceChainList.Items[0]
					g.Expect(gotDPUServiceChain).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					gotDPUService = &dpuServiceList.Items[0]
					g.Expect(gotDPUService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				for i := range expectedDPUSetSpecs {
					expectedDPUSetSpecs[i].DPUTemplate.Spec.Cluster = &provisioningv1.ClusterSpec{
						NodeLabels: map[string]string{
							"svc.dpu.nvidia.com/dpuservice-someservice-version": gotDPUService.Name,
							dpuservicev1.ParentDPUDeploymentNameLabel:           fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name),
							"svc.dpu.nvidia.com/dpuservicechain-version":        gotDPUServiceChain.Name,
						},
					}
				}

				By("waiting for the initial DPUSets to be applied")
				firstDPUSetUIDs := make([]types.UID, 0, 2)
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
					for _, dpuSet := range gotDPUSetList.Items {
						firstDPUSetUIDs = append(firstDPUSetUIDs, dpuSet.UID)
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUServiceConfiguration object and checking the outcome")
				dpuServiceConfiguration := &dpuservicev1.DPUServiceConfiguration{}
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNS.Name, Name: "someconfiguration"}, dpuServiceConfiguration)).To(Succeed())
				dpuServiceConfiguration.Spec.Interfaces[0].Name = "newname"
				dpuServiceConfiguration.SetManagedFields(nil)
				dpuServiceConfiguration.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
				Expect(testClient.Patch(ctx, dpuServiceConfiguration, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName))).To(Succeed())

				dpuServiceTemplate := &dpuservicev1.DPUServiceTemplate{}
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNS.Name, Name: "sometemplate"}, dpuServiceTemplate)).To(Succeed())

				versionDigest2 := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)

				By("checking that the DPUServices are correctly updated")
				Eventually(func(g Gomega) {
					gotDPUServices := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServices)).To(Succeed())
					g.Expect(gotDPUServices.Items).To(HaveLen(1))

					By("checking the object metadata")
					for _, dpuService := range gotDPUServices.Items {
						g.Expect(dpuService.Labels).To(HaveLen(1))
						g.Expect(dpuService.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuService.Annotations).To(HaveKeyWithValue("svc.dpu.nvidia.com/dpuservice-version", versionDigest2))
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("checking that the DPUSets are correctly updated")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						// Validate that the same object is updated
						g.Expect(firstDPUSetUIDs).To(ContainElement(dpuSet.UID))

						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]provisioningv1.DPUSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should update the existing DPUSets labels on update of a disruptive DPUServiceConfiguration", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("retrieving the DPUServiceChain and DPUService")
				var gotDPUServiceChain *dpuservicev1.DPUServiceChain
				Eventually(func(g Gomega) {
					dpuServiceChainList := getDPUServiceChainList()
					g.Expect(dpuServiceChainList.Items).To(HaveLen(1))
					gotDPUServiceChain = &dpuServiceChainList.Items[0]
					g.Expect(gotDPUServiceChain).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				var gotInitialDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					gotInitialDPUService = &dpuServiceList.Items[0]
					g.Expect(gotInitialDPUService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				for i := range expectedDPUSetSpecs {
					expectedDPUSetSpecs[i].DPUTemplate.Spec.Cluster = &provisioningv1.ClusterSpec{
						NodeLabels: map[string]string{
							"svc.dpu.nvidia.com/dpuservice-someservice-version": gotInitialDPUService.Name,
							dpuservicev1.ParentDPUDeploymentNameLabel:           fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name),
							"svc.dpu.nvidia.com/dpuservicechain-version":        gotDPUServiceChain.Name,
						},
					}
				}

				By("waiting for the initial DPUSets to be applied")
				firstDPUSetUIDs := make([]types.UID, 0, 2)
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
					for _, dpuSet := range gotDPUSetList.Items {
						firstDPUSetUIDs = append(firstDPUSetUIDs, dpuSet.UID)
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUServiceConfiguration object and checking the outcome")
				dpuServiceConfiguration := &dpuservicev1.DPUServiceConfiguration{}
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNS.Name, Name: "someconfiguration"}, dpuServiceConfiguration)).To(Succeed())
				dpuServiceConfiguration.Spec.Interfaces[0].Name = "newname"
				// make it disruptive
				dpuServiceConfiguration.Spec.NodeEffect = ptr.To(true)
				dpuServiceConfiguration.SetManagedFields(nil)
				dpuServiceConfiguration.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
				Expect(testClient.Patch(ctx, dpuServiceConfiguration, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName))).To(Succeed())

				dpuServiceTemplate := &dpuservicev1.DPUServiceTemplate{}
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNS.Name, Name: "sometemplate"}, dpuServiceTemplate)).To(Succeed())

				versionDigest2 := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)

				var gotDPUService *dpuservicev1.DPUService
				By("checking that the DPUServices are correctly updated")
				Eventually(func(g Gomega) {
					gotDPUServices := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServices)).To(Succeed())
					g.Expect(gotDPUServices.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuService := range gotDPUServices.Items {
						g.Expect(dpuService.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						if dpuService.Annotations["svc.dpu.nvidia.com/dpuservice-version"] == versionDigest2 {
							gotDPUService = &dpuService
						}
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				Expect(gotDPUService).ToNot(Equal(gotInitialDPUService))
				for i := range expectedDPUSetSpecs {
					expectedDPUSetSpecs[i].DPUTemplate.Spec.Cluster = &provisioningv1.ClusterSpec{
						NodeLabels: map[string]string{
							"svc.dpu.nvidia.com/dpuservice-someservice-version": gotDPUService.Name,
							dpuservicev1.ParentDPUDeploymentNameLabel:           fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name),
							"svc.dpu.nvidia.com/dpuservicechain-version":        gotDPUServiceChain.Name,
						},
					}
				}

				By("checking that the DPUSets are correctly updated")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						// Validate that the same object is updated
						g.Expect(firstDPUSetUIDs).To(ContainElement(dpuSet.UID))

						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]provisioningv1.DPUSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("marking the DPUService ready")
				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				Expect(gotDPUServiceList.Items).To(HaveLen(2))
				for _, dpuService := range gotDPUServiceList.Items {
					if dpuService.Annotations["svc.dpu.nvidia.com/dpuservice-version"] == versionDigest2 {
						dpuService.Status.Conditions = []metav1.Condition{
							{
								Type:               string(conditions.TypeReady),
								Status:             metav1.ConditionTrue,
								Reason:             string(conditions.ReasonSuccess),
								LastTransitionTime: metav1.NewTime(time.Now()),
							},
						}
						dpuService.SetGroupVersionKind(dpuservicev1.DPUServiceGroupVersionKind)
						dpuService.SetManagedFields(nil)
						Expect(testClient.Status().Patch(ctx, &dpuService, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
					}
				}

				By("checking that the DPUServices are correctly updated")
				Eventually(func(g Gomega) {
					gotDPUServices := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServices)).To(Succeed())
					g.Expect(gotDPUServices.Items).To(HaveLen(1))

					By("checking the object metadata")
					for _, dpuService := range gotDPUServices.Items {
						g.Expect(dpuService.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuService.Annotations).To(HaveKeyWithValue("svc.dpu.nvidia.com/dpuservice-version", versionDigest2))
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("checking that the DPUSets are correctly updated")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						// Validate that the same object is updated
						g.Expect(firstDPUSetUIDs).To(ContainElement(dpuSet.UID))

						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]provisioningv1.DPUSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should update the existing DPUSets labels on update of a disruptive DPUServiceChain", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)
				chainDigest := calculateDPUServiceChainVersionDigest(dpuDeployment.Spec.ServiceChains.Switches)

				By("retrieving the DPUServiceChain and DPUService")
				var gotInitialDPUServiceChain *dpuservicev1.DPUServiceChain
				Eventually(func(g Gomega) {
					dpuServiceChainList := getDPUServiceChainList()
					g.Expect(dpuServiceChainList.Items).To(HaveLen(1))
					gotInitialDPUServiceChain = &dpuServiceChainList.Items[0]
					g.Expect(gotInitialDPUServiceChain).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					gotDPUService = &dpuServiceList.Items[0]
					g.Expect(gotDPUService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				for i := range expectedDPUSetSpecs {
					expectedDPUSetSpecs[i].DPUTemplate.Spec.Cluster = &provisioningv1.ClusterSpec{
						NodeLabels: map[string]string{
							"svc.dpu.nvidia.com/dpuservice-someservice-version": gotDPUService.Name,
							dpuservicev1.ParentDPUDeploymentNameLabel:           fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name),
							"svc.dpu.nvidia.com/dpuservicechain-version":        gotInitialDPUServiceChain.Name,
						},
					}
				}

				By("waiting for the initial DPUSets to be applied")
				firstDPUSetUIDs := make([]types.UID, 0, 2)
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
					for _, dpuSet := range gotDPUSetList.Items {
						firstDPUSetUIDs = append(firstDPUSetUIDs, dpuSet.UID)
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the dpudeployment service chain and checking the outcome")
				dpuDeployment.Spec.ServiceChains = dpuservicev1.ServiceChains{
					// make the chain disruptive
					NodeEffect: ptr.To(true),
					Switches: []dpuservicev1.DPUDeploymentSwitch{
						{
							Ports: []dpuservicev1.DPUDeploymentPort{
								{
									Service: &dpuservicev1.DPUDeploymentService{
										InterfaceName: "someinterface",
										Name:          "somedpuservice",
									},
								},
								{
									Service: &dpuservicev1.DPUDeploymentService{
										InterfaceName: "someinterface2",
										Name:          "somedpuservice2",
										IPAM: &dpuservicev1.IPAM{
											MatchLabels: map[string]string{
												"ipamkey1": "ipamvalue1",
											},
										},
									},
								},
							},
						},
						{
							Ports: []dpuservicev1.DPUDeploymentPort{
								{
									Service: &dpuservicev1.DPUDeploymentService{
										InterfaceName: "someotherinterface",
										Name:          "someotherservice",
									},
								},
							},
						},
						{
							Ports: []dpuservicev1.DPUDeploymentPort{
								{
									ServiceInterface: &dpuservicev1.ServiceIfc{
										MatchLabels: map[string]string{
											"key": "value",
										},
										IPAM: &dpuservicev1.IPAM{
											MatchLabels: map[string]string{
												"ipamkey2": "ipamvalue2",
											},
										},
									},
								},
							},
						},
					},
				}
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())
				chainDigest2 := calculateDPUServiceChainVersionDigest(dpuDeployment.Spec.ServiceChains.Switches)
				Expect(chainDigest2).NotTo(Equal(chainDigest))

				By("checking that the DPUServiceChain is correctly updated")
				var gotDPUServiceChain *dpuservicev1.DPUServiceChain
				Eventually(func(g Gomega) {
					gotDPUServiceChains := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChains)).To(Succeed())
					g.Expect(gotDPUServiceChains.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuServiceChain := range gotDPUServiceChains.Items {
						g.Expect(dpuServiceChain.Labels).To(HaveLen(1))
						g.Expect(dpuServiceChain.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						if dpuServiceChain.Annotations["svc.dpu.nvidia.com/dpuservicechain-version"] == chainDigest2 {
							gotDPUServiceChain = &dpuServiceChain
						}
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				Expect(gotDPUServiceChain).ToNot(Equal(gotInitialDPUServiceChain))
				for i := range expectedDPUSetSpecs {
					expectedDPUSetSpecs[i].DPUTemplate.Spec.Cluster = &provisioningv1.ClusterSpec{
						NodeLabels: map[string]string{
							"svc.dpu.nvidia.com/dpuservice-someservice-version": gotDPUService.Name,
							dpuservicev1.ParentDPUDeploymentNameLabel:           fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name),
							"svc.dpu.nvidia.com/dpuservicechain-version":        gotDPUServiceChain.Name,
						},
					}
				}

				By("checking that the DPUSets are correctly updated")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						// Validate that the same object is updated
						g.Expect(firstDPUSetUIDs).To(ContainElement(dpuSet.UID))

						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]provisioningv1.DPUSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should keep the existing DPUSets labels on update of a dpudeployment service chain", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)
				chainDigest := calculateDPUServiceChainVersionDigest(dpuDeployment.Spec.ServiceChains.Switches)

				By("retrieving the DPUServiceChain and DPUService")
				var gotDPUServiceChain *dpuservicev1.DPUServiceChain
				Eventually(func(g Gomega) {
					dpuServiceChainList := getDPUServiceChainList()
					g.Expect(dpuServiceChainList.Items).To(HaveLen(1))
					gotDPUServiceChain = &dpuServiceChainList.Items[0]
					g.Expect(gotDPUServiceChain).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					gotDPUService = &dpuServiceList.Items[0]
					g.Expect(gotDPUService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				for i := range expectedDPUSetSpecs {
					expectedDPUSetSpecs[i].DPUTemplate.Spec.Cluster = &provisioningv1.ClusterSpec{
						NodeLabels: map[string]string{
							"svc.dpu.nvidia.com/dpuservice-someservice-version": gotDPUService.Name,
							dpuservicev1.ParentDPUDeploymentNameLabel:           fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name),
							"svc.dpu.nvidia.com/dpuservicechain-version":        gotDPUServiceChain.Name,
						},
					}
				}

				By("waiting for the initial DPUSets to be applied")
				firstDPUSetUIDs := make([]types.UID, 0, 2)
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
					for _, dpuSet := range gotDPUSetList.Items {
						firstDPUSetUIDs = append(firstDPUSetUIDs, dpuSet.UID)
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the dpudeployment service chain and checking the outcome")
				dpuDeployment.Spec.ServiceChains.Switches = []dpuservicev1.DPUDeploymentSwitch{
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someinterface",
									Name:          "somedpuservice",
								},
							},
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someinterface2",
									Name:          "somedpuservice2",
									IPAM: &dpuservicev1.IPAM{
										MatchLabels: map[string]string{
											"ipamkey1": "ipamvalue1",
										},
									},
								},
							},
						},
					},
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someotherinterface",
									Name:          "someotherservice",
								},
							},
						},
					},
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								ServiceInterface: &dpuservicev1.ServiceIfc{
									MatchLabels: map[string]string{
										"key": "value",
									},
									IPAM: &dpuservicev1.IPAM{
										MatchLabels: map[string]string{
											"ipamkey2": "ipamvalue2",
										},
									},
								},
							},
						},
					},
				}
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())

				By("checking that the DPUServiceChain is correctly updated")
				Eventually(func(g Gomega) {
					gotDPUServiceChains := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChains)).To(Succeed())
					g.Expect(gotDPUServiceChains.Items).To(HaveLen(1))

					By("checking the object metadata")
					for _, dpuServiceChain := range gotDPUServiceChains.Items {
						g.Expect(dpuServiceChain.Labels).To(HaveLen(1))
						g.Expect(dpuServiceChain.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuServiceChain.Annotations).To(HaveKeyWithValue(dpuServiceChainVersionLabelAnnotationKey, chainDigest))
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("checking that the DPUSets are correctly updated")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						// Validate that the same object is updated
						g.Expect(firstDPUSetUIDs).To(ContainElement(dpuSet.UID))

						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]provisioningv1.DPUSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should update the existing DPUSets on update of the referenced BFB", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)

				By("retrieving the DPUServiceChain and DPUService")
				var gotDPUServiceChain *dpuservicev1.DPUServiceChain
				Eventually(func(g Gomega) {
					dpuServiceChainList := getDPUServiceChainList()
					g.Expect(dpuServiceChainList.Items).To(HaveLen(1))
					gotDPUServiceChain = &dpuServiceChainList.Items[0]
					g.Expect(gotDPUServiceChain).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					gotDPUService = &dpuServiceList.Items[0]
					g.Expect(gotDPUService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				for i := range expectedDPUSetSpecs {
					expectedDPUSetSpecs[i].DPUTemplate.Spec.Cluster = &provisioningv1.ClusterSpec{
						NodeLabels: map[string]string{
							"svc.dpu.nvidia.com/dpuservice-someservice-version": gotDPUService.Name,
							dpuservicev1.ParentDPUDeploymentNameLabel:           fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name),
							"svc.dpu.nvidia.com/dpuservicechain-version":        gotDPUServiceChain.Name,
						},
					}
				}

				By("waiting for the initial DPUSets to be applied")
				firstDPUSetUIDs := make([]types.UID, 0, 2)
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
					for _, dpuSet := range gotDPUSetList.Items {
						firstDPUSetUIDs = append(firstDPUSetUIDs, dpuSet.UID)
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("creating a new BFB object and checking the outcome")
				bfb2 := createMinimalBFBWithStatus("somebfb2", testNS.Name)

				By("Updating the DPUDeployment object to reference the new BFB")
				dpuDeployment.Spec.DPUs.BFB = bfb2.Name
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())

				// Update the expected DPUSetSpecs
				expectedDPUSetSpecs[0].DPUTemplate.Spec.BFB.Name = bfb2.Name
				expectedDPUSetSpecs[1].DPUTemplate.Spec.BFB.Name = bfb2.Name

				By("checking that the DPUSets are correctly updated")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						// Validate that the same object is updated
						g.Expect(firstDPUSetUIDs).To(ContainElement(dpuSet.UID))

						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]provisioningv1.DPUSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should update existing and create new DPUSets on update of the .spec.dpus in the DPUDeployment", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)

				By("retrieving the DPUServiceChain and DPUService")
				var gotDPUServiceChain *dpuservicev1.DPUServiceChain
				Eventually(func(g Gomega) {
					dpuServiceChainList := getDPUServiceChainList()
					g.Expect(dpuServiceChainList.Items).To(HaveLen(1))
					gotDPUServiceChain = &dpuServiceChainList.Items[0]
					g.Expect(gotDPUServiceChain).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					gotDPUService = &dpuServiceList.Items[0]
					g.Expect(gotDPUService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				for i := range expectedDPUSetSpecs {
					expectedDPUSetSpecs[i].DPUTemplate.Spec.Cluster = &provisioningv1.ClusterSpec{
						NodeLabels: map[string]string{
							"svc.dpu.nvidia.com/dpuservice-someservice-version": gotDPUService.Name,
							dpuservicev1.ParentDPUDeploymentNameLabel:           fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name),
							"svc.dpu.nvidia.com/dpuservicechain-version":        gotDPUServiceChain.Name,
						},
					}
				}

				By("waiting for the initial DPUSets to be applied")
				firstDPUSetUIDs := make(map[types.UID]interface{})
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
					for _, dpuSet := range gotDPUSetList.Items {
						firstDPUSetUIDs[dpuSet.UID] = struct{}{}
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUDeployment object and checking the outcome")
				dpuDeployment.Spec.DPUs.DPUSets[1].DPUAnnotations["newkey"] = "newvalue"
				dpuDeployment.Spec.DPUs.DPUSets = append(dpuDeployment.Spec.DPUs.DPUSets, dpuservicev1.DPUSet{
					NameSuffix: "dpuset3",
					NodeSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nodekey3": "nodevalue3",
						},
					},
					DPUSelector: map[string]string{
						"dpukey3": "dpuvalue3",
					},
					DPUAnnotations: map[string]string{
						"annotationkey3": "annotationvalue3",
					},
				})
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())
				By("checking that correct DPUSets are created")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(3))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))

						delete(firstDPUSetUIDs, dpuSet.UID)

						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					// Validate that all original objects are there and not recreated
					g.Expect(firstDPUSetUIDs).To(BeEmpty())

					By("checking the specs")
					specs := make([]provisioningv1.DPUSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					expectedDPUSetSpecs[1].DPUTemplate.Annotations["newkey"] = "newvalue"
					expectedDPUSetSpecs = append(expectedDPUSetSpecs, provisioningv1.DPUSetSpec{
						NodeSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"nodekey3": "nodevalue3",
							},
						},
						DPUSelector: map[string]string{
							"dpukey3": "dpuvalue3",
						},
						Strategy: &provisioningv1.DPUSetStrategy{
							Type: provisioningv1.RollingUpdateStrategyType,
						},
						DPUTemplate: provisioningv1.DPUTemplate{
							Annotations: map[string]string{
								"annotationkey3": "annotationvalue3",
							},
							Spec: provisioningv1.DPUTemplateSpec{
								BFB: provisioningv1.BFBReference{
									Name: "somebfb",
								},
								DPUFlavor: "someflavor",
								NodeEffect: &provisioningv1.NodeEffect{
									Drain: ptr.To(true),
								},
								AutomaticNodeReboot: ptr.To(true),
								Cluster: &provisioningv1.ClusterSpec{
									NodeLabels: map[string]string{
										"svc.dpu.nvidia.com/dpuservice-someservice-version": gotDPUService.Name,
										dpuservicev1.ParentDPUDeploymentNameLabel:           fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name),
										"svc.dpu.nvidia.com/dpuservicechain-version":        gotDPUServiceChain.Name,
									},
								},
							},
						},
					})

					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(30 * time.Second).Should(Succeed())

			})
			It("should delete DPUSets that are no longer part of the DPUDeployment", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)

				By("waiting for the initial DPUSets to be applied")
				firstDPUSetUIDs := make([]types.UID, 0, 2)
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
					for _, dpuSet := range gotDPUSetList.Items {
						firstDPUSetUIDs = append(firstDPUSetUIDs, dpuSet.UID)
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUDeployment object and checking the outcome")
				dpuDeployment.Spec.DPUs.DPUSets = dpuDeployment.Spec.DPUs.DPUSets[1:]
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())
				By("checking that correct DPUSets are created")
				expectedDPUSetSpecs = expectedDPUSetSpecs[1:]
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(1))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should create new DPUSets on manual deletion of the DPUSets", func() {
				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("waiting for the initial DPUSets to be applied")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("manually deleting the DPUSets")
				Consistently(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					if len(gotDPUSetList.Items) == 0 {
						return
					}
					objs := []client.Object{}
					for _, dpuSet := range gotDPUSetList.Items {
						objs = append(objs, &dpuSet)
					}
					g.Expect(testutils.CleanupAndWait(ctx, testClient, objs...)).To(Succeed())
				}).WithTimeout(5 * time.Second).Should(Succeed())

				By("checking that the DPUSets is created")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DPUSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
		})
		Context("When checking reconcileDPUServiceInterfaces()", func() {
			BeforeEach(func() {
				By("Creating the dependencies")
				bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

				dpuFlavor := getMinimalDPUFlavor(testNS.Name)
				Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

				DeferCleanup(cleanDPUDeploymentDerivatives, testNS.Name)
			})
			It("should create the correct DPUServiceInterfaces", func() {
				By("Creating the dependencies")
				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "some_interface",
						Network: "nad1",
					},
					{
						Name:    "someotherinterface",
						Network: "nad2",
					},
					{
						Name:           "virtualinterface",
						Network:        "nad3",
						VirtualNetwork: ptr.To("vnet1"),
					},
				}
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				versionDigest := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)
				By("checking that correct DPUServiceInterfaces are created")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(3))

					By("checking the object metadata")
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						g.Expect(dpuServiceInterface.Labels).To(HaveLen(1))
						g.Expect(dpuServiceInterface.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuServiceInterface.Annotations).To(HaveKeyWithValue("svc.dpu.nvidia.com/dpuservice-version", versionDigest))
						g.Expect(dpuServiceInterface.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("retrieving the DPUService")
					var gotDPUService *dpuservicev1.DPUService
					Eventually(func(g Gomega) {
						dpuServiceList := getDPUServiceList()
						g.Expect(dpuServiceList.Items).To(HaveLen(1))
						gotDPUService = &dpuServiceList.Items[0]
						g.Expect(gotDPUService).ToNot(BeNil())
					}).WithTimeout(30 * time.Second).Should(Succeed())

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceInterfaceSpec, 0, len(gotDPUServiceInterfaceList.Items))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						specs = append(specs, dpuServiceInterface.Spec)
					}

					genExpectedDPUServiceInterfaceSpecs := func(dpuserviceName, networkName, ifName string, virtualNetworkName *string) dpuservicev1.DPUServiceInterfaceSpec {
						return dpuservicev1.DPUServiceInterfaceSpec{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{dpuserviceName},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: ifName,
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:      "dpudeployment_dpudeployment_someservice",
												Network:        networkName,
												InterfaceName:  ifName,
												VirtualNetwork: virtualNetworkName,
											},
										},
									},
								},
							},
						}
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceInterfaceSpec{
						genExpectedDPUServiceInterfaceSpecs(gotDPUService.Name, "nad1", "some_interface", nil),
						genExpectedDPUServiceInterfaceSpecs(gotDPUService.Name, "nad2", "someotherinterface", nil),
						genExpectedDPUServiceInterfaceSpecs(gotDPUService.Name, "nad3", "virtualinterface", ptr.To("vnet1")),
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should patch a manually modified DPUServiceInterface as long as the modification is not on the version annotation", func() {
				By("Creating the dependencies")
				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "some_interface",
						Network: "nad1",
					},
				}
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := getMinimalDPUServiceTemplate(testNS.Name)
				Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
				patchDPUServiceTemplateWithStatus(dpuServiceTemplate)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("checking that the DPUServiceInterface is created")
				var gotDPUServiceInterface *dpuservicev1.DPUServiceInterface
				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(1))
					gotDPUServiceInterface = &gotDPUServiceInterfaceList.Items[0]
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(1))
					gotDPUService = &gotDPUServiceList.Items[0]

					By("checking the original value of the network")
					g.Expect(gotDPUServiceInterface.Spec.Template.Spec.Template.Spec.Service.Network).To(Equal("nad1"))

					By("checking the nodeSelector")
					g.Expect(gotDPUServiceInterface.Spec.Template.Spec.NodeSelector).To(BeComparableTo(&metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{gotDPUService.Name},
							},
							{
								Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUServiceInterface manually")
				gotDPUServiceInterface.Spec.Template.Spec.Template.Spec.Service.Network = "nad15"
				gotDPUServiceInterface.SetManagedFields(nil)
				gotDPUServiceInterface.SetGroupVersionKind(dpuservicev1.DPUServiceInterfaceGroupVersionKind)
				Expect(testClient.Patch(ctx, gotDPUServiceInterface, client.Apply, client.ForceOwnership, client.FieldOwner("some-test-controller"))).To(Succeed())

				By("checking that the DPUServiceInterface is reverted to the original")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(1))
					g.Expect(gotDPUServiceInterfaceList.Items[0].UID).To(Equal(gotDPUServiceInterface.UID))

					By("checking the original value of the network")
					g.Expect(gotDPUServiceInterfaceList.Items[0].Spec.Template.Spec.Template.Spec.Service.Network).To(Equal("nad1"))

					By("checking the nodeSelector")
					g.Expect(gotDPUServiceInterfaceList.Items[0].Spec.Template.Spec.NodeSelector).To(BeComparableTo(&metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{gotDPUService.Name},
							},
							{
								Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should update the existing DPUServiceInterfaces on update of non-disruptive DPUServiceConfiguration", func() {
				By("Creating the dependencies")
				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "some_interface",
						Network: "nad1",
					},
					{
						Name:    "someotherinterface",
						Network: "nad2",
					},
				}
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("retrieving the DPUService")
				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					gotDPUService = &dpuServiceList.Items[0]
					g.Expect(gotDPUService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("waiting for the initial DPUServiceInterfaces to be applied")
				firstDPUServiceInterfaceUIDs := make([]types.UID, 0, 2)
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))
					for _, dpuService := range gotDPUServiceInterfaceList.Items {
						firstDPUServiceInterfaceUIDs = append(firstDPUServiceInterfaceUIDs, dpuService.UID)
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUServiceConfiguration object and checking the outcome")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuServiceConfiguration), dpuServiceConfiguration)).To(Succeed())
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "some_interface",
						Network: "nad3",
					},
					{
						Name:    "someotherinterface",
						Network: "nad4",
					},
				}
				dpuServiceConfiguration.SetManagedFields(nil)
				dpuServiceConfiguration.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
				Expect(testClient.Patch(ctx, dpuServiceConfiguration, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName))).To(Succeed())

				versionDigest := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)
				By("checking that the DPUServiceInterfaces are updated")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))

					// Validate that these are the same objects
					for _, serviceInterface := range gotDPUServiceInterfaceList.Items {
						g.Expect(firstDPUServiceInterfaceUIDs).To(ContainElement(serviceInterface.UID))
					}

					By("checking the object metadata")
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						g.Expect(dpuServiceInterface.Labels).To(HaveLen(1))
						g.Expect(dpuServiceInterface.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuServiceInterface.Annotations).To(HaveKeyWithValue("svc.dpu.nvidia.com/dpuservice-version", versionDigest))
						g.Expect(dpuServiceInterface.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceInterfaceSpec, 0, len(gotDPUServiceInterfaceList.Items))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						specs = append(specs, dpuServiceInterface.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceInterfaceSpec{
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "some_interface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad3",
												InterfaceName: "some_interface",
											},
										},
									},
								},
							},
						},
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someotherinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad4",
												InterfaceName: "someotherinterface",
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should create new DPUServiceInterfaces on update of disruptive DPUServiceConfiguration", func() {
				By("Creating the dependencies")
				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				// Make the dpuService disruptive
				dpuServiceConfiguration.Spec.NodeEffect = ptr.To(true)
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "someinterface",
						Network: "nad1",
					},
					{
						Name:    "someotherinterface",
						Network: "nad2",
					},
				}
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				versionDigest := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("retrieving the initial DPUService")
				var gotInitialDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					gotInitialDPUService = &dpuServiceList.Items[0]
					g.Expect(gotInitialDPUService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("waiting for the initial DPUServiceInterface to be applied")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUServiceConfiguration object and checking the outcome")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuServiceConfiguration), dpuServiceConfiguration)).To(Succeed())
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "someinterface",
						Network: "nad3",
					},
					{
						Name:    "someotherinterface",
						Network: "nad4",
					},
				}
				dpuServiceConfiguration.SetManagedFields(nil)
				dpuServiceConfiguration.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
				Expect(testClient.Patch(ctx, dpuServiceConfiguration, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName))).To(Succeed())

				versionDigest2 := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)
				Expect(versionDigest).ToNot(Equal(versionDigest2))

				By("retrieving the new DPUService")
				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))
					for _, dpuService := range gotDPUServiceList.Items {
						if dpuService.GetAnnotations()[dpuServiceVersionAnnotationKey] == versionDigest2 {
							gotDPUService = &dpuService
						}
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("checking that the old DPUServiceInterfaces co-exist with the new ones until the current DPUService becomes ready")
				Eventually(func(g Gomega) {
					instancesThatShouldExistPerVersionDigest := map[string]int{
						versionDigest:  2,
						versionDigest2: 2,
					}
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(4))

					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						g.Expect(dpuServiceInterface.Labels).To(HaveLen(1))
						g.Expect(dpuServiceInterface.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuServiceInterface.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))

						versionAnnotationKey := "svc.dpu.nvidia.com/dpuservice-version"
						g.Expect(dpuServiceInterface.Annotations).To(HaveKey(versionAnnotationKey))
						versionAnnotationValue := dpuServiceInterface.Annotations["svc.dpu.nvidia.com/dpuservice-version"]
						g.Expect(instancesThatShouldExistPerVersionDigest).To(HaveKey(versionAnnotationValue))
						instancesThatShouldExistPerVersionDigest[versionAnnotationValue]--
					}
					for k := range instancesThatShouldExistPerVersionDigest {
						g.Expect(instancesThatShouldExistPerVersionDigest[k]).To(BeZero())
					}

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceInterfaceSpec, 0, len(gotDPUServiceInterfaceList.Items))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						specs = append(specs, dpuServiceInterface.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceInterfaceSpec{
						// Old
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotInitialDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad1",
												InterfaceName: "someinterface",
											},
										},
									},
								},
							},
						},
						// Old
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotInitialDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someotherinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad2",
												InterfaceName: "someotherinterface",
											},
										},
									},
								},
							},
						},
						// New
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad3",
												InterfaceName: "someinterface",
											},
										},
									},
								},
							},
						},
						// New
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someotherinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad4",
												InterfaceName: "someotherinterface",
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).MustPassRepeatedly(10).Should(Succeed())

				By("Marking the DPUService ready")
				patcher := patch.NewSerialPatcher(gotDPUService, testClient)
				gotDPUService.Status.Conditions = []metav1.Condition{
					{
						Type:               string(conditions.TypeReady),
						Status:             metav1.ConditionTrue,
						Reason:             string(conditions.ReasonSuccess),
						LastTransitionTime: metav1.NewTime(time.Now()),
					},
				}
				Expect(patcher.Patch(ctx, gotDPUService, patch.WithFieldOwner("test"))).To(Succeed())

				By("checking that the DPUServiceInterfaces are updated")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						g.Expect(dpuServiceInterface.Labels).To(HaveLen(1))
						g.Expect(dpuServiceInterface.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuServiceInterface.Annotations).To(HaveKeyWithValue("svc.dpu.nvidia.com/dpuservice-version", versionDigest2))
						g.Expect(dpuServiceInterface.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceInterfaceSpec, 0, len(gotDPUServiceInterfaceList.Items))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						specs = append(specs, dpuServiceInterface.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceInterfaceSpec{
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad3",
												InterfaceName: "someinterface",
											},
										},
									},
								},
							},
						},
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someotherinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad4",
												InterfaceName: "someotherinterface",
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should not delete the old DPUServiceInterfaces on update of disruptive DPUServiceConfiguration "+
				"that changes the name of an interface", func() {
				By("Creating the dependencies")
				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				// Make the dpuService disruptive
				dpuServiceConfiguration.Spec.NodeEffect = ptr.To(true)
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "someinterface",
						Network: "nad1",
					},
					{
						Name:    "someotherinterface",
						Network: "nad2",
					},
				}
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := getMinimalDPUServiceTemplate(testNS.Name)
				Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
				patchDPUServiceTemplateWithStatus(dpuServiceTemplate)

				versionDigest := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("retrieving the initial DPUService")
				var gotInitialDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					gotInitialDPUService = &dpuServiceList.Items[0]
					g.Expect(gotInitialDPUService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("waiting for the initial DPUServiceInterface to be applied")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUServiceConfiguration object and checking the outcome")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuServiceConfiguration), dpuServiceConfiguration)).To(Succeed())
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "newnameforsomeinterface",
						Network: "nad3",
					},
					{
						Name:    "someotherinterface",
						Network: "nad4",
					},
				}
				dpuServiceConfiguration.SetManagedFields(nil)
				dpuServiceConfiguration.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
				Expect(testClient.Patch(ctx, dpuServiceConfiguration, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName))).To(Succeed())

				versionDigest2 := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)
				Expect(versionDigest).ToNot(Equal(versionDigest2))

				By("retrieving the new DPUService")
				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))
					for _, dpuService := range gotDPUServiceList.Items {
						if dpuService.GetAnnotations()[dpuServiceVersionAnnotationKey] == versionDigest2 {
							gotDPUService = &dpuService
						}
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("checking that the old DPUServiceInterfaces co-exist with the new ones until the current DPUService becomes ready")
				Eventually(func(g Gomega) {
					instancesThatShouldExistPerVersionDigest := map[string]int{
						versionDigest:  2,
						versionDigest2: 2,
					}
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(4))

					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						g.Expect(dpuServiceInterface.Labels).To(HaveLen(1))
						g.Expect(dpuServiceInterface.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuServiceInterface.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))

						versionAnnotationKey := "svc.dpu.nvidia.com/dpuservice-version"
						g.Expect(dpuServiceInterface.Annotations).To(HaveKey(versionAnnotationKey))
						versionAnnotationValue := dpuServiceInterface.Annotations["svc.dpu.nvidia.com/dpuservice-version"]
						g.Expect(instancesThatShouldExistPerVersionDigest).To(HaveKey(versionAnnotationValue))
						instancesThatShouldExistPerVersionDigest[versionAnnotationValue]--
					}
					for k := range instancesThatShouldExistPerVersionDigest {
						g.Expect(instancesThatShouldExistPerVersionDigest[k]).To(BeZero())
					}

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceInterfaceSpec, 0, len(gotDPUServiceInterfaceList.Items))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						specs = append(specs, dpuServiceInterface.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceInterfaceSpec{
						// Old
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotInitialDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad1",
												InterfaceName: "someinterface",
											},
										},
									},
								},
							},
						},
						// Old
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotInitialDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someotherinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad2",
												InterfaceName: "someotherinterface",
											},
										},
									},
								},
							},
						},
						// New
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "newnameforsomeinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad3",
												InterfaceName: "newnameforsomeinterface",
											},
										},
									},
								},
							},
						},
						// New
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someotherinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad4",
												InterfaceName: "someotherinterface",
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).MustPassRepeatedly(10).Should(Succeed())

				By("Marking the DPUService ready")
				patcher := patch.NewSerialPatcher(gotDPUService, testClient)
				gotDPUService.Status.Conditions = []metav1.Condition{
					{
						Type:               string(conditions.TypeReady),
						Status:             metav1.ConditionTrue,
						Reason:             string(conditions.ReasonSuccess),
						LastTransitionTime: metav1.NewTime(time.Now()),
					},
				}
				Expect(patcher.Patch(ctx, gotDPUService, patch.WithFieldOwner("test"))).To(Succeed())

				By("checking that the DPUServiceInterfaces are updated")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						g.Expect(dpuServiceInterface.Labels).To(HaveLen(1))
						g.Expect(dpuServiceInterface.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuServiceInterface.Annotations).To(HaveKeyWithValue("svc.dpu.nvidia.com/dpuservice-version", versionDigest2))
						g.Expect(dpuServiceInterface.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceInterfaceSpec, 0, len(gotDPUServiceInterfaceList.Items))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						specs = append(specs, dpuServiceInterface.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceInterfaceSpec{
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "newnameforsomeinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad3",
												InterfaceName: "newnameforsomeinterface",
											},
										},
									},
								},
							},
						},
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someotherinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad4",
												InterfaceName: "someotherinterface",
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should not delete any of the DPUServiceInterfaces associated with service 2 on update of disruptive "+
				"DPUServiceConfiguration that changes the name of an interface that is associated with service 1", func() {
				By("Creating the dependencies for service 1")
				dpuServiceConfiguration1 := getMinimalDPUServiceConfiguration(testNS.Name)
				// Make the dpuService disruptive
				dpuServiceConfiguration1.Name = "service-1"
				dpuServiceConfiguration1.Spec.DeploymentServiceName = "service-1"
				dpuServiceConfiguration1.Spec.NodeEffect = ptr.To(true)
				dpuServiceConfiguration1.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "someinterface",
						Network: "nad1",
					},
					{
						Name:    "someotherinterface",
						Network: "nad2",
					},
				}
				Expect(testClient.Create(ctx, dpuServiceConfiguration1)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration1)

				dpuServiceTemplate1 := getMinimalDPUServiceTemplate(testNS.Name)
				dpuServiceTemplate1.Name = "service-1"
				dpuServiceTemplate1.Spec.DeploymentServiceName = "service-1"
				Expect(testClient.Create(ctx, dpuServiceTemplate1)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate1)
				patchDPUServiceTemplateWithStatus(dpuServiceTemplate1)

				versionDigest1 := calculateDPUServiceVersionDigest(dpuServiceConfiguration1, dpuServiceTemplate1)

				By("Creating the dependencies for service 2")
				dpuServiceConfiguration2 := getMinimalDPUServiceConfiguration(testNS.Name)
				// Make the dpuService disruptive
				dpuServiceConfiguration2.Name = "service-2"
				dpuServiceConfiguration2.Spec.DeploymentServiceName = "service-2"
				dpuServiceConfiguration2.Spec.NodeEffect = ptr.To(true)
				dpuServiceConfiguration2.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "if1",
						Network: "nad3",
					},
					{
						Name:    "if2",
						Network: "nad4",
					},
				}
				Expect(testClient.Create(ctx, dpuServiceConfiguration2)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration2)

				dpuServiceTemplate2 := getMinimalDPUServiceTemplate(testNS.Name)
				dpuServiceTemplate2.Name = "service-2"
				dpuServiceTemplate2.Spec.DeploymentServiceName = "service-2"
				Expect(testClient.Create(ctx, dpuServiceTemplate2)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate2)
				patchDPUServiceTemplateWithStatus(dpuServiceTemplate2)

				versionDigest2 := calculateDPUServiceVersionDigest(dpuServiceConfiguration2, dpuServiceTemplate2)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.Services = map[string]dpuservicev1.DPUDeploymentServiceConfiguration{
					"service-1": {
						ServiceTemplate:      dpuServiceTemplate1.Name,
						ServiceConfiguration: dpuServiceConfiguration1.Name,
					},
					"service-2": {
						ServiceTemplate:      dpuServiceTemplate2.Name,
						ServiceConfiguration: dpuServiceConfiguration2.Name,
					},
				}
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("retrieving the initial DPUServices and DPUServiceInterfaces")
				gotInitialDPUServices := make(map[string]dpuservicev1.DPUService)
				gotInitialDPUServiceInterfaces := make(map[string][]dpuservicev1.DPUServiceInterface)
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(2))
					for _, dpuService := range dpuServiceList.Items {
						versionAnnotationKey := "svc.dpu.nvidia.com/dpuservice-version"
						g.Expect(dpuService.Annotations).To(HaveKey(versionAnnotationKey))
						gotInitialDPUServices[dpuService.Annotations[versionAnnotationKey]] = dpuService
					}

					dpuServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, dpuServiceInterfaceList)).To(Succeed())
					g.Expect(dpuServiceInterfaceList.Items).To(HaveLen(4))
					for _, dpuServiceInterface := range dpuServiceInterfaceList.Items {
						versionAnnotationKey := "svc.dpu.nvidia.com/dpuservice-version"
						g.Expect(dpuServiceInterface.Annotations).To(HaveKey(versionAnnotationKey))
						gotInitialDPUServiceInterfaces[dpuServiceInterface.Annotations[versionAnnotationKey]] = append(gotInitialDPUServiceInterfaces[dpuServiceInterface.Annotations[versionAnnotationKey]], dpuServiceInterface)
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("waiting for the initial DPUServiceInterfaces to be applied")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(4))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUServiceConfiguration object and checking the outcome")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuServiceConfiguration1), dpuServiceConfiguration1)).To(Succeed())
				dpuServiceConfiguration1.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "newnameif1",
						Network: "nad15",
					},
					{
						Name:    "newnameif2",
						Network: "nad23",
					},
				}
				dpuServiceConfiguration1.SetManagedFields(nil)
				dpuServiceConfiguration1.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
				Expect(testClient.Patch(ctx, dpuServiceConfiguration1, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName))).To(Succeed())

				versionDigest1b := calculateDPUServiceVersionDigest(dpuServiceConfiguration1, dpuServiceTemplate1)
				Expect(versionDigest1b).ToNot(Equal(versionDigest1))

				By("retrieving the updated DPUServices")
				gotUpdatedDPUServices := make(map[string]dpuservicev1.DPUService)
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(3))
					for _, dpuService := range dpuServiceList.Items {
						versionAnnotationKey := "svc.dpu.nvidia.com/dpuservice-version"
						g.Expect(dpuService.Annotations).To(HaveKey(versionAnnotationKey))
						gotUpdatedDPUServices[dpuService.Annotations[versionAnnotationKey]] = dpuService
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("checking that the old DPUServiceInterfaces co-exist with the new ones until the current DPUService becomes ready")
				Eventually(func(g Gomega) {
					instancesThatShouldExistPerVersionDigest := map[string]int{
						versionDigest1:  2,
						versionDigest2:  2,
						versionDigest1b: 2,
					}
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(6))

					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						g.Expect(dpuServiceInterface.Labels).To(HaveLen(1))
						g.Expect(dpuServiceInterface.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuServiceInterface.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))

						versionAnnotationKey := "svc.dpu.nvidia.com/dpuservice-version"
						g.Expect(dpuServiceInterface.Annotations).To(HaveKey(versionAnnotationKey))
						versionAnnotationValue := dpuServiceInterface.Annotations["svc.dpu.nvidia.com/dpuservice-version"]
						g.Expect(instancesThatShouldExistPerVersionDigest).To(HaveKey(versionAnnotationValue))
						instancesThatShouldExistPerVersionDigest[versionAnnotationValue]--
					}
					for k := range instancesThatShouldExistPerVersionDigest {
						g.Expect(instancesThatShouldExistPerVersionDigest[k]).To(BeZero())
					}

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceInterfaceSpec, 0, len(gotDPUServiceInterfaceList.Items))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						specs = append(specs, dpuServiceInterface.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceInterfaceSpec{
						// service 2 - intact
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotInitialDPUServices[versionDigest2].Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_service-2",
												ServiceInterfaceInterfaceNameLabel: "if1",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_service-2",
												Network:       "nad3",
												InterfaceName: "if1",
											},
										},
									},
								},
							},
						},
						// service 2 - intact
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotInitialDPUServices[versionDigest2].Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_service-2",
												ServiceInterfaceInterfaceNameLabel: "if2",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_service-2",
												Network:       "nad4",
												InterfaceName: "if2",
											},
										},
									},
								},
							},
						},
						// service 1 - old
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-1-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotInitialDPUServices[versionDigest1].Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_service-1",
												ServiceInterfaceInterfaceNameLabel: "someinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_service-1",
												Network:       "nad1",
												InterfaceName: "someinterface",
											},
										},
									},
								},
							},
						},
						// service 1 - old
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-1-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotInitialDPUServices[versionDigest1].Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_service-1",
												ServiceInterfaceInterfaceNameLabel: "someotherinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_service-1",
												Network:       "nad2",
												InterfaceName: "someotherinterface",
											},
										},
									},
								},
							},
						},
						// service 1 - new
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-1-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotUpdatedDPUServices[versionDigest1b].Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_service-1",
												ServiceInterfaceInterfaceNameLabel: "newnameif1",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_service-1",
												Network:       "nad15",
												InterfaceName: "newnameif1",
											},
										},
									},
								},
							},
						},
						// service 1 - new
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-1-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotUpdatedDPUServices[versionDigest1b].Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_service-1",
												ServiceInterfaceInterfaceNameLabel: "newnameif2",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_service-1",
												Network:       "nad23",
												InterfaceName: "newnameif2",
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).MustPassRepeatedly(10).Should(Succeed())

				By("Marking the DPUService ready")
				currentDPUService := gotUpdatedDPUServices[versionDigest1b]
				patcher := patch.NewSerialPatcher(&currentDPUService, testClient)
				currentDPUService.Status.Conditions = []metav1.Condition{
					{
						Type:               string(conditions.TypeReady),
						Status:             metav1.ConditionTrue,
						Reason:             string(conditions.ReasonSuccess),
						LastTransitionTime: metav1.NewTime(time.Now()),
					},
				}
				Expect(patcher.Patch(ctx, &currentDPUService, patch.WithFieldOwner("test"))).To(Succeed())

				By("checking that the DPUServiceInterfaces are updated")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(4))

					By("checking the object metadata")
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						g.Expect(dpuServiceInterface.Labels).To(HaveLen(1))
						g.Expect(dpuServiceInterface.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuServiceInterface.Annotations).To(HaveKey("svc.dpu.nvidia.com/dpuservice-version"))
						g.Expect(dpuServiceInterface.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("Checking that original objects associated with service-2 are not deleted")
					for _, originalDPUServiceInterface := range gotInitialDPUServiceInterfaces[versionDigest2] {
						var found bool
						for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
							if originalDPUServiceInterface.UID == dpuServiceInterface.UID {
								found = true
							}
						}
						g.Expect(found).To(BeTrue())
					}

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceInterfaceSpec, 0, len(gotDPUServiceInterfaceList.Items))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						specs = append(specs, dpuServiceInterface.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceInterfaceSpec{
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotInitialDPUServices[versionDigest2].Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_service-2",
												ServiceInterfaceInterfaceNameLabel: "if1",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_service-2",
												Network:       "nad3",
												InterfaceName: "if1",
											},
										},
									},
								},
							},
						},
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotInitialDPUServices[versionDigest2].Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_service-2",
												ServiceInterfaceInterfaceNameLabel: "if2",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_service-2",
												Network:       "nad4",
												InterfaceName: "if2",
											},
										},
									},
								},
							},
						},
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-1-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotUpdatedDPUServices[versionDigest1b].Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_service-1",
												ServiceInterfaceInterfaceNameLabel: "newnameif1",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_service-1",
												Network:       "nad15",
												InterfaceName: "newnameif1",
											},
										},
									},
								},
							},
						},
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-1-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotUpdatedDPUServices[versionDigest1b].Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_service-1",
												ServiceInterfaceInterfaceNameLabel: "newnameif2",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_service-1",
												Network:       "nad23",
												InterfaceName: "newnameif2",
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should delete DPUServiceInterfaces that are no longer part of the DPUServiceConfiguration", func() {
				By("Creating the dependencies")
				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "someinterface",
						Network: "nad1",
					},
					{
						Name:    "someotherinterface",
						Network: "nad2",
					},
				}
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("retrieving the DPUService")
				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					gotDPUService = &dpuServiceList.Items[0]
					g.Expect(gotDPUService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("waiting for the initial DPUServiceInterface to be applied")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUServiceConfiguration object and checking the outcome")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuServiceConfiguration), dpuServiceConfiguration)).To(Succeed())
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "someinterface",
						Network: "nad1",
					},
				}
				dpuServiceConfiguration.SetManagedFields(nil)
				dpuServiceConfiguration.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
				Expect(testClient.Patch(ctx, dpuServiceConfiguration, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName))).To(Succeed())

				By("Marking the DPUService ready")
				patcher := patch.NewSerialPatcher(gotDPUService, testClient)
				gotDPUService.Status.Conditions = []metav1.Condition{
					{
						Type:               string(conditions.TypeReady),
						Status:             metav1.ConditionTrue,
						Reason:             string(conditions.ReasonSuccess),
						LastTransitionTime: metav1.NewTime(time.Now()),
					},
				}
				Expect(patcher.Patch(ctx, gotDPUService, patch.WithFieldOwner("test"))).To(Succeed())

				By("checking that the DPUServiceInterfaces are updated")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(1))

					By("checking the object metadata")
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						g.Expect(dpuServiceInterface.Labels).To(HaveLen(1))
						g.Expect(dpuServiceInterface.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuServiceInterface.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceInterfaceSpec, 0, len(gotDPUServiceInterfaceList.Items))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						specs = append(specs, dpuServiceInterface.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceInterfaceSpec{
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad1",
												InterfaceName: "someinterface",
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should create new DPUServiceInterfaces on update of the DPUServiceConfiguration", func() {
				By("Creating the dependencies")
				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "someinterface",
						Network: "nad1",
					},
				}
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("retrieving the DPUService")
				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					dpuServiceList := getDPUServiceList()
					g.Expect(dpuServiceList.Items).To(HaveLen(1))
					gotDPUService = &dpuServiceList.Items[0]
					g.Expect(gotDPUService).ToNot(BeNil())
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("waiting for the initial DPUServiceInterface to be applied")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(1))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUServiceConfiguration object and checking the outcome")
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuServiceConfiguration), dpuServiceConfiguration)).To(Succeed())
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "someinterface",
						Network: "nad1",
					},
					{
						Name:    "someotherinterface",
						Network: "nad2",
					},
				}
				dpuServiceConfiguration.SetManagedFields(nil)
				dpuServiceConfiguration.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
				Expect(testClient.Patch(ctx, dpuServiceConfiguration, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName))).To(Succeed())

				By("checking that the DPUServiceInterfaces are updated")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						g.Expect(dpuServiceInterface.Labels).To(HaveLen(1))
						g.Expect(dpuServiceInterface.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuServiceInterface.OwnerReferences).To(ContainElement(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceInterfaceSpec, 0, len(gotDPUServiceInterfaceList.Items))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						specs = append(specs, dpuServiceInterface.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceInterfaceSpec{
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad1",
												InterfaceName: "someinterface",
											},
										},
									},
								},
							},
						},
						{
							Template: dpuservicev1.ServiceInterfaceSetSpecTemplate{
								Spec: dpuservicev1.ServiceInterfaceSetSpec{
									NodeSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
									Template: dpuservicev1.ServiceInterfaceSpecTemplate{
										ObjectMeta: dpuservicev1.ObjectMeta{
											Labels: map[string]string{
												dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someservice",
												ServiceInterfaceInterfaceNameLabel: "someotherinterface",
											},
										},
										Spec: dpuservicev1.ServiceInterfaceSpec{
											InterfaceType: dpuservicev1.InterfaceTypeService,
											Service: &dpuservicev1.ServiceDef{
												ServiceID:     "dpudeployment_dpudeployment_someservice",
												Network:       "nad2",
												InterfaceName: "someotherinterface",
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should recreate the DPUServiceInterfaces on manual delete of the DPUServiceInterfaces", func() {
				By("Creating the dependencies")
				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{
					{
						Name:    "someinterface",
						Network: "nad1",
					},
				}
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("waiting for the initial DPUServiceInterface to be applied")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(1))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("manually deleting the DPUServiceInterface")
				Consistently(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					if len(gotDPUServiceInterfaceList.Items) == 0 {
						return
					}
					g.Expect(testutils.CleanupAndWait(ctx, testClient, &gotDPUServiceInterfaceList.Items[0])).To(Succeed())
				}).WithTimeout(5 * time.Second).Should(Succeed())

				By("checking that the DPUServiceInterface are created")
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(1))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
		})
		Context("When checking reconcileDPUServices()", func() {
			BeforeEach(func() {
				By("Creating the dependencies")
				bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

				dpuFlavor := getMinimalDPUFlavor(testNS.Name)
				Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

				DeferCleanup(cleanDPUDeploymentDerivatives, testNS.Name)
			})
			It("should create the correct DPUServices", func() {
				By("Creating the dependencies")
				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				dpuServiceConfiguration.Name = "service-1"
				dpuServiceConfiguration.Spec.DeploymentServiceName = "service-1"
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations = make(map[string]string)
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations["annkey1"] = "annval1"
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels = make(map[string]string)
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels["labelkey1"] = "labelval1"
				dpuServiceConfiguration.Spec.ServiceConfiguration.DeployInCluster = ptr.To(true)
				dpuServiceConfiguration.Spec.ServiceConfiguration.HelmChart.Values = &runtime.RawExtension{Raw: []byte(`{"key1":"value1"}`)}
				dpuServiceConfiguration.Spec.Interfaces = nil
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := getMinimalDPUServiceTemplate(testNS.Name)
				dpuServiceTemplate.Name = "service-1"
				dpuServiceTemplate.Spec.DeploymentServiceName = "service-1"
				dpuServiceTemplate.Spec.HelmChart.Values = &runtime.RawExtension{Raw: []byte(`{"key1":"someothervalue"}`)}
				Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
				patchDPUServiceTemplateWithStatus(dpuServiceTemplate)
				versionDigest1 := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)

				dpuServiceConfiguration = getMinimalDPUServiceConfiguration(testNS.Name)
				dpuServiceConfiguration.Name = "service-2"
				dpuServiceConfiguration.Spec.DeploymentServiceName = "service-2"
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations = make(map[string]string)
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations["annkey2"] = "annval2"
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels = make(map[string]string)
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels["labelkey2"] = "labelval2"
				dpuServiceConfiguration.Spec.ServiceConfiguration.HelmChart.Values = &runtime.RawExtension{Raw: []byte(`{"key2":"value2"}`)}
				dpuServiceConfiguration.Spec.Interfaces = []dpuservicev1.ServiceInterfaceTemplate{{Name: "if2", Network: "nad2"}, {Name: "if3", Network: "nad3"}}
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate = getMinimalDPUServiceTemplate(testNS.Name)
				dpuServiceTemplate.Name = "service-2"
				dpuServiceTemplate.Spec.DeploymentServiceName = "service-2"
				dpuServiceTemplate.Spec.HelmChart.Values = &runtime.RawExtension{Raw: []byte(`{"key3":"value3"}`)}
				Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
				patchDPUServiceTemplateWithStatus(dpuServiceTemplate)
				versionDigest2 := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)

				dpuServiceConfiguration = getDisruptiveDPUServiceConfiguration(testNS.Name)
				dpuServiceConfiguration.Name = "service-3"
				dpuServiceConfiguration.Spec.DeploymentServiceName = "service-3"
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations = make(map[string]string)
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations["annkey3"] = "annval3"
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels = make(map[string]string)
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels["labelkey3"] = "labelval3"
				dpuServiceConfiguration.Spec.Interfaces = nil
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate = getMinimalDPUServiceTemplate(testNS.Name)
				dpuServiceTemplate.Name = "service-3"
				dpuServiceTemplate.Spec.DeploymentServiceName = "service-3"
				Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
				patchDPUServiceTemplateWithStatus(dpuServiceTemplate)
				versionDigest3 := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.Services = make(map[string]dpuservicev1.DPUDeploymentServiceConfiguration)
				dpuDeployment.Spec.Services["service-1"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-1",
					ServiceConfiguration: "service-1",
				}
				dpuDeployment.Spec.Services["service-2"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-2",
					ServiceConfiguration: "service-2",
				}
				dpuDeployment.Spec.Services["service-3"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-3",
					ServiceConfiguration: "service-3",
				}
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				versions := map[string]string{
					"service-1": versionDigest1,
					"service-2": versionDigest2,
					"service-3": versionDigest3}
				By("checking that correct DPUServices are created")
				Eventually(func(g Gomega) {
					By("capturing the created DPUServiceInterfaces")
					gotDPUServiceInterfaceNames := make(map[string][]string)
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						for serviceName := range dpuDeployment.Spec.Services {
							if strings.Contains(dpuServiceInterface.Name, serviceName) {
								gotDPUServiceInterfaceNames[serviceName] = append(gotDPUServiceInterfaceNames[serviceName], dpuServiceInterface.Name)
								slices.Sort(gotDPUServiceInterfaceNames[serviceName])
							}
						}
					}

					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(3))

					By("checking the object metadata")
					for _, dpuService := range gotDPUServiceList.Items {
						g.Expect(dpuService.Labels).To(HaveLen(1))
						g.Expect(dpuService.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuService.Annotations).To(HaveKeyWithValue("svc.dpu.nvidia.com/dpuservice-version", versions[strings.Join(strings.SplitN(dpuService.Name, "-", 3)[0:2], "-")]))
						g.Expect(dpuService.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					names := make(map[string]string)
					specs := make([]dpuservicev1.DPUServiceSpec, 0, 3)
					for _, dpuService := range gotDPUServiceList.Items {
						specs = append(specs, dpuService.Spec)
						names[strings.Join(strings.SplitN(dpuService.Name, "-", 3)[0:2], "-")] = dpuService.Name
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceSpec{
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
								Values: &runtime.RawExtension{Raw: []byte(`{"key1":"value1"}`)},
							},
							ServiceID: ptr.To("dpudeployment_dpudeployment_service-1"),
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								Labels:      map[string]string{"labelkey1": "labelval1"},
								Annotations: map[string]string{"annkey1": "annval1"},
							},
							DeployInCluster: ptr.To(true),
						},
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
								Values: &runtime.RawExtension{Raw: []byte(`{"key2":"value2","key3":"value3"}`)},
							},
							ServiceID: ptr.To("dpudeployment_dpudeployment_service-2"),
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								Labels:      map[string]string{"labelkey2": "labelval2"},
								Annotations: map[string]string{"annkey2": "annval2"},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{names["service-2"]},
												},
												{
													Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name)},
												},
											},
										},
									},
								},
							},
							Interfaces: gotDPUServiceInterfaceNames["service-2"],
						},
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID: ptr.To("dpudeployment_dpudeployment_service-3"),
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								Labels:      map[string]string{"labelkey3": "labelval3"},
								Annotations: map[string]string{"annkey3": "annval3"},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "svc.dpu.nvidia.com/dpuservice-service-3-version",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{names["service-3"]},
												},
												{
													Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name)},
												},
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should patch a manually modified DPUService as long as the modification is not on the version annotation", func() {
				By("Creating the dependencies")
				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := getMinimalDPUServiceTemplate(testNS.Name)
				Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
				patchDPUServiceTemplateWithStatus(dpuServiceTemplate)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("checking that the DPUService is created")
				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(1))
					gotDPUService = &gotDPUServiceList.Items[0]

					By("checking the original value of the chart version")
					g.Expect(gotDPUService.Spec.HelmChart.Source.Version).To(Equal("someversion"))

					By("checking the nodeSelector")
					g.Expect(gotDPUService.Spec.ServiceDaemonSet.NodeSelector).To(BeComparableTo(&corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{gotDPUService.Name},
									},
									{
										Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name)},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUService manually")
				gotDPUService.Spec.HelmChart.Source.Version = "someotherversion"
				gotDPUService.SetManagedFields(nil)
				gotDPUService.SetGroupVersionKind(dpuservicev1.DPUServiceGroupVersionKind)
				Expect(testClient.Patch(ctx, gotDPUService, client.Apply, client.ForceOwnership, client.FieldOwner("some-test-controller"))).To(Succeed())

				By("checking that the DPUService is reverted to the original")
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(1))
					g.Expect(gotDPUServiceList.Items[0].UID).To(Equal(gotDPUService.UID))

					By("checking the chart version")
					g.Expect(gotDPUServiceList.Items[0].Spec.HelmChart.Source.Version).To(Equal("someversion"))

					By("checking the nodeSelector")
					g.Expect(gotDPUServiceList.Items[0].Spec.ServiceDaemonSet.NodeSelector).To(BeComparableTo(&corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "svc.dpu.nvidia.com/dpuservice-someservice-version",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{gotDPUService.Name},
									},
									{
										Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name)},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should update the existing DPUService on update of non-disruptive DPUServiceConfiguration", func() {
				By("Creating the dependencies")
				versionDigest1, _ := createReconcileDPUServicesNonDisruptiveDependencies(testNS.Name)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.Services = make(map[string]dpuservicev1.DPUDeploymentServiceConfiguration)
				dpuDeployment.Spec.Services["service-1"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-1",
					ServiceConfiguration: "service-1",
				}
				dpuDeployment.Spec.Services["service-2"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-2",
					ServiceConfiguration: "service-2",
				}
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("waiting for the initial DPUServices to be applied")
				firstDPUServiceUIDs := make([]types.UID, 0, 2)
				gotFirstDPUServiceNames := make(map[string]string)
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))
					for _, dpuService := range gotDPUServiceList.Items {
						firstDPUServiceUIDs = append(firstDPUServiceUIDs, dpuService.UID)
						for serviceName := range dpuDeployment.Spec.Services {
							if strings.Contains(dpuService.Name, serviceName) {
								gotFirstDPUServiceNames[serviceName] = dpuService.Name
							}
						}
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the first DPUServiceConfiguration object")
				dpuServiceConfiguration := &dpuservicev1.DPUServiceConfiguration{}
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNS.Name, Name: "service-1"}, dpuServiceConfiguration)).To(Succeed())
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels = map[string]string{"newlabel": "newvalue"}
				dpuServiceConfiguration.SetManagedFields(nil)
				dpuServiceConfiguration.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
				Expect(testClient.Patch(ctx, dpuServiceConfiguration, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName))).To(Succeed())

				dpuServiceTemplate := &dpuservicev1.DPUServiceTemplate{}
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNS.Name, Name: "service-1"}, dpuServiceTemplate)).To(Succeed())
				versionDigest1b := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)
				Expect(versionDigest1b).ToNot(Equal(versionDigest1))

				By("modifying the second DPUServiceConfiguration object")
				dpuServiceConfiguration = &dpuservicev1.DPUServiceConfiguration{}
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNS.Name, Name: "service-2"}, dpuServiceConfiguration)).To(Succeed())
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels = map[string]string{"newlabel2": "newvalue2"}
				dpuServiceConfiguration.SetManagedFields(nil)
				dpuServiceConfiguration.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
				Expect(testClient.Update(ctx, dpuServiceConfiguration)).To(Succeed())

				dpuServiceTemplate = &dpuservicev1.DPUServiceTemplate{}
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNS.Name, Name: "service-2"}, dpuServiceTemplate)).To(Succeed())
				versionDigest2 := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)

				By("checking that the DPUServices are updated as expected")
				Eventually(func(g Gomega) {
					By("capturing the created DPUServiceInterfaces")
					gotDPUServiceInterfaceNames := make(map[string][]string)
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						for serviceName := range dpuDeployment.Spec.Services {
							if strings.Contains(dpuServiceInterface.Name, serviceName) {
								gotDPUServiceInterfaceNames[serviceName] = append(gotDPUServiceInterfaceNames[serviceName], dpuServiceInterface.Name)
								slices.Sort(gotDPUServiceInterfaceNames[serviceName])
							}
						}
					}

					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))

					versions := map[string]string{
						"service-1": versionDigest1b,
						"service-2": versionDigest2}
					By("checking the object metadata")
					for _, dpuService := range gotDPUServiceList.Items {
						g.Expect(dpuService.Labels).To(HaveLen(1))
						g.Expect(dpuService.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuService.Annotations).To(HaveKeyWithValue("svc.dpu.nvidia.com/dpuservice-version", versions[strings.Join(strings.SplitN(dpuService.Name, "-", 3)[0:2], "-")]))
						// Validate that the object was not recreated
						g.Expect(firstDPUServiceUIDs).To(ContainElement(dpuService.UID))

						g.Expect(dpuService.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceSpec, 0, len(gotDPUServiceList.Items))
					for _, dpuService := range gotDPUServiceList.Items {
						specs = append(specs, dpuService.Spec)
					}
					g.Expect(specs).To(BeComparableTo([]dpuservicev1.DPUServiceSpec{
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID:  ptr.To("dpudeployment_dpudeployment_service-1"),
							Interfaces: gotDPUServiceInterfaceNames["service-1"],
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								Labels: map[string]string{"newlabel": "newvalue"},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "svc.dpu.nvidia.com/dpuservice-service-1-version",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{gotFirstDPUServiceNames["service-1"]},
												},
												{
													Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name)},
												},
											},
										},
									},
								},
							},
						},
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID:  ptr.To("dpudeployment_dpudeployment_service-2"),
							Interfaces: gotDPUServiceInterfaceNames["service-2"],
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								Labels: map[string]string{"newlabel2": "newvalue2"},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{gotFirstDPUServiceNames["service-2"]},
												},
												{
													Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name)},
												},
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should update the existing disruptive DPUService to non disruptive", func() {
				By("Creating the dependencies")
				versionDigest1, versionDigest2 := createReconcileDPUServicesDisruptiveDependencies(testNS.Name)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.Services = make(map[string]dpuservicev1.DPUDeploymentServiceConfiguration)
				dpuDeployment.Spec.Services["service-1"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-1",
					ServiceConfiguration: "service-1",
				}
				dpuDeployment.Spec.Services["service-2"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-2",
					ServiceConfiguration: "service-2",
				}
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("waiting for the initial DPUServices to be applied")
				firstDPUServiceUIDs := make(map[types.UID]interface{})
				names := map[string]string{}
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))
					for _, dpuService := range gotDPUServiceList.Items {
						firstDPUServiceUIDs[dpuService.UID] = struct{}{}
						names[strings.Join(strings.SplitN(dpuService.Name, "-", 3)[0:2], "-")] = dpuService.Name
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUServiceConfiguration object")
				dpuServiceConfiguration := &dpuservicev1.DPUServiceConfiguration{}
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNS.Name, Name: "service-2"}, dpuServiceConfiguration)).To(Succeed())
				origConfig := dpuServiceConfiguration.DeepCopy()
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels = map[string]string{"newlabel": "newvalue"}
				// make non-disruptive
				dpuServiceConfiguration.Spec.NodeEffect = nil
				dpuServiceConfiguration.SetManagedFields(nil)
				dpuServiceConfiguration.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
				Expect(testClient.Patch(ctx, dpuServiceConfiguration, client.MergeFrom(origConfig))).To(Succeed())
				versionDigest2b := calculateVersionDigest("service-2", testNS.Name)
				Expect(versionDigest2b).ToNot(Equal(versionDigest2))

				By("checking that the DPUService is updated as expected")
				Eventually(func(g Gomega) {
					By("capturing the created DPUServiceInterfaces")
					gotDPUServiceInterfaceNames := make(map[string][]string)
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						for serviceName := range dpuDeployment.Spec.Services {
							if strings.Contains(dpuServiceInterface.Name, serviceName) {
								gotDPUServiceInterfaceNames[serviceName] = append(gotDPUServiceInterfaceNames[serviceName], dpuServiceInterface.Name)
								slices.Sort(gotDPUServiceInterfaceNames[serviceName])
							}
						}
					}
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))

					By("Checking that one of the services has the correct new version annotation")
					var found bool
					for _, dpuService := range gotDPUServiceList.Items {
						if dpuService.GetAnnotations()[dpuServiceVersionAnnotationKey] == versionDigest2b {
							found = true
						}
					}
					g.Expect(found).To(BeTrue())

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceSpec, 0, len(gotDPUServiceList.Items))
					for _, dpuService := range gotDPUServiceList.Items {
						specs = append(specs, dpuService.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceSpec{
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID: ptr.To("dpudeployment_dpudeployment_service-1"),
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "svc.dpu.nvidia.com/dpuservice-service-1-version",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{names["service-1"]},
												},
												{
													Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
												},
											},
										},
									},
								},
							},
							Interfaces: gotDPUServiceInterfaceNames["service-1"],
						},
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID: ptr.To("dpudeployment_dpudeployment_service-2"),
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								Labels: map[string]string{"newlabel": "newvalue"},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{names["service-2"]},
												},
												{
													Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
												},
											},
										},
									},
								},
							},
							Interfaces: gotDPUServiceInterfaceNames["service-2"],
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("making the new version ready")
				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				Expect(gotDPUServiceList.Items).To(HaveLen(2))

				var currentSvc *dpuservicev1.DPUService
				for _, dpuService := range gotDPUServiceList.Items {
					if dpuService.GetAnnotations()[dpuServiceVersionAnnotationKey] == versionDigest2b {
						currentSvc = &dpuService
					}
				}
				Expect(currentSvc).NotTo(BeNil())
				patcher := patch.NewSerialPatcher(currentSvc, testClient)
				currentSvc.Status.Conditions = []metav1.Condition{
					{
						Type:               string(conditions.TypeReady),
						Status:             metav1.ConditionTrue,
						Reason:             string(conditions.ReasonSuccess),
						LastTransitionTime: metav1.NewTime(time.Now()),
					},
				}
				Expect(patcher.Patch(ctx, currentSvc, patch.WithFieldOwner("test"))).To(Succeed())

				By("checking that the DPUService is updated as expected")
				Eventually(func(g Gomega) {
					By("capturing the created DPUServiceInterfaces")
					gotDPUServiceInterfaceNames := make(map[string][]string)
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						for serviceName := range dpuDeployment.Spec.Services {
							if strings.Contains(dpuServiceInterface.Name, serviceName) {
								gotDPUServiceInterfaceNames[serviceName] = append(gotDPUServiceInterfaceNames[serviceName], dpuServiceInterface.Name)
								slices.Sort(gotDPUServiceInterfaceNames[serviceName])
							}
						}
					}
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))

					serviceUIDs := firstDPUServiceUIDs
					versions := map[string]string{
						"service-1": versionDigest1,
						"service-2": versionDigest2b}
					By("checking the object metadata")
					for _, dpuService := range gotDPUServiceList.Items {
						g.Expect(dpuService.Labels).To(HaveLen(1))
						g.Expect(dpuService.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuService.Annotations).To(HaveKeyWithValue("svc.dpu.nvidia.com/dpuservice-version", versions[strings.Join(strings.SplitN(dpuService.Name, "-", 3)[0:2], "-")]))
						delete(serviceUIDs, dpuService.UID)

						g.Expect(dpuService.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					// Validate that here is no new object created
					g.Expect(serviceUIDs).To(BeEmpty())

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceSpec, 0, len(gotDPUServiceList.Items))
					for _, dpuService := range gotDPUServiceList.Items {
						specs = append(specs, dpuService.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceSpec{
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID: ptr.To("dpudeployment_dpudeployment_service-1"),
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "svc.dpu.nvidia.com/dpuservice-service-1-version",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{names["service-1"]},
												},
												{
													Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
												},
											},
										},
									},
								},
							},
							Interfaces: gotDPUServiceInterfaceNames["service-1"],
						},
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID: ptr.To("dpudeployment_dpudeployment_service-2"),
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								Labels: map[string]string{"newlabel": "newvalue"},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{names["service-2"]},
												},
												{
													Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
												},
											},
										},
									},
								},
							},
							Interfaces: gotDPUServiceInterfaceNames["service-2"],
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			// TODO: Split the test for disruptive upgrade and revision history check
			It("should create new DPUService on update of disruptive DPUServiceConfiguration and respect revision history", func() {
				revisionHistoryLimit := 5
				By("Creating the dependencies")
				versionDigest1, _ := createReconcileDPUServicesDisruptiveDependencies(testNS.Name)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.RevisionHistoryLimit = ptr.To(int32(revisionHistoryLimit))
				dpuDeployment.Spec.Services = make(map[string]dpuservicev1.DPUDeploymentServiceConfiguration)
				dpuDeployment.Spec.Services["service-1"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-1",
					ServiceConfiguration: "service-1",
				}
				dpuDeployment.Spec.Services["service-2"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-2",
					ServiceConfiguration: "service-2",
				}
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("waiting for the initial DPUServices to be applied")
				firstDPUServiceUIDs := make(map[types.UID]interface{})
				names := make(map[string]string)
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))
					for _, dpuService := range gotDPUServiceList.Items {
						firstDPUServiceUIDs[dpuService.UID] = struct{}{}
						names[strings.Join(strings.SplitN(dpuService.Name, "-", 3)[0:2], "-")] = dpuService.Name
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("capturing the created DPUServiceInterfaces")
				gotDPUServiceInterfaceNames := make(map[string][]string)
				Eventually(func(g Gomega) {
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						for serviceName := range dpuDeployment.Spec.Services {
							if strings.Contains(dpuServiceInterface.Name, serviceName) {
								gotDPUServiceInterfaceNames[serviceName] = append(gotDPUServiceInterfaceNames[serviceName], dpuServiceInterface.Name)
								slices.Sort(gotDPUServiceInterfaceNames[serviceName])
							}
						}
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUServiceConfiguration object")
				expected := []dpuservicev1.DPUServiceSpec{
					{
						HelmChart: dpuservicev1.HelmChart{
							Source: dpuservicev1.ApplicationSource{
								RepoURL: "oci://someurl/repo",
								Path:    "somepath",
								Version: "someversion",
								Chart:   "somechart",
							},
						},
						ServiceID: ptr.To("dpudeployment_dpudeployment_service-1"),
						ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
							NodeSelector: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-1-version",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{names["service-1"]},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
								},
							},
						},
						Interfaces: gotDPUServiceInterfaceNames["service-1"],
					},
					{
						HelmChart: dpuservicev1.HelmChart{
							Source: dpuservicev1.ApplicationSource{
								RepoURL: "oci://someurl/repo",
								Path:    "somepath",
								Version: "someversion",
								Chart:   "somechart",
							},
						},
						ServiceID: ptr.To("dpudeployment_dpudeployment_service-2"),
						ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
							NodeSelector: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{names["service-2"]},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
								},
							},
						},
						Interfaces: gotDPUServiceInterfaceNames["service-2"],
						// paused while waiting for the new version to be ready
						Paused: ptr.To(true),
					},
				}

				// This is a hack because object creationTimeStamp use rfc3339 format
				// which has second precision, so we need to wait for 1 second to make sure
				// the creationTimeStamp is different for the first new version at least
				time.Sleep(1 * time.Second)
				var (
					versionDigest  string
					dpuServiceName string
				)
				for i := 0; i < revisionHistoryLimit; i++ {
					dpuServiceConfiguration := &dpuservicev1.DPUServiceConfiguration{}
					Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNS.Name, Name: "service-2"}, dpuServiceConfiguration)).To(Succeed())
					initialConfig := dpuServiceConfiguration.DeepCopy()
					dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels = map[string]string{fmt.Sprintf("somelabel%d", i): "val"}
					dpuServiceConfiguration.SetManagedFields(nil)
					dpuServiceConfiguration.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
					Expect(testClient.Patch(ctx, dpuServiceConfiguration, client.MergeFrom(initialConfig))).To(Succeed())
					newVersionDigest := calculateVersionDigest("service-2", testNS.Name)
					Expect(newVersionDigest).ToNot(Equal(versionDigest))
					versionDigest = newVersionDigest
					var newDPUServiceName string
					var newDPUServiceInterface string

					Eventually(func(g Gomega) {
						gotDPUServiceList := &dpuservicev1.DPUServiceList{}
						g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
						for _, dpuService := range gotDPUServiceList.Items {
							if dpuService.GetAnnotations()[dpuServiceVersionAnnotationKey] == versionDigest {
								newDPUServiceName = dpuService.Name
							}
						}
						g.Expect(newDPUServiceName).ToNot(BeEmpty())
						gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
						g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
						for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
							if dpuServiceInterface.GetAnnotations()[dpuServiceVersionAnnotationKey] == versionDigest {
								newDPUServiceInterface = dpuServiceInterface.Name
							}
						}
						g.Expect(newDPUServiceInterface).ToNot(BeEmpty())
					}).WithTimeout(30 * time.Second).Should(Succeed())

					Expect(newDPUServiceName).ToNot(Equal(dpuServiceName))
					dpuServiceName = newDPUServiceName
					svc := dpuservicev1.DPUServiceSpec{
						HelmChart: dpuservicev1.HelmChart{
							Source: dpuservicev1.ApplicationSource{
								RepoURL: "oci://someurl/repo",
								Path:    "somepath",
								Version: "someversion",
								Chart:   "somechart",
							},
						},
						ServiceID: ptr.To("dpudeployment_dpudeployment_service-2"),
						ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
							Labels: map[string]string{fmt.Sprintf("somelabel%d", i): "val"},
							NodeSelector: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{dpuServiceName},
											},
											{
												Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
											},
										},
									},
								},
							},
						},
						Interfaces: []string{newDPUServiceInterface},
					}

					// remove the oldest version from the expected list
					// if the list has reached the revisionHistoryLimit
					if len(expected) == revisionHistoryLimit+1 {
						expected = append(expected[:1], expected[2:]...)
					}

					expected = append(expected, svc)

					for i := 1; i < len(expected)-1; i++ {
						// paused while waiting for the new version to be ready
						expected[i].Paused = ptr.To(true)
					}

					By("checking that the DPUService is updated as expected and that we have as many DPUServiceInterfaces")
					Eventually(func(g Gomega) {
						gotDPUServiceList := &dpuservicev1.DPUServiceList{}
						g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
						g.Expect(gotDPUServiceList.Items).To(HaveLen(len(expected)))

						By("Checking that the DPUServiceInterfaces that have exceeded the revision history have been deleted")
						gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
						g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
						g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(len(expected)))

						By("checking the specs")
						specs := make([]dpuservicev1.DPUServiceSpec, 0, len(gotDPUServiceList.Items))
						for _, dpuService := range gotDPUServiceList.Items {
							specs = append(specs, dpuService.Spec)
						}
						g.Expect(specs).To(ConsistOf(expected))
					}).WithTimeout(30 * time.Second).Should(Succeed())
				}

				By("making the new version ready")
				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				Expect(gotDPUServiceList.Items).To(HaveLen(len(expected)))

				var currentSvc *dpuservicev1.DPUService
				for _, dpuService := range gotDPUServiceList.Items {
					if dpuService.GetAnnotations()[dpuServiceVersionAnnotationKey] == versionDigest {
						currentSvc = &dpuService
					}
				}
				Expect(currentSvc).NotTo(BeNil())
				patcher := patch.NewSerialPatcher(currentSvc, testClient)
				currentSvc.Status.Conditions = []metav1.Condition{
					{
						Type:               string(conditions.TypeReady),
						Status:             metav1.ConditionTrue,
						Reason:             string(conditions.ReasonSuccess),
						LastTransitionTime: metav1.NewTime(time.Now()),
					},
				}
				Expect(patcher.Patch(ctx, currentSvc, patch.WithFieldOwner("test"))).To(Succeed())

				By("checking that the DPUService is updated as expected")
				Eventually(func(g Gomega) {
					By("capturing the created DPUServiceInterfaces")
					gotDPUServiceInterfaceNames := make(map[string][]string)
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						for serviceName := range dpuDeployment.Spec.Services {
							if strings.Contains(dpuServiceInterface.Name, serviceName) {
								gotDPUServiceInterfaceNames[serviceName] = append(gotDPUServiceInterfaceNames[serviceName], dpuServiceInterface.Name)
								slices.Sort(gotDPUServiceInterfaceNames[serviceName])
							}
						}
					}

					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))

					serviceUIDs := firstDPUServiceUIDs
					versions := map[string]string{
						"service-1": versionDigest1,
						"service-2": versionDigest}
					By("checking the object metadata")
					for _, dpuService := range gotDPUServiceList.Items {
						g.Expect(dpuService.Labels).To(HaveLen(1))
						g.Expect(dpuService.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
						g.Expect(dpuService.Annotations).To(HaveKeyWithValue("svc.dpu.nvidia.com/dpuservice-version", versions[strings.Join(strings.SplitN(dpuService.Name, "-", 3)[0:2], "-")]))
						delete(serviceUIDs, dpuService.UID)

						g.Expect(dpuService.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					// Validate that all original objects are there and not recreated
					g.Expect(serviceUIDs).To(HaveLen(1))

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceSpec, 0, len(gotDPUServiceList.Items))
					for _, dpuService := range gotDPUServiceList.Items {
						specs = append(specs, dpuService.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceSpec{
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID: ptr.To("dpudeployment_dpudeployment_service-1"),
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "svc.dpu.nvidia.com/dpuservice-service-1-version",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{names["service-1"]},
												},
												{
													Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
												},
											},
										},
									},
								},
							},
							Interfaces: gotDPUServiceInterfaceNames["service-1"],
						},
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID: ptr.To("dpudeployment_dpudeployment_service-2"),
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								Labels: map[string]string{"somelabel4": "val"},
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{currentSvc.Name},
												},
												{
													Key:      dpuservicev1.ParentDPUDeploymentNameLabel,
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{fmt.Sprintf("%s_%s", testNS.Name, dpuDeployment.Name)},
												},
											},
										},
									},
								},
							},
							Interfaces: gotDPUServiceInterfaceNames["service-2"],
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should delete DPUServices that are no longer part of the DPUDeployment", func() {
				By("Creating the dependencies")
				_, versionDigest2 := createReconcileDPUServicesNonDisruptiveDependencies(testNS.Name)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.Services = make(map[string]dpuservicev1.DPUDeploymentServiceConfiguration)
				dpuDeployment.Spec.Services["service-1"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-1",
					ServiceConfiguration: "service-1",
				}
				dpuDeployment.Spec.Services["service-2"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-2",
					ServiceConfiguration: "service-2",
				}
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)

				By("waiting for the initial DPUServices to be applied")
				var gotDPUService *dpuservicev1.DPUService
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))
					for _, dpuService := range gotDPUServiceList.Items {
						if dpuService.GetAnnotations()[dpuServiceVersionAnnotationKey] == versionDigest2 {
							gotDPUService = &dpuService
						}
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUDeployment object and checking the outcome")
				delete(dpuDeployment.Spec.Services, "service-1")
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())

				By("Marking the DPUService ready")
				patcher = patch.NewSerialPatcher(gotDPUService, testClient)
				gotDPUService.Status.Conditions = []metav1.Condition{
					{
						Type:               string(conditions.TypeReady),
						Status:             metav1.ConditionTrue,
						Reason:             string(conditions.ReasonSuccess),
						LastTransitionTime: metav1.NewTime(time.Now()),
					},
				}
				Expect(patcher.Patch(ctx, gotDPUService, patch.WithFieldOwner("test"))).To(Succeed())

				By("checking that only one DPUService exists")
				Eventually(func(g Gomega) {
					By("capturing the created DPUServiceInterfaces")
					gotDPUServiceInterfaceNames := make(map[string][]string)
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(1))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						for serviceName := range dpuDeployment.Spec.Services {
							if strings.Contains(dpuServiceInterface.Name, serviceName) {
								gotDPUServiceInterfaceNames[serviceName] = append(gotDPUServiceInterfaceNames[serviceName], dpuServiceInterface.Name)
								slices.Sort(gotDPUServiceInterfaceNames[serviceName])
							}
						}
					}

					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(1))

					By("checking the spec")
					g.Expect(gotDPUServiceList.Items[0].Spec).To(BeComparableTo(dpuservicev1.DPUServiceSpec{
						HelmChart: dpuservicev1.HelmChart{
							Source: dpuservicev1.ApplicationSource{
								RepoURL: "oci://someurl/repo",
								Path:    "somepath",
								Version: "someversion",
								Chart:   "somechart",
							},
						},
						ServiceID:  ptr.To("dpudeployment_dpudeployment_service-2"),
						Interfaces: gotDPUServiceInterfaceNames["service-2"],
						ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
							NodeSelector: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{gotDPUService.Name},
											},
											{
												Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name)},
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should create new DPUServices on update of the .spec.services in the DPUDeployment", func() {
				By("Creating the dependencies")
				createReconcileDPUServicesNonDisruptiveDependencies(testNS.Name)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.Services = make(map[string]dpuservicev1.DPUDeploymentServiceConfiguration)
				dpuDeployment.Spec.Services["service-1"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-1",
					ServiceConfiguration: "service-1",
				}
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)

				By("waiting for the initial DPUService to be applied")
				var gotInitialDPUServiceName string
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(1))
					gotInitialDPUServiceName = gotDPUServiceList.Items[0].Name
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUDeployment object and checking the outcome")
				dpuDeployment.Spec.Services["service-2"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-2",
					ServiceConfiguration: "service-2",
				}
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())

				By("checking that two DPUServices exist")
				var gotDPUServiceName string
				Eventually(func(g Gomega) {
					By("capturing the created DPUServiceInterfaces")
					gotDPUServiceInterfaceNames := make(map[string][]string)
					gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
					g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(2))
					for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
						for serviceName := range dpuDeployment.Spec.Services {
							if strings.Contains(dpuServiceInterface.Name, serviceName) {
								gotDPUServiceInterfaceNames[serviceName] = append(gotDPUServiceInterfaceNames[serviceName], dpuServiceInterface.Name)
								slices.Sort(gotDPUServiceInterfaceNames[serviceName])
							}
						}
					}

					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))

					for _, dpuService := range gotDPUServiceList.Items {
						if dpuService.Name != gotInitialDPUServiceName {
							gotDPUServiceName = dpuService.Name
							break
						}
					}

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceSpec, 0, len(gotDPUServiceList.Items))
					for _, dpuService := range gotDPUServiceList.Items {
						specs = append(specs, dpuService.Spec)
					}

					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceSpec{
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID:  ptr.To("dpudeployment_dpudeployment_service-1"),
							Interfaces: gotDPUServiceInterfaceNames["service-1"],
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "svc.dpu.nvidia.com/dpuservice-service-1-version",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{gotInitialDPUServiceName},
												},
												{
													Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name)},
												},
											},
										},
									},
								},
							},
						},
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "oci://someurl/repo",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID:  ptr.To("dpudeployment_dpudeployment_service-2"),
							Interfaces: gotDPUServiceInterfaceNames["service-2"],
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								NodeSelector: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "svc.dpu.nvidia.com/dpuservice-service-2-version",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{gotDPUServiceName},
												},
												{
													Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
													Operator: corev1.NodeSelectorOpIn,
													Values:   []string{fmt.Sprintf("%s_%s", dpuDeployment.Namespace, dpuDeployment.Name)},
												},
											},
										},
									},
								},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should create new DPUServices on manual deletion of the DPUServices", func() {
				By("Creating the dependencies")
				createReconcileDPUServicesNonDisruptiveDependencies(testNS.Name)

				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.Services = make(map[string]dpuservicev1.DPUDeploymentServiceConfiguration)
				dpuDeployment.Spec.Services["service-1"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-1",
					ServiceConfiguration: "service-1",
				}
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("waiting for the initial DPUService to be applied")
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(1))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("manually deleting the DPUServices")
				Consistently(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					if len(gotDPUServiceList.Items) == 0 {
						return
					}
					g.Expect(testutils.CleanupAndWait(ctx, testClient, &gotDPUServiceList.Items[0])).To(Succeed())
				}).WithTimeout(5 * time.Second).Should(Succeed())

				By("checking that the DPUServices are created")
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(1))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
		})
		Context("When checking verifyResourceFitting()", func() {
			DescribeTable("behaves as expected", func(deps *dpuDeploymentDependencies, expectError bool) {
				err := verifyResourceFitting(deps)
				if expectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
				Entry("DPUFlavor doesn't specify dpuResources", &dpuDeploymentDependencies{
					DPUFlavor: &provisioningv1.DPUFlavor{
						Spec: provisioningv1.DPUFlavorSpec{},
					},
					DPUServiceTemplates: map[string]*dpuservicev1.DPUServiceTemplate{
						"service-1": {
							Spec: dpuservicev1.DPUServiceTemplateSpec{
								ResourceRequirements: corev1.ResourceList{
									"cpu":    resource.MustParse("1"),
									"memory": resource.MustParse("3Gi"),
								},
							},
						},
						"service-2": {
							Spec: dpuservicev1.DPUServiceTemplateSpec{
								ResourceRequirements: corev1.ResourceList{
									"cpu":    resource.MustParse("1"),
									"memory": resource.MustParse("1Gi"),
								},
							},
						},
					},
				}, false),
				Entry("DPUFlavor specifies dpuResources that fit but not systemReservedResources", &dpuDeploymentDependencies{
					DPUFlavor: &provisioningv1.DPUFlavor{
						Spec: provisioningv1.DPUFlavorSpec{
							DPUResources: corev1.ResourceList{
								"cpu":    resource.MustParse("2"),
								"memory": resource.MustParse("4Gi"),
							},
						},
					},
					DPUServiceTemplates: map[string]*dpuservicev1.DPUServiceTemplate{
						"service-1": {
							Spec: dpuservicev1.DPUServiceTemplateSpec{
								ResourceRequirements: corev1.ResourceList{
									"cpu":    resource.MustParse("1"),
									"memory": resource.MustParse("3Gi"),
								},
							},
						},
						"service-2": {
							Spec: dpuservicev1.DPUServiceTemplateSpec{
								ResourceRequirements: corev1.ResourceList{
									"cpu":    resource.MustParse("1"),
									"memory": resource.MustParse("1Gi"),
								},
							},
						},
					},
				}, false),
				Entry("requested resources fit leaving buffer", &dpuDeploymentDependencies{
					DPUFlavor: &provisioningv1.DPUFlavor{
						Spec: provisioningv1.DPUFlavorSpec{
							DPUResources: corev1.ResourceList{
								"cpu":    resource.MustParse("2"),
								"memory": resource.MustParse("4Gi"),
							},
							SystemReservedResources: corev1.ResourceList{
								"cpu":    resource.MustParse("1"),
								"memory": resource.MustParse("2Gi"),
							},
						},
					},
					DPUServiceTemplates: map[string]*dpuservicev1.DPUServiceTemplate{
						"service-1": {
							Spec: dpuservicev1.DPUServiceTemplateSpec{
								ResourceRequirements: corev1.ResourceList{
									"cpu":    resource.MustParse("0.5"),
									"memory": resource.MustParse("100Mi"),
								},
							},
						},
						"service-2": {
							Spec: dpuservicev1.DPUServiceTemplateSpec{
								ResourceRequirements: corev1.ResourceList{
									"cpu":    resource.MustParse("0.2"),
									"memory": resource.MustParse("1Gi"),
								},
							},
						},
					},
				}, false),
				Entry("requested resources fit exactly", &dpuDeploymentDependencies{
					DPUFlavor: &provisioningv1.DPUFlavor{
						Spec: provisioningv1.DPUFlavorSpec{
							DPUResources: corev1.ResourceList{
								"cpu":    resource.MustParse("3"),
								"memory": resource.MustParse("3Gi"),
							},
							SystemReservedResources: corev1.ResourceList{
								"cpu":    resource.MustParse("1"),
								"memory": resource.MustParse("1Gi"),
							},
						},
					},
					DPUServiceTemplates: map[string]*dpuservicev1.DPUServiceTemplate{
						"service-1": {
							Spec: dpuservicev1.DPUServiceTemplateSpec{
								ResourceRequirements: corev1.ResourceList{
									"cpu":    resource.MustParse("1"),
									"memory": resource.MustParse("1Gi"),
								},
							},
						},
						"service-2": {
							Spec: dpuservicev1.DPUServiceTemplateSpec{
								ResourceRequirements: corev1.ResourceList{
									"cpu":    resource.MustParse("1"),
									"memory": resource.MustParse("1Gi"),
								},
							},
						},
					},
				}, false),
				Entry("requested resources don't fit", &dpuDeploymentDependencies{
					DPUFlavor: &provisioningv1.DPUFlavor{
						Spec: provisioningv1.DPUFlavorSpec{
							DPUResources: corev1.ResourceList{
								"cpu":    resource.MustParse("1"),
								"memory": resource.MustParse("2Gi"),
							},
							SystemReservedResources: corev1.ResourceList{
								"cpu":    resource.MustParse("0.5"),
								"memory": resource.MustParse("1Gi"),
							},
						},
					},
					DPUServiceTemplates: map[string]*dpuservicev1.DPUServiceTemplate{
						"service-1": {
							Spec: dpuservicev1.DPUServiceTemplateSpec{
								ResourceRequirements: corev1.ResourceList{
									"cpu":    resource.MustParse("1"),
									"memory": resource.MustParse("3Gi"),
								},
							},
						},
						"service-2": {
							Spec: dpuservicev1.DPUServiceTemplateSpec{
								ResourceRequirements: corev1.ResourceList{
									"cpu":    resource.MustParse("1"),
									"memory": resource.MustParse("1Gi"),
								},
							},
						},
					},
				}, true),
				Entry("requested resource doesn't exist", &dpuDeploymentDependencies{
					DPUFlavor: &provisioningv1.DPUFlavor{
						Spec: provisioningv1.DPUFlavorSpec{
							DPUResources: corev1.ResourceList{
								"cpu": resource.MustParse("1"),
							},
							SystemReservedResources: corev1.ResourceList{
								"cpu": resource.MustParse("0.5"),
							},
						},
					},
					DPUServiceTemplates: map[string]*dpuservicev1.DPUServiceTemplate{
						"service-1": {
							Spec: dpuservicev1.DPUServiceTemplateSpec{
								ResourceRequirements: corev1.ResourceList{
									"cpu":    resource.MustParse("1"),
									"memory": resource.MustParse("3Gi"),
								},
							},
						},
						"service-2": {
							Spec: dpuservicev1.DPUServiceTemplateSpec{
								ResourceRequirements: corev1.ResourceList{
									"cpu":    resource.MustParse("1"),
									"memory": resource.MustParse("1Gi"),
								},
							},
						},
					},
				}, true),
			)
		})

		Context("When checking verifyVersionMatching()", func() {
			DescribeTable("behaves as expected", func(deps *dpuDeploymentDependencies, expectError bool) {
				err := verifyVersionMatching(deps)
				if expectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).ToNot(HaveOccurred())
				}
			},
				Entry("DPUServiceTemplates have no version constraint", &dpuDeploymentDependencies{
					BFB: &provisioningv1.BFB{
						Status: provisioningv1.BFBStatus{
							Versions: provisioningv1.BFBVersions{
								DOCA: "2.9.1",
							},
						},
					},

					DPUServiceTemplates: map[string]*dpuservicev1.DPUServiceTemplate{
						"service-1": {
							Status: dpuservicev1.DPUServiceTemplateStatus{},
						},
						"service-2": {
							Status: dpuservicev1.DPUServiceTemplateStatus{},
						},
					},
				}, false),
				Entry("DPUServiceTemplates have valid version constraints", &dpuDeploymentDependencies{
					BFB: &provisioningv1.BFB{
						Status: provisioningv1.BFBStatus{
							Versions: provisioningv1.BFBVersions{
								DOCA: "2.9.1",
							},
						},
					},

					DPUServiceTemplates: map[string]*dpuservicev1.DPUServiceTemplate{
						"service-1": {
							Status: dpuservicev1.DPUServiceTemplateStatus{
								Versions: map[string]string{
									"dpu.nvidia.com/doca-version": ">=2.8",
								},
							},
						},
						"service-2": {
							Status: dpuservicev1.DPUServiceTemplateStatus{
								Versions: map[string]string{
									"dpu.nvidia.com/doca-version": ">=2.5",
								},
							},
						},
					},
				}, false),
				Entry("DPUServiceTemplate has version constraints that are not satisfied", &dpuDeploymentDependencies{
					BFB: &provisioningv1.BFB{
						Status: provisioningv1.BFBStatus{
							Versions: provisioningv1.BFBVersions{
								DOCA: "2.9.1",
							},
						},
					},

					DPUServiceTemplates: map[string]*dpuservicev1.DPUServiceTemplate{
						"service-1": {
							Status: dpuservicev1.DPUServiceTemplateStatus{
								Versions: map[string]string{
									"dpu.nvidia.com/doca-version": ">=2.10",
								},
							},
						},
						"service-2": {
							Status: dpuservicev1.DPUServiceTemplateStatus{
								Versions: map[string]string{
									"dpu.nvidia.com/doca-version": ">=2.5",
								},
							},
						},
					},
				}, true),
				Entry("BFB has invalid version", &dpuDeploymentDependencies{
					BFB: &provisioningv1.BFB{
						Status: provisioningv1.BFBStatus{
							Versions: provisioningv1.BFBVersions{
								DOCA: "blabla",
							},
						},
					},

					DPUServiceTemplates: map[string]*dpuservicev1.DPUServiceTemplate{
						"service-1": {
							Status: dpuservicev1.DPUServiceTemplateStatus{
								Versions: map[string]string{
									"dpu.nvidia.com/doca-version": ">=2.8",
								},
							},
						},
						"service-2": {
							Status: dpuservicev1.DPUServiceTemplateStatus{
								Versions: map[string]string{
									"dpu.nvidia.com/doca-version": ">=2.5",
								},
							},
						},
					},
				}, true),
				Entry("DPUServiceTemplates have version constraints for a type of version that is unsupported", &dpuDeploymentDependencies{
					BFB: &provisioningv1.BFB{
						Status: provisioningv1.BFBStatus{
							Versions: provisioningv1.BFBVersions{
								DOCA: "2.9.1",
							},
						},
					},

					DPUServiceTemplates: map[string]*dpuservicev1.DPUServiceTemplate{
						"service-1": {
							Status: dpuservicev1.DPUServiceTemplateStatus{
								Versions: map[string]string{
									"dpu.nvidia.com/bsp-version": ">=2.10",
								},
							},
						},
						"service-2": {
							Status: dpuservicev1.DPUServiceTemplateStatus{
								Versions: map[string]string{
									"dpu.nvidia.com/doca-version": ">=2.5",
								},
							},
						},
					},
				}, false),
			)
		})

		Context("When checking reconcileDPUServiceChains()", func() {
			BeforeEach(func() {
				By("Creating the dependencies")
				bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

				dpuFlavor := getMinimalDPUFlavor(testNS.Name)
				Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

				dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				DeferCleanup(cleanDPUDeploymentDerivatives, testNS.Name)
			})
			It("should create the correct DPUServiceChain", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.ServiceChains.Switches = []dpuservicev1.DPUDeploymentSwitch{
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someinterface",
									Name:          "somedpuservice",
								},
							},
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someinterface2",
									Name:          "somedpuservice2",
									IPAM: &dpuservicev1.IPAM{
										MatchLabels: map[string]string{
											"ipamkey1": "ipamvalue1",
										},
									},
								},
							},
						},
					},
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someotherinterface",
									Name:          "someotherservice",
								},
							},
						},
					},
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								ServiceInterface: &dpuservicev1.ServiceIfc{
									MatchLabels: map[string]string{
										"key": "value",
									},
									IPAM: &dpuservicev1.IPAM{
										MatchLabels: map[string]string{
											"ipamkey2": "ipamvalue2",
										},
									},
								},
							},
						},
					},
				}
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				chainDigest := calculateDPUServiceChainVersionDigest(dpuDeployment.Spec.ServiceChains.Switches)

				By("checking that correct DPUServiceChain is created")
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))

					By("checking the object metadata")
					gotDPUServiceChain := gotDPUServiceChainList.Items[0]

					g.Expect(gotDPUServiceChain.Labels).To(HaveLen(1))
					g.Expect(gotDPUServiceChain.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
					g.Expect(gotDPUServiceChain.Annotations).To(HaveKeyWithValue(dpuServiceChainVersionLabelAnnotationKey, chainDigest))
					g.Expect(gotDPUServiceChain.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))

					By("checking the spec")
					g.Expect(gotDPUServiceChain.Spec).To(BeComparableTo(dpuservicev1.DPUServiceChainSpec{
						// TODO: Derive and add cluster selector
						Template: dpuservicev1.ServiceChainSetSpecTemplate{
							Spec: dpuservicev1.ServiceChainSetSpec{
								NodeSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      dpuServiceChainVersionLabelAnnotationKey,
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{gotDPUServiceChain.Name},
										},
										{
											Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{fmt.Sprintf("%s_dpudeployment", testNS.Name)},
										},
									},
								},
								Template: dpuservicev1.ServiceChainSpecTemplate{
									Spec: dpuservicev1.ServiceChainSpec{
										Switches: []dpuservicev1.Switch{
											{
												Ports: []dpuservicev1.Port{
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_somedpuservice",
																ServiceInterfaceInterfaceNameLabel: "someinterface",
															},
														},
													},
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_somedpuservice2",
																ServiceInterfaceInterfaceNameLabel: "someinterface2",
															},
															IPAM: &dpuservicev1.IPAM{
																MatchLabels: map[string]string{
																	"ipamkey1": "ipamvalue1",
																},
															},
														},
													},
												},
											},
											{
												Ports: []dpuservicev1.Port{
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someotherservice",
																ServiceInterfaceInterfaceNameLabel: "someotherinterface",
															},
														},
													},
												},
											},
											{
												Ports: []dpuservicev1.Port{
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																"key": "value",
															},
															IPAM: &dpuservicev1.IPAM{
																MatchLabels: map[string]string{
																	"ipamkey2": "ipamvalue2",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should patch a manually modified DPUServiceChain as long as the modification is not on the version annotation", func() {
				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("checking that the DPUServiceChain is created")
				var gotDPUServiceChain *dpuservicev1.DPUServiceChain
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))
					gotDPUServiceChain = &gotDPUServiceChainList.Items[0]

					By("checking the original value of the switches")
					g.Expect(gotDPUServiceChain.Spec.Template.Spec.Template.Spec.Switches).To(BeComparableTo([]dpuservicev1.Switch{
						{
							Ports: []dpuservicev1.Port{
								{
									ServiceInterface: dpuservicev1.ServiceIfc{
										MatchLabels: map[string]string{
											"svc.dpu.nvidia.com/service":   "dpudeployment_dpudeployment_someservice",
											"svc.dpu.nvidia.com/interface": "someinterface",
										},
									},
								},
							},
						},
					}))

					By("checking the nodeSelector")
					g.Expect(gotDPUServiceChain.Spec.Template.Spec.NodeSelector).To(BeComparableTo(&metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      dpuServiceChainVersionLabelAnnotationKey,
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{gotDPUServiceChain.Name},
							},
							{
								Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{fmt.Sprintf("%s_dpudeployment", testNS.Name)},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying the DPUServiceChain manually")
				gotDPUServiceChain.Spec.Template.Spec.Template.Spec.Switches = []dpuservicev1.Switch{
					{
						Ports: []dpuservicev1.Port{
							{
								ServiceInterface: dpuservicev1.ServiceIfc{
									MatchLabels: map[string]string{
										"somelabel": "somevalue",
									},
								},
							},
						},
					},
				}
				gotDPUServiceChain.SetManagedFields(nil)
				gotDPUServiceChain.SetGroupVersionKind(dpuservicev1.DPUServiceChainGroupVersionKind)
				Expect(testClient.Patch(ctx, gotDPUServiceChain, client.Apply, client.ForceOwnership, client.FieldOwner("some-test-controller"))).To(Succeed())

				By("checking that the DPUServiceChain is reverted to the original")
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))
					g.Expect(gotDPUServiceChainList.Items[0].UID).To(Equal(gotDPUServiceChain.UID))

					By("checking the value of the switches")
					g.Expect(gotDPUServiceChainList.Items[0].Spec.Template.Spec.Template.Spec.Switches).To(BeComparableTo([]dpuservicev1.Switch{
						{
							Ports: []dpuservicev1.Port{
								{
									ServiceInterface: dpuservicev1.ServiceIfc{
										MatchLabels: map[string]string{
											"svc.dpu.nvidia.com/service":   "dpudeployment_dpudeployment_someservice",
											"svc.dpu.nvidia.com/interface": "someinterface",
										},
									},
								},
							},
						},
					}))

					By("checking the nodeSelector")
					g.Expect(gotDPUServiceChainList.Items[0].Spec.Template.Spec.NodeSelector).To(BeComparableTo(&metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      dpuServiceChainVersionLabelAnnotationKey,
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{gotDPUServiceChain.Name},
							},
							{
								Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{fmt.Sprintf("%s_dpudeployment", testNS.Name)},
							},
						},
					}))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should update the disruptive DPUServiceChain on update of dpuDeployment.Spec.ServiceChains.Switches", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				// make the DPUServiceChain disruptive
				dpuDeployment.Spec.ServiceChains.NodeEffect = ptr.To(true)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)
				chainDigest := calculateDPUServiceChainVersionDigest(dpuDeployment.Spec.ServiceChains.Switches)

				By("waiting for the initial DPUServiceChains to be applied")
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying dpuDeployment.Spec.ServiceChains.Switches")
				dpuDeployment.Spec.ServiceChains.Switches = []dpuservicev1.DPUDeploymentSwitch{
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someinterface",
									Name:          "somedpuservice",
								},
							},
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someinterface2",
									Name:          "somedpuservice2",
									IPAM: &dpuservicev1.IPAM{
										MatchLabels: map[string]string{
											"ipamkey1": "ipamvalue1",
										},
									},
								},
							},
						},
					},
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someotherinterface",
									Name:          "someotherservice",
								},
							},
						},
					},
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								ServiceInterface: &dpuservicev1.ServiceIfc{
									MatchLabels: map[string]string{
										"key": "value",
									},
									IPAM: &dpuservicev1.IPAM{
										MatchLabels: map[string]string{
											"ipamkey2": "ipamvalue2",
										},
									},
								},
							},
						},
					},
				}

				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())

				chainDigest2 := calculateDPUServiceChainVersionDigest(dpuDeployment.Spec.ServiceChains.Switches)
				Expect(chainDigest2).NotTo(Equal(chainDigest))

				versions := []string{chainDigest, chainDigest2}

				By("checking that the DPUServiceChain is updated as expected")
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(2))

					By("checking the object metadata")
					obj := gotDPUServiceChainList.Items[0]

					g.Expect(obj.Labels).To(HaveLen(1))
					g.Expect(obj.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
					matchCount := 0
					for _, obj := range gotDPUServiceChainList.Items {
						for _, version := range versions {
							if obj.GetAnnotations()[dpuServiceChainVersionLabelAnnotationKey] == version {
								matchCount++
							}
						}
					}
					// expect the two versions to be present
					g.Expect(matchCount).To(Equal(len(versions)))
					g.Expect(obj.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("making the new version ready")
				gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
				Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
				Expect(gotDPUServiceChainList.Items).To(HaveLen(2))

				var currentSvcChain *dpuservicev1.DPUServiceChain
				for _, dpuServiceChain := range gotDPUServiceChainList.Items {
					if dpuServiceChain.GetAnnotations()[dpuServiceChainVersionLabelAnnotationKey] == chainDigest2 {
						currentSvcChain = &dpuServiceChain
					}
				}
				Expect(currentSvcChain).NotTo(BeNil())
				patcher = patch.NewSerialPatcher(currentSvcChain, testClient)
				currentSvcChain.Status.Conditions = []metav1.Condition{
					{
						Type:               string(conditions.TypeReady),
						Status:             metav1.ConditionTrue,
						Reason:             string(conditions.ReasonSuccess),
						LastTransitionTime: metav1.NewTime(time.Now()),
					},
				}
				Expect(patcher.Patch(ctx, currentSvcChain, patch.WithFieldOwner("test"))).To(Succeed())

				By("checking that the DPUServiceChain is updated as expected")
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))

					By("checking the object metadata")
					gotDPUServiceChain := gotDPUServiceChainList.Items[0]

					g.Expect(gotDPUServiceChain.Labels).To(HaveLen(1))
					g.Expect(gotDPUServiceChain.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
					g.Expect(gotDPUServiceChain.Annotations).To(HaveKeyWithValue(dpuServiceChainVersionLabelAnnotationKey, chainDigest2))
					g.Expect(gotDPUServiceChain.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))

					By("checking the spec")
					g.Expect(gotDPUServiceChain.Spec).To(BeComparableTo(dpuservicev1.DPUServiceChainSpec{
						Template: dpuservicev1.ServiceChainSetSpecTemplate{
							Spec: dpuservicev1.ServiceChainSetSpec{
								NodeSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      dpuServiceChainVersionLabelAnnotationKey,
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{gotDPUServiceChain.Name},
										},
										{
											Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{fmt.Sprintf("%s_dpudeployment", testNS.Name)},
										},
									},
								},
								Template: dpuservicev1.ServiceChainSpecTemplate{
									Spec: dpuservicev1.ServiceChainSpec{
										Switches: []dpuservicev1.Switch{
											{
												Ports: []dpuservicev1.Port{
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_somedpuservice",
																ServiceInterfaceInterfaceNameLabel: "someinterface",
															},
														},
													},
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_somedpuservice2",
																ServiceInterfaceInterfaceNameLabel: "someinterface2",
															},
															IPAM: &dpuservicev1.IPAM{
																MatchLabels: map[string]string{
																	"ipamkey1": "ipamvalue1",
																},
															},
														},
													},
												},
											},
											{
												Ports: []dpuservicev1.Port{
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someotherservice",
																ServiceInterfaceInterfaceNameLabel: "someotherinterface",
															},
														},
													},
												},
											},
											{
												Ports: []dpuservicev1.Port{
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																"key": "value",
															},
															IPAM: &dpuservicev1.IPAM{
																MatchLabels: map[string]string{
																	"ipamkey2": "ipamvalue2",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should update the disruptive DPUServiceChain to non-diruptive", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				// make the DPUServiceChain disruptive
				dpuDeployment.Spec.ServiceChains.NodeEffect = ptr.To(true)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)
				chainDigest := calculateDPUServiceChainVersionDigest(dpuDeployment.Spec.ServiceChains.Switches)

				By("waiting for the initial DPUServiceChains to be applied")
				var firstDPUServiceChain *dpuservicev1.DPUServiceChain
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := getDPUServiceChainList()
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))
					firstDPUServiceChain = &gotDPUServiceChainList.Items[0]
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying dpuDeployment.Spec.ServiceChains.Switches")
				dpuDeployment.Spec.ServiceChains = dpuservicev1.ServiceChains{
					// make the DPUServiceChain non-disruptive
					NodeEffect: ptr.To(false),
					Switches: []dpuservicev1.DPUDeploymentSwitch{
						{
							Ports: []dpuservicev1.DPUDeploymentPort{
								{
									Service: &dpuservicev1.DPUDeploymentService{
										InterfaceName: "someinterface",
										Name:          "somedpuservice",
									},
								},
								{
									Service: &dpuservicev1.DPUDeploymentService{
										InterfaceName: "someinterface2",
										Name:          "somedpuservice2",
										IPAM: &dpuservicev1.IPAM{
											MatchLabels: map[string]string{
												"ipamkey1": "ipamvalue1",
											},
										},
									},
								},
							},
						},
						{
							Ports: []dpuservicev1.DPUDeploymentPort{
								{
									Service: &dpuservicev1.DPUDeploymentService{
										InterfaceName: "someotherinterface",
										Name:          "someotherservice",
									},
								},
							},
						},
						{
							Ports: []dpuservicev1.DPUDeploymentPort{
								{
									ServiceInterface: &dpuservicev1.ServiceIfc{
										MatchLabels: map[string]string{
											"key": "value",
										},
										IPAM: &dpuservicev1.IPAM{
											MatchLabels: map[string]string{
												"ipamkey2": "ipamvalue2",
											},
										},
									},
								},
							},
						},
					},
				}
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())

				chainDigest2 := calculateDPUServiceChainVersionDigest(dpuDeployment.Spec.ServiceChains.Switches)
				Expect(chainDigest2).NotTo(Equal(chainDigest))

				By("checking that the DPUServiceChain is updated as expected")
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))

					By("checking the object metadata")
					gotDPUServiceChain := gotDPUServiceChainList.Items[0]

					g.Expect(gotDPUServiceChain.Labels).To(HaveLen(1))
					g.Expect(gotDPUServiceChain.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
					g.Expect(gotDPUServiceChain.Annotations).To(HaveKeyWithValue(dpuServiceChainVersionLabelAnnotationKey, chainDigest2))
					g.Expect(gotDPUServiceChain.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))

					By("checking the spec")
					g.Expect(gotDPUServiceChain.Spec).To(BeComparableTo(dpuservicev1.DPUServiceChainSpec{
						Template: dpuservicev1.ServiceChainSetSpecTemplate{
							Spec: dpuservicev1.ServiceChainSetSpec{
								NodeSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      dpuServiceChainVersionLabelAnnotationKey,
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{firstDPUServiceChain.Name},
										},
										{
											Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{fmt.Sprintf("%s_dpudeployment", testNS.Name)},
										},
									},
								},
								Template: dpuservicev1.ServiceChainSpecTemplate{
									Spec: dpuservicev1.ServiceChainSpec{
										Switches: []dpuservicev1.Switch{
											{
												Ports: []dpuservicev1.Port{
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_somedpuservice",
																ServiceInterfaceInterfaceNameLabel: "someinterface",
															},
														},
													},
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_somedpuservice2",
																ServiceInterfaceInterfaceNameLabel: "someinterface2",
															},
															IPAM: &dpuservicev1.IPAM{
																MatchLabels: map[string]string{
																	"ipamkey1": "ipamvalue1",
																},
															},
														},
													},
												},
											},
											{
												Ports: []dpuservicev1.Port{
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someotherservice",
																ServiceInterfaceInterfaceNameLabel: "someotherinterface",
															},
														},
													},
												},
											},
											{
												Ports: []dpuservicev1.Port{
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																"key": "value",
															},
															IPAM: &dpuservicev1.IPAM{
																MatchLabels: map[string]string{
																	"ipamkey2": "ipamvalue2",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
			It("should update the DPUServiceChain on update of dpuDeployment.Spec.ServiceChains.Switches", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)
				chainDigest := calculateDPUServiceChainVersionDigest(dpuDeployment.Spec.ServiceChains.Switches)

				By("waiting for the initial DPUServiceChains to be applied")
				var dpuServiceChainUID types.UID
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))
					for _, dpuServiceChain := range gotDPUServiceChainList.Items {
						dpuServiceChainUID = dpuServiceChain.UID
					}
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("modifying dpuDeployment.Spec.ServiceChains.Switches")
				dpuDeployment.Spec.ServiceChains.Switches = []dpuservicev1.DPUDeploymentSwitch{
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someinterface",
									Name:          "somedpuservice",
								},
							},
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someinterface2",
									Name:          "somedpuservice2",
									IPAM: &dpuservicev1.IPAM{
										MatchLabels: map[string]string{
											"ipamkey1": "ipamvalue1",
										},
									},
								},
							},
						},
					},
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someotherinterface",
									Name:          "someotherservice",
								},
							},
						},
					},
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								ServiceInterface: &dpuservicev1.ServiceIfc{
									MatchLabels: map[string]string{
										"key": "value",
									},
									IPAM: &dpuservicev1.IPAM{
										MatchLabels: map[string]string{
											"ipamkey2": "ipamvalue2",
										},
									},
								},
							},
						},
					},
				}

				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())

				chainDigest2 := calculateDPUServiceChainVersionDigest(dpuDeployment.Spec.ServiceChains.Switches)
				Expect(chainDigest2).NotTo(Equal(chainDigest))

				By("checking that the DPUServiceChain is updated as expected")
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))

					By("checking the object metadata")
					gotDPUServiceChain := gotDPUServiceChainList.Items[0]

					g.Expect(gotDPUServiceChain.Labels).To(HaveLen(1))
					g.Expect(gotDPUServiceChain.Labels).To(HaveKeyWithValue("svc.dpu.nvidia.com/owned-by-dpudeployment", fmt.Sprintf("%s_dpudeployment", testNS.Name)))
					g.Expect(gotDPUServiceChain.Annotations).To(HaveKeyWithValue(dpuServiceChainVersionLabelAnnotationKey, chainDigest2))
					g.Expect(gotDPUServiceChain.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					g.Expect(dpuServiceChainUID).To(Equal(gotDPUServiceChain.UID))

					By("checking the spec")
					g.Expect(gotDPUServiceChain.Spec).To(BeComparableTo(dpuservicev1.DPUServiceChainSpec{
						Template: dpuservicev1.ServiceChainSetSpecTemplate{
							Spec: dpuservicev1.ServiceChainSetSpec{
								NodeSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      dpuServiceChainVersionLabelAnnotationKey,
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{gotDPUServiceChain.Name},
										},
										{
											Key:      "svc.dpu.nvidia.com/owned-by-dpudeployment",
											Operator: metav1.LabelSelectorOpIn,
											Values:   []string{fmt.Sprintf("%s_dpudeployment", testNS.Name)},
										},
									},
								},
								Template: dpuservicev1.ServiceChainSpecTemplate{
									Spec: dpuservicev1.ServiceChainSpec{
										Switches: []dpuservicev1.Switch{
											{
												Ports: []dpuservicev1.Port{
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_somedpuservice",
																ServiceInterfaceInterfaceNameLabel: "someinterface",
															},
														},
													},
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_somedpuservice2",
																ServiceInterfaceInterfaceNameLabel: "someinterface2",
															},
															IPAM: &dpuservicev1.IPAM{
																MatchLabels: map[string]string{
																	"ipamkey1": "ipamvalue1",
																},
															},
														},
													},
												},
											},
											{
												Ports: []dpuservicev1.Port{
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																dpuservicev1.DPFServiceIDLabelKey:  "dpudeployment_dpudeployment_someotherservice",
																ServiceInterfaceInterfaceNameLabel: "someotherinterface",
															},
														},
													},
												},
											},
											{
												Ports: []dpuservicev1.Port{
													{
														ServiceInterface: dpuservicev1.ServiceIfc{
															MatchLabels: map[string]string{
																"key": "value",
															},
															IPAM: &dpuservicev1.IPAM{
																MatchLabels: map[string]string{
																	"ipamkey2": "ipamvalue2",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})

			It("should create new DPUServiceChain on manual deletion of the DPUServiceChain", func() {
				By("Creating the DPUDeployment")
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("waiting for the initial DPUServiceChain to be applied")
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))
				}).WithTimeout(30 * time.Second).Should(Succeed())

				By("manually deleting the DPUServiceChain")
				Consistently(func(g Gomega) {
					gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					if len(gotDPUServiceChainList.Items) == 0 {
						return
					}
					g.Expect(testutils.CleanupAndWait(ctx, testClient, &gotDPUServiceChainList.Items[0])).To(Succeed())
				}).WithTimeout(5 * time.Second).Should(Succeed())

				By("checking that the DPUServiceChain is created")
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))
				}).WithTimeout(30 * time.Second).Should(Succeed())
			})
		})
	})

	Context("When checking the status transitions", func() {
		var testNS *corev1.Namespace
		var i *informer.TestInformer
		BeforeEach(func() {
			By("Creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			DeferCleanup(testClient.Delete, ctx, testNS)

			By("Creating the informer infrastructure for DPUDeployment")
			i = informer.NewInformer(cfg, dpuservicev1.DPUDeploymentGroupVersionKind, testNS.Name, "dpudeployments")
			DeferCleanup(i.Cleanup)
			go i.Run()

			DeferCleanup(cleanDPUDeploymentDerivatives, testNS.Name)
		})
		It("DPUDeployment has all the conditions with Pending Reason at start of the reconciliation loop", func() {
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &dpuservicev1.DPUDeployment{}
				newObj := &dpuservicev1.DPUDeployment{}
				g.Expect(testClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(testClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

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
					HaveField("Type", string(dpuservicev1.ConditionPreReqsReady)),
					HaveField("Status", metav1.ConditionUnknown),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionResourceFittingReady)),
					HaveField("Status", metav1.ConditionUnknown),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionVersionMatchingReady)),
					HaveField("Status", metav1.ConditionUnknown),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUSetsReconciled)),
					HaveField("Status", metav1.ConditionUnknown),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServicesReconciled)),
					HaveField("Status", metav1.ConditionUnknown),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceChainsReconciled)),
					HaveField("Status", metav1.ConditionUnknown),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceInterfacesReconciled)),
					HaveField("Status", metav1.ConditionUnknown),
					HaveField("Reason", string(conditions.ReasonPending)),
				),

				// We have success at the following conditions because there is no object in the cluster to watch on
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUSetsReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServicesReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceChainsReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceInterfacesReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
			))
		})
		It("DPUDeployment has all *Reconciled conditions with Success Reason at the end of a successful reconciliation loop but *Ready with Pending reason on underlying object not ready", func() {
			By("Creating the dependencies")
			bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

			dpuFlavor := getMinimalDPUFlavor(testNS.Name)
			Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

			dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
			Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

			dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

			By("Creating the DPUDeployment")
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			By("Checking the conditions")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &dpuservicev1.DPUDeployment{}
				newObj := &dpuservicev1.DPUDeployment{}
				g.Expect(testClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(testClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", string(dpuservicev1.ConditionPreReqsReady)),
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
					HaveField("Type", string(dpuservicev1.ConditionPreReqsReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionResourceFittingReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionVersionMatchingReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUSetsReconciled)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServicesReconciled)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceChainsReconciled)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceInterfacesReconciled)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),

				// This one is always true because we don't watch on the DPUSet status yet. See code for reasoning.
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUSetsReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServicesReady)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceChainsReady)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceInterfacesReady)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
			))
		})
		It("DPUDeployment has all conditions with Success Reason at the end of a successful reconciliation loop and underlying object ready", func() {
			By("Creating the dependencies")
			bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

			dpuFlavor := getMinimalDPUFlavor(testNS.Name)
			Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

			dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
			Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

			dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

			By("Creating the DPUDeployment")
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			By("Updating the status of the underlying dependencies")
			Eventually(func(g Gomega) {
				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				g.Expect(gotDPUServiceList.Items).ToNot(BeEmpty())
				for _, dpuService := range gotDPUServiceList.Items {
					dpuService.Status.Conditions = []metav1.Condition{
						{
							Type:               string(conditions.TypeReady),
							Status:             metav1.ConditionTrue,
							Reason:             string(conditions.ReasonSuccess),
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
					}
					dpuService.SetGroupVersionKind(dpuservicev1.DPUServiceGroupVersionKind)
					dpuService.SetManagedFields(nil)
					g.Expect(testClient.Status().Patch(ctx, &dpuService, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
				}

				gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
				g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
				g.Expect(gotDPUServiceChainList.Items).ToNot(BeEmpty())
				for _, dpuServiceChain := range gotDPUServiceChainList.Items {
					dpuServiceChain.Status.Conditions = []metav1.Condition{
						{
							Type:               string(conditions.TypeReady),
							Status:             metav1.ConditionTrue,
							Reason:             string(conditions.ReasonSuccess),
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
					}
					dpuServiceChain.SetGroupVersionKind(dpuservicev1.DPUServiceChainGroupVersionKind)
					dpuServiceChain.SetManagedFields(nil)
					g.Expect(testClient.Status().Patch(ctx, &dpuServiceChain, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
				}

				gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
				g.Expect(gotDPUServiceInterfaceList.Items).ToNot(BeEmpty())
				for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
					dpuServiceInterface.Status.Conditions = []metav1.Condition{
						{
							Type:               string(conditions.TypeReady),
							Status:             metav1.ConditionTrue,
							Reason:             string(conditions.ReasonSuccess),
							LastTransitionTime: metav1.NewTime(time.Now()),
						},
					}
					dpuServiceInterface.SetGroupVersionKind(dpuservicev1.DPUServiceInterfaceGroupVersionKind)
					dpuServiceInterface.SetManagedFields(nil)
					g.Expect(testClient.Status().Patch(ctx, &dpuServiceInterface, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
				}
			}).WithTimeout(30 * time.Second).Should(Succeed())

			By("Checking the conditions")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &dpuservicev1.DPUDeployment{}
				newObj := &dpuservicev1.DPUDeployment{}
				g.Expect(testClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(testClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())
				return newObj.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ConsistOf(
				And(
					HaveField("Type", string(conditions.TypeReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionPreReqsReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionResourceFittingReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionVersionMatchingReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUSetsReconciled)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServicesReconciled)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceChainsReconciled)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceInterfacesReconciled)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),

				// This one is always true because we don't watch on the DPUSet status yet. See code for reasoning.
				// Also, we don't create one DPUSet with the minimal DPUDeployment.
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUSetsReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServicesReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceChainsReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceInterfacesReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
			))
		})
		It("DPUDeployment has condition ResourceFittingReady with Failed Reason when the resources of the underlying DPUServices can't fit the selected DPUs", func() {
			By("Creating the dependencies")
			bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

			dpuFlavor := getMinimalDPUFlavor(testNS.Name)
			dpuFlavor.Spec.DPUResources = corev1.ResourceList{"cpu": resource.MustParse("5")}
			Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

			dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
			Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

			dpuServiceTemplate := getMinimalDPUServiceTemplate(testNS.Name)
			dpuServiceTemplate.Spec.ResourceRequirements = make(corev1.ResourceList)
			dpuServiceTemplate.Spec.ResourceRequirements["cpu"] = resource.MustParse("6")
			Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
			patchDPUServiceTemplateWithStatus(dpuServiceTemplate)

			By("Creating the DPUDeployment")
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			By("Checking the conditions")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &dpuservicev1.DPUDeployment{}
				newObj := &dpuservicev1.DPUDeployment{}
				g.Expect(testClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(testClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", string(dpuservicev1.ConditionPreReqsReady)),
						HaveField("Status", metav1.ConditionUnknown),
						HaveField("Reason", string(conditions.ReasonPending)),
					),
				))
				return newObj.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ContainElement(
				And(
					HaveField("Type", string(dpuservicev1.ConditionResourceFittingReady)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonFailure)),
				),
			))
		})
		It("DPUDeployment has condition PrerequisitesReady with Error Reason at the end of first reconciliation loop that failed on dependencies", func() {
			// Add DPUDeployment
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &dpuservicev1.DPUDeployment{}
				newObj := &dpuservicev1.DPUDeployment{}
				g.Expect(testClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(testClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", string(dpuservicev1.ConditionPreReqsReady)),
						HaveField("Status", metav1.ConditionUnknown),
						HaveField("Reason", string(conditions.ReasonPending)),
					),
				))
				return newObj.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ContainElement(
				And(
					HaveField("Type", string(dpuservicev1.ConditionPreReqsReady)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonPending)),
				),
			))
		})
		It("DPUDeployment has condition VersionMatchingReady with Error Reason at the end of first reconciliation loop that failed on dependencies", func() {
			By("Creating the dependencies")
			bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

			dpuFlavor := getMinimalDPUFlavor(testNS.Name)
			Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

			dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
			Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

			dpuServiceTemplate := getMinimalDPUServiceTemplate(testNS.Name)
			Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
			dpuServiceTemplate.Status.Versions = make(map[string]string)
			dpuServiceTemplate.Status.Versions["dpu.nvidia.com/doca-version"] = ">=2.10"
			patchDPUServiceTemplateWithStatus(dpuServiceTemplate)

			By("Creating the DPUDeployment")
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &dpuservicev1.DPUDeployment{}
				newObj := &dpuservicev1.DPUDeployment{}
				g.Expect(testClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(testClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())

				g.Expect(oldObj.Status.Conditions).To(ContainElement(
					And(
						HaveField("Type", string(dpuservicev1.ConditionVersionMatchingReady)),
						HaveField("Status", metav1.ConditionUnknown),
						HaveField("Reason", string(conditions.ReasonPending)),
					),
				))
				return newObj.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ContainElement(
				And(
					HaveField("Type", string(dpuservicev1.ConditionVersionMatchingReady)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonError)),
				),
			))
		})
		It("DPUDeployment has condition Deleting with AwaitingDeletion Reason when there are still objects in the cluster", func() {
			By("Creating the dependencies")
			bfb := createMinimalBFBWithStatus("somebfb", testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, bfb)

			dpuFlavor := getMinimalDPUFlavor(testNS.Name)
			Expect(testClient.Create(ctx, dpuFlavor)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuFlavor)

			dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
			Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

			dpuServiceTemplate := createMinimalDPUServiceTemplateWithStatus(testNS.Name)
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
			objs := make(map[client.Object]interface{})

			By("Creating the DPUDeployment")
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			dpuDeployment.Spec.DPUs.DPUSets = []dpuservicev1.DPUSet{
				{
					NameSuffix: "dpuset1",
					NodeSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nodekey1": "nodevalue1",
						},
					},
					DPUSelector: map[string]string{
						"dpukey1": "dpuvalue1",
					},
					DPUAnnotations: map[string]string{
						"annotationkey1": "annotationvalue1",
					},
				},
			}
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			By("Checking that the underlying resources are created and adding fake finalizer")
			DeferCleanup(func() {
				By("Cleaning up the finalizers so that objects can be deleted")
				for obj := range objs {
					Expect(client.IgnoreNotFound(testClient.Patch(ctx, obj, client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"finalizers":[]}}`))))).To(Succeed())
				}
			})
			Eventually(func(g Gomega) {
				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				g.Expect(gotDPUServiceList.Items).ToNot(BeEmpty())
				for _, dpuService := range gotDPUServiceList.Items {
					objs[&dpuService] = struct{}{}
					dpuService.SetFinalizers([]string{"test.dpu.nvidia.com/test"})
					dpuService.SetGroupVersionKind(dpuservicev1.DPUServiceGroupVersionKind)
					dpuService.SetManagedFields(nil)
					g.Expect(testClient.Patch(ctx, &dpuService, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
				}

				gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
				g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
				g.Expect(gotDPUServiceChainList.Items).ToNot(BeEmpty())
				for _, dpuServiceChain := range gotDPUServiceChainList.Items {
					objs[&dpuServiceChain] = struct{}{}
					dpuServiceChain.SetFinalizers([]string{"test.dpu.nvidia.com/test"})
					dpuServiceChain.SetGroupVersionKind(dpuservicev1.DPUServiceChainGroupVersionKind)
					dpuServiceChain.SetManagedFields(nil)
					g.Expect(testClient.Patch(ctx, &dpuServiceChain, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
				}
				gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
				g.Expect(gotDPUServiceInterfaceList.Items).ToNot(BeEmpty())
				for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
					objs[&dpuServiceInterface] = struct{}{}
					dpuServiceInterface.SetFinalizers([]string{"test.dpu.nvidia.com/test"})
					dpuServiceInterface.SetGroupVersionKind(dpuservicev1.DPUServiceInterfaceGroupVersionKind)
					dpuServiceInterface.SetManagedFields(nil)
					g.Expect(testClient.Patch(ctx, &dpuServiceInterface, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
				}
				gotDPUSetList := &provisioningv1.DPUSetList{}
				g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
				g.Expect(gotDPUSetList.Items).ToNot(BeEmpty())
				for _, dpuSet := range gotDPUSetList.Items {
					objs[&dpuSet] = struct{}{}
					dpuSet.SetFinalizers([]string{"test.dpu.nvidia.com/test"})
					dpuSet.SetGroupVersionKind(provisioningv1.DPUSetGroupVersionKind)
					dpuSet.SetManagedFields(nil)
					g.Expect(testClient.Patch(ctx, &dpuSet, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
				}
			}).WithTimeout(30 * time.Second).Should(Succeed())

			By("Deleting the DPUDeployment")
			Expect(testClient.Delete(ctx, dpuDeployment)).To(Succeed())

			By("Checking the conditions")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &dpuservicev1.DPUDeployment{}
				newObj := &dpuservicev1.DPUDeployment{}
				g.Expect(testClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(testClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())
				return newObj.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ContainElements(
				And(
					HaveField("Type", string(conditions.TypeReady)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonAwaitingDeletion)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServicesReconciled)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonAwaitingDeletion)),
					HaveField("Message", ContainSubstring("1")),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceChainsReconciled)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonAwaitingDeletion)),
					HaveField("Message", ContainSubstring("1")),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceInterfacesReconciled)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonAwaitingDeletion)),
					HaveField("Message", ContainSubstring("1")),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUSetsReconciled)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonAwaitingDeletion)),
					HaveField("Message", ContainSubstring("1")),
				),
			))

			By("Removing finalizer from all the underlying objects but the DPUSets to check the next status")
			Eventually(func(g Gomega) {
				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				for _, dpuService := range gotDPUServiceList.Items {
					g.Expect(testClient.Patch(ctx, &dpuService, client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"finalizers":[]}}`)))).To(Succeed())
				}

				gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
				g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
				for _, dpuServiceChain := range gotDPUServiceChainList.Items {
					g.Expect(testClient.Patch(ctx, &dpuServiceChain, client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"finalizers":[]}}`)))).To(Succeed())
				}

				gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceInterfaceList)).To(Succeed())
				for _, dpuServiceInterface := range gotDPUServiceInterfaceList.Items {
					g.Expect(testClient.Patch(ctx, &dpuServiceInterface, client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"finalizers":[]}}`)))).To(Succeed())
				}
			}).WithTimeout(30 * time.Second).Should(Succeed())

			By("Checking the conditions")
			Eventually(func(g Gomega) []metav1.Condition {
				ev := &informer.Event{}
				g.Eventually(i.UpdateEvents).Should(Receive(ev))
				oldObj := &dpuservicev1.DPUDeployment{}
				newObj := &dpuservicev1.DPUDeployment{}
				g.Expect(testClient.Scheme().Convert(ev.OldObj, oldObj, nil)).ToNot(HaveOccurred())
				g.Expect(testClient.Scheme().Convert(ev.NewObj, newObj, nil)).ToNot(HaveOccurred())
				return newObj.Status.Conditions
			}).WithTimeout(10 * time.Second).Should(ContainElements(
				And(
					HaveField("Type", string(conditions.TypeReady)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonAwaitingDeletion)),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServicesReconciled)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonAwaitingDeletion)),
					HaveField("Message", ContainSubstring("are deleted")),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceChainsReconciled)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonAwaitingDeletion)),
					HaveField("Message", ContainSubstring("are deleted")),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUServiceInterfacesReconciled)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonAwaitingDeletion)),
					HaveField("Message", ContainSubstring("are deleted")),
				),
				And(
					HaveField("Type", string(dpuservicev1.ConditionDPUSetsReconciled)),
					HaveField("Status", metav1.ConditionFalse),
					HaveField("Reason", string(conditions.ReasonAwaitingDeletion)),
					HaveField("Message", ContainSubstring("1")),
				),
			))

			By("Removing finalizer from the DPUSets to ensure deletion")
			Eventually(func(g Gomega) {
				gotDPUSetList := &provisioningv1.DPUSetList{}
				g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
				for _, dpuSet := range gotDPUSetList.Items {
					g.Expect(testClient.Patch(ctx, &dpuSet, client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"finalizers":[]}}`)))).To(Succeed())
				}
			}).WithTimeout(30 * time.Second).Should(Succeed())
		})
	})
})

var _ = Describe("API Validations for DPUDeployment related objects", func() {
	var testNS *corev1.Namespace
	BeforeEach(func() {
		By("Creating the namespaces")
		testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
		Expect(testClient.Create(ctx, testNS)).To(Succeed())
		DeferCleanup(testClient.Delete, ctx, testNS)
	})
	Context("When checking the DPUServiceConfiguration API validations", func() {
		DescribeTable("Validates the interfaces and deployInCluster correctly", func(deployInCluster *bool, hasInterfaces bool, expectError bool) {
			dpuServiceConfiguration := getMinimalDPUServiceConfiguration(testNS.Name)
			dpuServiceConfiguration.Spec.ServiceConfiguration.DeployInCluster = deployInCluster
			if !hasInterfaces {
				dpuServiceConfiguration.Spec.Interfaces = nil
			}
			err := testClient.Create(ctx, dpuServiceConfiguration)
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		},
			Entry("valid config - without specifying deployInCluster and with interfaces", nil, true, false),
			Entry("valid config - without specifying deployInCluster and without interfaces", nil, false, false),
			Entry("valid config - with deployInCluster=false and with interfaces", ptr.To(false), true, false),
			Entry("valid config - with deployInCluster=false and without interfaces", ptr.To(false), false, false),
			Entry("valid config - with deployInCluster=true and without interfaces", ptr.To(true), false, false),
			Entry("invalid config - with deployInCluster=true and with interfaces", ptr.To(true), true, true),
		)
	})
	Context("When checking the DPUDeployment API validations", func() {
		It("should not create the DPUDeployment if system annotations are present", func() {
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			dpuDeployment.Spec.DPUs.DPUSets = []dpuservicev1.DPUSet{
				{
					NameSuffix: "dpuset1",
					DPUSelector: map[string]string{
						"dpukey1": "dpuvalue1",
					},
					DPUAnnotations: map[string]string{
						"dpu.nvidia.com": "not allowed",
					},
				},
			}
			Expect(testClient.Create(ctx, dpuDeployment)).ToNot(Succeed())
			dpuDeployment.Spec.DPUs.DPUSets[0].DPUAnnotations = map[string]string{
				"annKey": "annVal",
			}
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			dpuDeployment.Spec.DPUs.DPUSets[0].DPUAnnotations = map[string]string{
				"anything.dpu.nvidia.com": "not allowed",
			}
			Expect(testClient.Create(ctx, dpuDeployment)).ToNot(Succeed())
			dpuDeployment.Spec.DPUs.DPUSets[0].DPUAnnotations = map[string]string{
				"anything.dpu.nvidia.com/anything": "not allowed",
			}
			Expect(testClient.Create(ctx, dpuDeployment)).ToNot(Succeed())
		})

		It("should fail to create the DPUServiceConfiguration with restricted label/annotation in servicedaemonset", func() {
			By("creating the DPUServiceConfiguration with servicedaemonset which has a label/annotation which ends with dpu.nvidia.com")
			dpuServiceConfigurarion := getMinimalDPUServiceConfiguration(testNS.Name)
			dpuServiceConfigurarion.Spec.ServiceConfiguration.ServiceDaemonSet.Labels = map[string]string{
				"dpu.nvidia.com": "not allowed",
			}
			Expect(testClient.Create(ctx, dpuServiceConfigurarion)).ToNot(Succeed())

			dpuServiceConfigurarion = getMinimalDPUServiceConfiguration(testNS.Name)
			dpuServiceConfigurarion.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations = map[string]string{
				"dpu.nvidia.com": "not allowed",
			}
			Expect(testClient.Create(ctx, dpuServiceConfigurarion)).ToNot(Succeed())

			dpuServiceConfigurarion = getMinimalDPUServiceConfiguration(testNS.Name)
			dpuServiceConfigurarion.Spec.ServiceConfiguration.ServiceDaemonSet.Labels = map[string]string{
				"anything.dpu.nvidia.com": "not allowed",
			}
			Expect(testClient.Create(ctx, dpuServiceConfigurarion)).ToNot(Succeed())
			dpuServiceConfigurarion.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations = map[string]string{
				"anything.dpu.nvidia.com": "not allowed",
			}
			Expect(testClient.Create(ctx, dpuServiceConfigurarion)).ToNot(Succeed())
		})

		It("should fail to create the DPUServiceConfiguration with restricted label/annotation in servicedaemonset", func() {
			By("creating the DPUServiceConfiguration with servicedaemonset which has a label/annotation which contains dpu.nvidia.com/")
			dpuServiceConfigurarion := getMinimalDPUServiceConfiguration(testNS.Name)
			dpuServiceConfigurarion.Spec.ServiceConfiguration.ServiceDaemonSet.Labels = map[string]string{
				"anything.dpu.nvidia.com/anything": "not allowed",
			}
			Expect(testClient.Create(ctx, dpuServiceConfigurarion)).ToNot(Succeed())
			dpuServiceConfigurarion = getMinimalDPUServiceConfiguration(testNS.Name)
			dpuServiceConfigurarion.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations = map[string]string{
				"anything.dpu.nvidia.com/anything": "not allowed",
			}
			Expect(testClient.Create(ctx, dpuServiceConfigurarion)).ToNot(Succeed())
		})
	})
})

func getMinimalDPUDeployment(namespace string) *dpuservicev1.DPUDeployment {
	return &dpuservicev1.DPUDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dpudeployment",
			Namespace: namespace,
		},
		Spec: dpuservicev1.DPUDeploymentSpec{
			DPUs: dpuservicev1.DPUs{
				BFB:    "somebfb",
				Flavor: "someflavor",
			},
			Services: map[string]dpuservicev1.DPUDeploymentServiceConfiguration{
				"someservice": {
					ServiceTemplate:      "sometemplate",
					ServiceConfiguration: "someconfiguration",
				},
			},
			ServiceChains: dpuservicev1.ServiceChains{
				Switches: []dpuservicev1.DPUDeploymentSwitch{
					{
						Ports: []dpuservicev1.DPUDeploymentPort{
							{
								Service: &dpuservicev1.DPUDeploymentService{
									InterfaceName: "someinterface",
									Name:          "someservice",
								},
							},
						},
					},
				},
			},
		},
	}
}

func createMinimalBFBWithStatus(name, namespace string) *provisioningv1.BFB {
	bfb := getMinimalBFB(name, namespace)
	Expect(testClient.Create(ctx, bfb)).To(Succeed())
	bfb.Status.Phase = provisioningv1.BFBReady
	bfb.Status.Versions = provisioningv1.BFBVersions{DOCA: "2.9.1"}
	bfb.SetGroupVersionKind(provisioningv1.BFBGroupVersionKind)
	bfb.SetManagedFields(nil)
	Expect(testClient.Status().Patch(ctx, bfb, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
	return bfb
}

func getMinimalBFB(name, namespace string) *provisioningv1.BFB {
	return &provisioningv1.BFB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: provisioningv1.BFBSpec{
			URL: fmt.Sprintf("http://somewebserver/%s.bfb", name),
		},
	}
}

func getMinimalDPUFlavor(namespace string) *provisioningv1.DPUFlavor {
	return &provisioningv1.DPUFlavor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "someflavor",
			Namespace: namespace,
		},
		Spec: provisioningv1.DPUFlavorSpec{
			// TODO: Remove those required fields from DPUFlavor
			Grub: provisioningv1.DPUFlavorGrub{
				KernelParameters: []string{},
			},
			Sysctl: provisioningv1.DPUFLavorSysctl{
				Parameters: []string{},
			},
		},
	}
}

func createMinimalDPUServiceTemplateWithStatus(namespace string) *dpuservicev1.DPUServiceTemplate {
	dpuServiceTemplate := getMinimalDPUServiceTemplate(namespace)
	Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
	patchDPUServiceTemplateWithStatus(dpuServiceTemplate)
	return dpuServiceTemplate
}

func patchDPUServiceTemplateWithStatus(dpuServiceTemplate *dpuservicev1.DPUServiceTemplate) {
	dpuServiceTemplate.Status.Conditions = []metav1.Condition{
		{
			Type:               string(conditions.TypeReady),
			Status:             metav1.ConditionTrue,
			Reason:             string(conditions.ReasonSuccess),
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
	}
	dpuServiceTemplate.Status.ObservedGeneration = dpuServiceTemplate.Generation
	dpuServiceTemplate.SetGroupVersionKind(dpuservicev1.DPUServiceTemplateGroupVersionKind)
	dpuServiceTemplate.SetManagedFields(nil)
	Expect(testClient.Status().Patch(ctx, dpuServiceTemplate, client.Apply, client.ForceOwnership, client.FieldOwner("test"))).To(Succeed())
}

func getMinimalDPUServiceTemplate(namespace string) *dpuservicev1.DPUServiceTemplate {
	return &dpuservicev1.DPUServiceTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sometemplate",
			Namespace: namespace,
		},
		Spec: dpuservicev1.DPUServiceTemplateSpec{
			DeploymentServiceName: "someservice",
			HelmChart: dpuservicev1.HelmChart{
				Source: dpuservicev1.ApplicationSource{
					RepoURL: "oci://someurl/repo",
					Path:    "somepath",
					Version: "someversion",
					Chart:   "somechart",
				},
			},
		},
	}
}

func getMinimalDPUServiceConfiguration(namespace string) *dpuservicev1.DPUServiceConfiguration {
	return &dpuservicev1.DPUServiceConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "someconfiguration",
			Namespace: namespace,
		},
		Spec: dpuservicev1.DPUServiceConfigurationSpec{
			DeploymentServiceName: "someservice",
			Interfaces: []dpuservicev1.ServiceInterfaceTemplate{
				{
					Name:    "someinterface",
					Network: "somenad",
				},
			},
		},
	}
}

func getDisruptiveDPUServiceConfiguration(namespace string) *dpuservicev1.DPUServiceConfiguration {
	config := getMinimalDPUServiceConfiguration(namespace)
	config.Spec.NodeEffect = ptr.To(true)
	return config
}

// cleanDPUDeploymentDerivatives removes all the objects that a DPUDeployment creates in a particular namespace
func cleanDPUDeploymentDerivatives(namespace string) {
	By("Ensuring DPUSets, DPUServiceChains, DPUServiceInterfaces and DPUServices are deleted")
	dpuSetList := &provisioningv1.DPUSetList{}
	Expect(testClient.List(ctx, dpuSetList, client.InNamespace(namespace))).To(Succeed())
	objs := []client.Object{}
	for i := range dpuSetList.Items {
		objs = append(objs, &dpuSetList.Items[i])
	}
	dpuServiceChainList := &dpuservicev1.DPUServiceChainList{}
	Expect(testClient.List(ctx, dpuServiceChainList, client.InNamespace(namespace))).To(Succeed())
	for i := range dpuServiceChainList.Items {
		objs = append(objs, &dpuServiceChainList.Items[i])
	}
	dpuServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
	Expect(testClient.List(ctx, dpuServiceInterfaceList, client.InNamespace(namespace))).To(Succeed())
	for i := range dpuServiceInterfaceList.Items {
		objs = append(objs, &dpuServiceInterfaceList.Items[i])
	}
	dpuServiceList := &dpuservicev1.DPUServiceList{}
	Expect(testClient.List(ctx, dpuServiceList, client.InNamespace(namespace))).To(Succeed())
	for i := range dpuServiceList.Items {
		objs = append(objs, &dpuServiceList.Items[i])
	}

	Eventually(func(g Gomega) {
		g.Expect(testutils.CleanupAndWait(ctx, testClient, objs...)).To(Succeed())
	}).WithTimeout(180 * time.Second).Should(Succeed())
}

// createReconcileDPUServicesNonDisruptiveDependencies creates 2 sets of dependencies that are used for the majority of the
// reconcileDPUSets tests
func createReconcileDPUServicesNonDisruptiveDependencies(namespace string) (string, string) {
	dpuServiceConfiguration, dpuServiceTemplate := createDPUServicesDependencies(namespace, "service-1", false)
	versionDigest1 := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)

	dpuServiceConfiguration, dpuServiceTemplate = createDPUServicesDependencies(namespace, "service-2", false)
	versionDigest2 := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)

	return versionDigest1, versionDigest2
}

// createReconcileDPUServicesDisruptiveDependencies creates 2 sets of dependencies that are used for the majority of the
// reconcileDPUSets tests
func createReconcileDPUServicesDisruptiveDependencies(namespace string) (string, string) {
	dpuServiceConfiguration, dpuServiceTemplate := createDPUServicesDependencies(namespace, "service-1", true)
	versionDigest1 := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)

	dpuServiceConfiguration, dpuServiceTemplate = createDPUServicesDependencies(namespace, "service-2", true)
	versionDigest2 := calculateDPUServiceVersionDigest(dpuServiceConfiguration, dpuServiceTemplate)

	return versionDigest1, versionDigest2
}

func createDPUServicesDependencies(namespace, name string, disruptive bool) (*dpuservicev1.DPUServiceConfiguration, *dpuservicev1.DPUServiceTemplate) {
	dpuServiceConfiguration := getMinimalDPUServiceConfiguration(namespace)
	if disruptive {
		dpuServiceConfiguration = getDisruptiveDPUServiceConfiguration(namespace)
	}
	dpuServiceConfiguration.Name = name
	dpuServiceConfiguration.Spec.DeploymentServiceName = name
	Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
	DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

	dpuServiceTemplate := getMinimalDPUServiceTemplate(namespace)
	dpuServiceTemplate.Name = name
	dpuServiceTemplate.Spec.DeploymentServiceName = name
	Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
	DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
	patchDPUServiceTemplateWithStatus(dpuServiceTemplate)

	return dpuServiceConfiguration, dpuServiceTemplate
}

func calculateVersionDigest(name, namespace string) string {
	config := &dpuservicev1.DPUServiceConfiguration{}
	Expect(testClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, config)).To(Succeed())
	template := &dpuservicev1.DPUServiceTemplate{}
	Expect(testClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, template)).To(Succeed())
	versionDigest := calculateDPUServiceVersionDigest(config, template)
	return versionDigest
}

func getDPUServiceChainList() *dpuservicev1.DPUServiceChainList {
	dpuServiceChainList := &dpuservicev1.DPUServiceChainList{}
	Expect(testClient.List(ctx, dpuServiceChainList)).To(Succeed())

	return dpuServiceChainList
}

func getDPUServiceList() *dpuservicev1.DPUServiceList {
	dpuServiceList := &dpuservicev1.DPUServiceList{}
	Expect(testClient.List(ctx, dpuServiceList)).To(Succeed())

	return dpuServiceList
}
