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

package hostcniprovisioner_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	hostcniprovisioner "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/host"
	networkhelperMock "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/networkhelper/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"go.uber.org/mock/gomock"
	"golang.org/x/exp/maps"
	clock "k8s.io/utils/clock/testing"
	kexec "k8s.io/utils/exec"
	kexecTesting "k8s.io/utils/exec/testing"
)

var _ = Describe("Host CNI Provisioner", func() {
	Context("When it runs once for the first time", func() {
		It("should configure the system fully", func() {
			testCtrl := gomock.NewController(GinkgoT())
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			fakeExec := &kexecTesting.FakeExec{}
			pfIPNet, err := netlink.ParseIPNet("192.168.1.2/24")
			Expect(err).ToNot(HaveOccurred())
			provisioner := hostcniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), networkhelper, fakeExec, "ens25f0np0", pfIPNet)

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "hostcniprovisioner")
			DeferCleanup(os.RemoveAll, tmpDir)
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir

			ovnDBsPath := filepath.Join(tmpDir, "/var/lib/ovn-ic/etc/")
			err = os.MkdirAll(ovnDBsPath, 0755)
			Expect(err).NotTo(HaveOccurred())
			_, err = os.Create(filepath.Join(ovnDBsPath, "file1.db"))
			Expect(err).NotTo(HaveOccurred())
			_, err = os.Create(filepath.Join(ovnDBsPath, "file2.db"))
			Expect(err).NotTo(HaveOccurred())

			sriovNumVfsPath := filepath.Join(tmpDir, "/sys/class/net/ens25f0np0/device/sriov_numvfs")
			err = os.MkdirAll(filepath.Dir(sriovNumVfsPath), 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(sriovNumVfsPath, []byte(strconv.Itoa(2)), 0444)
			Expect(err).NotTo(HaveOccurred())

			nmConfigPath := filepath.Join(tmpDir, "/etc/NetworkManager/conf.d/")
			err = os.MkdirAll(nmConfigPath, 0755)
			Expect(err).NotTo(HaveOccurred())

			networkhelper.EXPECT().LinkIPAddressExists("ens25f0np0", pfIPNet).Return(false, nil)
			networkhelper.EXPECT().SetLinkIPAddress("ens25f0np0", pfIPNet)

			networkhelper.EXPECT().DummyLinkExists("pf0vf0").Return(false, nil)
			networkhelper.EXPECT().AddDummyLink("pf0vf0")
			networkhelper.EXPECT().DummyLinkExists("pf0vf1").Return(false, nil)
			networkhelper.EXPECT().AddDummyLink("pf0vf1")

			networkhelper.EXPECT().NeighbourExists(net.ParseIP("169.254.169.1"), "br-ex").Return(true, nil)
			networkhelper.EXPECT().DeleteNeighbour(net.ParseIP("169.254.169.1"), "br-ex")
			networkhelper.EXPECT().NeighbourExists(net.ParseIP("169.254.169.4"), "br-ex").Return(true, nil)
			networkhelper.EXPECT().DeleteNeighbour(net.ParseIP("169.254.169.4"), "br-ex")
			hostMasqueradeIP, _ := netlink.ParseIPNet("169.254.169.2/29")
			networkhelper.EXPECT().LinkIPAddressExists("br-ex", hostMasqueradeIP).Return(true, nil)
			networkhelper.EXPECT().DeleteLinkIPAddress("br-ex", hostMasqueradeIP)
			kubernetesServiceCIDR, _ := netlink.ParseIPNet("172.30.0.0/16")
			networkhelper.EXPECT().RouteExists(kubernetesServiceCIDR, net.ParseIP("169.254.169.4"), "br-ex").Return(true, nil)
			networkhelper.EXPECT().DeleteRoute(kubernetesServiceCIDR, net.ParseIP("169.254.169.4"), "br-ex")
			ovnMasqueradeIP, _ := netlink.ParseIPNet("169.254.169.1/32")
			networkhelper.EXPECT().RouteExists(ovnMasqueradeIP, nil, "br-ex").Return(true, nil)
			networkhelper.EXPECT().DeleteRoute(ovnMasqueradeIP, nil, "br-ex")

			fakeExec.CommandScript = append(fakeExec.CommandScript, kexecTesting.FakeCommandAction(func(cmd string, args ...string) kexec.Cmd {
				Expect(cmd).To(Equal("systemctl"))
				Expect(args).To(Equal([]string{"reload", "NetworkManager"}))
				return kexec.New().Command("echo")
			}))

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())

			verifyFS(tmpDir, ovnDBsPath, map[string]verifyFSEntry{
				"/var/lib/ovn-ic/etc": {
					isDir: true,
				},
				"/var/lib/ovn-ic/etc/dpf-cleanup-done": {},
			})

			verifyFS(tmpDir, nmConfigPath, map[string]verifyFSEntry{
				"/etc/NetworkManager/conf.d": {
					isDir: true,
				},
				"/etc/NetworkManager/conf.d/dpf.conf": {
					content: "[keyfile]\nunmanaged-devices=interface-name:ens25f0np0",
				},
			})

			By("Asserting fake filesystem")
			assertFakeFilesystem(tmpDir)
		})
		It("should ensure that the OVN management link is always in place", func(ctx context.Context) {
			testCtrl := gomock.NewController(GinkgoT())
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			ctx, cancel := context.WithCancel(ctx)
			c := clock.NewFakeClock(time.Now())
			provisioner := hostcniprovisioner.New(ctx, c, networkhelper, nil, "", nil)

			networkhelper.EXPECT().DummyLinkExists("pf0vf0").DoAndReturn(func(link string) (bool, error) {
				c.Step(2 * time.Second)
				return false, errors.New("some-error")
			})

			networkhelper.EXPECT().DummyLinkExists("pf0vf0").DoAndReturn(func(link string) (bool, error) {
				By("error occurred, it retries")
				return false, nil
			})
			networkhelper.EXPECT().AddDummyLink("pf0vf0").Do(func(link string) {
				c.Step(2 * time.Second)
			})

			networkhelper.EXPECT().DummyLinkExists("pf0vf0").DoAndReturn(func(link string) (bool, error) {
				By("link exists, it should not add a link again")
				cancel()
				return true, nil
			})

			Eventually(func(g Gomega) {
				c.Step(2 * time.Second)
				provisioner.EnsureConfiguration()
			}).Should(BeNil())

		}, SpecTimeout(5*time.Second))
	})
	Context("When checking for idempotency", func() {
		It("should not error out on subsequent runs when network calls are mocked", func(ctx context.Context) {
			testCtrl := gomock.NewController(GinkgoT(), gomock.WithOverridableExpectations())
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			fakeExec := &kexecTesting.FakeExec{}
			pfIPNet, err := netlink.ParseIPNet("192.168.1.2/24")
			Expect(err).ToNot(HaveOccurred())
			provisioner := hostcniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), networkhelper, fakeExec, "ens25f0np0", pfIPNet)

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "hostcniprovisioner")
			DeferCleanup(os.RemoveAll, tmpDir)
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir

			ovnDBsPath := filepath.Join(tmpDir, "/var/lib/ovn-ic/etc/")
			err = os.MkdirAll(ovnDBsPath, 0755)
			Expect(err).NotTo(HaveOccurred())

			sriovNumVfsPath := filepath.Join(tmpDir, "/sys/class/net/ens25f0np0/device/sriov_numvfs")
			err = os.MkdirAll(filepath.Dir(sriovNumVfsPath), 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(sriovNumVfsPath, []byte(strconv.Itoa(2)), 0444)
			Expect(err).NotTo(HaveOccurred())

			nmConfigPath := filepath.Join(tmpDir, "/etc/NetworkManager/conf.d/")
			err = os.MkdirAll(nmConfigPath, 0755)
			Expect(err).NotTo(HaveOccurred())

			networkHelperMockAll(networkhelper)

			fakeExec.CommandScript = append(fakeExec.CommandScript, kexecTesting.FakeCommandAction(func(cmd string, args ...string) kexec.Cmd {
				Expect(cmd).To(Equal("systemctl"))
				Expect(args).To(Equal([]string{"reload", "NetworkManager"}))
				return kexec.New().Command("echo")
			}))

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())
		})
		It("should not cleanup the OVN databases if original cleanup is already done", func(ctx context.Context) {
			testCtrl := gomock.NewController(GinkgoT(), gomock.WithOverridableExpectations())
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			fakeExec := &kexecTesting.FakeExec{}
			pfIPNet, err := netlink.ParseIPNet("192.168.1.2/24")
			Expect(err).ToNot(HaveOccurred())
			provisioner := hostcniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), networkhelper, fakeExec, "ens25f0np0", pfIPNet)

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "hostcniprovisioner")
			DeferCleanup(os.RemoveAll, tmpDir)
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir

			ovnDBsPath := filepath.Join(tmpDir, "/var/lib/ovn-ic/etc/")
			err = os.MkdirAll(ovnDBsPath, 0755)
			Expect(err).NotTo(HaveOccurred())
			_, err = os.Create(filepath.Join(ovnDBsPath, "file1.db"))
			Expect(err).NotTo(HaveOccurred())
			_, err = os.Create(filepath.Join(ovnDBsPath, "dpf-cleanup-done"))
			Expect(err).NotTo(HaveOccurred())

			sriovNumVfsPath := filepath.Join(tmpDir, "/sys/class/net/ens25f0np0/device/sriov_numvfs")
			err = os.MkdirAll(filepath.Dir(sriovNumVfsPath), 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(sriovNumVfsPath, []byte(strconv.Itoa(2)), 0444)
			Expect(err).NotTo(HaveOccurred())

			nmConfigPath := filepath.Join(tmpDir, "/etc/NetworkManager/conf.d/")
			err = os.MkdirAll(nmConfigPath, 0755)
			Expect(err).NotTo(HaveOccurred())

			networkHelperMockAll(networkhelper)

			fakeExec.CommandScript = append(fakeExec.CommandScript, kexecTesting.FakeCommandAction(func(cmd string, args ...string) kexec.Cmd {
				Expect(cmd).To(Equal("systemctl"))
				Expect(args).To(Equal([]string{"reload", "NetworkManager"}))
				return kexec.New().Command("echo")
			}))

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())

			verifyFS(tmpDir, ovnDBsPath, map[string]verifyFSEntry{
				"/var/lib/ovn-ic/etc": {
					isDir: true,
				},
				"/var/lib/ovn-ic/etc/dpf-cleanup-done": {},
				"/var/lib/ovn-ic/etc/file1.db":         {},
			})
		})
		It("should not reload the NetworkManager if the config file identical", func(ctx context.Context) {
			testCtrl := gomock.NewController(GinkgoT(), gomock.WithOverridableExpectations())
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			fakeExec := &kexecTesting.FakeExec{}
			pfIPNet, err := netlink.ParseIPNet("192.168.1.2/24")
			Expect(err).ToNot(HaveOccurred())
			provisioner := hostcniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), networkhelper, fakeExec, "ens25f0np0", pfIPNet)

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "hostcniprovisioner")
			DeferCleanup(os.RemoveAll, tmpDir)
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir

			ovnDBsPath := filepath.Join(tmpDir, "/var/lib/ovn-ic/etc/")
			err = os.MkdirAll(ovnDBsPath, 0755)
			Expect(err).NotTo(HaveOccurred())
			_, err = os.Create(filepath.Join(ovnDBsPath, "file1.db"))
			Expect(err).NotTo(HaveOccurred())
			_, err = os.Create(filepath.Join(ovnDBsPath, "dpf-cleanup-done"))
			Expect(err).NotTo(HaveOccurred())

			sriovNumVfsPath := filepath.Join(tmpDir, "/sys/class/net/ens25f0np0/device/sriov_numvfs")
			err = os.MkdirAll(filepath.Dir(sriovNumVfsPath), 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(sriovNumVfsPath, []byte(strconv.Itoa(2)), 0444)
			Expect(err).NotTo(HaveOccurred())

			nmConfigPath := filepath.Join(tmpDir, "/etc/NetworkManager/conf.d/")
			err = os.MkdirAll(nmConfigPath, 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(filepath.Join(nmConfigPath, "dpf.conf"), []byte("[keyfile]\nunmanaged-devices=interface-name:ens25f0np0"), 0644)
			Expect(err).NotTo(HaveOccurred())

			networkHelperMockAll(networkhelper)

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func assertFakeFilesystem(tmpDir string) {
	expectedEntries := map[string]verifyFSEntry{
		"/var/dpf/sys/class/net": {
			isDir: true,
		},
		"/var/dpf/sys/class/net/ens25f0np0": {
			isDir: true,
		},
		"/var/dpf/sys/class/net/ens25f0np0/subsystem": {
			isSymlink: true,
			content:   "/var/dpf/sys/class/net",
		},
		"/var/dpf/sys/class/net/ens25f0np0/phys_switch_id": {
			isDir:   false,
			content: "custom_value",
		},
		"/var/dpf/sys/class/net/ens25f0np0/phys_port_name": {
			isDir:   false,
			content: "p0",
		},
		"/var/dpf/sys/class/net/pf0vf0": {
			isDir: true,
		},
		"/var/dpf/sys/class/net/pf0vf0/phys_switch_id": {
			isDir:   false,
			content: "custom_value",
		},
		"/var/dpf/sys/class/net/pf0vf0/phys_port_name": {
			isDir:   false,
			content: "c1pf0vf0",
		},
		"/var/dpf/sys/class/net/pf0vf1": {
			isDir: true,
		},
		"/var/dpf/sys/class/net/pf0vf1/phys_switch_id": {
			isDir:   false,
			content: "custom_value",
		},
		"/var/dpf/sys/class/net/pf0vf1/phys_port_name": {
			isDir:   false,
			content: "c1pf0vf1",
		},
	}

	pathToVerify := filepath.Join(tmpDir, "/var/dpf/sys/class/net")
	verifyFS(tmpDir, pathToVerify, expectedEntries)
}

// networkHelperMockAll mocks all networkhelper functions. Useful for tests where we don't test
func networkHelperMockAll(networkHelper *networkhelperMock.MockNetworkHelper) {
	networkHelper.EXPECT().AddDummyLink(gomock.Any()).AnyTimes()
	networkHelper.EXPECT().DeleteLinkIPAddress(gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().DeleteNeighbour(gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().DeleteRoute(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().DummyLinkExists(gomock.Any()).AnyTimes()
	networkHelper.EXPECT().LinkIPAddressExists(gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().NeighbourExists(gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().RouteExists(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().SetLinkIPAddress(gomock.Any(), gomock.Any()).AnyTimes()
}

// verifyFSEntry is a struct used in verifyFS that helps define the fs entries that we expect
type verifyFSEntry struct {
	isDir     bool
	isSymlink bool
	content   string
}

// verifyFS verifies that the filesystem matches the expectedEntries specified
func verifyFS(tmpDir string, pathToWalk string, expectedEntries map[string]verifyFSEntry) {
	foundPaths := []string{}
	err := filepath.WalkDir(pathToWalk, func(path string, dirEntry fs.DirEntry, err error) error {
		Expect(err).ToNot(HaveOccurred())
		By(fmt.Sprintf("Checking %s", path))

		pathWithoutTmpDir, err := filepath.Rel(tmpDir, path)
		Expect(err).ToNot(HaveOccurred())
		pathWithoutTmpDir = filepath.Join("/", pathWithoutTmpDir)
		foundPaths = append(foundPaths, pathWithoutTmpDir)

		expectedEntry, ok := expectedEntries[pathWithoutTmpDir]
		Expect(ok).To(BeTrue())
		Expect(dirEntry.IsDir()).To(Equal(expectedEntry.isDir))
		Expect(dirEntry.Type().Type() == fs.ModeSymlink).To(Equal(expectedEntry.isSymlink))

		if !expectedEntry.isDir && !expectedEntry.isSymlink {
			content, err := os.ReadFile(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(content).To(Equal([]byte(expectedEntry.content)))
		}

		if expectedEntry.isSymlink {
			link, err := filepath.EvalSymlinks(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(link).To(Equal(link))
		}

		return nil
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(foundPaths).To(ConsistOf(maps.Keys(expectedEntries)))
}
