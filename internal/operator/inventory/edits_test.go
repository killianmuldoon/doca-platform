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

package inventory

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Test Edits", func() {
	Context("Edits Usage", func() {
		var objs []*unstructured.Unstructured
		var objsByKind map[ObjectKind]*unstructured.Unstructured

		BeforeEach(func() {
			var err error
			deployment := &appsv1.Deployment{TypeMeta: metav1.TypeMeta{Kind: "Deployment"}}
			service := &corev1.Service{TypeMeta: metav1.TypeMeta{Kind: "Service"}}
			objs = nil
			objsByKind = make(map[ObjectKind]*unstructured.Unstructured)

			obj := &unstructured.Unstructured{}
			obj.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(deployment)
			Expect(err).ToNot(HaveOccurred())
			objs = append(objs, obj)
			objsByKind[DeploymentKind] = obj

			obj = &unstructured.Unstructured{}
			obj.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(service)
			Expect(err).ToNot(HaveOccurred())
			objs = append(objs, obj)
			objsByKind[ServiceKind] = obj
		})

		It("edits all objects", func() {
			Expect(NewEdits().AddForAll(NamespaceEdit("foo")).Apply(objs)).ToNot(HaveOccurred())
			for _, obj := range objs {
				Expect(obj.GetNamespace()).To(Equal("foo"))
			}
		})

		It("edits by kind unstructured", func() {
			Expect(NewEdits().AddForKind(DeploymentKind, NamespaceEdit("foo")).Apply(objs)).ToNot(HaveOccurred())
			Expect(objsByKind[DeploymentKind].GetNamespace()).To(Equal("foo"))
			Expect(objsByKind[ServiceKind].GetNamespace()).To(BeEmpty())
		})

		It("edits by kind structured", func() {
			nsEdit := func(obj client.Object) error {
				d := obj.(*appsv1.Deployment)
				d.Namespace = "foo"
				return nil
			}

			Expect(NewEdits().AddForKindS(DeploymentKind, nsEdit).Apply(objs)).ToNot(HaveOccurred())
			Expect(objsByKind[DeploymentKind].GetNamespace()).To(Equal("foo"))
			Expect(objsByKind[ServiceKind].GetNamespace()).To(BeEmpty())
		})

		It("fails if a single edit fails", func() {
			failingEdit := func(_ *unstructured.Unstructured) error {
				return fmt.Errorf("error")
			}
			failingEditForDeployment := func(_ client.Object) error {
				return fmt.Errorf("error")
			}
			Expect(NewEdits().AddForAll(failingEdit).Apply(objs)).To(HaveOccurred())
			Expect(NewEdits().AddForKind(ServiceKind, failingEdit).Apply(objs)).To(HaveOccurred())
			Expect(NewEdits().AddForKindS(DeploymentKind, failingEditForDeployment).Apply(objs)).To(HaveOccurred())
		})

		It("fails if conversion to concrete type does not exist", func() {
			nsEditForService := func(obj client.Object) error {
				d := obj.(*corev1.Service)
				d.Namespace = "foo"
				return nil
			}
			err := NewEdits().AddForKindS(ServiceKind, nsEditForService).Apply(objs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing conversion"))
		})
	})
})
