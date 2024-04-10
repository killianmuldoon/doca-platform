/*
Copyright 2024 NVIDIA.

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

package config

// DPUCNIProvisionerConfig is the format of the config file the DPU CNI Provisioner is expecting. Notice that this is
// one config for all nodes because this is how the Operator is going to deploy that.
// TODO: Consider moving this structure to the Operator if it makes more sense
type DPUCNIProvisionerConfig struct {
	// VTEPIPs repesents the IPs that will be assigned to the VTEP interface on each DPU.
	VTEPIPs map[string]string `json:"vtepIPs"`
}
