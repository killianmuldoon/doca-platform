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
)

// These tests are written in BDD-style using Ginkgo framework. Refer to
// http://onsi.github.io/ginkgo to learn more.
var _ = Describe("Bfb", func() {

	const (
		DefaultURL = "http://example.com/dummy.bfb"
	)

	var getObjKey = func(obj *Bfb) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *Bfb {
		return &Bfb{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec:   BfbSpec{},
			Status: BfbStatus{},
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
			obj.Spec.URL = DefaultURL
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj_fetched := &Bfb{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched).To(Equal(obj))
		})

		It("delete object", func() {
			obj := createObj("obj-2")
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
			obj := createObj("obj-3")
			obj.Spec.URL = DefaultURL
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj_fetched := &Bfb{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched).To(Equal(obj))
		})

		It("spec.url is mandatory", func() {
			obj := createObj("obj-4")
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("spec.url validation", func() {
			obj := createObj("obj-5")
			obj.Spec.URL = "http://example.com/dummy.tar"
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj = createObj("obj-5-0")
			obj.Spec.URL = "http://8.8.8.8/dummy.bfb"
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj = createObj("obj-5-1")
			obj.Spec.URL = "https://example.com/dummy.bfb"
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj = createObj("obj-5-2")
			obj.Spec.URL = "https://8.8.8.8/dummy.bfb"
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj = createObj("obj-5-3")
			obj.Spec.URL = "example.com/dummy.bfb"
			err = k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("spec.url is immutable", func() {
			ref_value := DefaultURL

			obj := createObj("obj-6")
			obj.Spec.URL = ref_value
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.URL = "http://example.com/dummy_clone.bfb"
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &Bfb{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.URL).To(Equal(ref_value))
		})

		It("spec.file_name default", func() {
			obj := createObj("obj-7")
			obj.Spec.URL = DefaultURL
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj.Spec.FileName).To(BeEquivalentTo(obj.Namespace + "-" + obj.Name + ".bfb"))
		})

		It("spec.file_name is invalid (not *.bfb)", func() {
			obj := createObj("obj-8")
			obj.Spec.FileName = "dummy.tar"
			obj.Spec.URL = DefaultURL
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("spec.file_name is immutable", func() {
			ref_value := "dummy.bfb"

			obj := createObj("obj-9")
			obj.Spec.FileName = ref_value
			obj.Spec.URL = DefaultURL
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.FileName = "dummy_clone.bfb"
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &Bfb{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.FileName).To(Equal(ref_value))
		})

		It("spec.bf_cfg is immutable", func() {
			ref_value := "uname -r"

			obj := createObj("obj-10")
			obj.Spec.BFCFG = ref_value
			obj.Spec.URL = DefaultURL
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.BFCFG = "pwd"
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &Bfb{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.BFCFG).To(Equal(ref_value))
		})
	})
})
