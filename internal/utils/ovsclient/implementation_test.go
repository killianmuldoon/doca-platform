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

package ovsclient

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetExternalIDsAsMap(t *testing.T) {
	g := NewWithT(t)
	cases := []struct {
		msg            string
		input          string
		expectedOutput map[string]string
		expectedError  bool
	}{
		{
			msg:            "empty",
			input:          "{}",
			expectedOutput: make(map[string]string),
			expectedError:  false,
		},
		{
			msg:   "one external id",
			input: "{owner=ovs-cni.network.kubevirt.io}",
			expectedOutput: map[string]string{
				"owner": "ovs-cni.network.kubevirt.io",
			},
			expectedError: false,
		},
		{
			msg:   "one external id with extra space at the beginning and end",
			input: " {owner=ovs-cni.network.kubevirt.io} ",
			expectedOutput: map[string]string{
				"owner": "ovs-cni.network.kubevirt.io",
			},
			expectedError: false,
		},
		{
			msg:   "multiple external ids",
			input: `{dpf-id="dpf-operator-system/dpu-cplane-tenant1-doca-hbn-ds-98svc/p1_if", iface-id=""}`,
			expectedOutput: map[string]string{
				"dpf-id":   `"dpf-operator-system/dpu-cplane-tenant1-doca-hbn-ds-98svc/p1_if"`,
				"iface-id": `""`,
			},
			expectedError: false,
		},
		{
			msg:   "malformed input due to no closing bracket",
			input: `{dpf-id="dpf-operator-system/dpu-cplane-tenant1-doca-hbn-ds-98svc/p1_if", iface-id=""`,
			expectedOutput: map[string]string{
				"dpf-id":   `"dpf-operator-system/dpu-cplane-tenant1-doca-hbn-ds-98svc/p1_if"`,
				"iface-id": `""`,
			},
			expectedError: false,
		},
		{
			msg:            "malformed input due to multiple equals",
			input:          `{dpf-id=="dpf-operator-system/dpu-cplane-tenant1-doca-hbn-ds-98svc/p1_if", iface-id=""}`,
			expectedOutput: make(map[string]string),
			expectedError:  true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.msg, func(t *testing.T) {
			output, err := getExternalIDsAsMap(tt.input)
			if tt.expectedError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(output).To(BeComparableTo(tt.expectedOutput))
		})
	}
}
