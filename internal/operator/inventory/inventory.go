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

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Manifests holds kubernetes object manifests to be deployed by the operator.
type Manifests struct {
	DPUService              DPUServiceControllerObjects
	ProvCtrl                ProvCtrlObjects
	ServiceFunctionChainSet fromDPUService
	Multus                  fromDPUService
	SRIOVDevicePlugin       fromDPUService
	Flannel                 fromDPUService
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
)

// New returns a new Manifests inventory with data preloaded but parsing not completed.
func New() *Manifests {
	return &Manifests{
		DPUService: DPUServiceControllerObjects{
			data: dpuServiceData,
		},
		ProvCtrl: NewProvisionCtrlObjects(),
		ServiceFunctionChainSet: fromDPUService{
			name: "serviceFunctionChainSet",
			data: serviceChainSetData,
		},
		Multus: fromDPUService{
			name: "multus",
			data: multusData,
		},
		SRIOVDevicePlugin: fromDPUService{
			name: "sriovDevicePlugin",
			data: sriovDevicePluginData,
		},
		Flannel: fromDPUService{
			name: "flannel",
			data: flannelData,
		},
	}
}

// Parse creates typed Kubernetes objects for all manifests related to the DPFOperator.
func (m *Manifests) Parse() error {
	if err := m.DPUService.Parse(); err != nil {
		return err
	}
	if err := m.ProvCtrl.Parse(); err != nil {
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

	return nil
}

// Objects returns all Kubernetes objects.
func (m *Manifests) Objects() []client.Object {
	out := []client.Object{}
	out = append(out, m.DPUService.Objects()...)
	return out
}

func (m *Manifests) setDPUService(input DPUServiceControllerObjects) *Manifests {
	m.DPUService = input
	return m
}

func (m *Manifests) setMultus(input fromDPUService) *Manifests {
	m.Multus = input
	return m

}

func (m *Manifests) setSRIOVDevicePlugin(input fromDPUService) *Manifests {
	m.SRIOVDevicePlugin = input
	return m
}

func (m *Manifests) setServiceFunctionChainSet(input fromDPUService) *Manifests {
	m.ServiceFunctionChainSet = input
	return m
}

func (m *Manifests) setFlannel(input fromDPUService) *Manifests {
	m.Flannel = input
	return m
}
