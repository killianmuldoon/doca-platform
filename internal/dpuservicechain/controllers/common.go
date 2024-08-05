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
	"context"
	"fmt"

	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// objectsInDPUClustersReconciler is an interface that enables host cluster reconcilers to reconcile objects in the DPU
// clusters in a standardized way.
type objectsInDPUClustersReconciler interface {
	// getObjectsInDPUCluster is the method called by the reconcileObjectDeletionInDPUClusters function which deletes
	// objects in the DPU cluster related to the given parentObject. The implementation should get the created objects
	// in the DPU cluster.
	getObjectsInDPUCluster(ctx context.Context, dpuClusterClient client.Client, parentObject client.Object) ([]unstructured.Unstructured, error)
	// createOrUpdateChild is the method called by the reconcileObjectsInDPUClusters function which applies changes to
	// the DPU clusters based on the given parentObject. The implementation should create and update objects in the DPU
	// cluster.
	createOrUpdateObjectsInDPUCluster(ctx context.Context, dpuClusterClient client.Client, parentObject client.Object) error
	// deleteObjectsInDPUCluster is the method called by the reconcileObjectDeletionInDPUClusters function which deletes
	// objects in the DPU cluster related to the given parentObject. The implementation should delete objects in the
	// DPU cluster.
	deleteObjectsInDPUCluster(ctx context.Context, dpuClusterClient client.Client, parentObject client.Object) error
}

// shouldRequeueError is an error returned by the functions below that indicates that an event should be requeued. This
// error is useful when one wants to requeue after a specific amount of time.
type shouldRequeueError struct {
	err error
}

func (e *shouldRequeueError) Error() string {
	return e.err.Error()
}

// reconcileObjectDeletionInDPUClusters handles the delete reconciliation loop for objects in the DPU clusters. It
// ensures that objects are completely removed from the DPU clusters. It returns ShouldRequeue error if the request
// should be requeued.
//
//nolint:unparam
func reconcileObjectDeletionInDPUClusters(ctx context.Context,
	r objectsInDPUClustersReconciler,
	k8sClient client.Client,
	dpuServiceObject client.Object,
) error {

	//TODO implement list clusters with label selector
	clusters, err := controlplane.GetDPFClusters(ctx, k8sClient)
	if err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return err
	}
	var existingObjs int
	for _, c := range clusters {
		cl, err := c.NewClient(ctx, k8sClient)
		if err != nil {
			return err
		}
		if err := r.deleteObjectsInDPUCluster(ctx, cl, dpuServiceObject); err != nil {
			return err
		}
		objs, err := r.getObjectsInDPUCluster(ctx, cl, dpuServiceObject)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
		existingObjs += len(objs)
	}

	if existingObjs > 0 {
		return &shouldRequeueError{err: fmt.Errorf("%d objects still exist across all DPU clusters", existingObjs)}
	}

	return nil
}

// reconcileObjectsInDPUClusters handles the main reconciliation loop for objects in the DPU clusters
//
//nolint:unparam
func reconcileObjectsInDPUClusters(ctx context.Context,
	r objectsInDPUClustersReconciler,
	k8sClient client.Client,
	dpuServiceObject client.Object,
) error {

	// Get the list of clusters this DPUServiceObject targets.
	//TODO implement list clusters with label selector
	clusters, err := controlplane.GetDPFClusters(ctx, k8sClient)
	if err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return err
	}
	for _, c := range clusters {
		cl, err := c.NewClient(ctx, k8sClient)
		if err != nil {
			return err
		}
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceObject.GetNamespace()}}
		if err := cl.Create(ctx, ns); err != nil {
			// Fail if this returns any error other than alreadyExists.
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
		if err := r.createOrUpdateObjectsInDPUCluster(ctx, cl, dpuServiceObject); err != nil {
			return err
		}
	}
	return nil
}
