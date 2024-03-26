/*
Copyright 2024.

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

package utils

import (
	"bytes"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// BytesToUnstructured converts a slice of Bytes to Unstructured objects. This function is useful when you want to read
// a yaml manifest and covert it to Kubernetes objects.
func BytesToUnstructured(b []byte) ([]*unstructured.Unstructured, error) {
	objs := []*unstructured.Unstructured{}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(b), 4096)
	for {
		obj := unstructured.Unstructured{}
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to unmarshal content into unstructured: %w", err)
		}

		if obj.GetKind() == "" {
			continue
		}

		objs = append(objs, &obj)
	}
	return objs, nil
}
