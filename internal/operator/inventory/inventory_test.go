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

	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/operator/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/operator/utils"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/release"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestManifests_Parse_Generate_All(t *testing.T) {
	g := NewGomegaWithT(t)
	defaults := &release.Defaults{}
	g.Expect(defaults.Parse()).To(Succeed())

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
				name: ServiceSetControllerName,
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if ServiceFunctionChainSet data has an unexpected object",
			inventory: New().setServiceFunctionChainSet(fromDPUService{
				name: ServiceSetControllerName,
				data: addUnexpectedKindToObjects(g, serviceChainSetData),
			}),
			wantErr: true,
		},
		{
			name: "fail if ServiceFunctionChainSet is missing the DPUService",
			inventory: New().setServiceFunctionChainSet(fromDPUService{
				name: ServiceSetControllerName,
				data: removeKindFromObjects(g, "DPUService", serviceChainSetData),
			}),
			wantErr: true,
		},
		// multus
		{
			name: "fail if Multus data is nil",
			inventory: New().setMultus(fromDPUService{
				name: MultusName,
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if Multus data has an unexpected object",
			inventory: New().setMultus(fromDPUService{
				name: MultusName,
				data: addUnexpectedKindToObjects(g, serviceChainSetData),
			}),
			wantErr: true,
		},
		{
			name: "fail if Multus is missing the DPUService",
			inventory: New().setMultus(fromDPUService{
				name: MultusName,
				data: removeKindFromObjects(g, "DPUService", serviceChainSetData),
			}),
			wantErr: true,
		},
		// sriovDevicePlugin
		{
			name: "fail if sriovDevicePlugin data is nil",
			inventory: New().setSRIOVDevicePlugin(fromDPUService{
				name: SRIOVDevicePluginName,
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if SRIOVDevicePlugin data has an unexpected object",
			inventory: New().setSRIOVDevicePlugin(fromDPUService{
				name: SRIOVDevicePluginName,
				data: addUnexpectedKindToObjects(g, serviceChainSetData),
			}),
			wantErr: true,
		},
		{
			name: "fail if SRIOVDevicePlugin is missing the DPUService",
			inventory: New().setSRIOVDevicePlugin(fromDPUService{
				name: SRIOVDevicePluginName,
				data: removeKindFromObjects(g, "DPUService", serviceChainSetData),
			}),
			wantErr: true,
		},
		// flannel
		{
			name: "fail if flannel data is nil",
			inventory: New().setFlannel(fromDPUService{
				name: FlannelName,
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if flannel data has an unexpected object",
			inventory: New().setFlannel(fromDPUService{
				name: FlannelName,
				data: addUnexpectedKindToObjects(g, flannelData),
			}),
			wantErr: true,
		},
		{
			name: "fail if flannel is missing the DPUService",
			inventory: New().setFlannel(fromDPUService{
				name: FlannelName,
				data: removeKindFromObjects(g, "DPUService", flannelData),
			}),
			wantErr: true,
		},
		// nv-ipam
		{
			name: "fail if nv-ipam data is nil",
			inventory: New().setNvK8sIpam(fromDPUService{
				name: NVIPAMName,
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if nv-ipam data has an unexpected object",
			inventory: New().setNvK8sIpam(fromDPUService{
				name: NVIPAMName,
				data: addUnexpectedKindToObjects(g, nvK8sIpamData),
			}),
			wantErr: true,
		},
		{
			name: "fail if nv-ipam is missing the DPUService",
			inventory: New().setNvK8sIpam(fromDPUService{
				name: NVIPAMName,
				data: removeKindFromObjects(g, "DPUService", nvK8sIpamData),
			}),
			wantErr: true,
		},
		// ovs-cni
		{
			name: "fail if ovs-cni data is nil",
			inventory: New().setOvsCni(fromDPUService{
				name: OVSCNIName,
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if ovs-cni data has an unexpected object",
			inventory: New().setOvsCni(fromDPUService{
				name: OVSCNIName,
				data: addUnexpectedKindToObjects(g, ovsCniData),
			}),
			wantErr: true,
		},
		{
			name: "fail if ovs-cni is missing the DPUService",
			inventory: New().setOvsCni(fromDPUService{
				name: OVSCNIName,
				data: removeKindFromObjects(g, "DPUService", ovsCniData),
			}),
			wantErr: true,
		},
		// sfc-controller
		{
			name: "fail if sfc-controller data is nil",
			inventory: New().setSfcController(fromDPUService{
				name: SFCControllerName,
				data: nil,
			}),
			wantErr: true,
		},
		{
			name: "fail if sfc-controller data has an unexpected object",
			inventory: New().setSfcController(fromDPUService{
				name: SFCControllerName,
				data: addUnexpectedKindToObjects(g, sfcControllerData),
			}),
			wantErr: true,
		},
		{
			name: "fail if sfc-controller is missing the DPUService",
			inventory: New().setSfcController(fromDPUService{
				name: SFCControllerName,
				data: removeKindFromObjects(g, "DPUService", sfcControllerData),
			}),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			vars := newDefaultVariables(defaults)
			vars.DPFProvisioningController = DPFProvisioningVariables{
				BFBPersistentVolumeClaimName: bfbVolumeName,
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
	defaults := release.NewDefaults()
	g.Expect(defaults.Parse()).To(Succeed())
	i := New()
	g.Expect(i.ParseAll()).NotTo(HaveOccurred())
	vars := newDefaultVariables(defaults)
	vars.DPFProvisioningController = DPFProvisioningVariables{
		BFBPersistentVolumeClaimName: bfbVolumeName,
	}
	tests := []struct {
		name               string
		componentToDisable string
		expectedMissing    string
		wantErr            bool
	}{
		{
			name:               "Generate all manifests",
			componentToDisable: "",
			wantErr:            false,
			expectedMissing:    "",
		},
		{
			name:               "Disable multus manifests",
			componentToDisable: MultusName,
			wantErr:            false,
			expectedMissing:    MultusName,
		},
		{
			name:               "Disable sriovDevicePlugin manifests",
			componentToDisable: SRIOVDevicePluginName,
			wantErr:            false,
			expectedMissing:    SRIOVDevicePluginName,
		},
		{
			name:               "Disable flannel manifests",
			componentToDisable: FlannelName,
			wantErr:            false,
			expectedMissing:    FlannelName,
		},
		{
			name:               "Disable nvidia-k8s-ipam manifests",
			componentToDisable: NVIPAMName,
			wantErr:            false,
			expectedMissing:    NVIPAMName,
		},
		{
			name:               "Disable DPFProvisioningController manifests",
			componentToDisable: ProvisioningControllerName,
			wantErr:            false,
			expectedMissing:    ProvisioningControllerName,
		},
		{
			name:               "Disable DPUServiceController manifests",
			componentToDisable: SRIOVDevicePluginName,
			wantErr:            false,
			expectedMissing:    SRIOVDevicePluginName,
		},
		{
			name:               "Disable ovs-cni manifests",
			componentToDisable: OVSCNIName,
			wantErr:            false,
			expectedMissing:    OVSCNIName,
		},
		{
			name:               "Disable sfc-controller manifests",
			componentToDisable: SFCControllerName,
			wantErr:            false,
			expectedMissing:    SFCControllerName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars.DisableSystemComponents = map[string]bool{
				tt.componentToDisable: true,
			}
			got, err := i.generateAllManifests(vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateAllManifests() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, obj := range got {

				// Manifests should not be generated if disabled.
				if tt.expectedMissing != "" {
					g.Expect(obj.GetName()).ToNot(ContainSubstring(tt.expectedMissing))
				}

				// These labels should always be present.
				g.Expect(hasLabel(obj, operatorv1.DPFComponentLabelKey)).To(BeTrue())
			}
		})
	}
}

func hasLabel(obj client.Object, label string) bool {
	if len(obj.GetLabels()) == 0 {
		return false
	}
	for l := range obj.GetLabels() {
		if l == label {
			return true
		}
	}
	return false
}
