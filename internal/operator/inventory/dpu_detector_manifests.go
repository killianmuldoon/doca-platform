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
	"context"
	_ "embed"
	"fmt"

	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/operator/utils"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dpfDPUDetectorName = "dpf-dpu-detector"
)

var _ Component = &dpuDetectorObjects{}

// dpuDetectorObjects contains objects that are used to generate dpu detector manifests.
// dpuDetectorObjects objects should be immutable after Parse()
type dpuDetectorObjects struct {
	data    []byte
	objects []*unstructured.Unstructured
}

func (p *dpuDetectorObjects) Name() string {
	return "DPFDPUDetector"
}

// Parse returns typed objects for the DPU Detector daemonset.
func (p *dpuDetectorObjects) Parse() (err error) {
	if p.data == nil {
		return fmt.Errorf("dpuDetectorObjects.data can not be empty")
	}
	objs, err := utils.BytesToUnstructured(p.data)
	if err != nil {
		return fmt.Errorf("error while converting DPU Detector manifests to objects: %w", err)
	} else if len(objs) == 0 {
		return fmt.Errorf("no objects found in DPU Detector manifests")
	}

	daemonsetFound := false
	for _, obj := range objs {
		// Exclude Namespace and CustomResourceDefinition as the operator should not deploy these resources.
		if obj.GetKind() == string(NamespaceKind) || obj.GetKind() == string(CustomResourceDefinitionKind) {
			continue
		}
		// If the object is the dpf-dpu-detector DeamonSet validate it
		if obj.GetKind() == string(DaemonsetKind) && obj.GetName() == dpfDPUDetectorName {
			daemonsetFound = true
		}
		p.objects = append(p.objects, obj)
	}

	if !daemonsetFound {
		return fmt.Errorf("error while converting DPU detector manifests to objects: DaemonSet not found")
	}

	return nil
}

// GenerateManifests applies edits and returns objects
func (p *dpuDetectorObjects) GenerateManifests(vars Variables, options ...GenerateManifestOption) ([]client.Object, error) {
	if _, ok := vars.DisableSystemComponents[p.Name()]; ok {
		return []client.Object{}, nil
	}

	opts := &GenerateManifestOptions{}
	for _, option := range options {
		option.Apply(opts)
	}

	// make a copy of the objects
	objsCopy := make([]*unstructured.Unstructured, 0, len(p.objects))
	for i := range p.objects {
		objsCopy = append(objsCopy, p.objects[i].DeepCopy())
	}

	// apply edits
	if err := NewEdits().
		AddForAll(NamespaceEdit(vars.Namespace)).
		AddForKindS(DaemonsetKind, ImagePullSecretsEditForDaemonSetEdit(vars.ImagePullSecrets...)).
		Apply(objsCopy); err != nil {
		return nil, err
	}

	// return as Objects
	ret := make([]client.Object, 0, len(objsCopy))
	for i := range objsCopy {
		ret = append(ret, objsCopy[i])
	}

	return ret, nil
}

// IsReady reports the readiness of the dpu detector objects. It returns an error when the number of Replicas in
// the single dpu detector daemonset is true.
func (p *dpuDetectorObjects) IsReady(ctx context.Context, c client.Client, namespace string) error {
	return daemonsetReadyCheck(ctx, c, namespace, p.objects)
}
