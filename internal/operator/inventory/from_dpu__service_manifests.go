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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Component = &fromDPUService{}

type fromDPUService struct {
	data       []byte
	name       string
	dpuService *dpuservicev1.DPUService
}

func (f *fromDPUService) Name() string {
	return f.name
}

func (f *fromDPUService) Parse() error {
	if f.data == nil {
		return fmt.Errorf("data for DPUService %s can not be empty", f.name)
	}
	objects, err := utils.BytesToUnstructured(f.data)
	if err != nil {
		return fmt.Errorf("error while converting DPUService %v manifest to object: %w", f.name, err)
	}
	for _, obj := range objects {
		switch obj.GetObjectKind().GroupVersionKind().Kind {
		case "DPUService":
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &f.dpuService)
			if err != nil {
				return fmt.Errorf("error while converting data to objects: %w", err)
			}
		default:
			return fmt.Errorf("manifests for %s should only contain a DPUService object: found %v", f.name, obj.GetObjectKind().GroupVersionKind().Kind)
		}
	}
	if f.dpuService == nil {
		return fmt.Errorf("error parsing manifests for %v: DPUService not found", f.name)
	}
	return nil
}

func (f *fromDPUService) GenerateManifests(variables Variables) ([]client.Object, error) {
	if _, ok := variables.DisableSystemComponents[f.Name()]; ok {
		return []client.Object{}, nil
	}
	f.dpuService.SetNamespace(variables.Namespace)
	copy := f.dpuService.DeepCopy()
	if variables.ImagePullSecrets != nil {
		var localObjectRefs []corev1.LocalObjectReference

		for _, secret := range variables.ImagePullSecrets {
			localObjectRefs = append(localObjectRefs, corev1.LocalObjectReference{Name: secret})
		}
		copy.Spec.Values = &runtime.RawExtension{
			Object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"imagePullSecrets": localObjectRefs,
				},
			},
		}
	} else {
		copy.Spec.Values = nil
	}
	return []client.Object{copy}, nil
}
