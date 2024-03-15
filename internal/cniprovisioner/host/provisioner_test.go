/*
Copyright 2024.

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
	"net"
	"os"
	"path/filepath"

	hostcniprovisioner "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/host"
	networkhelperMock "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/networkhelper/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Host CNI Provisioner", func() {
	Context("When it runs once for the first time", func() {
		It("should configure the system fully", func() {
			testCtrl := gomock.NewController(GinkgoT())
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			provisioner := hostcniprovisioner.New(networkhelper)

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "hostcniprovisioner")
			defer func() {
				err := os.RemoveAll(tmpDir)
				Expect(err).ToNot(HaveOccurred())
			}()
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir
			ovnDBsPath := filepath.Join(tmpDir, "/var/lib/ovn-ic/etc/")
			err = os.MkdirAll(ovnDBsPath, 0755)
			Expect(err).NotTo(HaveOccurred())

			_, err = os.Create(filepath.Join(ovnDBsPath, "file1.db"))
			Expect(err).NotTo(HaveOccurred())
			_, err = os.Create(filepath.Join(ovnDBsPath, "file2.db"))
			Expect(err).NotTo(HaveOccurred())

			ipNet, _ := netlink.ParseIPNet("192.168.1.2/24")
			networkhelper.EXPECT().SetLinkIPAddress("ens2f0np0", ipNet)

			networkhelper.EXPECT().DeleteNeighbour(net.ParseIP("169.254.169.1"), "br-ex")
			networkhelper.EXPECT().DeleteNeighbour(net.ParseIP("169.254.169.4"), "br-ex")
			hostMasqueradeIP, _ := netlink.ParseIPNet("169.254.169.2/29")
			networkhelper.EXPECT().DeleteLinkIPAddress("br-ex", hostMasqueradeIP)
			kubernetesServiceCIDR, _ := netlink.ParseIPNet("172.30.0.0/16")
			networkhelper.EXPECT().DeleteRoute(kubernetesServiceCIDR, net.ParseIP("169.254.169.4"), "br-ex")
			ovnMasqueradeIP, _ := netlink.ParseIPNet("169.254.169.1/32")
			networkhelper.EXPECT().DeleteRoute(ovnMasqueradeIP, nil, "br-ex")

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())

			files, err := os.ReadDir(ovnDBsPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(BeEmpty())
		})
	})
})
