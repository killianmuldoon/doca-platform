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
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils"

	"github.com/fluxcd/pkg/runtime/patch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

				DeferCleanup(func() {
					dpuSet := &provisioningv1.DpuSet{}
					Expect(testClient.DeleteAllOf(ctx, dpuSet, client.InNamespace(testNS.Name))).To(Succeed())
					Eventually(func(g Gomega) {
						dpuSetList := &provisioningv1.DpuSetList{}
						g.Expect(testClient.List(ctx, &provisioningv1.DpuSetList{})).To(Succeed())
						g.Expect(dpuSetList.Items).To(BeEmpty())
					}).WithTimeout(5 * time.Second).Should(Succeed())
				})
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
