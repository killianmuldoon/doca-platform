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
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/utils"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type argoCDObjects struct {
	data    []byte
	objects []unstructured.Unstructured
}

func (a *argoCDObjects) Name() string {
	return "argocd"
}

func (a *argoCDObjects) Parse() error {
	argoObjects, err := utils.BytesToUnstructured(a.data)
	if err != nil {
		return err
	}
	for _, obj := range argoObjects {
		a.objects = append(a.objects, *obj)
	}
	return nil
}

func (a *argoCDObjects) GenerateManifests(vars Variables) ([]client.Object, error) {
	if _, ok := vars.DisableSystemComponents[a.Name()]; ok {
		return []client.Object{}, nil
	}
	ret := []client.Object{}
	objects, err := setNamespace(vars.Namespace, a.objects)
	if err != nil {
		return nil, err
	}
	for i := range objects {
		ret = append(ret, objects[i].DeepCopy())
	}
	return ret, nil
}
