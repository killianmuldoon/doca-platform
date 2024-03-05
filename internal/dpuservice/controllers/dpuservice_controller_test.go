/*
Copyright 2024.

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
	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("DPUService Controller", func() {
	Context("When reconciling a resource", func() {
		testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}
		dpuServiceName := "dpu-one"

		dpuServiceKey := types.NamespacedName{}
		dpuService := &dpuservicev1.DPUService{}

		BeforeEach(func() {
			By("creating the namespace")
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			dpuServiceKey = types.NamespacedName{Name: dpuServiceName, Namespace: testNS.Name}
			dpuService = &dpuservicev1.DPUService{
				ObjectMeta: metav1.ObjectMeta{Name: dpuServiceName, Namespace: testNS.Name},
			}
			By("creating the DPUService")
			Expect(testClient.Create(ctx, dpuService)).To(Succeed())
		})
		AfterEach(func() {
			By("Cleanup the DPUService and Namespace")
			Expect(testClient.Delete(ctx, dpuService)).To(Succeed())
			Expect(testClient.Delete(ctx, testNS)).To(Succeed())
		})
		It("should successfully add the DPUService finalizer", func() {
			By("Reconciling the created resource for the first time")
			controllerReconciler := &DPUServiceReconciler{
				Client: testClient,
				Scheme: testClient.Scheme(),
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: dpuServiceKey,
			})
			Expect(err).NotTo(HaveOccurred())
			gotDPUService := &dpuservicev1.DPUService{}
			Expect(testClient.Get(ctx, dpuServiceKey, gotDPUService)).To(Succeed())
			Expect(gotDPUService.GetFinalizers()).To(ConsistOf(dpuservicev1.DPUServiceFinalizer))
		})
	})
})

var _ = Describe("getClusters", func() {
	Context("When reconciling a resource", func() {
		testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "testns-"}}

		var clusters []types.NamespacedName
		BeforeEach(func() {
			By("creating the namespace")
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			By("creating the Secrets")
			clusters = []types.NamespacedName{
				{Namespace: testNS.Name, Name: "cluster-one"},
				{Namespace: testNS.Name, Name: "cluster-two"},
				{Namespace: testNS.Name, Name: "cluster-three"},
			}
			secrets := []*corev1.Secret{
				testKamajiClusterSecret(clusters[0]),
				testKamajiClusterSecret(clusters[1]),
				testKamajiClusterSecret(clusters[2]),
			}
			for _, s := range secrets {
				Expect(testClient.Create(ctx, s)).To(Succeed())
			}
		})
		AfterEach(func() {
			By("Cleanup the test Namespace")
			Expect(testClient.Delete(ctx, testNS)).To(Succeed())
		})
		It("should list the clusters referenced by admin-kubeconfig secrets", func() {
			clusters, err := getClusters(ctx, testClient)
			Expect(err).NotTo(HaveOccurred())
			Expect(clusters).To(ConsistOf(clusters))
		})
	})
})

func testKamajiClusterSecret(cluster types.NamespacedName) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "cluster-",
			Namespace:    cluster.Namespace,
			Labels: map[string]string{
				"kamaji.clastix.io/name":      cluster.Name,
				"kamaji.clastix.io/component": "admin-kubeconfig",
				"kamaji.clastix.io/project":   "kamaji",
			},
		},
	}
}
