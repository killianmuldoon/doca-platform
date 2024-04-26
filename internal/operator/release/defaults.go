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

package release

import (
	_ "embed"
	"errors"

	"sigs.k8s.io/yaml"
)

//go:embed manifests/defaults.yaml
var defaultsContent []byte

// Defaults structure contains the default artifacts that the operator is using
type Defaults struct {
	// CustomOVNKubernetesDPUImage is the default custom OVN Kubernetes image deployed by the operator to the DPU
	// enabled workers.
	CustomOVNKubernetesDPUImage string `yaml:"customOVNKubernetesDPUImage"`
	// CustomOVNKubernetesNonDPUImage is the default custom OVN Kubernetes image deployed by the operator to the
	// non DPU nodes.
	CustomOVNKubernetesNonDPUImage string `yaml:"customOVNKubernetesNonDPUImage"`
}

// Parse parses the defaults from the embedded generated YAML file
// TODO: Add more validations here as needed.
func (d *Defaults) Parse() error {
	err := yaml.Unmarshal(defaultsContent, d)
	if err != nil {
		return err
	}
	if len(d.CustomOVNKubernetesDPUImage) == 0 {
		return errors.New("customOVNKubernetesDPUImage can't be empty")
	}
	if len(d.CustomOVNKubernetesNonDPUImage) == 0 {
		return errors.New("customOVNKubernetesNonDPUImage can't be empty")
	}
	return nil
}

// NewDefaults creates a new defaults object
func NewDefaults() *Defaults {
	return &Defaults{}
}
