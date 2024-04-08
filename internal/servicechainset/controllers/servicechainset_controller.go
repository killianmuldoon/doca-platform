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

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ServiceChainSetReconciler reconciles a ServiceChainSet object
type ServiceChainSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	ServiceChainSetNameLabel      = "sfc.dpf.nvidia.com/servicechainset-name"
	ServiceChainSetNamespaceLabel = "sfc.dpf.nvidia.com/servicechainset-namespace"
	serviceChainSetControllerName = "service-chain-set-controller"
)

//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechainsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechainsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechainsets/finalizers,verbs=update
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

func (r *ServiceChainSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling")
	scs := &sfcv1.ServiceChainSet{}
	if err := r.Client.Get(ctx, req.NamespacedName, scs); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	return r.reconcile(ctx, scs)
}

func (r *ServiceChainSetReconciler) reconcile(ctx context.Context, scs *sfcv1.ServiceChainSet) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	// Get node map by nodeSelector
	nodeMap, err := r.getNodeMap(ctx, scs.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get Node list: %w", err)
	}
	// Get ServiceChain map which are owned by serviceChainSet
	serviceChainMap, err := r.getServiceChainMap(ctx, scs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get ServiceChain list: %w", err)
	}
	// create ServiceChain for the node
	for nodeName := range nodeMap {
		if _, ok := serviceChainMap[nodeName]; !ok {
			if err = r.createServiceChain(ctx, scs, nodeName); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to create ServiceChain: %w", err)
			}
		} else {
			delete(serviceChainMap, nodeName)
		}
	}
	// delete ServiceChain if node does not exist
	for nodeName, sc := range serviceChainMap {
		if err := r.Delete(ctx, sc); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete ServiceChain: %w", err)
		}
		log.Info("ServiceChain is deleted", "nodename", nodeName, "serviceChain", sc)
	}

	return ctrl.Result{}, nil
}

func (r *ServiceChainSetReconciler) getNodeMap(ctx context.Context, selector *metav1.LabelSelector) (map[string]struct{}, error) {
	nodeMap := make(map[string]struct{})
	nodeList := &corev1.NodeList{}
	nodeSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nodeMap, err
	}
	listOptions := client.ListOptions{
		LabelSelector: nodeSelector,
	}

	if err := r.List(ctx, nodeList, &listOptions); err != nil {
		return nodeMap, err
	}

	for i := range nodeList.Items {
		node := nodeList.Items[i]
		nodeMap[node.Name] = struct{}{}
	}
	return nodeMap, nil
}

func (r *ServiceChainSetReconciler) getServiceChainMap(ctx context.Context, scs *sfcv1.ServiceChainSet) (map[string]*sfcv1.ServiceChain, error) {
	serviceChainMap := make(map[string]*sfcv1.ServiceChain)
	serviceChainList := &sfcv1.ServiceChainList{}
	if err := r.List(ctx, serviceChainList, client.MatchingLabels{
		ServiceChainSetNameLabel:      scs.Name,
		ServiceChainSetNamespaceLabel: scs.Namespace,
	}); err != nil {
		return serviceChainMap, err
	}
	for i := range serviceChainList.Items {
		sc := serviceChainList.Items[i]
		serviceChainMap[sc.Spec.Node] = &sc
	}
	return serviceChainMap, nil
}

func (r *ServiceChainSetReconciler) createServiceChain(ctx context.Context, serviceChainSet *sfcv1.ServiceChainSet,
	nodeName string) error {
	log := log.FromContext(ctx)
	labels := map[string]string{ServiceChainSetNameLabel: serviceChainSet.Name,
		ServiceChainSetNamespaceLabel: serviceChainSet.Namespace}
	for k, v := range serviceChainSet.Labels {
		labels[k] = v
	}
	scName := serviceChainSet.Name + "-" + nodeName
	owner := metav1.NewControllerRef(serviceChainSet,
		sfcv1.GroupVersion.WithKind("ServiceChainSet"))

	sc := &sfcv1.ServiceChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:            scName,
			Namespace:       serviceChainSet.Namespace,
			Labels:          labels,
			Annotations:     serviceChainSet.Annotations,
			OwnerReferences: []metav1.OwnerReference{*owner},
		},
		Spec: sfcv1.ServiceChainSpec{
			Node: nodeName,
		},
	}
	sc.ObjectMeta.ManagedFields = nil
	sc.SetGroupVersionKind(sfcv1.GroupVersion.WithKind("ServiceChain"))
	if err := r.Client.Patch(ctx, sc, client.Apply, client.ForceOwnership, client.FieldOwner(serviceChainSetControllerName)); err != nil {
		return err
	}
	log.Info("ServiceChain is created", "ServiceChain", sc)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceChainSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfcv1.ServiceChainSet{}).
		Owns(&sfcv1.ServiceChain{}).
		Watches(&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.nodeToServiceChainSetReq),
			builder.WithPredicates(predicate.LabelChangedPredicate{})).
		Complete(r)
}

func (r *ServiceChainSetReconciler) nodeToServiceChainSetReq(ctx context.Context, resource client.Object) []reconcile.Request {
	requests := make([]reconcile.Request, 0)
	serviceChainSetList := &sfcv1.ServiceChainSetList{}
	if err := r.List(ctx, serviceChainSetList); err == nil {
		for _, item := range serviceChainSetList.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				}})
		}
	}
	return requests
}
