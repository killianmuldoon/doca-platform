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
	"encoding/json"
	"testing"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_fromDPUService_GenerateManifests(t *testing.T) {
	g := NewWithT(t)
	initialValuesObject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"value-one": "value-again",
		},
	}
	initialValuesData, err := json.Marshal(initialValuesObject)
	g.Expect(err).NotTo(HaveOccurred())
	initialValuesObjectAfterMerge := initialValuesObject.DeepCopy()
	initialValuesObjectAfterMerge.Object["imagePullSecrets"] = []corev1.LocalObjectReference{
		{Name: "secret-one"},
		{Name: "secret-two"},
	}

	imagePullSecretsVars := Variables{ImagePullSecrets: []string{"secret-one", "secret-two"}}

	tests := []struct {
		name    string
		in      *dpuservicev1.DPUService
		vars    Variables
		want    *dpuservicev1.DPUService
		wantErr bool
	}{
		{
			name: "Preserve values from the template",
			in: &dpuservicev1.DPUService{
				Spec: dpuservicev1.DPUServiceSpec{
					Values: &runtime.RawExtension{
						Raw: initialValuesData,
					},
				},
			},
			vars: Variables{},
			want: &dpuservicev1.DPUService{
				Spec: dpuservicev1.DPUServiceSpec{
					Values: &runtime.RawExtension{
						Raw: initialValuesData,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Merge imagepullsecrets into the the template",
			in: &dpuservicev1.DPUService{
				Spec: dpuservicev1.DPUServiceSpec{
					Values: &runtime.RawExtension{
						Raw: initialValuesData,
					},
				},
			},
			vars: imagePullSecretsVars,
			want: &dpuservicev1.DPUService{
				Spec: dpuservicev1.DPUServiceSpec{
					Values: &runtime.RawExtension{
						Object: initialValuesObjectAfterMerge,
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fromDPUService{
				name:       "testService",
				dpuService: tt.in,
			}
			got, err := f.GenerateManifests(tt.vars)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			}
			if !tt.wantErr {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(got[0]).To(Equal(tt.want))
		})
	}
}
