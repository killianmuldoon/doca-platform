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

package dpucniprovisioner_test

import (
	"context"
	"net"
	"os"
	"time"

	dpucniprovisioner "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/dpu"
	networkhelperMock "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/networkhelper/mock"
	ovsclientMock "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/ovsclient/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"go.uber.org/mock/gomock"
	clock "k8s.io/utils/clock/testing"
	kexecTesting "k8s.io/utils/exec/testing"
	"k8s.io/utils/ptr"
)

var _ = Describe("DPU CNI Provisioner", func() {
	Context("When it runs once for the first time", func() {
		It("should configure the system fully when different subnets per DPU", func() {
			testCtrl := gomock.NewController(GinkgoT())
			ovsClient := ovsclientMock.NewMockOVSClient(testCtrl)
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			fakeExec := &kexecTesting.FakeExec{}
			vtepIPNet, err := netlink.ParseIPNet("192.168.1.1/24")
			Expect(err).ToNot(HaveOccurred())
			gateway := net.ParseIP("192.168.1.10/24")
			vtepCIDR, err := netlink.ParseIPNet("192.168.1.0/23")
			Expect(err).ToNot(HaveOccurred())
			hostCIDR, err := netlink.ParseIPNet("10.0.100.1/24")
			Expect(err).ToNot(HaveOccurred())
			provisioner := dpucniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), ovsClient, networkhelper, fakeExec, vtepIPNet, gateway, vtepCIDR, hostCIDR)

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "dpucniprovisioner")
			defer func() {
				err := os.RemoveAll(tmpDir)
				Expect(err).ToNot(HaveOccurred())
			}()
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir

			networkhelper.EXPECT().LinkIPAddressExists("br-ovn", vtepIPNet)
			networkhelper.EXPECT().SetLinkIPAddress("br-ovn", vtepIPNet)
			networkhelper.EXPECT().SetLinkUp("br-ovn")
			networkhelper.EXPECT().RouteExists(vtepCIDR, gateway, "br-ovn")
			networkhelper.EXPECT().AddRoute(vtepCIDR, gateway, "br-ovn", nil)
			networkhelper.EXPECT().RouteExists(hostCIDR, gateway, "br-ovn")
			networkhelper.EXPECT().AddRoute(hostCIDR, gateway, "br-ovn", ptr.To[int](10000))

			ovsClient.EXPECT().SetOVNEncapIP(net.ParseIP("192.168.1.1"))

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())
		})
		It("should configure the system fully when same subnet across DPUs", func() {
			testCtrl := gomock.NewController(GinkgoT())
			ovsClient := ovsclientMock.NewMockOVSClient(testCtrl)
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			fakeExec := &kexecTesting.FakeExec{}
			vtepIPNet, err := netlink.ParseIPNet("192.168.1.1/24")
			Expect(err).ToNot(HaveOccurred())
			gateway := net.ParseIP("192.168.1.10/24")
			_, vtepCIDR, err := net.ParseCIDR("192.168.1.0/24")
			Expect(err).ToNot(HaveOccurred())
			_, hostCIDR, err := net.ParseCIDR("10.0.100.1/24")
			Expect(err).ToNot(HaveOccurred())
			provisioner := dpucniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), ovsClient, networkhelper, fakeExec, vtepIPNet, gateway, vtepCIDR, hostCIDR)

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "dpucniprovisioner")
			defer func() {
				err := os.RemoveAll(tmpDir)
				Expect(err).ToNot(HaveOccurred())
			}()
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir

			Expect(vtepIPNet.String()).To(Equal("192.168.1.1/24"))
			_, vtepNetwork, _ := net.ParseCIDR(vtepIPNet.String())
			Expect(vtepNetwork.String()).To(Equal("192.168.1.0/24"))
			Expect(vtepCIDR).To(Equal(vtepNetwork))
			networkhelper.EXPECT().LinkIPAddressExists("br-ovn", vtepIPNet)
			networkhelper.EXPECT().SetLinkIPAddress("br-ovn", vtepIPNet)
			networkhelper.EXPECT().SetLinkUp("br-ovn")
			networkhelper.EXPECT().RouteExists(hostCIDR, gateway, "br-ovn")
			networkhelper.EXPECT().AddRoute(hostCIDR, gateway, "br-ovn", ptr.To[int](10000))

			ovsClient.EXPECT().SetOVNEncapIP(net.ParseIP("192.168.1.1"))

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())
		})
	})
	Context("When checking for idempotency", func() {
		It("should not error out on subsequent runs when network calls and OVS calls are fully mocked", func(ctx context.Context) {
			testCtrl := gomock.NewController(GinkgoT())
			ovsClient := ovsclientMock.NewMockOVSClient(testCtrl)
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			fakeExec := &kexecTesting.FakeExec{}
			vtepIPNet, err := netlink.ParseIPNet("192.168.1.1/24")
			Expect(err).ToNot(HaveOccurred())
			gateway := net.ParseIP("192.168.1.10/24")
			vtepCIDR, err := netlink.ParseIPNet("192.168.1.0/23")
			Expect(err).ToNot(HaveOccurred())
			hostCIDR, err := netlink.ParseIPNet("10.0.100.1/24")
			Expect(err).ToNot(HaveOccurred())
			provisioner := dpucniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), ovsClient, networkhelper, fakeExec, vtepIPNet, gateway, vtepCIDR, hostCIDR)

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "dpucniprovisioner")
			defer func() {
				err := os.RemoveAll(tmpDir)
				Expect(err).ToNot(HaveOccurred())
			}()
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir

			networkHelperMockAll(networkhelper)
			ovsClientMockAll(ovsClient)

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())
		})
		It("should not error out when network and ovs clients are mocked like in the real world", func(ctx context.Context) {
			testCtrl := gomock.NewController(GinkgoT())
			ovsClient := ovsclientMock.NewMockOVSClient(testCtrl)
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			fakeExec := &kexecTesting.FakeExec{}
			vtepIPNet, err := netlink.ParseIPNet("192.168.1.1/24")
			Expect(err).ToNot(HaveOccurred())
			gateway := net.ParseIP("192.168.1.10/24")
			vtepCIDR, err := netlink.ParseIPNet("192.168.1.0/23")
			Expect(err).ToNot(HaveOccurred())
			hostCIDR, err := netlink.ParseIPNet("10.0.100.1/24")
			Expect(err).ToNot(HaveOccurred())
			provisioner := dpucniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), ovsClient, networkhelper, fakeExec, vtepIPNet, gateway, vtepCIDR, hostCIDR)

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "dpucniprovisioner")
			defer func() {
				err := os.RemoveAll(tmpDir)
				Expect(err).ToNot(HaveOccurred())
			}()
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir

			By("Checking the first run")
			networkhelper.EXPECT().LinkIPAddressExists("br-ovn", vtepIPNet)
			networkhelper.EXPECT().SetLinkIPAddress("br-ovn", vtepIPNet)
			networkhelper.EXPECT().SetLinkUp("br-ovn")
			networkhelper.EXPECT().RouteExists(vtepCIDR, gateway, "br-ovn")
			networkhelper.EXPECT().AddRoute(vtepCIDR, gateway, "br-ovn", nil)
			networkhelper.EXPECT().RouteExists(hostCIDR, gateway, "br-ovn")
			networkhelper.EXPECT().AddRoute(hostCIDR, gateway, "br-ovn", ptr.To[int](10000))

			ovsClient.EXPECT().SetOVNEncapIP(net.ParseIP("192.168.1.1"))

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())

			By("Checking the second run")
			networkhelper.EXPECT().LinkIPAddressExists("br-ovn", vtepIPNet).Return(true, nil)
			networkhelper.EXPECT().SetLinkUp("br-ovn")
			networkhelper.EXPECT().RouteExists(vtepCIDR, gateway, "br-ovn").Return(true, nil)
			networkhelper.EXPECT().RouteExists(hostCIDR, gateway, "br-ovn").Return(true, nil)

			ovsClient.EXPECT().SetOVNEncapIP(net.ParseIP("192.168.1.1"))

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

// networkHelperMockAll mocks all networkhelper functions. Useful for tests where we don't test the network calls
func networkHelperMockAll(networkHelper *networkhelperMock.MockNetworkHelper) {
	networkHelper.EXPECT().AddRoute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().LinkIPAddressExists(gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().RouteExists(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().SetLinkIPAddress(gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().SetLinkUp(gomock.Any()).AnyTimes()
}

// ovsClientMockAll mocks all ovsclient functions. Useful for tests where we don't test the ovsclient calls
func ovsClientMockAll(ovsClient *ovsclientMock.MockOVSClient) {
	ovsClient.EXPECT().SetOVNEncapIP(gomock.Any()).AnyTimes()
}
