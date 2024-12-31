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
package nvme

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"

	osMock "github.com/nvidia/doca-platform/internal/storage/snap/csi-plugin/wrappers/os/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

type dummyDirEntry struct {
	name string
}

func (d *dummyDirEntry) Name() string {
	return d.name
}

func (d *dummyDirEntry) IsDir() bool {
	panic("not implemented")
}

func (d *dummyDirEntry) Type() fs.FileMode {
	panic("not implemented")
}

func (d *dummyDirEntry) Info() (fs.FileInfo, error) {
	panic("not implemented")
}

var _ = Describe("Nvme utils tests", func() {
	var (
		nvmeUtils Utils
		osLib     *osMock.MockPkgWrapper
		testCtrl  *gomock.Controller
	)

	BeforeEach(func() {
		testCtrl = gomock.NewController(GinkgoT())
		osLib = osMock.NewMockPkgWrapper(testCtrl)
		nvmeUtils = New(osLib)
	})

	AfterEach(func() {
		testCtrl.Finish()
	})

	Context("GetBlockDeviceNameForNS", func() {
		It("fail to read nvme driver dir", func() {
			osLib.EXPECT().ReadDir("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme").Return(nil, fmt.Errorf("test error"))
			_, err := nvmeUtils.GetBlockDeviceNameForNS("0000:3b:00.2", int32(10))
			ExpectWithOffset(1, err).To(HaveOccurred())
		})
		It("no nvme controller", func() {
			osLib.EXPECT().ReadDir("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme").Return(nil, nil)
			_, err := nvmeUtils.GetBlockDeviceNameForNS("0000:3b:00.2", int32(10))
			ExpectWithOffset(1, err).To(HaveOccurred())
		})
		It("fail to read nvme controller dir", func() {
			osLib.EXPECT().ReadDir("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme").Return([]os.DirEntry{&dummyDirEntry{name: "nvme3"}}, nil)
			osLib.EXPECT().ReadDir("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme/nvme3").Return(nil, fmt.Errorf("test error"))
			_, err := nvmeUtils.GetBlockDeviceNameForNS("0000:3b:00.2", int32(10))
			ExpectWithOffset(1, err).To(HaveOccurred())
		})
		It("fail to read nsid", func() {
			osLib.EXPECT().ReadDir("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme").Return([]os.DirEntry{&dummyDirEntry{name: "nvme3"}}, nil)
			osLib.EXPECT().ReadDir("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme/nvme3").Return([]os.DirEntry{&dummyDirEntry{name: "nvme2n1"}}, nil)
			osLib.EXPECT().ReadFile("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme/nvme3/nvme2n1/nsid").Return(nil, fmt.Errorf("test error"))
			dev, err := nvmeUtils.GetBlockDeviceNameForNS("0000:3b:00.2", int32(10))
			Expect(dev).To(BeEmpty())
			Expect(errors.Is(err, ErrBlockDeviceNotFound)).To(BeTrue())
		})
		It("invalid nsid", func() {
			osLib.EXPECT().ReadDir("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme").Return([]os.DirEntry{&dummyDirEntry{name: "nvme3"}}, nil)
			osLib.EXPECT().ReadDir("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme/nvme3").Return([]os.DirEntry{&dummyDirEntry{name: "nvme2n1"}}, nil)
			osLib.EXPECT().ReadFile("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme/nvme3/nvme2n1/nsid").Return([]byte("something_\n"), nil)
			dev, err := nvmeUtils.GetBlockDeviceNameForNS("0000:3b:00.2", int32(10))
			Expect(dev).To(BeEmpty())
			Expect(errors.Is(err, ErrBlockDeviceNotFound)).To(BeTrue())
		})
		It("nsid not found", func() {
			osLib.EXPECT().ReadDir("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme").Return([]os.DirEntry{&dummyDirEntry{name: "nvme3"}}, nil)
			osLib.EXPECT().ReadDir("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme/nvme3").Return([]os.DirEntry{&dummyDirEntry{name: "nvme2n1"}}, nil)
			osLib.EXPECT().ReadFile("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme/nvme3/nvme2n1/nsid").Return([]byte(strconv.Itoa(11)), nil)
			dev, err := nvmeUtils.GetBlockDeviceNameForNS("0000:3b:00.2", int32(10))
			Expect(dev).To(BeEmpty())
			Expect(errors.Is(err, ErrBlockDeviceNotFound)).To(BeTrue())
		})
		It("found", func() {
			osLib.EXPECT().ReadDir("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme").Return([]os.DirEntry{&dummyDirEntry{name: "nvme3"}}, nil)
			osLib.EXPECT().ReadDir("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme/nvme3").Return([]os.DirEntry{&dummyDirEntry{name: "nvme2n1"}}, nil)
			osLib.EXPECT().ReadFile("/sys/bus/pci/drivers/nvme/0000:3b:00.2/nvme/nvme3/nvme2n1/nsid").Return([]byte(strconv.Itoa(10)), nil)
			dev, err := nvmeUtils.GetBlockDeviceNameForNS("0000:3b:00.2", int32(10))
			Expect(dev).To(BeEquivalentTo("nvme2n1"))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
