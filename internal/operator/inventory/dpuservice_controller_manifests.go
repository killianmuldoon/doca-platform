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
	"fmt"

	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/operator/utils"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Component = &dpuServiceControllerObjects{}

// dpuServiceControllerObjects contains Kubernetes objects to be created by the DPUService controller.
type dpuServiceControllerObjects struct {
	data    []byte
	objects []*unstructured.Unstructured
}

func (d *dpuServiceControllerObjects) Name() string {
	return "DPUServiceController"
}

// Parse returns typed objects for the DPUService controller deployment.
func (d *dpuServiceControllerObjects) Parse() error {
	if d.data == nil {
		return fmt.Errorf("dpuServiceControllerObjects.data can not be empty")
	}
	var err error
	objects, err := utils.BytesToUnstructured(d.data)
	if err != nil {
		return fmt.Errorf("error while converting DPUService controller manifests to objects: %w", err)
	}

	for i, obj := range objects {
		switch ObjectKind(obj.GetKind()) {
		// Namespace and CustomResourceDefinition are dropped as they are handled elsewhere.
		case NamespaceKind:
			continue
		case CustomResourceDefinitionKind:
			continue
		}
		d.objects = append(d.objects, objects[i])
	}
	return nil
}

// GenerateManifests returns all objects as a list.
func (d *dpuServiceControllerObjects) GenerateManifests(vars Variables) ([]client.Object, error) {
	if _, ok := vars.DisableSystemComponents[d.Name()]; ok {
		return []client.Object{}, nil
	}

	// make a copy of the objects
	objsCopy := make([]*unstructured.Unstructured, 0, len(d.objects))
	for i := range d.objects {
		objsCopy = append(objsCopy, d.objects[i].DeepCopy())
	}

	// apply edits
	// TODO: make it generic to not edit every kind one-by-one.
	if err := NewEdits().
		AddForAll(NamespaceEdit(vars.Namespace)).
		AddForKindS(DeploymentKind, ImagePullSecretsEditForDeploymentEdit(vars.ImagePullSecrets...)).
		AddForKindS(DeploymentKind, NodeAffinityEdit(&controlPlaneNodeAffinity)).
		AddForKindS(StatefulSetKind, NodeAffinityEdit(&controlPlaneNodeAffinity)).
		AddForKindS(DeploymentKind, TolerationsEdit(controlPlaneTolerations)).
		AddForKindS(StatefulSetKind, TolerationsEdit(controlPlaneTolerations)).
		AddForKindS(DaemonSetKind, TolerationsEdit(controlPlaneTolerations)).
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

// IsReady reports the readiness of the dpuservice controller objects. It returns an error when the number of Replicas in
// the single provisioning controller deployment is true.
func (d *dpuServiceControllerObjects) IsReady(ctx context.Context, c client.Client, namespace string) error {
	return deploymentReadyCheck(ctx, c, namespace, d.objects)
}
