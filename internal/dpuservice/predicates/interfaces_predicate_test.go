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

package predicates

import (
	"testing"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestDPUServiceInterfaceChangePredicate_Update(t *testing.T) {
	interfaceA := createInterface("serviceA")
	interfaceB := createInterface("serviceB")
	emptyInterface := &sfcv1.DPUServiceInterface{}
	notAInterface := &unstructured.Unstructured{}

	tests := []struct {
		name string
		old  client.Object
		new  client.Object
		want bool
	}{
		{name: "same service definition", old: interfaceA, new: interfaceA, want: false},
		{name: "diff service definition", old: interfaceA, new: interfaceB, want: true},
		{name: "new with service", old: emptyInterface, new: interfaceA, want: true},
		{name: "old with service", old: interfaceA, new: emptyInterface, want: false},
		{name: "old not a dpuServiceInterface", old: notAInterface, new: interfaceA, want: false},
		{name: "new not a dpuServiceInterface", old: interfaceA, new: notAInterface, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			so := DPUServiceInterfaceChangePredicate{}
			e := event.UpdateEvent{
				ObjectOld: tt.old,
				ObjectNew: tt.new,
			}
			g.Expect(so.Update(e)).To(Equal(tt.want))
		})
	}
}

func createInterface(service string) *sfcv1.DPUServiceInterface {
	return &sfcv1.DPUServiceInterface{
		Spec: sfcv1.DPUServiceInterfaceSpec{
			Template: sfcv1.ServiceInterfaceSetSpecTemplate{
				Spec: sfcv1.ServiceInterfaceSetSpec{
					Template: sfcv1.ServiceInterfaceSpecTemplate{
						Spec: sfcv1.ServiceInterfaceSpec{
							InterfaceType: sfcv1.InterfaceTypeService,
							InterfaceName: ptr.To("net1"),
							Service: &sfcv1.ServiceDef{
								ServiceID: service,
								Network:   "mybrsfc",
							},
						},
					},
				},
			},
		},
	}
}
