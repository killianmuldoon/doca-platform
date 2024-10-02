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
var _ = Describe("BFB", func() {

	const (
		DefaultObjName = "obj-bfb"
		DefaultURL     = "http://example.com/dummy.bfb"
	)

	var getObjKey = func(obj *BFB) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *BFB {
		return &BFB{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec:   BFBSpec{},
			Status: BFBStatus{},
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
			obj.Spec.URL = DefaultURL
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &BFB{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched).To(Equal(obj))
		})

		It("delete object", func() {
			obj := createObj(DefaultObjName)
			obj.Spec.URL = DefaultURL
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
			obj.Spec.URL = DefaultURL
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj_fetched := &BFB{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched).To(Equal(obj))
		})

		It("spec.url is mandatory", func() {
			obj := createObj(DefaultObjName)
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("spec.url validation", func() {
			obj := createObj("obj-0")
			obj.Spec.URL = "http://example.com/dummy.tar"
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj = createObj("obj-1")
			obj.Spec.URL = "http://8.8.8.8/dummy.bfb"
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj = createObj("obj-2")
			obj.Spec.URL = "https://example.com/dummy.bfb"
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj = createObj("obj-3")
			obj.Spec.URL = "https://8.8.8.8/dummy.bfb"
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj = createObj("obj-4")
			obj.Spec.URL = "example.com/dummy.bfb"
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("spec.url is immutable", func() {
			ref_value := DefaultURL

			obj := createObj(DefaultObjName)
			obj.Spec.URL = ref_value
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj.Spec.URL = "http://example.com/dummy_clone.bfb"
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &BFB{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.URL).To(Equal(ref_value))
		})

		It("spec.fileName default", func() {
			obj := createObj(DefaultObjName)
			obj.Spec.URL = DefaultURL
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.FileName).To(BeEquivalentTo(obj.Namespace + "-" + obj.Name + ".bfb"))
			DeferCleanup(k8sClient.Delete, ctx, obj)
		})

		It("spec.fileName is validation", func() {
			obj := createObj(DefaultObjName)
			obj.Spec.FileName = "dummy_NAME-1.2.3.bfb"
			obj.Spec.URL = DefaultURL
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj = createObj(DefaultObjName)
			obj.Spec.FileName = "dummy.tar"
			obj.Spec.URL = DefaultURL
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj = createObj(DefaultObjName)
			obj.Spec.FileName = " dummy.bfb"
			obj.Spec.URL = DefaultURL
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj = createObj(DefaultObjName)
			obj.Spec.FileName = "/dummy.bfb"
			obj.Spec.URL = DefaultURL
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj = createObj(DefaultObjName)
			obj.Spec.FileName = "dummy with spaces.bfb"
			obj.Spec.URL = DefaultURL
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("spec.fileName is immutable", func() {
			ref_value := "dummy.bfb"

			obj := createObj(DefaultObjName)
			obj.Spec.FileName = ref_value
			obj.Spec.URL = DefaultURL
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj.Spec.FileName = "dummy_clone.bfb"
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &BFB{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.FileName).To(Equal(ref_value))
		})

		It("create from yaml", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: BFB
metadata:
  name: obj-bfb
  namespace: default
spec:
  fileName: "bf-bundle-2.7.0-33.bfb"
  url: "http://bfb-server.dpf-operator-system/bf-bundle-2.7.0-33_24.04_ubuntu-22.04_unsigned.bfb"
`)
			obj := &BFB{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)
		})

		It("create from yaml minimal", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: BFB
metadata:
  name: obj-bfb
  namespace: default
spec:
  url: "http://bfb-server.dpf-operator-system/bf-bundle-2.7.0-33_24.04_ubuntu-22.04_unsigned.bfb"
`)
			obj := &BFB{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(k8sClient.Delete, ctx, obj)
		})

		It("create from yaml validation issue (w/o url)", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: BFB
metadata:
  name: obj-bfb
  namespace: default
`)
			obj := &BFB{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("create from yaml validation issue (url is empty)", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: BFB
metadata:
  name: obj-bfb
  namespace: default
spec:
  url: ""
`)
			obj := &BFB{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("status.phase default", func() {
			obj := createObj(DefaultObjName)
			obj.Spec.URL = DefaultURL
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Status.Phase).To(BeEquivalentTo(BFBInitializing))
			DeferCleanup(k8sClient.Delete, ctx, obj)

			obj_fetched := &BFB{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Status.Phase).To(BeEquivalentTo(BFBInitializing))
		})
	})
})
