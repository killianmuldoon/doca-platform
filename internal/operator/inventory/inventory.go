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
	DPUService              DPUServiceObjects
	ProvCtrl                ProvCtrlObjects
	ServiceFunctionChainSet ServiceFunctionChainSetObjects
}

// Embed manifests for Kubernetes objects created by the controller.
var (
	//go:embed manifests/dpuservice-controller.yaml
	dpuServiceData []byte

	//go:embed manifests/servicefunctionchainset-controller.yaml
	serviceChainSetData []byte
)

// New returns a new Manifests inventory with data preloaded but parsing not completed.
func New() *Manifests {
	return &Manifests{
		DPUService: DPUServiceObjects{
			data: dpuServiceData,
		},
		ProvCtrl: NewProvisionCtrlObjects(),
		ServiceFunctionChainSet: ServiceFunctionChainSetObjects{
			data: serviceChainSetData,
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
	return nil
}

// Objects returns all Kubernetes objects.
func (m *Manifests) Objects() []client.Object {
	out := []client.Object{}
	out = append(out, m.DPUService.Objects()...)
	return out
}
