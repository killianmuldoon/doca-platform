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

package v1alpha1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// These tests are written in BDD-style using Ginkgo framework. Refer to
// http://onsi.github.io/ginkgo to learn more.
var _ = Describe("DPU", func() {

	var getObjKey = func(obj *DPU) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *DPU {
		return &DPU{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec:   DPUSpec{},
			Status: DPUStatus{},
		}
	}

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	Context("obj test context", func() {
		ctx := context.Background()

		It("create and get object", func() {
			obj := createObj("obj-1")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &DPU{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched).To(Equal(obj))
		})

		It("delete object", func() {
			obj := createObj("obj-2")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Delete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, getObjKey(obj), obj)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("update object", func() {
			obj := createObj("obj-3")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &DPU{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched).To(Equal(obj))
		})

		It("spec.nodeEffect default", func() {
			obj := createObj("obj-4")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &DPU{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Spec.NodeEffect.Drain).NotTo(BeNil())
			Expect(objFetched.Spec.NodeEffect.Drain.AutomaticNodeReboot).To(BeTrue())
		})

		It("spec.nodeName is immutable", func() {
			refValue := "dummy_node"

			obj := createObj("obj-5")
			obj.Spec.NodeName = refValue
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.NodeName = "dummy_new_node"
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			objFetched := &DPU{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Spec.NodeName).To(Equal(refValue))
		})

		It("spec.pciAddress is immutable", func() {
			refValue := "spec.pci_address"

			obj := createObj("obj-6")
			obj.Spec.PCIAddress = refValue
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.PCIAddress = "spec.pciAddress_new"
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			objFetched := &DPU{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Spec.PCIAddress).To(Equal(refValue))
		})

		It("spec.Cluster is mutable", func() {
			refValue := `dummy_cluster`
			newValue := `dummy_new_cluster`

			obj := createObj("obj-7")
			obj.Spec.Cluster = K8sCluster{
				Name:      refValue,
				NameSpace: `default`,
			}
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.Cluster.Name = newValue
			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &DPU{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Spec.Cluster.Name).To(Equal(newValue))
		})

		It("spec.cluster can be updated from unassigned state", func() {
			newValueName := `dummy_new_cluster_name`
			newValueNamespace := `dummy_new_cluster_namespace`

			obj := createObj("obj-dpu")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			objFetched := &DPU{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Spec.Cluster.Name).To(Equal(""))
			Expect(objFetched.Spec.Cluster.NameSpace).To(Equal(""))

			obj.Spec.Cluster.Name = newValueName
			obj.Spec.Cluster.NameSpace = newValueNamespace
			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Spec.Cluster.Name).To(Equal(newValueName))
			Expect(objFetched.Spec.Cluster.NameSpace).To(Equal(newValueNamespace))
		})

		It("create from yaml", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPU
metadata:
  name: obj-8
  namespace: default
spec:
  nodeName: "dpu-bf2"
  bfb: "doca-24.04"
  pciAddress: "0000:04:00.0"
  dpuFlavor: "dpu-flavor"
  cluster:
    name: "tenant-00"
    namespace: "tenant-00-ns"
    nodeLabels:
      "dpf.node.dpu/role": "worker"
  nodeEffect:
    taint:
      key: "dpu"
      value: "provisioning"
      effect: NoSchedule
`)
			obj := &DPU{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("create from yaml minimal", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPU
metadata:
  name: obj-9
  namespace: default
`)
			obj := &DPU{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("status.phase default", func() {
			obj := createObj("obj-10")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Status.Phase).To(BeEquivalentTo(DPUInitializing))

			objFetched := &DPU{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched.Status.Phase).To(BeEquivalentTo(DPUInitializing))
		})
	})
})
