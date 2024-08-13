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

	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/operator/utils"

	. "github.com/onsi/gomega"
)

func TestManifests_Parse_Generate_All(t *testing.T) {
	g := NewGomegaWithT(t)

	tests := []struct {
		name      string
		inventory *SystemComponents
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
		// ovs-cni
		{
			name: "fail if ovs-cni data is nil",
			inventory: New().setOvsCni(fromDPUService{
				name: "ovs-cni",
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if ovs-cni data has an unexpected object",
			inventory: New().setOvsCni(fromDPUService{
				name: "ovs-cni",
				data: addUnexpectedKindToObjects(g, ovsCniData),
			}),
			wantErr: true,
		},
		{
			name: "fail if ovs-cni is missing the DPUService",
			inventory: New().setOvsCni(fromDPUService{
				name: "ovs-cni",
				data: removeKindFromObjects(g, "DPUService", ovsCniData),
			}),
			wantErr: true,
		},
		// sfc-controller
		{
			name: "fail if sfc-controller data is nil",
			inventory: New().setSfcController(fromDPUService{
				name: "sfc-controller",
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if sfc-controller data has an unexpected object",
			inventory: New().setSfcController(fromDPUService{
				name: "sfc-controller",
				data: addUnexpectedKindToObjects(g, sfcControllerData),
			}),
			wantErr: true,
		},
		{
			name: "fail if sfc-controller is missing the DPUService",
			inventory: New().setSfcController(fromDPUService{
				name: "sfc-controller",
				data: removeKindFromObjects(g, "DPUService", sfcControllerData),
			}),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := Variables{
				DPFProvisioningController: DPFProvisioningVariables{
					BFBPersistentVolumeClaimName:        bfbVolumeName,
					ImagePullSecretForDMSAndHostNetwork: "secret",
					DHCP:                                "192.168.1.1",
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

//nolint:unparam
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

func TestManifests_generateAllManifests(t *testing.T) {
	g := NewWithT(t)
	i := New()
	g.Expect(i.ParseAll()).NotTo(HaveOccurred())
	tests := []struct {
		name            string
		vars            Variables
		expectedMissing string
		wantErr         bool
	}{
		{
			name: "Generate all manifests",
			vars: Variables{
				DPFProvisioningController: DPFProvisioningVariables{
					BFBPersistentVolumeClaimName:        bfbVolumeName,
					ImagePullSecretForDMSAndHostNetwork: "secret",
					DHCP:                                "192.168.1.1",
				},
			},
			wantErr:         false,
			expectedMissing: "",
		},
		{
			name: "Disable multus manifests",
			vars: Variables{
				DPFProvisioningController: DPFProvisioningVariables{
					BFBPersistentVolumeClaimName:        bfbVolumeName,
					ImagePullSecretForDMSAndHostNetwork: "secret",
					DHCP:                                "192.168.1.1",
				},
				DisableSystemComponents: map[string]bool{
					"multus": true,
				},
			},
			wantErr:         false,
			expectedMissing: "multus",
		},
		{
			name: "Disable sriovDevicePlugin manifests",
			vars: Variables{
				DPFProvisioningController: DPFProvisioningVariables{
					BFBPersistentVolumeClaimName:        bfbVolumeName,
					ImagePullSecretForDMSAndHostNetwork: "secret",
					DHCP:                                "192.168.1.1",
				},
				DisableSystemComponents: map[string]bool{
					"sriovDevicePlugin": true,
				},
			},
			wantErr:         false,
			expectedMissing: "sriovDevicePlugin",
		},
		{
			name: "Disable flannel manifests",
			vars: Variables{
				DPFProvisioningController: DPFProvisioningVariables{
					BFBPersistentVolumeClaimName:        bfbVolumeName,
					ImagePullSecretForDMSAndHostNetwork: "secret",
					DHCP:                                "192.168.1.1",
				},
				DisableSystemComponents: map[string]bool{
					"flannel": true,
				},
			},
			wantErr:         false,
			expectedMissing: "flannel",
		},
		{
			name: "Disable nvidia-k8s-ipam manifests",
			vars: Variables{
				DPFProvisioningController: DPFProvisioningVariables{
					BFBPersistentVolumeClaimName:        bfbVolumeName,
					ImagePullSecretForDMSAndHostNetwork: "secret",
					DHCP:                                "192.168.1.1",
				},
				DisableSystemComponents: map[string]bool{
					"nvidia-k8s-ipam": true,
				},
			},
			wantErr:         false,
			expectedMissing: "nvidia-k8s-ipam",
		},
		{
			name: "Disable DPFProvisioningController manifests",
			vars: Variables{
				DPFProvisioningController: DPFProvisioningVariables{
					BFBPersistentVolumeClaimName:        bfbVolumeName,
					ImagePullSecretForDMSAndHostNetwork: "secret",
					DHCP:                                "192.168.1.1",
				},
				DisableSystemComponents: map[string]bool{
					"DPFProvisioningController": true,
				},
			},
			wantErr:         false,
			expectedMissing: "DPFProvisioningController",
		},
		{
			name: "Disable DPUServiceController manifests",
			vars: Variables{
				DPFProvisioningController: DPFProvisioningVariables{
					BFBPersistentVolumeClaimName:        bfbVolumeName,
					ImagePullSecretForDMSAndHostNetwork: "secret",
					DHCP:                                "192.168.1.1",
				},
				DisableSystemComponents: map[string]bool{
					"DPUServiceController": true,
				},
			},
			wantErr:         false,
			expectedMissing: "DPUServiceController",
		},
		{
			name: "Disable ovs-cni manifests",
			vars: Variables{
				DPFProvisioningController: DPFProvisioningVariables{
					BFBPersistentVolumeClaimName:        bfbVolumeName,
					ImagePullSecretForDMSAndHostNetwork: "secret",
					DHCP:                                "192.168.1.1",
				},
				DisableSystemComponents: map[string]bool{
					"ovs-cni": true,
				},
			},
			wantErr:         false,
			expectedMissing: "ovs-cni",
		},
		{
			name: "Disable sfc-controller manifests",
			vars: Variables{
				DPFProvisioningController: DPFProvisioningVariables{
					BFBPersistentVolumeClaimName:        bfbVolumeName,
					ImagePullSecretForDMSAndHostNetwork: "secret",
					DHCP:                                "192.168.1.1",
				},
				DisableSystemComponents: map[string]bool{
					"sfc-controller": true,
				},
			},
			wantErr:         false,
			expectedMissing: "sfc-controller",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := i.generateAllManifests(tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateAllManifests() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, obj := range got {
				if tt.expectedMissing != "" {
					g.Expect(obj.GetName()).ToNot(ContainSubstring(tt.expectedMissing))
				}
			}
		})
	}
}
