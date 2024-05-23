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
	_ "embed"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Component describes the responsibilities of an item in the Inventory.
type Component interface {
	Name() string
	Parse() error
	GenerateManifests(variables Variables) ([]client.Object, error)
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
	ImagePullSecret              string
	DHCP                         string
}

// Manifests holds kubernetes object manifests to be deployed by the operator.
type Manifests struct {
	ArgoCD                  Component
	DPUService              Component
	DPFProvisioning         Component
	ServiceFunctionChainSet Component
	Multus                  Component
	SRIOVDevicePlugin       Component
	NvIPAM                  Component
	OvsCni                  Component
	Flannel                 Component
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

	//go:embed manifests/dpf-provisioning-controller.yaml
	dpfProvisioningControllerData []byte

	//go:embed manifests/argocd.yaml
	argoCDData []byte
)

// New returns a new Manifests inventory with data preloaded but parsing not completed.
func New() *Manifests {
	return &Manifests{

		// TODO: Currently the source for the argo manifests is this repo.
		// Link them to the upstream argo repo and use kustomize to generate the files.
		ArgoCD: &argoCDObjects{
			data: argoCDData,
		},
		DPUService: &dpuServiceControllerObjects{
			data: dpuServiceData,
		},
		DPFProvisioning: &dpfProvisioningControllerObjects{
			data: dpfProvisioningControllerData,
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
	}
}

// ParseAll creates Kubernetes objects for all manifests related to the DPFOperator.
func (m *Manifests) ParseAll() error {
	if err := m.ArgoCD.Parse(); err != nil {
		return err
	}
	if err := m.DPUService.Parse(); err != nil {
		return err
	}
	if err := m.DPFProvisioning.Parse(); err != nil {
		return err
	}
	if err := m.ServiceFunctionChainSet.Parse(); err != nil {
		return err
	}
	if err := m.Multus.Parse(); err != nil {
		return err
	}
	if err := m.SRIOVDevicePlugin.Parse(); err != nil {
		return err
	}
	if err := m.Flannel.Parse(); err != nil {
		return err
	}
	if err := m.NvIPAM.Parse(); err != nil {
		return err
	}
	if err := m.OvsCni.Parse(); err != nil {
		return err
	}
	return nil
}

// generateAllManifests returns all Kubernetes objects.
func (m *Manifests) generateAllManifests(variables Variables) ([]client.Object, error) {
	out := []client.Object{}
	var errs []error
	objs, err := m.ArgoCD.GenerateManifests(variables)
	if err != nil {
		errs = append(errs, err)
	}
	out = append(out, objs...)
	objs, err = m.DPUService.GenerateManifests(variables)
	if err != nil {
		errs = append(errs, err)
	}
	out = append(out, objs...)
	objs, err = m.DPFProvisioning.GenerateManifests(variables)
	if err != nil {
		errs = append(errs, err)
	}
	out = append(out, objs...)
	objs, err = m.ServiceFunctionChainSet.GenerateManifests(variables)
	if err != nil {
		errs = append(errs, err)
	}
	out = append(out, objs...)
	objs, err = m.Multus.GenerateManifests(variables)
	if err != nil {
		errs = append(errs, err)
	}
	out = append(out, objs...)
	objs, err = m.SRIOVDevicePlugin.GenerateManifests(variables)
	if err != nil {
		errs = append(errs, err)
	}
	out = append(out, objs...)
	objs, err = m.Flannel.GenerateManifests(variables)
	if err != nil {
		errs = append(errs, err)
	}
	out = append(out, objs...)
	objs, err = m.NvIPAM.GenerateManifests(variables)
	if err != nil {
		errs = append(errs, err)
	}
	out = append(out, objs...)
	objs, err = m.OvsCni.GenerateManifests(variables)
	if err != nil {
		errs = append(errs, err)
	}
	out = append(out, objs...)
	if len(errs) != 0 {
		return nil, kerrors.NewAggregate(errs)
	}
	return out, nil
}

func (m *Manifests) setDPUService(input dpuServiceControllerObjects) *Manifests {
	m.DPUService = &input
	return m
}

func (m *Manifests) setMultus(input fromDPUService) *Manifests {
	m.Multus = &input
	return m

}

func (m *Manifests) setSRIOVDevicePlugin(input fromDPUService) *Manifests {
	m.SRIOVDevicePlugin = &input
	return m
}

func (m *Manifests) setServiceFunctionChainSet(input fromDPUService) *Manifests {
	m.ServiceFunctionChainSet = &input
	return m
}

func (m *Manifests) setFlannel(input fromDPUService) *Manifests {
	m.Flannel = &input
	return m
}

func (m *Manifests) setNvK8sIpam(input fromDPUService) *Manifests {
	m.NvIPAM = &input
	return m
}

func (m *Manifests) setOvsCni(input fromDPUService) *Manifests {
	m.OvsCni = &input
	return m
}
