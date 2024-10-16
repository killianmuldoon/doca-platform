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

package webhooks

import (
	"context"

	dpuservicev1 "github.com/nvidia/doca-platform/api/dpuservice/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("DPUServiceIPAM Validating Webhook", func() {
	var webhook *DPUServiceIPAMValidator

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(dpuservicev1.AddToScheme(s)).To(Succeed())
		fakeclient := fake.NewClientBuilder().WithScheme(s).Build()
		webhook = &DPUServiceIPAMValidator{
			Client: fakeclient,
		}
	})

	It("Errors out when both .spec.ipv4Network and .spec.ipv4Subnet are specified", func() {
		_, err := webhook.ValidateCreate(context.Background(), getFullyPopulatedDPUServiceIPAM())
		Expect(err).To(HaveOccurred())
	})

	It("Errors out when neither .spec.ipv4Network nor .spec.ipv4Subnet are specified", func() {
		ipam := getFullyPopulatedDPUServiceIPAM()
		ipam.Spec.IPV4Network = nil
		ipam.Spec.IPV4Subnet = nil
		_, err := webhook.ValidateCreate(context.Background(), ipam)
		Expect(err).To(HaveOccurred())
	})

	DescribeTable("Validates the .spec.ipv4Network correctly", func(ipam *dpuservicev1.DPUServiceIPAM, expectError bool) {
		_, err := webhook.ValidateCreate(context.Background(), ipam)
		if expectError {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
		}
	},
		Entry("bad network", func() *dpuservicev1.DPUServiceIPAM {
			return getFullyPopulatedDPUServiceIPAM()
		}(), true),

		Entry("bad network", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Subnet = nil
			ipam.Spec.IPV4Network.Network = "bad-network"
			return ipam
		}(), true),
		Entry("bad prefixSize", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Subnet = nil
			ipam.Spec.IPV4Network.PrefixSize = 10
			return ipam
		}(), true),
		Entry("bad exclusion - invalid IP", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Subnet = nil
			ipam.Spec.IPV4Network.Exclusions[0] = "bad-ip"
			return ipam
		}(), true),
		Entry("bad exclusion - IP not part of the network", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Subnet = nil
			ipam.Spec.IPV4Network.Exclusions[0] = "10.0.0.0"
			return ipam
		}(), true),
		Entry("bad allocation - invalid subnet", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Subnet = nil
			ipam.Spec.IPV4Network.Allocations["dpu-node-1"] = "bad-subnet"
			return ipam
		}(), true),
		Entry("bad allocation - subnet not part of the network due to IP", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Subnet = nil
			ipam.Spec.IPV4Network.Allocations["dpu-node-1"] = "10.0.0.0/24"
			return ipam
		}(), true),
		Entry("bad allocation - subnet not part of the network due to mask size", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Subnet = nil
			ipam.Spec.IPV4Network.Allocations["dpu-node-1"] = "192.168.1.0/10"
			return ipam
		}(), true),
		Entry("valid config", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Subnet = nil
			return ipam
		}(), false),
		Entry("bad route - dest not a valid cidr", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Subnet = nil
			ipam.Spec.IPV4Network.Routes[0].Dst = "not-a-cidr"
			return ipam
		}(), true),
		Entry("invalid route - default gateway true", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Subnet = nil
			ipam.Spec.IPV4Network.Routes[0].Dst = ipv4DefaultRoute
			return ipam
		}(), true),
		Entry("invalid route - not same family", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Subnet = nil
			ipam.Spec.IPV4Network.Routes[0].Dst = "2001:db8:3333:4444::0/64"
			return ipam
		}(), true),
	)

	DescribeTable("Validates the .spec.ipv4Subnet correctly", func(ipam *dpuservicev1.DPUServiceIPAM, expectError bool) {
		_, err := webhook.ValidateCreate(context.Background(), ipam)
		if expectError {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
		}
	},
		Entry("bad subnet", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Network = nil
			ipam.Spec.IPV4Subnet.Subnet = "bad-subnet"
			return ipam
		}(), true),
		Entry("bad gateway - invalid IP ", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Network = nil
			ipam.Spec.IPV4Subnet.Gateway = "bad-gateway"
			return ipam
		}(), true),
		Entry("bad gateway - IP not part of subnet", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Network = nil
			ipam.Spec.IPV4Subnet.Gateway = "10.0.0.0"
			return ipam
		}(), true),
		Entry("valid config", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Network = nil
			return ipam
		}(), false),
		Entry("bad route - dest not a valid cidr", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Network = nil
			ipam.Spec.IPV4Subnet.Routes[0].Dst = "not-a-cidr"
			return ipam
		}(), true),
		Entry("invalid route - default gateway true", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Network = nil
			ipam.Spec.IPV4Subnet.Routes[0].Dst = ipv4DefaultRoute
			return ipam
		}(), true),
		Entry("invalid route - not same family", func() *dpuservicev1.DPUServiceIPAM {
			ipam := getFullyPopulatedDPUServiceIPAM()
			ipam.Spec.IPV4Network = nil
			ipam.Spec.IPV4Subnet.Routes[0].Dst = "2011:db8:3333:4444::0/64"
			return ipam
		}(), true),
	)
})

// getFullyPopulatedDPUServiceIPAM returns an invalid but fully populated (for the validation context) DPUServiceIPAM
func getFullyPopulatedDPUServiceIPAM() *dpuservicev1.DPUServiceIPAM {
	return &dpuservicev1.DPUServiceIPAM{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-object",
		},
		Spec: dpuservicev1.DPUServiceIPAMSpec{
			IPV4Network: &dpuservicev1.IPV4Network{
				Network:      "192.168.0.0/20",
				GatewayIndex: 1,
				PrefixSize:   24,
				Exclusions: []string{
					"192.168.0.10",
					"192.168.2.30",
				},
				Allocations: map[string]string{
					"dpu-node-1": "192.168.1.0/24",
					"dpu-node-2": "192.168.2.0/24",
				},
				DefaultGateway: true,
				Routes:         []dpuservicev1.Route{{Dst: "5.5.5.0/16"}},
			},
			IPV4Subnet: &dpuservicev1.IPV4Subnet{
				Subnet:         "192.168.0.0/20",
				Gateway:        "192.168.0.1",
				PerNodeIPCount: 256,
				DefaultGateway: true,
				Routes:         []dpuservicev1.Route{{Dst: "5.5.5.0/16"}},
			},
		},
	}
}
