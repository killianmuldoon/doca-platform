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
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"

	dpucniprovisioner "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/dpu"
	networkhelperMock "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/networkhelper/mock"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/ovsclient"
	ovsclientMock "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/ovsclient/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"go.uber.org/mock/gomock"
	clock "k8s.io/utils/clock/testing"
	kexec "k8s.io/utils/exec"
	kexecTesting "k8s.io/utils/exec/testing"
	"k8s.io/utils/ptr"
)

const (
	ovsSystemdConfigContentDefault = `# This is a POSIX shell fragment                -*- sh -*-

# FORCE_COREFILES: If 'yes' then core files will be enabled.
# FORCE_COREFILES=yes

# OVS_CTL_OPTS: Extra options to pass to ovs-ctl.  This is, for example,
# a suitable place to specify --ovs-vswitchd-wrapper=valgrind.
# OVS_CTL_OPTS=`
	ovsSystemdConfigContentPopulated = `# This is a POSIX shell fragment                -*- sh -*-

# FORCE_COREFILES: If 'yes' then core files will be enabled.
# FORCE_COREFILES=yes

# OVS_CTL_OPTS: Extra options to pass to ovs-ctl.  This is, for example,
# a suitable place to specify --ovs-vswitchd-wrapper=valgrind.
# OVS_CTL_OPTS=
OVS_CTL_OPTS="--ovsdb-server-options=--remote=ptcp:8500"`
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
			provisioner := dpucniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), ovsClient, networkhelper, fakeExec, vtepIPNet, gateway, vtepCIDR, hostCIDR, "ens25f0np0")

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "dpucniprovisioner")
			defer func() {
				err := os.RemoveAll(tmpDir)
				Expect(err).ToNot(HaveOccurred())
			}()
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir
			ovsSystemdConfigPath := filepath.Join(tmpDir, "/etc/default/openvswitch-switch")
			err = os.MkdirAll(filepath.Dir(ovsSystemdConfigPath), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(ovsSystemdConfigPath, []byte(ovsSystemdConfigContentDefault), 0644)
			Expect(err).NotTo(HaveOccurred())
			ovsClient.EXPECT().SetDOCAInit(true)
			fakeExec.CommandScript = append(fakeExec.CommandScript, kexecTesting.FakeCommandAction(func(cmd string, args ...string) kexec.Cmd {
				Expect(cmd).To(Equal("systemctl"))
				Expect(args).To(Equal([]string{"restart", "openvswitch-switch.service"}))
				return kexec.New().Command("echo")
			}))

			ovsClient.EXPECT().DeleteBridgeIfExists("ovsbr1")
			ovsClient.EXPECT().DeleteBridgeIfExists("ovsbr2")

			ovsClient.EXPECT().AddBridgeIfNotExists("br-int")
			ovsClient.EXPECT().SetBridgeDataPathType("br-int", ovsclient.NetDev)
			ovsClient.EXPECT().SetBridgeController("br-int", "ptcp:8510:169.254.55.1")
			ovsClient.EXPECT().AddBridgeIfNotExists("ens25f0np0")
			ovsClient.EXPECT().SetBridgeDataPathType("ens25f0np0", ovsclient.NetDev)
			ovsClient.EXPECT().SetBridgeController("ens25f0np0", "ptcp:8511")
			ovsClient.EXPECT().AddBridgeIfNotExists("br-ovn")
			ovsClient.EXPECT().SetBridgeDataPathType("br-ovn", ovsclient.NetDev)

			ovsClient.EXPECT().AddPortIfNotExists("ens25f0np0", "ens25f0np0-to-br-ovn").Return(errors.New("some error related to creating patch ports"))
			ovsClient.EXPECT().SetPortType("ens25f0np0-to-br-ovn", ovsclient.Patch)
			ovsClient.EXPECT().SetPatchPortPeer("ens25f0np0-to-br-ovn", "br-ovn-to-ens25f0np0")
			ovsClient.EXPECT().AddPortIfNotExists("br-ovn", "br-ovn-to-ens25f0np0").Return(errors.New("some error related to creating patch ports"))
			ovsClient.EXPECT().SetPortType("br-ovn-to-ens25f0np0", ovsclient.Patch)
			ovsClient.EXPECT().SetPatchPortPeer("br-ovn-to-ens25f0np0", "ens25f0np0-to-br-ovn")

			ovsClient.EXPECT().AddPortIfNotExists("br-ovn", "vtep0")
			ovsClient.EXPECT().SetPortType("vtep0", ovsclient.Internal)

			ovsClient.EXPECT().AddPortIfNotExists("ens25f0np0", "pf0hpf")
			ovsClient.EXPECT().SetPortType("pf0hpf", ovsclient.DPDK)
			ovsClient.EXPECT().SetBridgeHostToServicePort("ens25f0np0", "pf0hpf")
			ovsClient.EXPECT().SetBridgeUplinkPort("ens25f0np0", "ens25f0np0-to-br-ovn")

			ovsClient.EXPECT().SetOVNEncapIP(net.ParseIP("192.168.1.1"))

			mac, _ := net.ParseMAC("00:00:00:00:00:01")
			networkhelper.EXPECT().GetPFRepMACAddress("pf0hpf").Return(mac, nil)
			ovsClient.EXPECT().SetBridgeMAC("ens25f0np0", mac)

			networkhelper.EXPECT().LinkIPAddressExists("vtep0", vtepIPNet)
			networkhelper.EXPECT().SetLinkIPAddress("vtep0", vtepIPNet)
			networkhelper.EXPECT().SetLinkUp("vtep0")
			networkhelper.EXPECT().RouteExists(vtepCIDR, gateway, "vtep0")
			networkhelper.EXPECT().AddRoute(vtepCIDR, gateway, "vtep0", nil)
			networkhelper.EXPECT().RouteExists(hostCIDR, gateway, "vtep0")
			networkhelper.EXPECT().AddRoute(hostCIDR, gateway, "vtep0", ptr.To[int](10000))

			networkhelper.EXPECT().LinkExists("pf0vf0").Return(true, nil)
			networkhelper.EXPECT().SetLinkDown("pf0vf0")
			networkhelper.EXPECT().RenameLink("pf0vf0", "ovn-k8s-mp0_0")
			networkhelper.EXPECT().SetLinkUp("ovn-k8s-mp0_0")

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())

			ovsSystemdConfig, err := os.ReadFile(ovsSystemdConfigPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(ovsSystemdConfig)).To(Equal(ovsSystemdConfigContentPopulated))
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
			provisioner := dpucniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), ovsClient, networkhelper, fakeExec, vtepIPNet, gateway, vtepCIDR, hostCIDR, "ens25f0np0")

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "dpucniprovisioner")
			defer func() {
				err := os.RemoveAll(tmpDir)
				Expect(err).ToNot(HaveOccurred())
			}()
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir
			ovsSystemdConfigPath := filepath.Join(tmpDir, "/etc/default/openvswitch-switch")
			err = os.MkdirAll(filepath.Dir(ovsSystemdConfigPath), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(ovsSystemdConfigPath, []byte(ovsSystemdConfigContentDefault), 0644)
			Expect(err).NotTo(HaveOccurred())
			ovsClient.EXPECT().SetDOCAInit(true)
			fakeExec.CommandScript = append(fakeExec.CommandScript, kexecTesting.FakeCommandAction(func(cmd string, args ...string) kexec.Cmd {
				Expect(cmd).To(Equal("systemctl"))
				Expect(args).To(Equal([]string{"restart", "openvswitch-switch.service"}))
				return kexec.New().Command("echo")
			}))

			ovsClient.EXPECT().DeleteBridgeIfExists("ovsbr1")
			ovsClient.EXPECT().DeleteBridgeIfExists("ovsbr2")

			ovsClient.EXPECT().AddBridgeIfNotExists("br-int")
			ovsClient.EXPECT().SetBridgeDataPathType("br-int", ovsclient.NetDev)
			ovsClient.EXPECT().SetBridgeController("br-int", "ptcp:8510:169.254.55.1")
			ovsClient.EXPECT().AddBridgeIfNotExists("ens25f0np0")
			ovsClient.EXPECT().SetBridgeDataPathType("ens25f0np0", ovsclient.NetDev)
			ovsClient.EXPECT().SetBridgeController("ens25f0np0", "ptcp:8511")
			ovsClient.EXPECT().AddBridgeIfNotExists("br-ovn")
			ovsClient.EXPECT().SetBridgeDataPathType("br-ovn", ovsclient.NetDev)

			ovsClient.EXPECT().AddPortIfNotExists("ens25f0np0", "ens25f0np0-to-br-ovn").Return(errors.New("some error related to creating patch ports"))
			ovsClient.EXPECT().SetPortType("ens25f0np0-to-br-ovn", ovsclient.Patch)
			ovsClient.EXPECT().SetPatchPortPeer("ens25f0np0-to-br-ovn", "br-ovn-to-ens25f0np0")
			ovsClient.EXPECT().AddPortIfNotExists("br-ovn", "br-ovn-to-ens25f0np0").Return(errors.New("some error related to creating patch ports"))
			ovsClient.EXPECT().SetPortType("br-ovn-to-ens25f0np0", ovsclient.Patch)
			ovsClient.EXPECT().SetPatchPortPeer("br-ovn-to-ens25f0np0", "ens25f0np0-to-br-ovn")

			ovsClient.EXPECT().AddPortIfNotExists("br-ovn", "vtep0")
			ovsClient.EXPECT().SetPortType("vtep0", ovsclient.Internal)

			ovsClient.EXPECT().AddPortIfNotExists("ens25f0np0", "pf0hpf")
			ovsClient.EXPECT().SetPortType("pf0hpf", ovsclient.DPDK)
			ovsClient.EXPECT().SetBridgeHostToServicePort("ens25f0np0", "pf0hpf")
			ovsClient.EXPECT().SetBridgeUplinkPort("ens25f0np0", "ens25f0np0-to-br-ovn")

			ovsClient.EXPECT().SetOVNEncapIP(net.ParseIP("192.168.1.1"))

			mac, _ := net.ParseMAC("00:00:00:00:00:01")
			networkhelper.EXPECT().GetPFRepMACAddress("pf0hpf").Return(mac, nil)
			ovsClient.EXPECT().SetBridgeMAC("ens25f0np0", mac)

			Expect(vtepIPNet.String()).To(Equal("192.168.1.1/24"))
			_, vtepNetwork, _ := net.ParseCIDR(vtepIPNet.String())
			Expect(vtepNetwork.String()).To(Equal("192.168.1.0/24"))
			Expect(vtepCIDR).To(Equal(vtepNetwork))
			networkhelper.EXPECT().LinkIPAddressExists("vtep0", vtepIPNet)
			networkhelper.EXPECT().SetLinkIPAddress("vtep0", vtepIPNet)
			networkhelper.EXPECT().SetLinkUp("vtep0")
			networkhelper.EXPECT().RouteExists(hostCIDR, gateway, "vtep0")
			networkhelper.EXPECT().AddRoute(hostCIDR, gateway, "vtep0", ptr.To[int](10000))

			networkhelper.EXPECT().LinkExists("pf0vf0").Return(true, nil)
			networkhelper.EXPECT().SetLinkDown("pf0vf0")
			networkhelper.EXPECT().RenameLink("pf0vf0", "ovn-k8s-mp0_0")
			networkhelper.EXPECT().SetLinkUp("ovn-k8s-mp0_0")

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())

			ovsSystemdConfig, err := os.ReadFile(ovsSystemdConfigPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(ovsSystemdConfig)).To(Equal(ovsSystemdConfigContentPopulated))
		})
		It("should ensure that the OVN management link is always configured", func(ctx context.Context) {
			testCtrl := gomock.NewController(GinkgoT())
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			ctx, cancel := context.WithCancel(ctx)
			c := clock.NewFakeClock(time.Now())
			provisioner := dpucniprovisioner.New(ctx, c, nil, networkhelper, nil, nil, nil, nil, nil, "")

			networkhelper.EXPECT().LinkExists("pf0vf0").DoAndReturn(func(link string) (bool, error) {
				c.Step(30 * time.Second)
				return false, errors.New("some-error")
			})

			networkhelper.EXPECT().LinkExists("pf0vf0").DoAndReturn(func(link string) (bool, error) {
				By("error occurred, it retries and link exists so it should rename the link")
				return true, nil
			})

			networkhelper.EXPECT().SetLinkDown("pf0vf0")
			networkhelper.EXPECT().RenameLink("pf0vf0", "ovn-k8s-mp0_0")
			networkhelper.EXPECT().SetLinkUp("ovn-k8s-mp0_0").Do(func(link string) {
				c.Step(30 * time.Second)
			})

			networkhelper.EXPECT().LinkExists("pf0vf0").DoAndReturn(func(link string) (bool, error) {
				By("link doesn't exist, it should not try to rename the link, but ensure it's up")
				return false, nil
			})

			networkhelper.EXPECT().SetLinkUp("ovn-k8s-mp0_0").Do(func(link string) {
				cancel()
			})

			Eventually(func(g Gomega) {
				c.Step(30 * time.Second)
				provisioner.EnsureConfiguration()
			}).Should(BeNil())

		}, SpecTimeout(5*time.Second))
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
			provisioner := dpucniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), ovsClient, networkhelper, fakeExec, vtepIPNet, gateway, vtepCIDR, hostCIDR, "ens25f0np0")

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "dpucniprovisioner")
			defer func() {
				err := os.RemoveAll(tmpDir)
				Expect(err).ToNot(HaveOccurred())
			}()
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir
			ovsSystemdConfigPath := filepath.Join(tmpDir, "/etc/default/openvswitch-switch")
			err = os.MkdirAll(filepath.Dir(ovsSystemdConfigPath), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(ovsSystemdConfigPath, []byte(ovsSystemdConfigContentDefault), 0644)
			Expect(err).NotTo(HaveOccurred())
			fakeExec.CommandScript = append(fakeExec.CommandScript, kexecTesting.FakeCommandAction(func(cmd string, args ...string) kexec.Cmd {
				Expect(cmd).To(Equal("systemctl"))
				Expect(args).To(Equal([]string{"restart", "openvswitch-switch.service"}))
				return kexec.New().Command("echo")
			}))

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
			provisioner := dpucniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), ovsClient, networkhelper, fakeExec, vtepIPNet, gateway, vtepCIDR, hostCIDR, "ens25f0np0")

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "dpucniprovisioner")
			defer func() {
				err := os.RemoveAll(tmpDir)
				Expect(err).ToNot(HaveOccurred())
			}()
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir
			ovsSystemdConfigPath := filepath.Join(tmpDir, "/etc/default/openvswitch-switch")
			err = os.MkdirAll(filepath.Dir(ovsSystemdConfigPath), 0755)
			Expect(err).NotTo(HaveOccurred())
			fakeExec.CommandScript = append(fakeExec.CommandScript, kexecTesting.FakeCommandAction(func(cmd string, args ...string) kexec.Cmd {
				Expect(cmd).To(Equal("systemctl"))
				Expect(args).To(Equal([]string{"restart", "openvswitch-switch.service"}))
				return kexec.New().Command("echo")
			}))

			err = os.WriteFile(ovsSystemdConfigPath, []byte(ovsSystemdConfigContentDefault), 0644)
			Expect(err).NotTo(HaveOccurred())

			By("Checking the first run")
			ovsClient.EXPECT().SetDOCAInit(true)

			ovsClient.EXPECT().DeleteBridgeIfExists("ovsbr1")
			ovsClient.EXPECT().DeleteBridgeIfExists("ovsbr2")

			ovsClient.EXPECT().AddBridgeIfNotExists("br-int")
			ovsClient.EXPECT().SetBridgeDataPathType("br-int", ovsclient.NetDev)
			ovsClient.EXPECT().SetBridgeController("br-int", "ptcp:8510:169.254.55.1")
			ovsClient.EXPECT().AddBridgeIfNotExists("ens25f0np0")
			ovsClient.EXPECT().SetBridgeDataPathType("ens25f0np0", ovsclient.NetDev)
			ovsClient.EXPECT().SetBridgeController("ens25f0np0", "ptcp:8511")
			ovsClient.EXPECT().AddBridgeIfNotExists("br-ovn")
			ovsClient.EXPECT().SetBridgeDataPathType("br-ovn", ovsclient.NetDev)

			ovsClient.EXPECT().AddPortIfNotExists("ens25f0np0", "ens25f0np0-to-br-ovn").Return(errors.New("some error related to creating patch ports"))
			ovsClient.EXPECT().SetPortType("ens25f0np0-to-br-ovn", ovsclient.Patch)
			ovsClient.EXPECT().SetPatchPortPeer("ens25f0np0-to-br-ovn", "br-ovn-to-ens25f0np0")
			ovsClient.EXPECT().AddPortIfNotExists("br-ovn", "br-ovn-to-ens25f0np0").Return(errors.New("some error related to creating patch ports"))
			ovsClient.EXPECT().SetPortType("br-ovn-to-ens25f0np0", ovsclient.Patch)
			ovsClient.EXPECT().SetPatchPortPeer("br-ovn-to-ens25f0np0", "ens25f0np0-to-br-ovn")

			ovsClient.EXPECT().AddPortIfNotExists("br-ovn", "vtep0")
			ovsClient.EXPECT().SetPortType("vtep0", ovsclient.Internal)

			ovsClient.EXPECT().AddPortIfNotExists("ens25f0np0", "pf0hpf")
			ovsClient.EXPECT().SetPortType("pf0hpf", ovsclient.DPDK)
			ovsClient.EXPECT().SetBridgeHostToServicePort("ens25f0np0", "pf0hpf")
			ovsClient.EXPECT().SetBridgeUplinkPort("ens25f0np0", "ens25f0np0-to-br-ovn")

			ovsClient.EXPECT().SetOVNEncapIP(net.ParseIP("192.168.1.1"))

			mac, _ := net.ParseMAC("00:00:00:00:00:01")
			networkhelper.EXPECT().GetPFRepMACAddress("pf0hpf").Return(mac, nil)
			ovsClient.EXPECT().SetBridgeMAC("ens25f0np0", mac)

			networkhelper.EXPECT().LinkIPAddressExists("vtep0", vtepIPNet)
			networkhelper.EXPECT().SetLinkIPAddress("vtep0", vtepIPNet)
			networkhelper.EXPECT().SetLinkUp("vtep0")
			networkhelper.EXPECT().RouteExists(vtepCIDR, gateway, "vtep0")
			networkhelper.EXPECT().AddRoute(vtepCIDR, gateway, "vtep0", nil)
			networkhelper.EXPECT().RouteExists(hostCIDR, gateway, "vtep0")
			networkhelper.EXPECT().AddRoute(hostCIDR, gateway, "vtep0", ptr.To[int](10000))

			networkhelper.EXPECT().LinkExists("pf0vf0").Return(true, nil)
			networkhelper.EXPECT().SetLinkDown("pf0vf0")
			networkhelper.EXPECT().RenameLink("pf0vf0", "ovn-k8s-mp0_0")
			networkhelper.EXPECT().SetLinkUp("ovn-k8s-mp0_0")

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())

			By("Checking the second run")
			ovsClient.EXPECT().SetDOCAInit(true)

			ovsClient.EXPECT().DeleteBridgeIfExists("ovsbr1")
			ovsClient.EXPECT().DeleteBridgeIfExists("ovsbr2")

			ovsClient.EXPECT().AddBridgeIfNotExists("br-int")
			ovsClient.EXPECT().SetBridgeDataPathType("br-int", ovsclient.NetDev)
			ovsClient.EXPECT().SetBridgeController("br-int", "ptcp:8510:169.254.55.1")
			ovsClient.EXPECT().AddBridgeIfNotExists("ens25f0np0")
			ovsClient.EXPECT().SetBridgeDataPathType("ens25f0np0", ovsclient.NetDev)
			ovsClient.EXPECT().SetBridgeController("ens25f0np0", "ptcp:8511")
			ovsClient.EXPECT().AddBridgeIfNotExists("br-ovn")
			ovsClient.EXPECT().SetBridgeDataPathType("br-ovn", ovsclient.NetDev)

			ovsClient.EXPECT().AddPortIfNotExists("ens25f0np0", "ens25f0np0-to-br-ovn").Return(errors.New("some error related to creating patch ports"))
			ovsClient.EXPECT().SetPortType("ens25f0np0-to-br-ovn", ovsclient.Patch)
			ovsClient.EXPECT().SetPatchPortPeer("ens25f0np0-to-br-ovn", "br-ovn-to-ens25f0np0")
			ovsClient.EXPECT().AddPortIfNotExists("br-ovn", "br-ovn-to-ens25f0np0").Return(errors.New("some error related to creating patch ports"))
			ovsClient.EXPECT().SetPortType("br-ovn-to-ens25f0np0", ovsclient.Patch)
			ovsClient.EXPECT().SetPatchPortPeer("br-ovn-to-ens25f0np0", "ens25f0np0-to-br-ovn")

			ovsClient.EXPECT().AddPortIfNotExists("br-ovn", "vtep0")
			ovsClient.EXPECT().SetPortType("vtep0", ovsclient.Internal)

			ovsClient.EXPECT().AddPortIfNotExists("ens25f0np0", "pf0hpf")
			ovsClient.EXPECT().SetPortType("pf0hpf", ovsclient.DPDK)
			ovsClient.EXPECT().SetBridgeHostToServicePort("ens25f0np0", "pf0hpf")
			ovsClient.EXPECT().SetBridgeUplinkPort("ens25f0np0", "ens25f0np0-to-br-ovn")

			ovsClient.EXPECT().SetOVNEncapIP(net.ParseIP("192.168.1.1"))

			mac, _ = net.ParseMAC("00:00:00:00:00:01")
			networkhelper.EXPECT().GetPFRepMACAddress("pf0hpf").Return(mac, nil)
			ovsClient.EXPECT().SetBridgeMAC("ens25f0np0", mac)

			networkhelper.EXPECT().LinkIPAddressExists("vtep0", vtepIPNet).Return(true, nil)
			networkhelper.EXPECT().SetLinkUp("vtep0")
			networkhelper.EXPECT().RouteExists(vtepCIDR, gateway, "vtep0").Return(true, nil)
			networkhelper.EXPECT().RouteExists(hostCIDR, gateway, "vtep0").Return(true, nil)

			networkhelper.EXPECT().LinkExists("pf0vf0").Return(false, nil)
			networkhelper.EXPECT().SetLinkUp("ovn-k8s-mp0_0")

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeExec.CommandCalls).To(Equal(1))
		})
		It("should not restart OVS if listener already configured", func(ctx context.Context) {
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
			provisioner := dpucniprovisioner.New(context.Background(), clock.NewFakeClock(time.Now()), ovsClient, networkhelper, fakeExec, vtepIPNet, gateway, vtepCIDR, hostCIDR, "ens25f0np0")

			// Prepare Filesystem
			tmpDir, err := os.MkdirTemp("", "dpucniprovisioner")
			defer func() {
				err := os.RemoveAll(tmpDir)
				Expect(err).ToNot(HaveOccurred())
			}()
			Expect(err).NotTo(HaveOccurred())
			provisioner.FileSystemRoot = tmpDir
			ovsSystemdConfigPath := filepath.Join(tmpDir, "/etc/default/openvswitch-switch")
			err = os.MkdirAll(filepath.Dir(ovsSystemdConfigPath), 0755)
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(ovsSystemdConfigPath, []byte(ovsSystemdConfigContentPopulated), 0644)
			Expect(err).NotTo(HaveOccurred())

			networkHelperMockAll(networkhelper)
			ovsClientMockAll(ovsClient)

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())
			Expect(fakeExec.CommandCalls).To(Equal(0))
		})

	})
})

