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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
		testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
		It("checks the DPFOperatorConfig takes no action when paused", func() {
			// Create the namespace for the test.
			Expect(testClient.Create(ctx, testNS)).To(Succeed())

			// Create the DPF ImagePullSecrets
			secretOneName := "secret-one"
			secretTwoName := "secret-two"
			Expect(testClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretOneName, Namespace: testNS.Name}})).To(Succeed())
			Expect(testClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretTwoName, Namespace: testNS.Name}})).To(Succeed())

			By("creating the DPFOperatorConfig")
			config := getMinimalDPFOperatorConfig(testNS.Name)
			config.Spec.ProvisioningConfiguration.BFBPersistentVolumeClaimName = "foo-pvc"
			config.Spec.ProvisioningConfiguration.ImagePullSecret = "foo-image-pull-secret"
			config.Spec.ImagePullSecrets = []string{"secret-one", "secret-two"}
			config.Spec.Overrides = &operatorv1.Overrides{}
			config.Spec.Overrides.Paused = true
			Expect(testClient.Create(ctx, config)).To(Succeed())
			Consistently(func(g Gomega) {
				gotConfig := &operatorv1.DPFOperatorConfig{}
				// Expect the config finalizers to not have been reconciled.
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(config), gotConfig)).To(Succeed())
				g.Expect(gotConfig.Finalizers).To(BeEmpty())

				// Expect secrets to not have been labelled.
				secrets := &corev1.SecretList{}
				g.Expect(testClient.List(ctx, secrets, client.HasLabels{dpuservicev1.DPFImagePullSecretLabelKey})).To(Succeed())
				g.Expect(secrets.Items).To(BeEmpty())

				// Expect no DPUServices to have been created.
				dpuServices := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, dpuServices)).To(Succeed())
				g.Expect(dpuServices.Items).To(BeEmpty())
			}).WithTimeout(4 * time.Second).Should(Succeed())
		})
		It("reconciles the DPFOperatorConfig and deploys the system components when unpaused", func() {
			By("unpausing the DPFOperatorConfig")
			config := getMinimalDPFOperatorConfig(testNS.Name)

			// Patch the DPFOperatorConfig to remove `spec.paused`
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(config), config)).To(Succeed())
			patch := client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf("{\"spec\": {\"overrides\": {\"paused\":%t}}}", false)))
			Expect(testClient.Patch(ctx, config, patch)).To(Succeed())

			By("checking the DPFOperatorConfig finalizer is reconciled")
			Eventually(func(g Gomega) []string {
				gotConfig := &operatorv1.DPFOperatorConfig{}
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(config), gotConfig)).To(Succeed())
				return gotConfig.Finalizers
			}).WithTimeout(30 * time.Second).Should(ConsistOf([]string{operatorv1.DPFOperatorConfigFinalizer}))

			By("checking the DPF ImagePullSecrets are reconciled with the correct label")
			Eventually(func(g Gomega) {
				secrets := &corev1.SecretList{}
				g.Expect(testClient.List(ctx, secrets, client.HasLabels{dpuservicev1.DPFImagePullSecretLabelKey})).To(Succeed())
				g.Expect(secrets.Items).To(HaveLen(2))
			}).WithTimeout(30 * time.Second).Should(Succeed())

			By("checking the argocd application controller statefulSet is created")
			waitForStatefulSet(config.Namespace, "argocd-application-controller")

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
		It("delete the DPFOperatorConfig", func() {
			By("deleting the DPFOperatorConfig")
			config := getMinimalDPFOperatorConfig(testNS.Name)
			Expect(testClient.Delete(ctx, config)).To(Succeed())
			By("checking the DPFOperatorConfig is deleted")
			Eventually(func(g Gomega) {
				g.Expect(apierrors.IsNotFound(testClient.Get(ctx, client.ObjectKeyFromObject(config), config))).To(BeTrue())
			}).WithTimeout(30 * time.Second).Should(Succeed())
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

func waitForStatefulSet(ns, name string) *appsv1.StatefulSet {
	statefulSet := &appsv1.StatefulSet{}
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx,
			client.ObjectKey{
				Namespace: ns,
				Name:      name,
			}, statefulSet)).To(Succeed())
	}).WithTimeout(30 * time.Second).Should(Succeed())
	return statefulSet
}

func verifyPVC(deployment *appsv1.Deployment, expected string) {
	var bfbPVC *corev1.PersistentVolumeClaimVolumeSource
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == "bfb-volume" && vol.PersistentVolumeClaim != nil {
			bfbPVC = vol.PersistentVolumeClaim
			break
		}
	}
	if bfbPVC == nil {
		Fail("no pvc volume found")
	}
	Expect(bfbPVC.ClaimName).To(Equal(expected))
}
