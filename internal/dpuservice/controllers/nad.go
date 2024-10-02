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

package controllers

import (
	"encoding/json"
	"fmt"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"

	"gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/types"
)

// addNetworkAnnotationToServiceDaemonSet adds the network annotation to the ServiceDaemonSet.
// It returns a copy of the ServiceDaemonSet with the network annotation added.
func addNetworkAnnotationToServiceDaemonSet(dpuService *dpuservicev1.DPUService, networks []types.NetworkSelectionElement) (*dpuservicev1.ServiceDaemonSetValues, error) {
	service := dpuService.DeepCopy()
	if service.Spec.ServiceDaemonSet == nil {
		service.Spec.ServiceDaemonSet = &dpuservicev1.ServiceDaemonSetValues{}
	}

	if service.Spec.ServiceDaemonSet.Annotations == nil {
		service.Spec.ServiceDaemonSet.Annotations = map[string]string{}
	}

	if len(networks) == 0 {
		// return serviceDaemonSet as is
		return service.Spec.ServiceDaemonSet, nil
	}

	// look for any existing annotations
	if existingNetworks, ok := service.Spec.ServiceDaemonSet.Annotations[networkAnnotationKey]; ok {
		// Unmarshal existing networks into networks.
		// This effectively merges the existing networks with the new ones.
		// If there are any duplicates, the existing networks take precedence
		err := json.Unmarshal([]byte(existingNetworks), &networks)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal existing networks: %v, expected format is a list of objects", err)
		}
	}

	values, err := json.Marshal(networks)
	if err != nil {
		return nil, err
	}
	// we always update the whole annotation, so we can just overwrite old values
	service.Spec.ServiceDaemonSet.Annotations[networkAnnotationKey] = string(values)

	//merge annotations in serviceDaemonSet
	return service.Spec.ServiceDaemonSet, nil
}

func updateAnnotationsWithNetworks(service *dpuservicev1.DPUService, dpuServiceInterfacesMap map[string]*dpuservicev1.DPUServiceInterface) (map[string]string, error) {
	networks := make([]types.NetworkSelectionElement, 0)
	for _, n := range service.Spec.Interfaces {
		if dpuServiceInterface, found := dpuServiceInterfacesMap[n]; found {
			networks = append(networks, newNetworkSelectionElement(dpuServiceInterface))
		}
	}

	values, err := json.Marshal(networks)
	if err != nil {
		return nil, err
	}
	annotations := service.Spec.ServiceDaemonSet.Annotations
	if len(networks) > 0 {
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[networkAnnotationKey] = string(values)
	}
	return annotations, nil
}

func newNetworkSelectionElement(dpuServiceInterface *dpuservicev1.DPUServiceInterface) types.NetworkSelectionElement {
	ns, name := dpuServiceInterface.Spec.GetTemplateSpec().GetTemplateSpec().Service.GetNetwork()
	var interfaceName string
	if dpuServiceInterface.Spec.GetTemplateSpec().GetTemplateSpec().InterfaceName != nil {
		interfaceName = *dpuServiceInterface.Spec.GetTemplateSpec().GetTemplateSpec().InterfaceName
	}
	return types.NetworkSelectionElement{
		Name:             name,
		Namespace:        ns,
		InterfaceRequest: interfaceName,
	}
}
