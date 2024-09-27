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

package controllers

import (
	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/types"
	"k8s.io/utils/ptr"
)

var _ = Describe("NetworkSelectionElement", func() {
	DescribeTable("Validate NetworkSelectionElement",
		func(dpuServiceInterface *sfcv1.DPUServiceInterface, expected types.NetworkSelectionElement) {
			resp := newNetworkSelectionElement(dpuServiceInterface)
			Expect(resp).To(Equal(expected))
		},
		Entry("network without namespace", &sfcv1.DPUServiceInterface{
			Spec: sfcv1.DPUServiceInterfaceSpec{
				Template: sfcv1.ServiceInterfaceSetSpecTemplate{
					Spec: sfcv1.ServiceInterfaceSetSpec{
						Template: sfcv1.ServiceInterfaceSpecTemplate{
							Spec: sfcv1.ServiceInterfaceSpec{
								InterfaceType: sfcv1.InterfaceTypeService,
								InterfaceName: ptr.To("net1"),
								Service: &sfcv1.ServiceDef{
									ServiceID: "service-one",
									Network:   "mybrsfc",
								},
							},
						},
					},
				},
			},
		}, types.NetworkSelectionElement{
			Name:             "mybrsfc",
			InterfaceRequest: "net1",
		}),
		Entry("network with namespace", &sfcv1.DPUServiceInterface{
			Spec: sfcv1.DPUServiceInterfaceSpec{
				Template: sfcv1.ServiceInterfaceSetSpecTemplate{
					Spec: sfcv1.ServiceInterfaceSetSpec{
						Template: sfcv1.ServiceInterfaceSpecTemplate{
							Spec: sfcv1.ServiceInterfaceSpec{
								InterfaceType: sfcv1.InterfaceTypeService,
								InterfaceName: ptr.To("net1"),
								Service: &sfcv1.ServiceDef{
									ServiceID: "service-one",
									Network:   "my-namespace/mybrsfc",
								},
							},
						},
					},
				},
			},
		}, types.NetworkSelectionElement{
			Name:             "mybrsfc",
			Namespace:        "my-namespace",
			InterfaceRequest: "net1",
		}),
	)
})
