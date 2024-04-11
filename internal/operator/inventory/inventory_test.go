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

package inventory

import (
	"testing"

	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/utils"

	. "github.com/onsi/gomega"
)

func TestDPUServiceObjects_ParseManifests(t *testing.T) {
	g := NewGomegaWithT(t)

	// Object which contains a kind that is not expected.
	objsWithUnexpectedKind, err := utils.BytesToUnstructured(dpuServiceData)
	g.Expect(err).NotTo(HaveOccurred())
	objsWithUnexpectedKind[0].SetKind("FakeKind")
	dataWithUnexpectedKind, err := utils.UnstructuredToBytes(objsWithUnexpectedKind)
	g.Expect(err).NotTo(HaveOccurred())

	// Object which is missing one of the expected Kinds.
	objsWithMissingKind, err := utils.BytesToUnstructured(dpuServiceData)
	g.Expect(err).NotTo(HaveOccurred())
	for i, obj := range objsWithMissingKind {
		if obj.GetObjectKind().GroupVersionKind().Kind == "Role" {
			objsWithMissingKind = append(objsWithMissingKind[:i], objsWithMissingKind[i+1:]...)
		}
	}
	dataWithMissingKind, err := utils.UnstructuredToBytes(objsWithMissingKind)
	g.Expect(err).NotTo(HaveOccurred())

	tests := []struct {
		name      string
		inventory *Manifests
		wantErr   bool
	}{
		{
			name:      "parse objects from release directory",
			inventory: New(),
		},
		{
			name: "fail if data is nil",
			inventory: &Manifests{
				DPUServiceObjects{
					data: nil,
				},
			},
			wantErr: true,
		},
		{
			name: "fail if an unexpected object is present",
			inventory: &Manifests{
				DPUServiceObjects{
					data: dataWithUnexpectedKind,
				},
			},
			wantErr: true,
		},
		{
			name: "fail if any object is missing",
			inventory: &Manifests{
				DPUServiceObjects{
					data: dataWithMissingKind,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.inventory.Parse()
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			for _, obj := range tt.inventory.Objects() {
				g.Expect(obj).ToNot(BeNil())
			}
		})
	}
}
