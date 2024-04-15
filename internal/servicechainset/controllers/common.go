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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type serviceSetReconciler interface {
	getChildMap(context.Context, client.Object) (map[string]client.Object, error)
	createOrUpdateChild(context.Context, client.Object, string) error
}

func reconcileSet(ctx context.Context, set client.Object, k8sClient client.Client,
	selector *metav1.LabelSelector, reconciler serviceSetReconciler) (ctrl.Result, error) {
	log := log.FromContext(ctx)
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
