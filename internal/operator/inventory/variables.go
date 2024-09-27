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
		DisableSystemComponents: map[string]bool{
			operatorv1.ProvisioningControllerName: false,
			operatorv1.DPUServiceControllerName:   false,
			operatorv1.ServiceSetControllerName:   false,
			operatorv1.FlannelName:                false,
			operatorv1.MultusName:                 false,
			operatorv1.SRIOVDevicePluginName:      false,
			operatorv1.OVSCNIName:                 false,
			operatorv1.NVIPAMName:                 false,
			operatorv1.SFCControllerName:          false,

			// Both control plane managers are disabled by default.
			operatorv1.StaticControlPlaneManagerName: true,
			operatorv1.HostedControlPlaneManagerName: true,
		},
		Images: map[string]string{
			operatorv1.ProvisioningControllerName:    defaults.DPFSystemImage,
			operatorv1.DPUServiceControllerName:      defaults.DPFSystemImage,
			operatorv1.StaticControlPlaneManagerName: defaults.DPFSystemImage,
			operatorv1.HostedControlPlaneManagerName: defaults.DPFSystemImage,

			// Components installed with helm charts which can also have their images overwritten.
			operatorv1.FlannelName:              defaults.DPUNetworkingHelmChart,
			operatorv1.MultusName:               defaults.DPUNetworkingHelmChart,
			operatorv1.SRIOVDevicePluginName:    defaults.DPUNetworkingHelmChart,
			operatorv1.NVIPAMName:               defaults.DPUNetworkingHelmChart,
			operatorv1.OVSCNIName:               defaults.DPUNetworkingHelmChart,
			operatorv1.SFCControllerName:        defaults.DPUNetworkingHelmChart,
			operatorv1.ServiceSetControllerName: defaults.DPUNetworkingHelmChart,
		},
		HelmCharts: map[string]string{
			operatorv1.FlannelName:              defaults.DPUNetworkingHelmChart,
			operatorv1.MultusName:               defaults.DPUNetworkingHelmChart,
			operatorv1.SRIOVDevicePluginName:    defaults.DPUNetworkingHelmChart,
			operatorv1.NVIPAMName:               defaults.DPUNetworkingHelmChart,
			operatorv1.OVSCNIName:               defaults.DPUNetworkingHelmChart,
			operatorv1.SFCControllerName:        defaults.DPUNetworkingHelmChart,
			operatorv1.ServiceSetControllerName: defaults.DPUNetworkingHelmChart,
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
	disableComponents := variables.DisableSystemComponents
	images := variables.Images
	helmCharts := variables.HelmCharts
	for _, componentConfig := range config.ComponentConfigs() {
		if componentConfig != nil {
			disableComponents[componentConfig.Name()] = componentConfig.Disabled()
			if componentConfig.GetImage() != nil {
				images[componentConfig.Name()] = *componentConfig.GetImage()
			}

			// If the component is a helmComponent set the helm chart.
			helmConfig, ok := componentConfig.(operatorv1.HelmComponentConfig)
			if ok && helmConfig.GetHelmChart() != nil {
				helmCharts[componentConfig.Name()] = *helmConfig.GetHelmChart()
			}
		}
	}

	variables.Namespace = config.Namespace
	variables.DPFProvisioningController = DPFProvisioningVariables{
		BFBPersistentVolumeClaimName: config.Spec.ProvisioningController.BFBPersistentVolumeClaimName,
		DMSTimeout:                   config.Spec.ProvisioningController.DMSTimeout,
	}
	variables.ImagePullSecrets = config.Spec.ImagePullSecrets
	variables.DisableSystemComponents = disableComponents
	variables.Images = images
	variables.HelmCharts = helmCharts

	return variables
}
