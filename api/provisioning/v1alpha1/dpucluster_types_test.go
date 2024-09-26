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
var _ = Describe("DPUCluster", func() {

	const (
		DefaultObjName = "obj-dpucluster"
		DefaultObjType = string(StaticCluster)
	)

	var getObjKey = func(obj *DPUCluster) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *DPUCluster {
		return &DPUCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec:   DPUClusterSpec{},
			Status: DPUClusterStatus{},
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
			obj := createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &DPUCluster{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched).To(Equal(obj))
		})

		It("delete object", func() {
			obj := createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Delete(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, getObjKey(obj), obj)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
		})

		It("update object", func() {
			obj := createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj_fetched := &DPUCluster{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched).To(Equal(obj))
		})

		It("spec.type validation", func() {
			obj := createObj(DefaultObjName)
			obj.Spec.Type = "dummy.com/type"
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj = createObj(DefaultObjName)
			obj.Spec.Type = "dummy.com"
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj = createObj(DefaultObjName)
			obj.Spec.Type = "dummy"
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj = createObj(DefaultObjName)
			obj.Spec.Type = ""
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("spec.maxNodes default", func() {
			obj := createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)
			Expect(obj.Spec.MaxNodes).To(BeEquivalentTo(1000))

			obj_fetched := &DPUCluster{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.MaxNodes).To(BeEquivalentTo(1000))
		})

		It("spec.maxNodes validation", func() {
			obj := createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			obj.Spec.MaxNodes = 1
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj = createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			obj.Spec.MaxNodes = 0
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj = createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			obj.Spec.MaxNodes = -1
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj = createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			obj.Spec.MaxNodes = 1001
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj = createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			obj.Spec.MaxNodes = 100000
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("spec.kubeconfig can be updated from unassigned value", func() {
			new_value := `dummy_new_kubeconfig`

			obj := createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &DPUCluster{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.Kubeconfig).To(Equal(""))

			obj.Spec.Kubeconfig = new_value
			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.Kubeconfig).To(Equal(new_value))
		})

		It("spec.kubeconfig can be updated from assigned empty value", func() {
			ref_value := ``
			new_value := `dummy_new_kubeconfig`

			obj := createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			obj.Spec.Kubeconfig = ref_value
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &DPUCluster{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.Kubeconfig).To(Equal(""))

			obj.Spec.Kubeconfig = new_value
			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.Kubeconfig).To(Equal(new_value))
		})

		It("spec.kubeconfig is immutable", func() {
			ref_value := `kubeconfig-test`
			new_value := `kubeconfig-test-new`

			obj := createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			obj.Spec.Kubeconfig = ref_value
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj.Spec.Kubeconfig = new_value
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &DPUCluster{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.Kubeconfig).To(Equal(ref_value))
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
      virtualRouterID: 1
      interface: br-dpu
`)
			obj := &DPUCluster{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &DPUCluster{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.Type).To(BeEquivalentTo(string(StaticCluster)))
			Expect(obj_fetched.Spec.MaxNodes).To(BeEquivalentTo(10))
			Expect(obj_fetched.Spec.Version).To(BeEquivalentTo("v1.31.0"))
		})

		It("status.phase default", func() {
			obj := createObj(DefaultObjName)
			obj.Spec.Type = DefaultObjType
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)
			Expect(obj.Status.Phase).To(BeEquivalentTo(PhasePending))

			obj_fetched := &DPUCluster{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Status.Phase).To(BeEquivalentTo(PhasePending))
		})
	})
})
