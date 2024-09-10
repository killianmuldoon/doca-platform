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
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj_fetched := &DPUFlavor{}
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

		It("update object with not default data", func() {
			obj := createObj("obj-4")
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

		It("check default settings", func() {
			obj := createObj("obj-5")
			err := k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			obj_fetched := &DPUFlavor{}
			err = k8sClient.Get(ctx, getObjKey(obj), obj_fetched)
			Expect(err).NotTo(HaveOccurred())
			Expect(obj_fetched).To(Equal(obj))
			Expect(obj_fetched.Spec.Grub.KernelParameters).To(BeEmpty())
			Expect(obj_fetched.Spec.Sysctl.Parameters).To(BeEmpty())
			Expect(obj_fetched.Spec.NVConfig).To(BeEmpty())
			Expect(obj_fetched.Spec.OVS.RawConfigScript).To(BeEmpty())
			Expect(obj_fetched.Spec.BFCfgParameters).To(BeEmpty())
			Expect(obj_fetched.Spec.ConfigFiles).To(BeEmpty())
			Expect(obj_fetched.Spec.ContainerdConfig.RegistryEndpoint).To(BeEmpty())
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

		It("create from yaml", func() {
			yml := []byte(`
apiVersion: provisioning.dpf.nvidia.com/v1alpha1
kind: DPUFlavor
metadata:
  name: obj-13
  namespace: default
spec:
  grub:
    kernelParameters:
      - console=hvc0
      - console=ttyAMA0
      - earlycon=pl011,0x13010000
      - fixrttc
      - net.ifnames=0
      - biosdevname=0
      - iommu.passthrough=1
      - cgroup_no_v1=net_prio,net_cls
      - hugepagesz=2048kB
      - hugepages=3072
  sysctl:
    parameters:
    - net.ipv4.ip_forward=1
    - net.ipv4.ip_forward_update_priority=0
  nvconfig:
    - device: "*"
      parameters:
        - PF_BAR2_ENABLE=0
        - PER_PF_NUM_SF=1
        - PF_TOTAL_SF=40
        - PF_SF_BAR_SIZE=10
        - NUM_PF_MSIX_VALID=0
        - PF_NUM_PF_MSIX_VALID=1
        - PF_NUM_PF_MSIX=228
        - INTERNAL_CPU_MODEL=1
        - SRIOV_EN=1
        - NUM_OF_VFS=30
        - LAG_RESOURCE_ALLOCATION=1
  ovs:
    rawConfigScript: |
      ovs-vsctl set Open_vSwitch . other_config:doca-init=true
      ovs-vsctl set Open_vSwitch . other_config:dpdk-extra="-a 0000:00:00.0"
      ovs-vsctl set Open_vSwitch . other_config:hw-offload-ct-size=64000
      ovs-vsctl set Open_vSwitch . other_config:dpdk-max-memzones="50000"
      ovs-vsctl set Open_vSwitch . other_config:hw-offload="true"
      ovs-vsctl set Open_vSwitch . other_config:pmd-quiet-idle=true
      ovs-vsctl set Open_vSwitch . other_config:max-idle=20000
      ovs-vsctl set Open_vSwitch . other_config:max-revalidator=5000
  bfcfgParameters:
    - ubuntu_PASSWORD=$1$rvRv4qpw$mS6kYODr8oMxORt.TkiTB0
    - WITH_NIC_FW_UPDATE=yes
    - ENABLE_SFC_HBN=no
  configFiles:
  - path: /etc/bla/blabla.cfg
    operation: append
    raw: |
        CREATE_OVS_BRIDGES="no"
        CREATE_OVS_BRIDGES="no"
    permissions: "0755"
`)
			obj := &DPUFlavor{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("create from yaml minimal", func() {
			yml := []byte(`
apiVersion: provisioning.dpf.nvidia.com/v1alpha1
kind: DPUFlavor
metadata:
  name: obj-14
  namespace: default
`)
			obj := &DPUFlavor{}
			err := yaml.UnmarshalStrict(yml, obj)
			Expect(err).To(Succeed())
			err = k8sClient.Create(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
