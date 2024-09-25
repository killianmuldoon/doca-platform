/*
COPYRIGHT 2024 NVIDIA

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

package controller

import (
	"context"
	"fmt"

	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// namespacedNameInCluster is a struct that points to a particular object within a cluster
//
//nolint:unused
type namespacedNameInCluster struct {
	Object  types.NamespacedName
	Cluster controlplane.DPFCluster
}

//nolint:unused
func (n *namespacedNameInCluster) String() string {
	return fmt.Sprintf("{Cluster: %s/%s, Object: %s/%s}", n.Cluster.Namespace, n.Cluster.Name, n.Object.Namespace, n.Object.Name)
}

type serviceSetReconciler interface {
	// getObjectsInDPUCluster is the method called by the reconcileReadinessOfObjectsInDPUClusters function which deletes
	// objects in the DPU cluster related to the given parentObject. The implementation should get the created objects
	// in the DPU cluster.
	getObjectsInDPUCluster(ctx context.Context, k8sClient client.Client, dpuObject client.Object) ([]unstructured.Unstructured, error)
	// getChildMap creates a map of ServiceChains for every node inside the cluster called by reconcileSet
	// and reconcileDelete.
	getChildMap(context.Context, client.Object) (map[string]client.Object, error)
	// createOrUpdateChild is the method called by the reconcileSet function which applies changes to
	// the DPU clusters based on the given parentObject. The implementation should create and update objects in the DPU
	// cluster.
	createOrUpdateChild(context.Context, client.Object, string) error
	// getUnreadyObjects is the method called by reconcileReadinessOfObjectsInDPUClusters function which returns whether
	// objects in the DPU cluster are ready. The input to the function is a list of objects that exist in a particular
	// cluster.
	getUnreadyObjects(objects []unstructured.Unstructured) ([]types.NamespacedName, error)
}

//nolint:unparam
func reconcileSet(
	ctx context.Context,
	set client.Object,
	k8sClient client.Client,
	selector *metav1.LabelSelector,
	reconciler serviceSetReconciler,
) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Get node list by nodeSelector
	nodeList, err := getNodeList(ctx, k8sClient, selector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get Node list: %w", err)
	}

	// Get Childs map (node->child) which are owned by Set
	childMap, err := reconciler.getChildMap(ctx, set)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get Child list: %w", err)
	}

	// create or update Child for the node
	for _, node := range nodeList.Items {
		if err = reconciler.createOrUpdateChild(ctx, set, node.Name); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create or update Child: %w", err)
		}
		delete(childMap, node.Name)
	}

	// delete Child if node does not exist
	for nodeName, child := range childMap {
		if err := k8sClient.Delete(ctx, child); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete child : %w", err)
		}
		log.Info("Child is deleted", "nodename", nodeName, "child", child)
	}

	return ctrl.Result{}, nil
}

func deleteSet(ctx context.Context, set client.Object, k8sClient client.Client,
	reconciler serviceSetReconciler) error {
	log := ctrllog.FromContext(ctx)

	// Get Childs map (node->child) which are owned by Set
	childMap, err := reconciler.getChildMap(ctx, set)
	if err != nil {
		return fmt.Errorf("failed to get Child list: %w", err)
	}

	// delete Child if node does not exist
	for nodeName, child := range childMap {
		if err := k8sClient.Delete(ctx, child); err != nil {
			return fmt.Errorf("failed to delete child : %w", err)
		}
		log.Info("Child is deleted", "nodename", nodeName, "child", child)
	}

	return nil
}

func getNodeList(ctx context.Context, k8sClient client.Client, selector *metav1.LabelSelector) (*corev1.NodeList, error) {
	nodeList := &corev1.NodeList{}
	nodeSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}
	listOptions := client.ListOptions{
		LabelSelector: nodeSelector,
	}

	if err := k8sClient.List(ctx, nodeList, &listOptions); err != nil {
		return nil, err
	}

	return nodeList, nil
}

//nolint:unparam
func reconcileDelete(ctx context.Context, set client.Object, k8sClient client.Client,
	reconciler serviceSetReconciler, finalizerStr string) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling delete")
	if err := deleteSet(ctx, set, k8sClient, reconciler); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(set, finalizerStr)

	return ctrl.Result{}, nil
}

// reconcileReadinessOfObjectsInDPUClusters handles the readiness reconciliation loop for objects in the DPU clusters
// TODO not implemented
//
//nolint:unparam,unused
func reconcileReadinessOfObjectsInDPUClusters(
	ctx context.Context,
	r serviceSetReconciler,
	k8sClient client.Client,
	serviceSet client.Object,
) ([]namespacedNameInCluster, error) {
	return nil, nil
}

// updateSummary updates the status conditions in the object.
// TODO not implemented
//
//nolint:unparam,unused
func updateSummary(
	ctx context.Context,
	r serviceSetReconciler,
	k8sClient client.Client,
	objReadyCondition conditions.ConditionType,
	serviceSet client.Object,
) error {
	return nil
}
