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
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/operator/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/release"
)

func newDefaultVariables(defaults *release.Defaults) Variables {
	return Variables{
		Images: map[string]string{
			ProvisioningControllerName: defaults.DPFSystemImage,
			DPUServiceControllerName:   defaults.DPFSystemImage,
		},
		HelmCharts: map[string]string{
			FlannelName:              defaults.DPUNetworkingHelmChart,
			MultusName:               defaults.DPUNetworkingHelmChart,
			SRIOVDevicePluginName:    defaults.DPUNetworkingHelmChart,
			NVIPAMName:               defaults.DPUNetworkingHelmChart,
			OVSCNIName:               defaults.DPUNetworkingHelmChart,
			SFCControllerName:        defaults.DPUNetworkingHelmChart,
			ServiceSetControllerName: defaults.DPUNetworkingHelmChart,
		},
	}
}

// Variables contains information required to generate manifests from the inventory.
type Variables struct {
	Namespace                 string
	DPFProvisioningController DPFProvisioningVariables
	DisableSystemComponents   map[string]bool
	ImagePullSecrets          []string
	Images                    map[string]string
	HelmCharts                map[string]string
}

type DPFProvisioningVariables struct {
	BFBPersistentVolumeClaimName string
	DMSTimeout                   *int
}

func VariablesFromDPFOperatorConfig(defaults *release.Defaults, config *operatorv1.DPFOperatorConfig) Variables {
	variables := newDefaultVariables(defaults)
	disableComponents := make(map[string]bool)
	images := variables.Images
	helmCharts := variables.HelmCharts
	if config.Spec.Overrides != nil {
		for _, item := range config.Spec.Overrides.DisableSystemComponents {
			disableComponents[item] = true
		}
		// TODO: Allow users to override images from the DPFOperatorConfig
	}
	variables.Namespace = config.Namespace
	variables.DPFProvisioningController = DPFProvisioningVariables{
		BFBPersistentVolumeClaimName: config.Spec.ProvisioningConfiguration.BFBPersistentVolumeClaimName,
		DMSTimeout:                   config.Spec.ProvisioningConfiguration.DMSTimeout,
	}
	variables.ImagePullSecrets = config.Spec.ImagePullSecrets
	variables.DisableSystemComponents = disableComponents
	variables.Images = images
	variables.HelmCharts = helmCharts

	return variables
}
