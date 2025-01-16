/*
Copyright 2025 NVIDIA

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

package webhooks

import (
	"context"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("DPUDevice", func() {
	var getObjKey = func(obj *provisioningv1.DPUDevice) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}
	var createObj = func(name string) *provisioningv1.DPUDevice {
		return &provisioningv1.DPUDevice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec: provisioningv1.DPUDeviceSpec{},
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

			objFetched := &provisioningv1.DPUDevice{}
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
			obj.Spec.BMCIP = "2.2.2.2"
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &provisioningv1.DPUDevice{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched).To(Equal(obj))
		})

		It("check default settings", func() {
			obj := createObj("obj-4")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &provisioningv1.DPUDevice{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched).To(Equal(obj))
			Expect(objFetched.Spec.BMCIP).To(BeEmpty())
			Expect(objFetched.Spec.PCIAddress).To(BeEmpty())
			Expect(objFetched.Spec.PSID).To(BeEmpty())
			Expect(objFetched.Spec.OPN).To(BeEmpty())
		})

		It("create from yaml", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUDevice
metadata:
  name: obj-5
  namespace: default
spec:
  bmcIp: 3.3.3.3
  pciAddress: 0000-04-00
  psid: MT_0000000034
  opn: 900-9D3B4-00SV-EA0
`)
			obj := &provisioningv1.DPUDevice{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("create from yaml minimal", func() {
			yml := []byte(`
apiVersion: provisioning.dpu.nvidia.com/v1alpha1
kind: DPUDevice
metadata:
  name: obj-6
  namespace: default
`)
			obj := &provisioningv1.DPUDevice{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
		It("update object - check immutability of PCI Address", func() {
			obj := createObj("obj-7")
			obj.Spec.PCIAddress = "0000-04-00"
			obj.Spec.PSID = "MT_0000000034"
			obj.Spec.OPN = "900-9D3B4-00SV-EA0"
			obj.Spec.BMCIP = "7.7.7.7"
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.PCIAddress = "0000-03-00"
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
		It("update object - check immutability of PSID", func() {
			obj := createObj("obj-8")
			obj.Spec.PSID = "MT_0000000034"
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.PSID = "MT_0000000039"
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
		It("update object - check immutability of OPN", func() {
			obj := createObj("obj-9")
			obj.Spec.OPN = "900-9D3B4-00SV-EA0"
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.OPN = "900-9D3B4-00SV-EAA"
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
		It("update object - check immutability of BMC IP", func() {
			obj := createObj("obj-10")
			obj.Spec.BMCIP = "22.22.22.22"
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.BMCIP = "4.4.4.4"
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
		It("create object with invalid PCIAddress", func() {
			obj := &provisioningv1.DPUDevice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "obj-11",
					Namespace: "default",
				},
				Spec: provisioningv1.DPUDeviceSpec{
					PCIAddress: "Invalid-PCI-Address",
				},
			}
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("create object with invalid PSID", func() {
			obj := &provisioningv1.DPUDevice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "obj-12",
					Namespace: "default",
				},
				Spec: provisioningv1.DPUDeviceSpec{
					PSID: "Invalid-PSID",
				},
			}
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("create object with invalid OPN", func() {
			obj := &provisioningv1.DPUDevice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "obj-13",
					Namespace: "default",
				},
				Spec: provisioningv1.DPUDeviceSpec{
					OPN: "Invalid-OPN",
				},
			}
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("create object with invalid BMCIP", func() {
			obj := &provisioningv1.DPUDevice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "obj-14",
					Namespace: "default",
				},
				Spec: provisioningv1.DPUDeviceSpec{
					BMCIP: "Invalid-IP-Address",
				},
			}
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})
		It("create object with multiple invalid specs should fail", func() {
			obj := &provisioningv1.DPUDevice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "obj-15",
					Namespace: "default",
				},
				Spec: provisioningv1.DPUDeviceSpec{
					PCIAddress: "0000:03:00.1",
					PSID:       "MT-0001234567",
					OPN:        "900-9D3B4-00SV-EA0F",
					BMCIP:      "10.1.2.3/24",
				},
			}
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("create object with valid specs should succeed", func() {
			obj := &provisioningv1.DPUDevice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "obj-16",
					Namespace: "default",
				},
				Spec: provisioningv1.DPUDeviceSpec{
					PCIAddress: "0000:03:00",
					PSID:       "MT_0001234567",
					OPN:        "900-9D3B4-00SV-EA0",
					BMCIP:      "10.1.2.3",
				},
			}
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			objFetched := &provisioningv1.DPUDevice{}
			err = k8sClient.Get(ctx, getObjKey(obj), objFetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(objFetched).To(Equal(obj))
		})
	})
})
