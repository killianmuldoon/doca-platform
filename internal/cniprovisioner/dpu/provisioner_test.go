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
	"errors"
	"net"
	"os"
	"path/filepath"

	dpucniprovisioner "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/dpu"
	networkhelperMock "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/networkhelper/mock"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/ovsclient"
	ovsclientMock "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/ovsclient/mock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"go.uber.org/mock/gomock"
	kexec "k8s.io/utils/exec"
	kexecTesting "k8s.io/utils/exec/testing"
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
		It("should configure the system fully", func() {
			testCtrl := gomock.NewController(GinkgoT())
			ovsClient := ovsclientMock.NewMockOVSClient(testCtrl)
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			fakeExec := &kexecTesting.FakeExec{}
			provisioner := dpucniprovisioner.New(ovsClient, networkhelper, fakeExec)

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

			ovsClient.EXPECT().BridgeExists("br-int").Return(false, nil)

			ovsClient.EXPECT().DeleteBridgeIfExists("ovsbr1")
			ovsClient.EXPECT().DeleteBridgeIfExists("ovsbr2")
			ovsClient.EXPECT().DeleteBridgeIfExists("br-int")
			ovsClient.EXPECT().DeleteBridgeIfExists("ens2f0np0")
			ovsClient.EXPECT().DeleteBridgeIfExists("br-ovn")

			ovsClient.EXPECT().AddBridge("br-int")
			ovsClient.EXPECT().SetBridgeDataPathType("br-int", ovsclient.NetDev)
			ovsClient.EXPECT().SetBridgeController("br-int", "ptcp:8510:10.100.1.1")
			ovsClient.EXPECT().AddBridge("ens2f0np0")
			ovsClient.EXPECT().SetBridgeDataPathType("ens2f0np0", ovsclient.NetDev)
			ovsClient.EXPECT().SetBridgeController("ens2f0np0", "ptcp:8511")
			ovsClient.EXPECT().AddBridge("br-ovn")
			ovsClient.EXPECT().SetBridgeDataPathType("br-ovn", ovsclient.NetDev)

			ovsClient.EXPECT().AddPort("ens2f0np0", "ens2f0np0-to-br-ovn").Return(errors.New("some error related to creating patch ports"))
			ovsClient.EXPECT().SetPortType("ens2f0np0-to-br-ovn", ovsclient.Patch)
			ovsClient.EXPECT().SetPatchPortPeer("ens2f0np0-to-br-ovn", "br-ovn-to-ens2f0np0")
			ovsClient.EXPECT().AddPort("br-ovn", "br-ovn-to-ens2f0np0").Return(errors.New("some error related to creating patch ports"))
			ovsClient.EXPECT().SetPortType("br-ovn-to-ens2f0np0", ovsclient.Patch)
			ovsClient.EXPECT().SetPatchPortPeer("br-ovn-to-ens2f0np0", "ens2f0np0-to-br-ovn")

			ovsClient.EXPECT().AddPort("br-ovn", "p0")
			ovsClient.EXPECT().SetPortType("p0", ovsclient.DPDK)
			ovsClient.EXPECT().AddPort("br-ovn", "vtep0")
			ovsClient.EXPECT().SetPortType("vtep0", ovsclient.Internal)

			ovsClient.EXPECT().AddPort("ens2f0np0", "pf0hpf")
			ovsClient.EXPECT().SetPortType("pf0hpf", ovsclient.DPDK)
			ovsClient.EXPECT().SetBridgeHostToServicePort("ens2f0np0", "pf0hpf")
			ovsClient.EXPECT().SetBridgeUplinkPort("ens2f0np0", "ens2f0np0-to-br-ovn")

			ovsClient.EXPECT().SetOVNEncapIP(net.ParseIP("192.168.1.1"))

			mac, _ := net.ParseMAC("00:00:00:00:00:01")
			networkhelper.EXPECT().GetPFRepMACAddress("pf0hpf").Return(mac, nil)
			ovsClient.EXPECT().SetBridgeMAC("ens2f0np0", mac)

			ipNet, _ := netlink.ParseIPNet("192.168.1.1/24")
			networkhelper.EXPECT().SetLinkIPAddress("vtep0", ipNet)
			networkhelper.EXPECT().SetLinkUp("vtep0")

			networkhelper.EXPECT().SetLinkDown("pf0vf0")
			networkhelper.EXPECT().RenameLink("pf0vf0", "ovn-k8s-mp0_0")
			networkhelper.EXPECT().SetLinkUp("ovn-k8s-mp0_0")

			err = provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())

			ovsSystemdConfig, err := os.ReadFile(ovsSystemdConfigPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(ovsSystemdConfig)).To(Equal(ovsSystemdConfigContentPopulated))
		})
	})
	Context("When it runs once when the system is configured", func() {
		It("should skip configuration", func() {
			testCtrl := gomock.NewController(GinkgoT())
			ovsClient := ovsclientMock.NewMockOVSClient(testCtrl)
			networkhelper := networkhelperMock.NewMockNetworkHelper(testCtrl)
			fakeExec := &kexecTesting.FakeExec{}
			provisioner := dpucniprovisioner.New(ovsClient, networkhelper, fakeExec)

			ovsClient.EXPECT().BridgeExists("br-int").Return(true, nil)
			ovsClient.EXPECT().BridgeExists("ens2f0np0").Return(true, nil)
			ovsClient.EXPECT().BridgeExists("br-ovn").Return(true, nil)

			err := provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
