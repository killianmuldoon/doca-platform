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
var _ = Describe("DpuSet", func() {

	var getObjKey = func(obj *DpuSet) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *DpuSet {
		return &DpuSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec:   DpuSetSpec{},
			Status: DpuSetStatus{},
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

			obj_fetched := &DpuSet{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched).To(Equal(obj))
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

			obj_fetched := &DpuSet{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched).To(Equal(obj))
		})

		It("spec.dpuTemplate.spec.k8s_cluster is mutable", func() {
			ref_value := `dummy_cluster`
			new_value := `dummy_new_cluster`

			obj := createObj("obj-4")
			obj.Spec.DpuTemplate.Spec.Cluster = K8sCluster{
				Name:      ref_value,
				NameSpace: `default`,
			}
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.DpuTemplate.Spec.Cluster.Name = new_value
			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj_fetched := &DpuSet{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.DpuTemplate.Spec.Cluster.Name).To(Equal(new_value))
		})

		It("create from yaml", func() {
			yml := []byte(`
apiVersion: provisioning.dpf.nvidia.com/v1alpha1
kind: DpuSet
metadata:
  name: obj-5
  namespace: default
spec:
  nodeSelector:
  strategy:
    rollingUpdate:
      maxUnavailable: 10%
    type: RollingUpdate
  dpuTemplate:
    annotations:
      nvidia.com/dpuOperator-override-powercycle-command: "cycle"
    spec:
      dpuFlavor: "hbn"
      BFB:
        bfb: "doca-24.04"
      nodeEffect:
        taint:
          key: "dpu"
          value: "provisioning"
          effect: NoSchedule
      k8s_cluster:
        name: "tenant-00"
        namespace: "tenant-00-ns"
        node_labels:
          "dpf.node.dpu/role": "worker"
`)
			obj := &DpuSet{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("create from yaml minimal", func() {
			yml := []byte(`
apiVersion: provisioning.dpf.nvidia.com/v1alpha1
kind: DpuSet
metadata:
  name: obj-6
  namespace: default
`)
			obj := &DpuSet{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
