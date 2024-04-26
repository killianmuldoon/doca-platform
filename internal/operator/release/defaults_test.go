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

package release

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestDefaults_Parse(t *testing.T) {
	g := NewGomegaWithT(t)

	tests := []struct {
		name    string
		content []byte
		wantErr bool
	}{
		{
			name:    "succeed on the generated yaml",
			content: defaultsContent,
			wantErr: false,
		},
		{
			name: "fail when customOVNKubernetesDPUImage empty/missing",
			content: []byte(`
customOVNKubernetesNonDPUImage: some-image:tag
`),
			wantErr: true,
		},
		{
			name: "fail when customOVNKubernetesNonDPUImage empty/missing",
			content: []byte(`
customOVNKubernetesDPUImage: some-image:tag
`),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultsContent = tt.content
			err := NewDefaults().Parse()
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
		})
	}
}
