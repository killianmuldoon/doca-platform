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

package dpucniprovisioner_test

import (
	"errors"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	dpucniprovisioner "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/dpu"
	utils "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/mock"
	utilsTypes "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/types"
	"go.uber.org/mock/gomock"
)

var _ = Describe("DPU CNI Provisioner", func() {
	Context("When it runs once", func() {
		It("should configure the system fully", func() {
			testCtrl := gomock.NewController(GinkgoT())
			ovsClient := utils.NewMockOVSClient(testCtrl)
			provisioner := dpucniprovisioner.New(ovsClient)

			ovsClient.EXPECT().AddBridge("br-int")
			ovsClient.EXPECT().SetBridgeDataPathType("br-int", utilsTypes.NetDev)
			ovsClient.EXPECT().SetBridgeController("br-int", "ptcp:8510:10.100.1.1")
			ovsClient.EXPECT().AddBridge("ens2f0np0")
			ovsClient.EXPECT().SetBridgeDataPathType("ens2f0np0", utilsTypes.NetDev)
			ovsClient.EXPECT().SetBridgeController("ens2f0np0", "ptcp:8511")
			ovsClient.EXPECT().AddBridge("br-ovn")
			ovsClient.EXPECT().SetBridgeDataPathType("br-ovn", utilsTypes.NetDev)

			ovsClient.EXPECT().AddPort("ens2f0np0", "ens2f0np0-to-br-ovn").Return(errors.New("some error related to creating patch ports"))
			ovsClient.EXPECT().SetPortType("ens2f0np0-to-br-ovn", utilsTypes.Patch)
			ovsClient.EXPECT().SetPatchPortPeer("ens2f0np0-to-br-ovn", "br-ovn-to-ens2f0np0")
			ovsClient.EXPECT().AddPort("br-ovn", "br-ovn-to-ens2f0np0").Return(errors.New("some error related to creating patch ports"))
			ovsClient.EXPECT().SetPortType("br-ovn-to-ens2f0np0", utilsTypes.Patch)
			ovsClient.EXPECT().SetPatchPortPeer("br-ovn-to-ens2f0np0", "ens2f0np0-to-br-ovn")

			ovsClient.EXPECT().AddPort("br-ovn", "p0")
			ovsClient.EXPECT().SetPortType("p0", utilsTypes.DPDK)
			ovsClient.EXPECT().AddPort("br-ovn", "vtep0")
			ovsClient.EXPECT().SetPortType("vtep0", utilsTypes.Internal)

			ovsClient.EXPECT().AddPort("ens2f0np0", "pf0hpf")
			ovsClient.EXPECT().SetPortType("pf0hpf", utilsTypes.DPDK)
			ovsClient.EXPECT().SetBridgeHostToServicePort("ens2f0np0", "pf0hpf")
			ovsClient.EXPECT().SetBridgeUplinkPort("ens2f0np0", "ens2f0np0-to-br-ovn")

			ovsClient.EXPECT().SetOVNEncapIP(net.ParseIP("192.168.1.1"))
			mac, _ := net.ParseMAC("00:00:00:00:00:01")
			ovsClient.EXPECT().SetBridgeMAC("ens2f0np0", mac)

			err := provisioner.RunOnce()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
