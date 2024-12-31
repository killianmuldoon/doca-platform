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

package pci

import (
	"fmt"
	"os"

	osMock "github.com/nvidia/doca-platform/internal/storage/snap/csi-plugin/wrappers/os/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Pci Utils package test", func() {
	Describe("helpers", func() {
		Context("ValidatePCIAddress", func() {
			It("valid", func() {
				add, err := ParsePCIAddress("0000:3b:00.2")
				Expect(err).NotTo(HaveOccurred())
				Expect(add).To(Equal("0000:3b:00.2"))
			})
			It("Valid - no leading zeroes", func() {
				add, err := ParsePCIAddress("3b:00.2")
				Expect(err).NotTo(HaveOccurred())
				Expect(add).To(Equal("0000:3b:00.2"))
			})
			It("invalid - empty", func() {
				_, err := ParsePCIAddress("")
				Expect(err).To(HaveOccurred())
			})
			It("invalid - invalid format", func() {
				_, err := ParsePCIAddress("foobar")
				Expect(err).To(HaveOccurred())
			})
		})
	})
	Describe("utils", func() {
		var (
			pciUtils Utils
			osLib    *osMock.MockPkgWrapper
			testCtrl *gomock.Controller
		)
		BeforeEach(func() {
			testCtrl = gomock.NewController(GinkgoT())
			osLib = osMock.NewMockPkgWrapper(testCtrl)
			pciUtils = New("", osLib, nil)
		})
		AfterEach(func() {
			testCtrl.Finish()
		})
		Context("LoadDriver", func() {
			It("failed to check driver", func() {
				osLib.EXPECT().Stat("/sys/bus/pci/devices/0000:3b:00.2/driver").Return(nil, fmt.Errorf("test error"))
				Expect(pciUtils.LoadDriver("0000:3b:00.2", "nvme")).To(HaveOccurred())
			})
			It("failed to check driver - can't read link", func() {
				osLib.EXPECT().Stat("/sys/bus/pci/devices/0000:3b:00.2/driver").Return(nil, nil)
				osLib.EXPECT().Readlink("/sys/bus/pci/devices/0000:3b:00.2/driver").Return("", fmt.Errorf("test error"))
				Expect(pciUtils.LoadDriver("0000:3b:00.2", "nvme")).To(HaveOccurred())
			})
			It("bind to a different driver", func() {
				osLib.EXPECT().Stat("/sys/bus/pci/devices/0000:3b:00.2/driver").Return(nil, nil)
				osLib.EXPECT().Readlink("/sys/bus/pci/devices/0000:3b:00.2/driver").Return("../../../../bus/pci/drivers/something", nil)
				Expect(pciUtils.LoadDriver("0000:3b:00.2", "nvme")).To(HaveOccurred())
			})
			It("already bind to the expected driver", func() {
				osLib.EXPECT().Stat("/sys/bus/pci/devices/0000:3b:00.2/driver").Return(nil, nil)
				osLib.EXPECT().Readlink("/sys/bus/pci/devices/0000:3b:00.2/driver").Return("../../../../bus/pci/drivers/nvme", nil)
				Expect(pciUtils.LoadDriver("0000:3b:00.2", "nvme")).NotTo(HaveOccurred())
			})
			It("fail to override", func() {
				osLib.EXPECT().Stat("/sys/bus/pci/devices/0000:3b:00.2/driver").Return(nil, os.ErrNotExist)
				osLib.EXPECT().WriteFile("/sys/bus/pci/devices/0000:3b:00.2/driver_override", []byte("nvme"), gomock.Any()).Return(fmt.Errorf("test error"))
				Expect(pciUtils.LoadDriver("0000:3b:00.2", "nvme")).To(HaveOccurred())
			})
			It("fail to bind", func() {
				osLib.EXPECT().Stat("/sys/bus/pci/devices/0000:3b:00.2/driver").Return(nil, os.ErrNotExist)
				osLib.EXPECT().WriteFile("/sys/bus/pci/devices/0000:3b:00.2/driver_override", []byte("nvme"), gomock.Any()).Return(nil)
				osLib.EXPECT().WriteFile("/sys/bus/pci/drivers/nvme/bind", []byte("0000:3b:00.2"), gomock.Any()).Return(fmt.Errorf("test error"))
				Expect(pciUtils.LoadDriver("0000:3b:00.2", "nvme")).To(HaveOccurred())
			})
			It("succeed", func() {
				osLib.EXPECT().Stat("/sys/bus/pci/devices/0000:3b:00.2/driver").Return(nil, os.ErrNotExist)
				osLib.EXPECT().WriteFile("/sys/bus/pci/drivers/nvme/bind", []byte("0000:3b:00.2"), gomock.Any()).Return(nil)
				osLib.EXPECT().WriteFile("/sys/bus/pci/devices/0000:3b:00.2/driver_override", []byte("nvme"), gomock.Any()).Return(nil)
				Expect(pciUtils.LoadDriver("0000:3b:00.2", "nvme")).NotTo(HaveOccurred())
			})
		})
	})
})
