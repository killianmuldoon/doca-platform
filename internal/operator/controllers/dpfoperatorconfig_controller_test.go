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

package controller

import (
	"fmt"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"
	testutils "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//nolint:dupl
var _ = Describe("DPFOperatorConfig Controller", func() {
	Context("When reconciling a resource", func() {
		var testNS *corev1.Namespace
		BeforeEach(func() {
			By("Creating the namespaces")
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
		})
		AfterEach(func() {
			By("Cleaning up the Namespace")
			Expect(testClient.Delete(ctx, testNS)).To(Succeed())
		})
		It("should successfully reconcile the DPFOperatorConfig", func() {
			By("Reconciling the created resource")

			dpfOperatorConfig := getMinimalDPFOperatorConfig(testNS.Name)
			Expect(testClient.Create(ctx, dpfOperatorConfig)).To(Succeed())
			// DPF Operator creates objects when reconciling the DPFOperatorConfig and we need to ensure that on
			// deletion of these objects there is no DPFOperatorConfig in the cluster to trigger recreation of those.
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, dpfOperatorConfig)

			Eventually(func(g Gomega) []string {
				gotConfig := &operatorv1.DPFOperatorConfig{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dpfOperatorConfig), gotConfig)).To(Succeed())
				return gotConfig.Finalizers
			}).WithTimeout(30 * time.Second).Should(ConsistOf([]string{operatorv1.DPFOperatorConfigFinalizer}))
		})
	})
})

var _ = Describe("DPFOperator controller settings", func() {
	Context("When setting up the controller", func() {
		It("Should restrict reconciliation to a specific namespace and name when ConfigSingletonNamespaceName is set", func() {
			s := scheme.Scheme
			Expect(operatorv1.AddToScheme(scheme.Scheme)).To(Succeed())
			singletonReconciler := &DPFOperatorConfigReconciler{
				Client: testClient,
				Scheme: s,
				Settings: &DPFOperatorConfigReconcilerSettings{
					ConfigSingletonNamespaceName: &types.NamespacedName{
						Namespace: "one-namespace",
						Name:      "one-name",
					},
				},
			}
			_, err := singletonReconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "one-namespace",
					Name:      "one-name",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = singletonReconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "different-namespace",
					Name:      "different-name",
				},
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("only one object"))
		})
		It("Should allow reconciliation in any namespace and name when ConfigSingletonNamespaceName is unset", func() {
			s := scheme.Scheme
			Expect(operatorv1.AddToScheme(scheme.Scheme)).To(Succeed())
			unrestrictedReconciler := &DPFOperatorConfigReconciler{
				Client: testClient,
				Scheme: s,
				Settings: &DPFOperatorConfigReconcilerSettings{
					ConfigSingletonNamespaceName: nil,
				},
			}
			_, err := unrestrictedReconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "one-namespace",
					Name:      "one-name",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = unrestrictedReconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "different-namespace",
					Name:      "different-name",
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

// getMinimalDPFOperatorConfig returns a minimal DPFOperatorConfig
func getMinimalDPFOperatorConfig(namespace string) *operatorv1.DPFOperatorConfig {
	return &operatorv1.DPFOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: namespace,
		},
		Spec: operatorv1.DPFOperatorConfigSpec{
			HostNetworkConfiguration: operatorv1.HostNetworkConfiguration{
				Hosts: []operatorv1.Host{},
			},
			ProvisioningConfiguration: operatorv1.ProvisioningConfiguration{
				BFBPersistentVolumeClaimName: "foo-pvc",
				ImagePullSecret:              "foo-image-pull-secret",
			},
		},
	}
}

var _ = Describe("DPFOperatorConfig Controller - Reconcile System Components", func() {
	Context("controller should create DPF System components", func() {
		var testNS *corev1.Namespace
		BeforeEach(func() {
			testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
		})
		It("reconciles the DPFOperatorConfig and deploys the system components", func() {
			By("creating the DPFOperatorConfig")
			config := getMinimalDPFOperatorConfig(testNS.Name)
			config.Spec.ProvisioningConfiguration.BFBPersistentVolumeClaimName = "foo-pvc"
			config.Spec.ProvisioningConfiguration.ImagePullSecret = "foo-image-pull-secret"
			DeferCleanup(testutils.CleanupAndWait, ctx, testClient, config)
			Expect(testClient.Create(ctx, config)).To(Succeed())

			By("checking the dpuservice-controller-manager deployment is created")
			waitForDeployment(config.Namespace, "dpuservice-controller-manager")

			By("checking the dpf-provisioning-controller deployment is created and configured")
			// TODO: We should set the namespace for this component in the same way.
			deployment := waitForDeployment(config.Namespace, "dpf-provisioning-controller-manager")
			verifyPVC(deployment, "foo-pvc")

			// Check the system components deployed as DPUServices are ready.
			waitForDPUService(config.Namespace, "servicefunctionchainset-controller")
			waitForDPUService(config.Namespace, "multus")
			waitForDPUService(config.Namespace, "sriov-device-plugin")
			waitForDPUService(config.Namespace, "flannel")
			waitForDPUService(config.Namespace, "nvidia-k8s-ipam")
		})
	})
})

func waitForDPUService(ns, name string) {
	By(fmt.Sprintf("checking %s dpuservice is created and correctly configured", name))
	dpuservice := &dpuservicev1.DPUService{}
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKey{
			Namespace: ns,
			Name:      name},
			dpuservice)).To(Succeed())
	}).WithTimeout(30 * time.Second).Should(Succeed())
}

func waitForDeployment(ns, name string) *appsv1.Deployment {
	deployment := &appsv1.Deployment{}
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKey{
			Namespace: ns,
			Name:      name},
			deployment)).To(Succeed())
	}).WithTimeout(30 * time.Second).Should(Succeed())
	return deployment
}

func verifyPVC(deployment *appsv1.Deployment, expected string) {
	var bfbPvc *corev1.PersistentVolumeClaimVolumeSource
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == "bfb-volume" && vol.PersistentVolumeClaim != nil {
			bfbPvc = vol.PersistentVolumeClaim
			break
		}
	}
	if bfbPvc == nil {
		Fail("no pvc volume found")
	}
	Expect(bfbPvc.ClaimName).To(Equal(expected))
}
