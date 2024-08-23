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

// These tes`s `are written in BDD-style using Ginkgo framework. Refer to
// http://onsi.github.io/ginkgo to learn more.
var _ = Describe("DPUFlavor", func() {

	var (
		DefaultGrub   = []string{`hugepagesz=2048kB`, `cgroup_no_v1=net_prio`}
		DefaultSysctl = []string{`net.mc_forwarding=2048kB`}
	)

	var getObjKey = func(obj *DPUFlavor) types.NamespacedName {
		return types.NamespacedName{
			Name:      obj.Name,
			Namespace: obj.Namespace,
		}
	}

	var createObj = func(name string) *DPUFlavor {
		return &DPUFlavor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec: DPUFlavorSpec{},
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
			obj.Spec.Grub.KernelParameters = DefaultGrub
			obj.Spec.Sysctl.Parameters = DefaultSysctl
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj_fetched := &DPUFlavor{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched).To(Equal(obj))
		})

		It("delete object", func() {
			obj := createObj("obj-2")
			obj.Spec.Grub.KernelParameters = DefaultGrub
			obj.Spec.Sysctl.Parameters = DefaultSysctl
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
			obj.Spec.Grub.KernelParameters = DefaultGrub
			obj.Spec.Sysctl.Parameters = DefaultSysctl
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Update(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj_fetched := &DPUFlavor{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched).To(Equal(obj))
		})

		It("spec.grub.kernelParameters is mandatory", func() {
			obj := createObj("obj-4")
			obj.Spec.Sysctl.Parameters = DefaultSysctl
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("spec.sysctl.parameters is mandatory", func() {
			obj := createObj("obj-5")
			obj.Spec.Grub.KernelParameters = DefaultGrub
			err := k8sClient.Create(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("spec.grub is immutable", func() {
			ref_value := DefaultGrub
			new_value := []string{`spec.grub`}

			obj := createObj("obj-6")
			obj.Spec.Grub.KernelParameters = ref_value
			obj.Spec.Sysctl.Parameters = DefaultSysctl
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.Grub.KernelParameters = new_value
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &DPUFlavor{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.Grub.KernelParameters[0]).To(Equal(ref_value[0]))
		})

		It("spec.sysctl is immutable", func() {
			ref_value := DefaultSysctl
			new_value := []string{`spec.sysctl`}

			obj := createObj("obj-7")
			obj.Spec.Grub.KernelParameters = DefaultGrub
			obj.Spec.Sysctl.Parameters = DefaultSysctl
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.Sysctl.Parameters = new_value
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &DPUFlavor{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.Sysctl.Parameters[0]).To(Equal(ref_value[0]))
		})

		It("spec.nvconfig is immutable", func() {
			ref_value := []string{`PF_BAR2_ENABLE=0`, `PER_PF_NUM_SF=1`}
			new_value := []string{`spec.nvconfig`}

			obj := createObj("obj-8")
			obj.Spec.Grub.KernelParameters = DefaultGrub
			obj.Spec.Sysctl.Parameters = DefaultSysctl
			obj.Spec.NVConfig = []DPUFlavorNVConfig{
				{Parameters: ref_value},
			}
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.NVConfig[0].Parameters = new_value
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &DPUFlavor{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.NVConfig[0].Parameters[0]).To(Equal(ref_value[0]))
		})

		It("spec.ovs is immutable", func() {
			ref_value := `ovs-vsct add-br br-hbn`
			new_value := `spec.ovs`

			obj := createObj("obj-9")
			obj.Spec.Grub.KernelParameters = DefaultGrub
			obj.Spec.Sysctl.Parameters = DefaultSysctl
			obj.Spec.OVS.RawConfigScript = ref_value
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.OVS.RawConfigScript = new_value
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &DPUFlavor{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.OVS.RawConfigScript).To(Equal(ref_value))
		})

		It("spec.bfcfgParameters is immutable", func() {
			ref_value := []string{`PF_BAR2_ENABLE=0`, `PER_PF_NUM_SF=1`}
			new_value := []string{`spec.bfcfgParameters`}

			obj := createObj("obj-10")
			obj.Spec.Grub.KernelParameters = DefaultGrub
			obj.Spec.Sysctl.Parameters = DefaultSysctl
			obj.Spec.BFCfgParameters = ref_value
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.BFCfgParameters = new_value
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &DPUFlavor{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.BFCfgParameters[0]).To(Equal(ref_value[0]))
		})

		It("spec.configFiles is immutable", func() {
			ref_value := `/etc/dummy.cfg`
			new_value := `spec.configFiles`

			obj := createObj("obj-11")
			obj.Spec.Grub.KernelParameters = DefaultGrub
			obj.Spec.Sysctl.Parameters = DefaultSysctl
			obj.Spec.ConfigFiles = []ConfigFile{
				{Path: ref_value},
			}
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.ConfigFiles[0].Path = new_value
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &DPUFlavor{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.ConfigFiles[0].Path).To(Equal(ref_value))
		})

		It("spec.ovs is immutable", func() {
			ref_value := `127.0.0.1:8001`
			new_value := `spec.ovs`

			obj := createObj("obj-12")
			obj.Spec.Grub.KernelParameters = DefaultGrub
			obj.Spec.Sysctl.Parameters = DefaultSysctl
			obj.Spec.ContainerdConfig.RegistryEndpoint = ref_value
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj.Spec.ContainerdConfig.RegistryEndpoint = new_value
			err = k8sClient.Update(ctx, obj)
			Expect(err).To(HaveOccurred())

			obj_fetched := &DPUFlavor{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched.Spec.ContainerdConfig.RegistryEndpoint).To(Equal(ref_value))
		})
	})
})
