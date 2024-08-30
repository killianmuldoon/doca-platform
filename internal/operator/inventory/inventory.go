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
	"context"
	_ "embed"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Component describes the responsibilities of an item in the Inventory.
type Component interface {
	Name() string
	Parse() error
	GenerateManifests(variables Variables) ([]client.Object, error)
	// IsReady reports an object and a field in that object which is used to check the ready status of a Component.
	// Returns an error if the object is not ready.
	IsReady(ctx context.Context, c client.Client, namespace string) error
}

// Variables contains information required to generate manifests from the inventory.
type Variables struct {
	Namespace                 string
	DPFProvisioningController DPFProvisioningVariables
	DisableSystemComponents   map[string]bool
	ImagePullSecrets          []string
}

type DPFProvisioningVariables struct {
	BFBPersistentVolumeClaimName string
	DMSTimeout                   *int
}

// SystemComponents holds kubernetes object manifests to be deployed by the operator.
type SystemComponents struct {
	DPUService              Component
	DPFProvisioning         Component
	ServiceFunctionChainSet Component
	Multus                  Component
	SRIOVDevicePlugin       Component
	NvIPAM                  Component
	OvsCni                  Component
	Flannel                 Component
	SfcController           Component
}

// Embed manifests for Kubernetes objects created by the controller.
var (
	//go:embed manifests/dpuservice-controller.yaml
	dpuServiceData []byte

	//go:embed manifests/servicefunctionchainset-controller.yaml
	serviceChainSetData []byte

	//go:embed manifests/sriov-device-plugin.yaml
	sriovDevicePluginData []byte

	//go:embed manifests/multus.yaml
	multusData []byte

	//go:embed manifests/flannel.yaml
	flannelData []byte

	//go:embed manifests/ovs-cni.yaml
	ovsCniData []byte

	//go:embed manifests/nv-k8s-ipam.yaml
	nvK8sIpamData []byte

	//go:embed manifests/provisioning-controller.yaml
	provisioningControllerData []byte

	//go:embed manifests/sfc-controller.yaml
	sfcControllerData []byte
)

// New returns a new SystemComponents inventory with data preloaded but parsing not completed.
func New() *SystemComponents {
	return &SystemComponents{
		DPUService: &dpuServiceControllerObjects{
			data: dpuServiceData,
		},
		DPFProvisioning: &provisioningControllerObjects{
			data: provisioningControllerData,
		},
		ServiceFunctionChainSet: &fromDPUService{
			name: "serviceFunctionChainSet",
			data: serviceChainSetData,
		},
		Multus: &fromDPUService{
			name: "multus",
			data: multusData,
		},
		SRIOVDevicePlugin: &fromDPUService{
			name: "sriovDevicePlugin",
			data: sriovDevicePluginData,
		},
		Flannel: &fromDPUService{
			name: "flannel",
			data: flannelData,
		},
		OvsCni: &fromDPUService{
			name: "ovs-cni",
			data: ovsCniData,
		},
		NvIPAM: &fromDPUService{
			name: "nvidia-k8s-ipam",
			data: nvK8sIpamData,
		},
		SfcController: &fromDPUService{
			name: "sfc-controller",
			data: sfcControllerData,
		},
	}
}

// SystemDPUServices returns DPUService Components deployed by the DPF Operator.
func (s *SystemComponents) SystemDPUServices() []Component {
	return []Component{
		s.ServiceFunctionChainSet,
		s.Multus,
		s.SRIOVDevicePlugin,
		s.Flannel,
		s.NvIPAM,
		s.OvsCni,
		s.SfcController,
	}
}

// AllComponents returns all Components deployed by the DPF Operator.
func (s *SystemComponents) AllComponents() []Component {
	return []Component{
		s.DPFProvisioning,
		s.DPUService,
		s.ServiceFunctionChainSet,
		s.Multus,
		s.SRIOVDevicePlugin,
		s.Flannel,
		s.NvIPAM,
		s.OvsCni,
		s.SfcController,
	}
}

// ParseAll creates Kubernetes objects for all manifests related to the DPFOperator.
func (s *SystemComponents) ParseAll() error {
	for _, component := range s.AllComponents() {
		if err := component.Parse(); err != nil {
			return err
		}
	}
	return nil
}

// generateAllManifests returns all Kubernetes objects.
func (s *SystemComponents) generateAllManifests(variables Variables) ([]client.Object, error) {
	out := []client.Object{}
	var errs []error
	for _, component := range s.AllComponents() {
		manifests, err := component.GenerateManifests(variables)
		if err != nil {
			errs = append(errs, err)
		}
		out = append(out, manifests...)
	}
	if len(errs) != 0 {
		return nil, kerrors.NewAggregate(errs)
	}
	return out, nil
}

func (s *SystemComponents) setDPUService(input dpuServiceControllerObjects) *SystemComponents {
	s.DPUService = &input
	return s
}

func (s *SystemComponents) setMultus(input fromDPUService) *SystemComponents {
	s.Multus = &input
	return s

}

func (s *SystemComponents) setSRIOVDevicePlugin(input fromDPUService) *SystemComponents {
	s.SRIOVDevicePlugin = &input
	return s
}

func (s *SystemComponents) setServiceFunctionChainSet(input fromDPUService) *SystemComponents {
	s.ServiceFunctionChainSet = &input
	return s
}

func (s *SystemComponents) setFlannel(input fromDPUService) *SystemComponents {
	s.Flannel = &input
	return s
}

func (s *SystemComponents) setNvK8sIpam(input fromDPUService) *SystemComponents {
	s.NvIPAM = &input
	return s
}

func (s *SystemComponents) setOvsCni(input fromDPUService) *SystemComponents {
	s.OvsCni = &input
	return s
}

func (s *SystemComponents) setSfcController(input fromDPUService) *SystemComponents {
	s.SfcController = &input
	return s
}
