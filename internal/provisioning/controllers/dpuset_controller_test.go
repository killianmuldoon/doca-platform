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
	"context"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DPUSet", func() {

	const (
		DefaultNS   = "dpf-provisioning-test"
		DefaultNode = "dpf-provisioning-node-test"
	)

	var (
		testNS   *corev1.Namespace
		testNode *corev1.Node
	)

	var getObjKey = func(obj *provisioningv1.DPUSet) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *provisioningv1.DPUSet {
		return &provisioningv1.DPUSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: testNS.Name,
			},
			Spec:   provisioningv1.DPUSetSpec{},
			Status: provisioningv1.DPUSetStatus{},
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
			obj := createObj("obj-dpuset")
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			objFetched := &provisioningv1.DPUSet{}

			By("checking the finalizer")
			Eventually(func(g Gomega) []string {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Finalizers
			}).WithTimeout(10 * time.Second).Should(ConsistOf([]string{provisioningv1.DPUSetFinalizer}))
		})

		It("create from yaml", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUSet
metadata:
  name: dpuset-1
  namespace: default
spec:
  nodeSelector:
    matchLabels:
      feature.node.kubernetes.io/dpu-enabled: "true"
  dpuSelector:
      feature.node.kubernetes.io/dpu-0-psid: "MT_0000000375"
      feature.node.kubernetes.io/dpu-0-pci-address: "0000-04-00"
  strategy:
    rollingUpdate:
      maxUnavailable: 10%
    type: RollingUpdate
  dpuTemplate:
    annotations:
      nvidia.com/dpuOperator-override-powercycle-command: "cycle"
    spec:
      dpuFlavor: "hbn"
      bfb:
        name: "doca-24.04"
      nodeEffect:
        taint:
          key: "dpu"
          value: "provisioning"
          effect: NoSchedule
      cluster:
        name: "tenant-00"
        namespace: "tenant-00-ns"
        nodeLabels:
          "dpf.node.dpu/role": "worker"
`)
			obj := &provisioningv1.DPUSet{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			objFetched := &provisioningv1.DPUSet{}

			By("checking the finalizer")
			Eventually(func(g Gomega) []string {
				g.Expect(k8sClient.Get(ctx, getObjKey(obj), objFetched)).To(Succeed())
				return objFetched.Finalizers
			}).WithTimeout(10 * time.Second).Should(ConsistOf([]string{provisioningv1.DPUSetFinalizer}))
		})
	})
})
