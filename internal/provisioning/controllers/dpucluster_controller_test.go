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

package provisioning_controller

import (
	"context"
	"time"

	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These tests are written in BDD-style using Ginkgo framework. Refer to
// http://onsi.github.io/ginkgo to learn more.
var _ = Describe("DpuCluster", func() {

	const (
		DefaultNS   = "dpf-provisioning-test"
		DefaultNode = "dpf-provisioning-node-test"
	)

	var (
		testNS   *corev1.Namespace
		testNode *corev1.Node
	)

	var getObjKey = func(obj *provisioningv1.DPUCluster) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *provisioningv1.DPUCluster {
		return &provisioningv1.DPUCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS.Name,
			},
			Spec:   provisioningv1.DPUClusterSpec{},
			Status: provisioningv1.DPUClusterStatus{},
		}
	}

	var createNode = func(ctx context.Context, name string, labels map[string]string) *corev1.Node {
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
		Expect(k8sClient.Create(ctx, node)).NotTo(HaveOccurred())
		return node
	}

	BeforeEach(func() {
		By("creating the namespace")
		testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: DefaultNS}}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, testNS))).To(Succeed())

		By("creating the node")
		testNode = createNode(ctx, DefaultNode, make(map[string]string))
	})

	AfterEach(func() {
		By("deleting the namespace")
		Expect(k8sClient.Delete(ctx, testNS)).To(Succeed())

		By("Cleaning the node")
		Expect(k8sClient.Delete(ctx, testNode)).To(Succeed())
	})

	Context("obj test context", func() {
		ctx := context.Background()

		It("create and destroy", func() {
			By("creating the obj")
			obj := createObj("obj-dpucluster")
			obj.Spec.Type = "static"
			obj.Spec.MaxNodes = 10
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &provisioningv1.DPUCluster{}

			By("checking the finalizer")
			Eventually(func(g Gomega) []string {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Finalizers
			}).WithTimeout(10 * time.Second).Should(ConsistOf([]string{provisioningv1.FinalizerInternalCleanUp}))
			time.Sleep(10 * time.Second)
		})

		It("create from yaml", func() {
			yml := []byte(`
apiVersion: provisioning.dpf.nvidia.com/v1alpha1
kind: DPUCluster
metadata:
  name: dpucluster-1
  namespace: default
spec:
  maxNodes: 10
  version: v1.31.0
  type: static
  clusterEndpoint:
    keepalived:
      vip: 10.10.10.10
`)
			obj := &provisioningv1.DPUCluster{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &provisioningv1.DPUCluster{}

			By("checking the finalizer")
			Eventually(func(g Gomega) []string {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), obj_fetched)).To(Succeed())
				return obj_fetched.Finalizers
			}).WithTimeout(10 * time.Second).Should(ConsistOf([]string{provisioningv1.FinalizerInternalCleanUp}))
		})
	})
})
