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

// HostCNIProvisionerConfig is the format of the config file the Host CNI Provisioner is expecting. Notice that this is
// one config for all nodes because this is how the Operator is going to deploy that.
type HostCNIProvisionerConfig struct {
	// PerNodeConfig represents the configuration settings for each particular Host cluster node. Key is the name of
	// the Host cluster node.
	PerNodeConfig map[string]PerNodeConfig `json:"perNodeConfig"`
}

// PerNodeConfig is the struct that holds configuration specific to a particular node. This config is extracted out of
// the HostCNIProvisionerConfig based on the node the Host CNI Provisioner is running on.
type PerNodeConfig struct {
	// PFIP repesents the IP that will be assigned to the PF interface on that particular Host.
	PFIP string `json:"pfIP"`
	// HostPF0 is the name of the PF0 on the host.
	HostPF0 string `json:"hostPF0"`
}
