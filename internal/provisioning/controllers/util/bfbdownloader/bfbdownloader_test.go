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

package bfbdownloader

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unit testing", func() {
	Context("parseDOCAVersionFromBSP()", func() {
		DescribeTable("parses the doca version correctly", func(bspVersion string, expectedDOCAVersion string, expectError bool) {
			docaVersion, err := parseDOCAVersionFromBSP(bspVersion)
			if expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(docaVersion).To(Equal(expectedDOCAVersion))
		},
			Entry("valid bsp version - real example", "4.2.3.11212", "2.2.3", false),
			Entry("valid bsp version - without the 4th part", "4.2.3", "2.2.3", false),
			Entry("invalid bsp version - major version below 2", "1.2.3.11212", "", true),
			Entry("invalid bsp version - characters in minor version", "4.abc.3.11212", "", true),
			Entry("invalid bsp version - empty", "", "", true),
		)
	})
})
