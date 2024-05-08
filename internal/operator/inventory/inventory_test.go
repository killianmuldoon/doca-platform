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

func TestManifests_Parse_Generate_All(t *testing.T) {
	g := NewGomegaWithT(t)

	tests := []struct {
		name      string
		inventory *Manifests
		wantErr   bool
	}{
		{
			name:      "parse objects from release directory",
			inventory: New(),
		},
		// dpuServiceControllerObjects
		{
			name:      "fail if DPUService controller data is nil",
			inventory: New().setDPUService(dpuServiceControllerObjects{data: nil}),
			wantErr:   true,
		},
		{
			name: "fail if an unexpected DPUService controller object is present",
			inventory: New().setDPUService(dpuServiceControllerObjects{
				data: addUnexpectedKindToObjects(g, dpuServiceData),
			}),
			wantErr: true,
		},
		{
			name: "fail if any DPUService controller object is missing",
			inventory: New().setDPUService(dpuServiceControllerObjects{
				data: removeKindFromObjects(g, "Deployment", dpuServiceData),
			}),
			wantErr: true,
		},
		// ServiceFunctionChainSetObjects
		{
			name: "fail if ServiceFunctionChainSet data is nil",
			inventory: New().setServiceFunctionChainSet(fromDPUService{
				name: "serviceFunctionChainSet",
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if ServiceFunctionChainSet data has an unexpected object",
			inventory: New().setServiceFunctionChainSet(fromDPUService{
				name: "serviceFunctionChainSet",
				data: addUnexpectedKindToObjects(g, serviceChainSetData),
			}),
			wantErr: true,
		},
		{
			name: "fail if ServiceFunctionChainSet is missing the DPUService",
			inventory: New().setServiceFunctionChainSet(fromDPUService{
				name: "serviceFunctionChainSet",
				data: removeKindFromObjects(g, "DPUService", serviceChainSetData),
			}),
			wantErr: true,
		},
		// multus
		{
			name: "fail if Multus data is nil",
			inventory: New().setMultus(fromDPUService{
				name: "multus",
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if Multus data has an unexpected object",
			inventory: New().setMultus(fromDPUService{
				name: "multus",
				data: addUnexpectedKindToObjects(g, serviceChainSetData),
			}),
			wantErr: true,
		},
		{
			name: "fail if Multus is missing the DPUService",
			inventory: New().setMultus(fromDPUService{
				name: "multus",
				data: removeKindFromObjects(g, "DPUService", serviceChainSetData),
			}),
			wantErr: true,
		},
		// sriovDevicePlugin
		{
			name: "fail if sriovDevicePlugin data is nil",
			inventory: New().setSRIOVDevicePlugin(fromDPUService{
				name: "sriovDevicePlugin",
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if SRIOVDevicePlugin data has an unexpected object",
			inventory: New().setSRIOVDevicePlugin(fromDPUService{
				name: "sriovDevicePlugin",
				data: addUnexpectedKindToObjects(g, serviceChainSetData),
			}),
			wantErr: true,
		},
		{
			name: "fail if SRIOVDevicePlugin is missing the DPUService",
			inventory: New().setSRIOVDevicePlugin(fromDPUService{
				name: "sriovDevicePlugin",
				data: removeKindFromObjects(g, "DPUService", serviceChainSetData),
			}),
			wantErr: true,
		},
		// flannel
		{
			name: "fail if flannel data is nil",
			inventory: New().setFlannel(fromDPUService{
				name: "flannel",
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if flannel data has an unexpected object",
			inventory: New().setFlannel(fromDPUService{
				name: "flannel",
				data: addUnexpectedKindToObjects(g, flannelData),
			}),
			wantErr: true,
		},
		{
			name: "fail if flannel is missing the DPUService",
			inventory: New().setFlannel(fromDPUService{
				name: "flannel",
				data: removeKindFromObjects(g, "DPUService", flannelData),
			}),
			wantErr: true,
		},
		// nv-ipam
		{
			name: "fail if nv-ipam data is nil",
			inventory: New().setNvK8sIpam(fromDPUService{
				name: "nv-ipam",
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if nv-ipam data has an unexpected object",
			inventory: New().setNvK8sIpam(fromDPUService{
				name: "nv-ipam",
				data: addUnexpectedKindToObjects(g, nvK8sIpamData),
			}),
			wantErr: true,
		},
		{
			name: "fail if nv-ipam is missing the DPUService",
			inventory: New().setNvK8sIpam(fromDPUService{
				name: "nv-ipam",
				data: removeKindFromObjects(g, "DPUService", nvK8sIpamData),
			}),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := Variables{
				DPFProvisioningController: DPFProvisioningVariables{
					BFBPersistentVolumeClaimName: bfbVolumeName,
					ImagePullSecret:              "secret",
				},
			}
			err := tt.inventory.ParseAll()
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			_, err = tt.inventory.generateAllManifests(vars)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}
func removeKindFromObjects(g Gomega, kindToRemove string, data []byte) []byte {
	// DPUService objects which is missing one of the expected Kinds.
	objsWithMissingKind, err := utils.BytesToUnstructured(data)
	g.Expect(err).NotTo(HaveOccurred())
	for i, obj := range objsWithMissingKind {
		if obj.GetObjectKind().GroupVersionKind().Kind == kindToRemove {
			objsWithMissingKind = append(objsWithMissingKind[:i], objsWithMissingKind[i+1:]...)
		}
	}
	dataWithMissingKind, err := utils.UnstructuredToBytes(objsWithMissingKind)
	g.Expect(err).NotTo(HaveOccurred())
	return dataWithMissingKind
}

func addUnexpectedKindToObjects(g Gomega, data []byte) []byte {
	objsWithUnexpectedKind, err := utils.BytesToUnstructured(data)
	g.Expect(err).NotTo(HaveOccurred())
	objsWithUnexpectedKind[0].SetKind("FakeKind")
	dataWithUnexpectedKind, err := utils.UnstructuredToBytes(objsWithUnexpectedKind)
	g.Expect(err).NotTo(HaveOccurred())
	return dataWithUnexpectedKind
}
