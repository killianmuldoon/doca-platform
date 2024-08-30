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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