// networkHelperMockAll mocks all networkhelper functions. Useful for tests where we don't test the network calls
func networkHelperMockAll(networkHelper *networkhelperMock.MockNetworkHelper) {
	networkHelper.EXPECT().AddRoute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().GetPFRepMACAddress(gomock.Any()).AnyTimes()
	networkHelper.EXPECT().LinkExists(gomock.Any()).AnyTimes()
	networkHelper.EXPECT().LinkIPAddressExists(gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().RenameLink(gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().RouteExists(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().SetLinkDown(gomock.Any()).AnyTimes()
	networkHelper.EXPECT().SetLinkIPAddress(gomock.Any(), gomock.Any()).AnyTimes()
	networkHelper.EXPECT().SetLinkUp(gomock.Any()).AnyTimes()
}

// ovsClientMockAll mocks all ovsclient functions. Useful for tests where we don't test the ovsclient calls
func ovsClientMockAll(ovsClient *ovsclientMock.MockOVSClient) {
	ovsClient.EXPECT().AddBridgeIfNotExists(gomock.Any()).AnyTimes()
	ovsClient.EXPECT().AddPortIfNotExists(gomock.Any(), gomock.Any()).AnyTimes()
	ovsClient.EXPECT().BridgeExists(gomock.Any()).AnyTimes()
	ovsClient.EXPECT().DeleteBridgeIfExists(gomock.Any()).AnyTimes()
	ovsClient.EXPECT().SetBridgeController(gomock.Any(), gomock.Any()).AnyTimes()
	ovsClient.EXPECT().SetBridgeDataPathType(gomock.Any(), gomock.Any()).AnyTimes()
	ovsClient.EXPECT().SetBridgeHostToServicePort(gomock.Any(), gomock.Any()).AnyTimes()
	ovsClient.EXPECT().SetBridgeMAC(gomock.Any(), gomock.Any()).AnyTimes()
	ovsClient.EXPECT().SetBridgeUplinkPort(gomock.Any(), gomock.Any()).AnyTimes()
	ovsClient.EXPECT().SetDOCAInit(gomock.Any()).AnyTimes()
	ovsClient.EXPECT().SetPatchPortPeer(gomock.Any(), gomock.Any()).AnyTimes()
	ovsClient.EXPECT().SetPortType(gomock.Any(), gomock.Any()).AnyTimes()
	ovsClient.EXPECT().SetOVNEncapIP(gomock.Any()).AnyTimes()
}
