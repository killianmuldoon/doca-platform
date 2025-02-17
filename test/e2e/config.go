/*
Copyright 2025 NVIDIA

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

package e2e

import (
	"os"

	"sigs.k8s.io/yaml"
)

type config struct {
	ProvisioningControllerPVCPath     string   `json:"provisioningControllerPVC"`
	BFBPath                           string   `json:"bfb"`
	DPUClusterPath                    string   `json:"dpuCluster"`
	DPUSetPath                        string   `json:"dpuSet"`
	DPUServiceInterfacePath           string   `json:"dpuServiceInterface"`
	DPUServiceChainPath               string   `json:"dpuServiceChain"`
	DPUDeploymentPath                 string   `json:"dpuDeployment"`
	DPUServiceChainTemplatePath       string   `json:"dpuServiceChainTemplate"`
	DPUServiceCredentialRequestPath   string   `json:"dpuServiceCredentialRequest"`
	IPPoolDPUServiceIPAMPath          string   `json:"ipPoolDPUServiceIPAM"`
	CIDRPoolDPUServiceIPAMPath        string   `json:"cidrPoolDPUServiceIPAM"`
	DPUServicePath                    string   `json:"dpuService"`
	DPUServiceTemplatePath            string   `json:"dpuServiceTemplate"`
	DPUServiceConfiguration           string   `json:"dpuServiceConfiguration"`
	DPUClusterPrerequisiteObjectPaths []string `json:"dpuClusterPrerequisiteObjectPath"`
	NumberOfDPUNodes                  int      `json:"numberOfDPUNodes"`
}

func readConfig(path string) (*config, error) {
	configData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	conf := &config{}
	if err = yaml.UnmarshalStrict(configData, conf); err != nil {
		return nil, err
	}
	return conf, nil
}
