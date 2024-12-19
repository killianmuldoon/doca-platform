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

package mount

import (
	"fmt"
	"os"

	mountLibWrapper "github.com/nvidia/doca-platform/internal/storage/csi-plugin/wrappers/mountlib"
	mountLibMock "github.com/nvidia/doca-platform/internal/storage/csi-plugin/wrappers/mountlib/mock"
	osMock "github.com/nvidia/doca-platform/internal/storage/csi-plugin/wrappers/os/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Mount Utils", func() {
	var (
		mountUtils Utils
		mountLib   *mountLibMock.MockPkgWrapper
		osLib      *osMock.MockPkgWrapper
		testCtrl   *gomock.Controller
	)
	BeforeEach(func() {
		testCtrl = gomock.NewController(GinkgoT())
		mountLib = mountLibMock.NewMockPkgWrapper(testCtrl)
		osLib = osMock.NewMockPkgWrapper(testCtrl)
		mountUtils = New(osLib, mountLib)
	})
	AfterEach(func() {
		testCtrl.Finish()
	})
	Context("Mount", func() {
		It("succeed", func() {
			mountLib.EXPECT().Mount("/dev/loop1", "/tmp/mount1", "ext4", []string{"remount", "rw"}).Return(nil)
			Expect(mountUtils.Mount("/dev/loop1", "/tmp/mount1", "ext4", []string{"remount", "rw"})).NotTo(HaveOccurred())
		})
		It("failed", func() {
			mountLib.EXPECT().Mount("/dev/loop1", "/tmp/mount1", "ext4", []string{"remount", "rw"}).Return(fmt.Errorf("test error"))
			Expect(mountUtils.Mount("/dev/loop1", "/tmp/mount1", "ext4", []string{"remount", "rw"})).To(HaveOccurred())
		})
	})
	Context("UnmountAndRemove", func() {
		It("path not found", func() {
			mountLib.EXPECT().PathExists("/tmp/mount1").Return(false, nil)
			osLib.EXPECT().RemoveAll("/tmp/mount1").Return(nil)
			Expect(mountUtils.UnmountAndRemove("/tmp/mount1")).NotTo(HaveOccurred())
		})
		It("mount path check failed", func() {
			mountLib.EXPECT().PathExists("/tmp/mount1").Return(false, fmt.Errorf("test error"))
			Expect(mountUtils.UnmountAndRemove("/tmp/mount1")).To(HaveOccurred())
		})
		It("failed to remove mount path", func() {
			mountLib.EXPECT().PathExists("/tmp/mount1").Return(false, nil)
			osLib.EXPECT().RemoveAll("/tmp/mount1").Return(fmt.Errorf("test error"))
			Expect(mountUtils.UnmountAndRemove("/tmp/mount1")).To(HaveOccurred())
		})
		It("mount not found - mount path removed", func() {
			mountLib.EXPECT().PathExists("/tmp/mount1").Return(false, nil)
			osLib.EXPECT().RemoveAll("/tmp/mount1").Return(nil)
			Expect(mountUtils.UnmountAndRemove("/tmp/mount1")).NotTo(HaveOccurred())
		})
		It("failed to parse mounts", func() {
			mountLib.EXPECT().ParseMountInfo(
				mountLibWrapper.DefaultProcMountInfoPath).Return(nil, fmt.Errorf("test error"))
			mountLib.EXPECT().PathExists("/tmp/mount1").Return(true, nil)
			Expect(mountUtils.UnmountAndRemove("/tmp/mount1")).To(HaveOccurred())
		})
		It("mount exist - failed to remove mount path", func() {
			mountLib.EXPECT().ParseMountInfo(
				mountLibWrapper.DefaultProcMountInfoPath).Return([]mountLibWrapper.MountInfo{{
				Source:       "_not_match",
				MountPoint:   "_not_match",
				FsType:       "ext4",
				MountOptions: []string{"remount", "rw"},
			}}, nil)
			mountLib.EXPECT().PathExists("/tmp/mount1").Return(true, nil)
			osLib.EXPECT().RemoveAll("/tmp/mount1").Return(fmt.Errorf("test error"))
			Expect(mountUtils.UnmountAndRemove("/tmp/mount1")).To(HaveOccurred())
		})
		It("failed to unmount", func() {
			mountLib.EXPECT().PathExists("/tmp/mount1").Return(true, nil).Times(2)
			mountLib.EXPECT().ParseMountInfo(
				mountLibWrapper.DefaultProcMountInfoPath).Return([]mountLibWrapper.MountInfo{{
				Source:       "/dev/loop1",
				MountPoint:   "/tmp/mount1",
				FsType:       "ext4",
				MountOptions: []string{"remount", "rw"},
			}}, nil).Times(2)
			mountLib.EXPECT().Unmount("/tmp/mount1").Return(fmt.Errorf("test error"))
			Expect(mountUtils.UnmountAndRemove("/tmp/mount1")).To(HaveOccurred())
		})
		It("mount exist - failed to remove mount point", func() {
			mountLib.EXPECT().ParseMountInfo(
				mountLibWrapper.DefaultProcMountInfoPath).Return([]mountLibWrapper.MountInfo{{
				Source:       "/dev/loop1",
				MountPoint:   "/tmp/mount1",
				FsType:       "ext4",
				MountOptions: []string{"remount", "rw"},
			}}, nil).Times(2)
			mountLib.EXPECT().PathExists("/tmp/mount1").Return(true, nil).Times(2)
			mountLib.EXPECT().Unmount("/tmp/mount1").Return(nil)
			osLib.EXPECT().RemoveAll("/tmp/mount1").Return(fmt.Errorf("test error"))
			Expect(mountUtils.UnmountAndRemove("/tmp/mount1")).To(HaveOccurred())
		})
		It("should succeed", func() {
			mountLib.EXPECT().PathExists("/tmp/mount1").Return(true, nil).Times(2)
			mountLib.EXPECT().ParseMountInfo(
				mountLibWrapper.DefaultProcMountInfoPath).Return([]mountLibWrapper.MountInfo{{
				Source:       "/dev/loop1",
				MountPoint:   "/tmp/mount1",
				FsType:       "ext4",
				MountOptions: []string{"remount", "rw"},
			}}, nil).Times(2)
			mountLib.EXPECT().Unmount("/tmp/mount1").Return(nil)
			osLib.EXPECT().RemoveAll("/tmp/mount1").Return(nil)
			Expect(mountUtils.UnmountAndRemove("/tmp/mount1")).NotTo(HaveOccurred())
		})
		It("no matching mounts", func() {
			mountLib.EXPECT().PathExists("/tmp/mount1").Return(true, nil)
			mountLib.EXPECT().ParseMountInfo(
				mountLibWrapper.DefaultProcMountInfoPath).Return([]mountLibWrapper.MountInfo{{
				Source:       "_not_match",
				MountPoint:   "_not_match",
				FsType:       "ext4",
				MountOptions: []string{"remount", "rw"},
			}}, nil)
			osLib.EXPECT().RemoveAll("/tmp/mount1").Return(nil)
			Expect(mountUtils.UnmountAndRemove("/tmp/mount1")).NotTo(HaveOccurred())
		})
	})

	Context("EnsureFileExist", func() {
		It("path already exist", func() {
			mountLib.EXPECT().PathExists("/tmp/mount1/test_file").Return(true, nil)
			Expect(mountUtils.EnsureFileExist("/tmp/mount1/test_file", os.FileMode(0750))).NotTo(HaveOccurred())
		})
		It("path exist check - failed", func() {
			mountLib.EXPECT().PathExists("/tmp/mount1/test_file").Return(false, fmt.Errorf("test error"))
			Expect(mountUtils.EnsureFileExist("/tmp/mount1/test_file", os.FileMode(0750))).To(HaveOccurred())
		})
		It("failed to create dir", func() {
			mountLib.EXPECT().PathExists("/tmp/mount1/test_file").Return(false, nil)
			osLib.EXPECT().MkdirAll("/tmp/mount1", gomock.Any()).Return(fmt.Errorf("test error"))
			Expect(mountUtils.EnsureFileExist("/tmp/mount1/test_file", os.FileMode(0750))).To(HaveOccurred())
		})
		It("file create failed", func() {
			mountLib.EXPECT().PathExists("/tmp/mount1/test_file").Return(false, nil)
			osLib.EXPECT().MkdirAll("/tmp/mount1", gomock.Any()).Return(nil)
			osLib.EXPECT().WriteFile("/tmp/mount1/test_file", []byte{}, os.FileMode(0750)).Return(fmt.Errorf("test error"))
			Expect(mountUtils.EnsureFileExist("/tmp/mount1/test_file", os.FileMode(0750))).To(HaveOccurred())
		})
		It("file created", func() {
			mountLib.EXPECT().PathExists("/tmp/mount1/test_file").Return(false, nil)
			osLib.EXPECT().MkdirAll("/tmp/mount1", gomock.Any()).Return(nil)
			osLib.EXPECT().WriteFile("/tmp/mount1/test_file", []byte{}, os.FileMode(0750)).Return(nil)
			Expect(mountUtils.EnsureFileExist("/tmp/mount1/test_file", os.FileMode(0750))).NotTo(HaveOccurred())
		})
	})
	Context("CheckMountExists", func() {
		It("error", func() {
			mountLib.EXPECT().ParseMountInfo(gomock.Any()).Return(nil, fmt.Errorf("test error"))
			found, _, err := mountUtils.CheckMountExists("/dev/loop1", "/tmp/mount1")
			Expect(found).To(BeFalse())
			Expect(err).To(HaveOccurred())
		})
		It("different target path", func() {
			mountLib.EXPECT().PathExists(gomock.Any()).Return(true, nil)
			mountLib.EXPECT().IsCorruptedMnt(nil).Return(false)
			mountLib.EXPECT().ParseMountInfo(gomock.Any()).Return([]mountLibWrapper.MountInfo{{
				Source:       "/dev/loop1",
				MountPoint:   "/tmp/mount2",
				FsType:       "ext4",
				MountOptions: []string{"remount", "rw"},
			}}, nil)
			found, _, err := mountUtils.CheckMountExists("/dev/loop1", "/tmp/mount1")
			Expect(found).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
		It("no mounts for source", func() {
			mountLib.EXPECT().ParseMountInfo(gomock.Any()).Return([]mountLibWrapper.MountInfo{{
				Source:       "/dev/loop2",
				MountPoint:   "/tmp/mount2",
				FsType:       "ext4",
				MountOptions: []string{"remount", "rw"},
			}}, nil)
			found, _, err := mountUtils.CheckMountExists("/dev/loop1", "/tmp/mount1")
			Expect(found).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
		It("found mount", func() {
			mountLib.EXPECT().PathExists(gomock.Any()).Return(true, nil)
			mountLib.EXPECT().IsCorruptedMnt(nil).Return(false)
			mountLib.EXPECT().ParseMountInfo(gomock.Any()).Return([]mountLibWrapper.MountInfo{{
				Source:       "/dev/loop1",
				MountPoint:   "/tmp/mount1",
				FsType:       "ext4",
				MountOptions: []string{"remount", "rw"},
			}}, nil)
			found, info, err := mountUtils.CheckMountExists("/dev/loop1", "/tmp/mount1")
			Expect(found).To(BeTrue())
			Expect(info.MountPoint).To(BeEquivalentTo("/tmp/mount1"))
			Expect(err).NotTo(HaveOccurred())
		})
		It("removed device", func() {
			mountLib.EXPECT().ParseMountInfo(gomock.Any()).Return([]mountLibWrapper.MountInfo{{
				Root:         "/loop1//deleted",
				Source:       "udev",
				MountPoint:   "/tmp/mount1",
				FsType:       "ext4",
				MountOptions: []string{"remount", "rw"},
			}}, nil)
			found, _, err := mountUtils.CheckMountExists("/dev/loop1", "/tmp/mount1")
			Expect(found).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
		It("path exist check failed", func() {
			mountLib.EXPECT().ParseMountInfo(gomock.Any()).Return([]mountLibWrapper.MountInfo{{
				Source:     "udev",
				Root:       "/loop1",
				MountPoint: "/tmp/mount1",
			}}, nil)
			mountLib.EXPECT().PathExists(gomock.Any()).Return(false, fmt.Errorf("test error"))
			mountLib.EXPECT().IsCorruptedMnt(gomock.Any()).Return(false)
			found, _, err := mountUtils.CheckMountExists("/dev/loop1", "/tmp/mount1")
			Expect(found).To(BeFalse())
			Expect(err).To(HaveOccurred())
		})
		It("invalid mount", func() {
			mountLib.EXPECT().ParseMountInfo(gomock.Any()).Return([]mountLibWrapper.MountInfo{{
				Source:     "udev",
				Root:       "/loop1",
				MountPoint: "/tmp/mount1",
			}}, nil)
			mountLib.EXPECT().PathExists(gomock.Any()).Return(false, &os.PathError{})
			mountLib.EXPECT().IsCorruptedMnt(gomock.Any()).Return(true)
			found, _, err := mountUtils.CheckMountExists("/dev/loop1", "/tmp/mount1")
			Expect(found).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
