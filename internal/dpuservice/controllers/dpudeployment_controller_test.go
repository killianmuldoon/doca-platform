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

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"
	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/argocd/api/application/v1alpha1"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils"

	"github.com/fluxcd/pkg/runtime/patch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//nolint:goconst
var _ = Describe("DPUDeployment Controller", func() {
	Context("When reconciling a resource", func() {
		var testNS *corev1.Namespace
		BeforeEach(func() {
			By("Creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			DeferCleanup(testClient.Delete, ctx, testNS)
		})
		It("should successfully reconcile the DPUDeployment", func() {
			By("Reconciling the created resource")
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			By("checking that finalizer is added")
			Eventually(func(g Gomega) []string {
				got := &dpuservicev1.DPUDeployment{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpuDeployment), got)).To(Succeed())
				return got.Finalizers
			}).WithTimeout(10 * time.Second).Should(ConsistOf([]string{dpuservicev1.DPUDeploymentFinalizer}))

			By("checking that the resource can be deleted (finalizer is removed)")
			Expect(testutils.CleanupAndWait(ctx, testClient, dpuDeployment)).To(Succeed())
		})
		It("should not create DPUSet, DPUService and DPUServiceChain if any of the dependencies does not exist", func() {
			By("Reconciling the created resource")
			dpuDeployment := getMinimalDPUDeployment(testNS.Name)
			Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

			By("checking that no object is created")
			Consistently(func(g Gomega) {
				gotDPUSetList := &provisioningv1.DpuSetList{}
				g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
				g.Expect(gotDPUSetList.Items).To(BeEmpty())

				gotDPUServiceList := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
				g.Expect(gotDPUServiceList.Items).To(BeEmpty())

				gotDPUServiceChainList := &sfcv1.DPUServiceChainList{}
				g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
				g.Expect(gotDPUServiceChainList.Items).To(BeEmpty())
			}).WithTimeout(5 * time.Second).Should(Succeed())
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
				bfb := getMinimalBFB(testNS.Name)
				Expect(testClient.Create(ctx, bfb)).To(Succeed())
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
		})
		Context("When checking reconcileDPUSets()", func() {
			var initialDPUSetSettings []dpuservicev1.DPUSet
			var expectedDPUSetSpecs []provisioningv1.DpuSetSpec
			BeforeEach(func() {
				initialDPUSetSettings = []dpuservicev1.DPUSet{
					{
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

				expectedDPUSetSpecs = []provisioningv1.DpuSetSpec{
					{
						NodeSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"nodekey1": "nodevalue1",
							},
						},
						DpuSelector: map[string]string{
							"dpukey1": "dpuvalue1",
						},
						Strategy: &provisioningv1.DpuSetStrategy{
							Type: provisioningv1.RollingUpdateStrategyType,
						},
						DpuTemplate: provisioningv1.DpuTemplate{
							Annotations: map[string]string{
								"annotationkey1": "annotationvalue1",
							},
							Spec: provisioningv1.DPUSpec{
								Bfb: provisioningv1.BFBSpec{
									BFBName: "somebfb",
								},
								DPUFlavor: "someflavor",
							},
						},
					},
					{
						NodeSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"nodekey2": "nodevalue2",
							},
						},
						DpuSelector: map[string]string{
							"dpukey2": "dpuvalue2",
						},
						Strategy: &provisioningv1.DpuSetStrategy{
							Type: provisioningv1.RollingUpdateStrategyType,
						},
						DpuTemplate: provisioningv1.DpuTemplate{
							Annotations: map[string]string{
								"annotationkey2": "annotationvalue2",
							},
							Spec: provisioningv1.DPUSpec{
								Bfb: provisioningv1.BFBSpec{
									BFBName: "somebfb",
								},
								DPUFlavor: "someflavor",
							},
						},
					},
				}

				By("Creating the dependencies")
				bfb := getMinimalBFB(testNS.Name)
				Expect(testClient.Create(ctx, bfb)).To(Succeed())
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

				DeferCleanup(cleanDPUDeploymentDerivatives, testNS.Name)
			})
			It("should create the correct DPUSets", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("checking that correct DPUSets are created")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DpuSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("dpf.nvidia.com/dpudeployment-name", "dpudeployment"))
						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]provisioningv1.DpuSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(5 * time.Second).Should(Succeed())
			})
			It("should update the existing DPUSets on update of the .spec.dpus in the DPUDeployment", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)

				By("waiting for the initial DPUSets to be applied")
				firstDPUSetUIDs := make([]types.UID, 0, 2)
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DpuSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
					for _, dpuSet := range gotDPUSetList.Items {
						firstDPUSetUIDs = append(firstDPUSetUIDs, dpuSet.UID)
					}
				}).WithTimeout(5 * time.Second).Should(Succeed())

				By("modifying the DPUDeployment object and checking the outcome")
				dpuDeployment.Spec.DPUs.DPUSets[1].DPUAnnotations["newkey"] = "newvalue"
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())
				By("checking that correct DPUSets are created")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DpuSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("dpf.nvidia.com/dpudeployment-name", "dpudeployment"))
						// Validate that the same object is updated
						g.Expect(firstDPUSetUIDs).To(ContainElement(dpuSet.UID))

						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]provisioningv1.DpuSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					expectedDPUSetSpecs[1].DpuTemplate.Annotations["newkey"] = "newvalue"
					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(5 * time.Second).Should(Succeed())
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
					gotDPUSetList := &provisioningv1.DpuSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
					for _, dpuSet := range gotDPUSetList.Items {
						firstDPUSetUIDs = append(firstDPUSetUIDs, dpuSet.UID)
					}
				}).WithTimeout(5 * time.Second).Should(Succeed())

				By("modifying the DPUDeployment object and checking the outcome")
				dpuDeployment.Spec.DPUs.DPUSets = dpuDeployment.Spec.DPUs.DPUSets[1:]
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())
				By("checking that correct DPUSets are created")
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DpuSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(1))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("dpf.nvidia.com/dpudeployment-name", "dpudeployment"))
						// Validate that the object was not recreated
						g.Expect(firstDPUSetUIDs).To(ContainElement(dpuSet.UID))

						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					By("checking the specs")
					specs := make([]provisioningv1.DpuSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					expectedDPUSetSpecs = expectedDPUSetSpecs[1:]
					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(5 * time.Second).Should(Succeed())
			})
			It("should update existing and create new DPUSets on update of the .spec.dpus in the DPUDeployment", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.DPUs.DPUSets = initialDPUSetSettings
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)
				patcher := patch.NewSerialPatcher(dpuDeployment, testClient)

				By("waiting for the initial DPUSets to be applied")
				firstDPUSetUIDs := make(map[types.UID]interface{})
				Eventually(func(g Gomega) {
					gotDPUSetList := &provisioningv1.DpuSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(2))
					for _, dpuSet := range gotDPUSetList.Items {
						firstDPUSetUIDs[dpuSet.UID] = struct{}{}
					}
				}).WithTimeout(5 * time.Second).Should(Succeed())

				By("modifying the DPUDeployment object and checking the outcome")
				dpuDeployment.Spec.DPUs.DPUSets[1].DPUAnnotations["newkey"] = "newvalue"
				dpuDeployment.Spec.DPUs.DPUSets = append(dpuDeployment.Spec.DPUs.DPUSets, dpuservicev1.DPUSet{
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
					gotDPUSetList := &provisioningv1.DpuSetList{}
					g.Expect(testClient.List(ctx, gotDPUSetList)).To(Succeed())
					g.Expect(gotDPUSetList.Items).To(HaveLen(3))

					By("checking the object metadata")
					for _, dpuSet := range gotDPUSetList.Items {
						g.Expect(dpuSet.Labels).To(HaveLen(1))
						g.Expect(dpuSet.Labels).To(HaveKeyWithValue("dpf.nvidia.com/dpudeployment-name", "dpudeployment"))

						delete(firstDPUSetUIDs, dpuSet.UID)

						g.Expect(dpuSet.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
					}

					// Validate that all original objects are there and not recreated
					g.Expect(firstDPUSetUIDs).To(BeEmpty())

					By("checking the specs")
					specs := make([]provisioningv1.DpuSetSpec, 0, len(gotDPUSetList.Items))
					for _, dpuSet := range gotDPUSetList.Items {
						specs = append(specs, dpuSet.Spec)
					}
					expectedDPUSetSpecs[1].DpuTemplate.Annotations["newkey"] = "newvalue"
					expectedDPUSetSpecs = append(expectedDPUSetSpecs, provisioningv1.DpuSetSpec{
						NodeSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"nodekey3": "nodevalue3",
							},
						},
						DpuSelector: map[string]string{
							"dpukey3": "dpuvalue3",
						},
						Strategy: &provisioningv1.DpuSetStrategy{
							Type: provisioningv1.RollingUpdateStrategyType,
						},
						DpuTemplate: provisioningv1.DpuTemplate{
							Annotations: map[string]string{
								"annotationkey3": "annotationvalue3",
							},
							Spec: provisioningv1.DPUSpec{
								Bfb: provisioningv1.BFBSpec{
									BFBName: "somebfb",
								},
								DPUFlavor: "someflavor",
							},
						},
					})

					g.Expect(specs).To(ConsistOf(expectedDPUSetSpecs))
				}).WithTimeout(5 * time.Second).Should(Succeed())

			})
		})
		Context("When checking reconcileDPUServices()", func() {
			BeforeEach(func() {
				By("Creating the dependencies")
				bfb := getMinimalBFB(testNS.Name)
				Expect(testClient.Create(ctx, bfb)).To(Succeed())
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
				dpuServiceConfiguration.Spec.Service = "service-1"
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations = make(map[string]string)
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations["annkey1"] = "annval1"
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels = make(map[string]string)
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels["labelkey1"] = "labelval1"
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.UpdateStrategy = &appsv1.DaemonSetUpdateStrategy{Type: appsv1.OnDeleteDaemonSetStrategyType}
				dpuServiceConfiguration.Spec.ServiceConfiguration.DeployInCluster = ptr.To[bool](true)
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceConfiguration = getMinimalDPUServiceConfiguration(testNS.Name)
				dpuServiceConfiguration.Name = "service-2"
				dpuServiceConfiguration.Spec.Service = "service-2"
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations = make(map[string]string)
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations["annkey2"] = "annval2"
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels = make(map[string]string)
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.Labels["labelkey2"] = "labelval2"
				dpuServiceConfiguration.Spec.ServiceConfiguration.ServiceDaemonSet.UpdateStrategy = &appsv1.DaemonSetUpdateStrategy{Type: appsv1.RollingUpdateDaemonSetStrategyType}
				Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

				dpuServiceTemplate := getMinimalDPUServiceTemplate(testNS.Name)
				dpuServiceTemplate.Name = "service-1"
				dpuServiceTemplate.Spec.Service = "service-1"
				dpuServiceTemplate.Spec.ServiceDaemonSet.Resources = corev1.ResourceList{"cpu": resource.MustParse("1"), "mem": resource.MustParse("1Gi")}
				Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

				dpuServiceTemplate = getMinimalDPUServiceTemplate(testNS.Name)
				dpuServiceTemplate.Name = "service-2"
				dpuServiceTemplate.Spec.Service = "service-2"
				dpuServiceTemplate.Spec.ServiceDaemonSet.Resources = corev1.ResourceList{"cpu": resource.MustParse("2"), "mem": resource.MustParse("2Gi")}
				Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

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

				By("checking that correct DPUServices are created")
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))

					By("checking the object metadata")
					for _, dpuService := range gotDPUServiceList.Items {
						g.Expect(dpuService.Labels).To(HaveLen(1))
						g.Expect(dpuService.Labels).To(HaveKeyWithValue("dpf.nvidia.com/dpudeployment-name", "dpudeployment"))
						g.Expect(dpuService.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))
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
									RepoURL: "someurl",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID: ptr.To[string]("service-1"),
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								Labels:         map[string]string{"labelkey1": "labelval1"},
								Annotations:    map[string]string{"annkey1": "annval1"},
								UpdateStrategy: &appsv1.DaemonSetUpdateStrategy{Type: appsv1.OnDeleteDaemonSetStrategyType},
								Resources:      corev1.ResourceList{"cpu": resource.MustParse("1"), "mem": resource.MustParse("1Gi")},
							},
							DeployInCluster: ptr.To[bool](true),
						},
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "someurl",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID: ptr.To[string]("service-2"),
							ServiceDaemonSet: &dpuservicev1.ServiceDaemonSetValues{
								Labels:      map[string]string{"labelkey2": "labelval2"},
								Annotations: map[string]string{"annkey2": "annval2"},
								UpdateStrategy: &appsv1.DaemonSetUpdateStrategy{
									Type: appsv1.RollingUpdateDaemonSetStrategyType,
								},
								Resources: corev1.ResourceList{"cpu": resource.MustParse("2"), "mem": resource.MustParse("2Gi")},
							},
						},
					}))
				}).WithTimeout(5 * time.Second).Should(Succeed())
			})
			It("should update the existing DPUService on update of the DPUServiceConfiguration", func() {
				By("Creating the dependencies")
				createReconcileDPUServicesDependencies(testNS.Name)

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
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))
				}).WithTimeout(5 * time.Second).Should(Succeed())

				By("modifying the DPUServiceConfiguration object and checking the outcome")
				dpuServiceConfiguration := &dpuservicev1.DPUServiceConfiguration{}
				Expect(testClient.Get(ctx, types.NamespacedName{Namespace: testNS.Name, Name: "service-2"}, dpuServiceConfiguration)).To(Succeed())
				dpuServiceConfiguration.Spec.ServiceConfiguration.DeployInCluster = ptr.To[bool](true)
				dpuServiceConfiguration.SetManagedFields(nil)
				dpuServiceConfiguration.SetGroupVersionKind(dpuservicev1.DPUServiceConfigurationGroupVersionKind)
				Expect(testClient.Patch(ctx, dpuServiceConfiguration, client.Apply, client.FieldOwner(dpuDeploymentControllerName))).To(Succeed())

				By("checking that the DPUService is updated as expected")
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceSpec, 0, len(gotDPUServiceList.Items))
					for _, dpuService := range gotDPUServiceList.Items {
						specs = append(specs, dpuService.Spec)
					}
					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceSpec{
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "someurl",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID: ptr.To[string]("service-1"),
						},
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "someurl",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID:       ptr.To[string]("service-2"),
							DeployInCluster: ptr.To[bool](true),
						},
					}))
				}).WithTimeout(5 * time.Second).Should(Succeed())
			})
			It("should delete DPUServices that are no longer part of the DPUDeployment", func() {
				By("Creating the dependencies")
				createReconcileDPUServicesDependencies(testNS.Name)

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
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))
				}).WithTimeout(5 * time.Second).Should(Succeed())

				By("modifying the DPUDeployment object and checking the outcome")
				delete(dpuDeployment.Spec.Services, "service-1")
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())

				By("checking that only one DPUService exists")
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(1))

					By("checking the spec")
					g.Expect(gotDPUServiceList.Items[0].Spec).To(BeComparableTo(dpuservicev1.DPUServiceSpec{
						HelmChart: dpuservicev1.HelmChart{
							Source: dpuservicev1.ApplicationSource{
								RepoURL: "someurl",
								Path:    "somepath",
								Version: "someversion",
								Chart:   "somechart",
							},
						},
						ServiceID: ptr.To[string]("service-2"),
					}))
				}).WithTimeout(5 * time.Second).Should(Succeed())
			})
			It("should create new DPUServices on update of the .spec.services in the DPUDeployment", func() {
				By("Creating the dependencies")
				createReconcileDPUServicesDependencies(testNS.Name)

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
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(1))
				}).WithTimeout(5 * time.Second).Should(Succeed())

				By("modifying the DPUDeployment object and checking the outcome")
				dpuDeployment.Spec.Services["service-2"] = dpuservicev1.DPUDeploymentServiceConfiguration{
					ServiceTemplate:      "service-2",
					ServiceConfiguration: "service-2",
				}
				Expect(patcher.Patch(ctx, dpuDeployment, patch.WithFieldOwner(dpuDeploymentControllerName))).To(Succeed())

				By("checking that two DPUServices exist")
				Eventually(func(g Gomega) {
					gotDPUServiceList := &dpuservicev1.DPUServiceList{}
					g.Expect(testClient.List(ctx, gotDPUServiceList)).To(Succeed())
					g.Expect(gotDPUServiceList.Items).To(HaveLen(2))

					By("checking the specs")
					specs := make([]dpuservicev1.DPUServiceSpec, 0, len(gotDPUServiceList.Items))
					for _, dpuService := range gotDPUServiceList.Items {
						specs = append(specs, dpuService.Spec)
					}

					g.Expect(specs).To(ConsistOf([]dpuservicev1.DPUServiceSpec{
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "someurl",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID: ptr.To[string]("service-1"),
						},
						{
							HelmChart: dpuservicev1.HelmChart{
								Source: dpuservicev1.ApplicationSource{
									RepoURL: "someurl",
									Path:    "somepath",
									Version: "someversion",
									Chart:   "somechart",
								},
							},
							ServiceID: ptr.To[string]("service-2"),
						},
					}))
				}).WithTimeout(5 * time.Second).Should(Succeed())
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
				Entry("requested resources fit leaving buffer", &dpuDeploymentDependencies{
					DPUFlavor: &provisioningv1.DPUFlavor{
						Spec: provisioningv1.DPUFlavorSpec{
							DPUDeploymentResources: corev1.ResourceList{
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
							DPUDeploymentResources: corev1.ResourceList{
								"cpu":    resource.MustParse("2"),
								"memory": resource.MustParse("2Gi"),
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
							DPUDeploymentResources: corev1.ResourceList{
								"cpu":    resource.MustParse("1"),
								"memory": resource.MustParse("2Gi"),
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
							DPUDeploymentResources: corev1.ResourceList{
								"cpu": resource.MustParse("1"),
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

		Context("When checking reconcileDPUServiceChains()", func() {
			BeforeEach(func() {
				By("Creating the dependencies")
				bfb := getMinimalBFB(testNS.Name)
				Expect(testClient.Create(ctx, bfb)).To(Succeed())
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

				DeferCleanup(cleanDPUDeploymentDerivatives, testNS.Name)
			})
			It("should create the correct DPUServiceChain", func() {
				dpuDeployment := getMinimalDPUDeployment(testNS.Name)
				dpuDeployment.Spec.ServiceChains = []sfcv1.Switch{
					{
						Ports: []sfcv1.Port{
							{
								Service: &sfcv1.Service{
									InterfaceName: "someinterface",
									Reference: &sfcv1.ObjectRef{
										Name: "somedpuservice",
									},
								},
							},
						},
					},
					{
						Ports: []sfcv1.Port{
							{
								Service: &sfcv1.Service{
									InterfaceName: "someotherinterface",
									Reference: &sfcv1.ObjectRef{
										Name: "someotherservice",
									},
								},
							},
						},
					},
				}
				Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())
				DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuDeployment)

				By("checking that correct DPUServiceChain is created")
				Eventually(func(g Gomega) {
					gotDPUServiceChainList := &sfcv1.DPUServiceChainList{}
					g.Expect(testClient.List(ctx, gotDPUServiceChainList)).To(Succeed())
					g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))

					By("checking the object metadata")
					obj := gotDPUServiceChainList.Items[0]

					g.Expect(obj.Labels).To(HaveLen(1))
					g.Expect(obj.Labels).To(HaveKeyWithValue("dpf.nvidia.com/dpudeployment-name", "dpudeployment"))
					g.Expect(obj.OwnerReferences).To(ConsistOf(*metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)))

					By("checking the spec")
					g.Expect(obj.Spec).To(BeComparableTo(sfcv1.DPUServiceChainSpec{
						// TODO: Derive and add cluster selector
						Template: sfcv1.ServiceChainSetSpecTemplate{
							Spec: sfcv1.ServiceChainSetSpec{
								// TODO: Figure out what to do with NodeSelector
								Template: sfcv1.ServiceChainSpecTemplate{
									Spec: sfcv1.ServiceChainSpec{
										Switches: []sfcv1.Switch{
											{
												Ports: []sfcv1.Port{
													{
														Service: &sfcv1.Service{
															InterfaceName: "someinterface",
															Reference: &sfcv1.ObjectRef{
																Name: "somedpuservice",
															},
														},
													},
												},
											},
											{
												Ports: []sfcv1.Port{
													{
														Service: &sfcv1.Service{
															InterfaceName: "someotherinterface",
															Reference: &sfcv1.ObjectRef{
																Name: "someotherservice",
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
				}).WithTimeout(5 * time.Second).Should(Succeed())
			})
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
			ServiceChains: []sfcv1.Switch{
				{
					Ports: []sfcv1.Port{
						{
							Service: &sfcv1.Service{
								InterfaceName: "someinterface",
								Reference: &sfcv1.ObjectRef{
									Name: "somedpuservice",
								},
							},
						},
					},
				},
			},
		},
	}
}

func getMinimalBFB(namespace string) *provisioningv1.Bfb {
	return &provisioningv1.Bfb{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "somebfb",
			Namespace: namespace,
		},
		Spec: provisioningv1.BfbSpec{
			URL: "http://somewebserver/somebfb.bfb",
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

func getMinimalDPUServiceTemplate(namespace string) *dpuservicev1.DPUServiceTemplate {
	return &dpuservicev1.DPUServiceTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sometemplate",
			Namespace: namespace,
		},
		Spec: dpuservicev1.DPUServiceTemplateSpec{
			Service: "someservice",
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
			Service: "someservice",
			ServiceConfiguration: dpuservicev1.ServiceConfiguration{
				HelmChart: dpuservicev1.HelmChart{
					Source: dpuservicev1.ApplicationSource{
						RepoURL: "someurl",
						Path:    "somepath",
						Version: "someversion",
						Chart:   "somechart",
					},
				},
			},
		},
	}
}

// cleanDPUDeploymentDerivatives removes all the objects that a DPUDeployment creates in a particular namespace
func cleanDPUDeploymentDerivatives(namespace string) {
	By("Ensuring DPUSets are deleted")
	dpuSet := &provisioningv1.DpuSet{}
	Expect(testClient.DeleteAllOf(ctx, dpuSet, client.InNamespace(namespace))).To(Succeed())
	Eventually(func(g Gomega) {
		dpuSetList := &provisioningv1.DpuSetList{}
		g.Expect(testClient.List(ctx, dpuSetList)).To(Succeed())
		g.Expect(dpuSetList.Items).To(BeEmpty())
	}).WithTimeout(30 * time.Second).Should(Succeed())

	By("Ensuring DPUServices are deleted")
	dpuService := &dpuservicev1.DPUService{}
	Expect(testClient.DeleteAllOf(ctx, dpuService, client.InNamespace(namespace))).To(Succeed())

	// Ensure the applications are deleted.
	Eventually(func(g Gomega) {
		applications := &argov1.ApplicationList{}
		g.Expect(testClient.List(ctx, applications)).To(Succeed())
		// We're not running the ArgoCD controllers in this test so the finalizers must be removed here.
		// Do this in each loop as there's a race condition where the Application is patched again
		// by the DPUService controller.
		for i := range applications.Items {
			err := testClient.Patch(ctx, &applications.Items[i], client.RawPatch(types.MergePatchType, []byte(`{"metadata":{"finalizers":[]}}`)))
			if err != nil && !apierrors.IsNotFound(err) {
				g.Expect(err).ToNot(HaveOccurred())
			}
		}
		g.Expect(applications.Items).To(BeEmpty())
	}).WithTimeout(30 * time.Second).Should(Succeed())

	Eventually(func(g Gomega) {
		dpuServiceList := &dpuservicev1.DPUServiceList{}
		g.Expect(testClient.List(ctx, dpuServiceList)).To(Succeed())
		g.Expect(dpuServiceList.Items).To(BeEmpty())
	}).WithTimeout(30 * time.Second).Should(Succeed())

	By("Ensuring DPUServiceChains are deleted")
	dpuServiceChain := &sfcv1.DPUServiceChain{}
	Expect(testClient.DeleteAllOf(ctx, dpuServiceChain, client.InNamespace(namespace))).To(Succeed())
	Eventually(func(g Gomega) {
		dpuServiceChainList := &sfcv1.DPUServiceChainList{}
		g.Expect(testClient.List(ctx, dpuServiceChainList)).To(Succeed())
		g.Expect(dpuServiceChainList.Items).To(BeEmpty())
	}).WithTimeout(30 * time.Second).Should(Succeed())
}

// createReconcileDPUServicesDependencies creates 2 sets of dependencies that are used for the majority of the
// reconcileDPUSets tests
func createReconcileDPUServicesDependencies(namespace string) {
	dpuServiceConfiguration := getMinimalDPUServiceConfiguration(namespace)
	dpuServiceConfiguration.Name = "service-1"
	dpuServiceConfiguration.Spec.Service = "service-1"
	Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
	DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

	dpuServiceConfiguration = getMinimalDPUServiceConfiguration(namespace)
	dpuServiceConfiguration.Name = "service-2"
	dpuServiceConfiguration.Spec.Service = "service-2"
	Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())
	DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceConfiguration)

	dpuServiceTemplate := getMinimalDPUServiceTemplate(namespace)
	dpuServiceTemplate.Name = "service-1"
	dpuServiceTemplate.Spec.Service = "service-1"
	Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
	DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)

	dpuServiceTemplate = getMinimalDPUServiceTemplate(namespace)
	dpuServiceTemplate.Name = "service-2"
	dpuServiceTemplate.Spec.Service = "service-2"
	Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())
	DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpuServiceTemplate)
}
