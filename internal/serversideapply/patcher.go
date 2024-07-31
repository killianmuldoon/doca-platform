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

// Package serversideapply implements Kubernetes server side apply patching
package serversideapply

import (
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Patch updates resources with server-side apply. If a status is set the status will be patched also.
func Patch(ctx context.Context, c client.Client, owner string, obj client.Object) error {
	objNamespaceName := fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())

	originalUnstructured := &unstructured.Unstructured{}
	if err := c.Scheme().Convert(obj, originalUnstructured, nil); err != nil {
		return fmt.Errorf("convert object %s to unstructured: %v", objNamespaceName, err)
	}
	desiredUnstructured := originalUnstructured.DeepCopy()
	originalUnstructured.SetManagedFields(nil)
	originalUnstructured.SetResourceVersion("")

	if err := c.Patch(ctx, originalUnstructured, client.Apply, client.ForceOwnership, client.FieldOwner(owner)); err != nil {
		return fmt.Errorf("patch object %s: %v", objNamespaceName, err)
	}

	return statusPatch(ctx, c, owner, desiredUnstructured, originalUnstructured)
}

// statusPatch checks if a status is even specified for this resource and if there is a diff between the current
// object and the desired object. If not it will return early to reduce API calls.
func statusPatch(ctx context.Context, c client.Client, owner string,
	desiredUnstructured, originalUnstructured *unstructured.Unstructured) error {
	objNamespaceName := fmt.Sprintf("%s/%s", originalUnstructured.GetNamespace(), originalUnstructured.GetName())

	// Return early if there is no status.
	statusAfter, exists, err := unstructured.NestedMap(desiredUnstructured.Object, "status")
	if err != nil {
		return fmt.Errorf("extract status from object %s: %v", objNamespaceName, err)
	}
	if !exists {
		return nil
	}

	statusBefore, _, err := unstructured.NestedMap(originalUnstructured.Object, "status")
	if err != nil {
		return fmt.Errorf("extract status from object %s: %v", objNamespaceName, err)
	}

	// Return early if there is no change to the status.
	if reflect.DeepEqual(statusBefore, statusAfter) {
		return nil
	}

	// We only want to set the status of our object. Thus, we have to create a new unstructured object and set all the
	// necessary metadata to identify the object.
	newStatus := &unstructured.Unstructured{}
	newStatus.SetAPIVersion(originalUnstructured.GetAPIVersion())
	newStatus.SetNamespace(originalUnstructured.GetNamespace())
	newStatus.SetKind(originalUnstructured.GetKind())
	newStatus.SetName(originalUnstructured.GetName())

	// Set the actual status that we want to update via server-side apply.
	if err := unstructured.SetNestedMap(newStatus.Object, statusAfter, "status"); err != nil {
		return fmt.Errorf("set new status to object %s: %v", objNamespaceName, err)
	}

	if err := c.Status().Patch(ctx, newStatus, client.Apply, client.ForceOwnership, client.FieldOwner(owner)); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("patch status of object %s: %v", objNamespaceName, err)
		}
	}

	return nil
}
