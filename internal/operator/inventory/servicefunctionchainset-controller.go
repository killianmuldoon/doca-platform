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
	"fmt"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/utils"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceFunctionChainSetObjects struct {
	data       []byte
	dpuService *dpuservicev1.DPUService
}

func (sfc *ServiceFunctionChainSetObjects) Parse() error {
	if sfc.data == nil {
		return fmt.Errorf("ServiceFunctionChainSetObjects.data can not be empty")
	}
	serviceFunctionChainObjects, err := utils.BytesToUnstructured(sfc.data)
	if err != nil {
		return fmt.Errorf("error while converting ServiceFunctionChainSet controller manifests to objects: %w", err)
	}
	for _, obj := range serviceFunctionChainObjects {
		switch obj.GetObjectKind().GroupVersionKind().Kind {
		case "DPUService":
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &sfc.dpuService)
			if err != nil {
				return fmt.Errorf("error while converting ServiceFunctionChainSet to objects: %w", err)
			}
		default:
			return fmt.Errorf("ServiceFunctionChainSet controller manifests should only contain a DPUService object: found %v", obj.GetObjectKind().GroupVersionKind().Kind)
		}
	}
	if sfc.dpuService == nil {
		return fmt.Errorf("error parsing ServiceFunctionChainSet controller manifests: DPUService not found")
	}
	return nil
}

func (sfc *ServiceFunctionChainSetObjects) Objects() []client.Object {
	return []client.Object{
		sfc.dpuService.DeepCopy(),
	}
}

func (sfc *ServiceFunctionChainSetObjects) SetNamespace(namespace string) {
	sfc.dpuService.SetNamespace(namespace)
}
