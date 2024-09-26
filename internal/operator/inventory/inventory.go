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

	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/operator/v1alpha1"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Component describes the responsibilities of an item in the Inventory.
type Component interface {
	Name() string
	Parse() error
	GenerateManifests(variables Variables, options ...GenerateManifestOption) ([]client.Object, error)
	// IsReady reports an object and a field in that object which is used to check the ready status of a Component.
	// Returns an error if the object is not ready.
	IsReady(ctx context.Context, c client.Client, namespace string) error
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
	NVidiaClusterManager    Component
	StaticClusterManager    Component
	DPUDetector             Component
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

	//go:embed manifests/nvidia-cluster-manager.yaml
	nvidiaCMData []byte

	//go:embed manifests/static-cluster-manager.yaml
	staticCMData []byte

	//go:embed manifests/dpu-detector.yaml
	dpuDetectorData []byte
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
			name: operatorv1.ServiceSetControllerName,
			data: serviceChainSetData,
		},
		Multus: &fromDPUService{
			name: operatorv1.MultusName,
			data: multusData,
		},
		SRIOVDevicePlugin: &fromDPUService{
			name: operatorv1.SRIOVDevicePluginName,
			data: sriovDevicePluginData,
		},
		Flannel: &fromDPUService{
			name: operatorv1.FlannelName,
			data: flannelData,
		},
		OvsCni: &fromDPUService{
			name: operatorv1.OVSCNIName,
			data: ovsCniData,
		},
		NvIPAM: &fromDPUService{
			name: operatorv1.NVIPAMName,
			data: nvK8sIpamData,
		},
		SfcController: &fromDPUService{
			name: operatorv1.SFCControllerName,
			data: sfcControllerData,
		},
		DPUDetector: &dpuDetectorObjects{
			data: dpuDetectorData,
		},
		NVidiaClusterManager: NewClusterManagerObjects(operatorv1.HostedControlPlaneManagerName, nvidiaCMData),
		StaticClusterManager: NewClusterManagerObjects(operatorv1.StaticControlPlaneManagerName, staticCMData),
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
		s.NVidiaClusterManager,
		s.StaticClusterManager,
		s.DPUDetector,
	}
}

// EnabledComponents returns the set of components which is not disabled.
func (s *SystemComponents) EnabledComponents(vars Variables) []Component {
	out := []Component{}
	for _, component := range s.AllComponents() {
		if disabled, found := vars.DisableSystemComponents[component.Name()]; found && !disabled {
			out = append(out, component)
		}
	}
	return out
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

type GenerateManifestOption interface {
	Apply(*GenerateManifestOptions)
}

type GenerateManifestOptions struct {
	skipApplySet bool
}

// skipApplySetCreationOption is GenerateManifestOption which skips the creation of the apply set.
// This option is purely for making testing around manifest generation easier.
type skipApplySetCreationOption struct{}

func (skipApplySetCreationOption) Apply(o *GenerateManifestOptions) {
	o.skipApplySet = true
}

// generateAllManifests returns all Kubernetes objects.
func (s *SystemComponents) generateAllManifests(variables Variables, opts ...GenerateManifestOption) ([]client.Object, error) {
	out := []client.Object{}
	var errs []error
	for _, component := range s.AllComponents() {
		manifests, err := component.GenerateManifests(variables, opts...)
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
