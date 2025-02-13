/*
Copyright 2025 NVIDIA

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

package redfish

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Check DPU Capacity from product description", func() {
	It("compares DPUFlavor and description", func() {
		type testCase struct {
			spec string
			// expected is the expected output of parsing spec
			expected corev1.ResourceList
			fit      []corev1.ResourceList
			notFit   []corev1.ResourceList
		}
		testCases := []testCase{
			{
				spec: "NVIDIA BlueField-3 B3220 P-Series FHHL DPU; 200GbE (default mode) / NDR200 IB; Dual-port QSFP112; PCIe Gen5.0 x16 with x16 PCIe extension option; 16 Arm cores; 32GB on-board DDR; integrated BMC; Crypto Enabled",
				expected: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("16"),
					corev1.ResourceMemory: resource.MustParse("32Gi"),
				},
				fit: []corev1.ResourceList{
					{
						corev1.ResourceCPU:    resource.MustParse("15"),
						corev1.ResourceMemory: resource.MustParse("32Gi"),
					},
					{
						corev1.ResourceCPU:    resource.MustParse("16"),
						corev1.ResourceMemory: resource.MustParse("31Gi"),
					},
					{
						corev1.ResourceCPU:    resource.MustParse("16"),
						corev1.ResourceMemory: resource.MustParse("32G"),
					},
				},
				notFit: []corev1.ResourceList{
					{
						corev1.ResourceCPU:    resource.MustParse("17"),
						corev1.ResourceMemory: resource.MustParse("32Gi"),
					},
					{
						corev1.ResourceCPU:    resource.MustParse("16"),
						corev1.ResourceMemory: resource.MustParse("33Gi"),
					},
					{
						corev1.ResourceCPU:    resource.MustParse("16"),
						corev1.ResourceMemory: resource.MustParse("33G"),
					},
				},
			},
		}
		for _, tc := range testCases {
			for _, f := range tc.fit {
				fit, err := compare(f, tc.spec)
				Expect(err).ToNot(HaveOccurred())
				Expect(fit).To(BeTrue())
			}
			for _, nf := range tc.notFit {
				fit, err := compare(nf, tc.spec)
				Expect(err).ToNot(HaveOccurred())
				Expect(fit).To(BeFalse())
			}
		}
	})
})
