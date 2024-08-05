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
	"maps"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"

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

var _ serviceSetReconciler = &ServiceChainSetReconciler{}

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

//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch;update
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechainsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechainsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechainsets/finalizers,verbs=update
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains,verbs=get;list;watch;create;update;patch;delete
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
	if !scs.ObjectMeta.DeletionTimestamp.IsZero() {
		// Return early, the object is deleting.
		return ctrl.Result{}, nil
	}
	return reconcileSet(ctx, scs, r.Client, scs.Spec.NodeSelector, r)
}

func (r *ServiceChainSetReconciler) getChildMap(ctx context.Context, set client.Object) (map[string]client.Object, error) {
	serviceChainMap := make(map[string]client.Object)
	serviceChainList := &sfcv1.ServiceChainList{}
	if err := r.List(ctx, serviceChainList, client.MatchingLabels{
		ServiceChainSetNameLabel:      set.GetName(),
		ServiceChainSetNamespaceLabel: set.GetNamespace(),
	}); err != nil {
		return serviceChainMap, err
	}
	for i := range serviceChainList.Items {
		sc := serviceChainList.Items[i]
		serviceChainMap[sc.Spec.Node] = &sc
	}
	return serviceChainMap, nil
}

func (r *ServiceChainSetReconciler) createOrUpdateChild(ctx context.Context, set client.Object, nodeName string) error {
	log := log.FromContext(ctx)
	serviceChainSet := set.(*sfcv1.ServiceChainSet)
	labels := map[string]string{ServiceChainSetNameLabel: serviceChainSet.Name,
		ServiceChainSetNamespaceLabel: serviceChainSet.Namespace}
	maps.Copy(labels, serviceChainSet.Spec.Template.ObjectMeta.Labels)
	scName := serviceChainSet.Name + "-" + nodeName
	owner := metav1.NewControllerRef(serviceChainSet,
		sfcv1.GroupVersion.WithKind("ServiceChainSet"))
	switches := make([]sfcv1.Switch, len(serviceChainSet.Spec.Template.Spec.Switches))
	for i, s := range serviceChainSet.Spec.Template.Spec.Switches {
		switches[i] = sfcv1.Switch{}
		ports := make([]sfcv1.Port, len(s.Ports))
		for j, p := range s.Ports {
			ports[j] = sfcv1.Port{}
			if p.Service != nil {
				ports[j].Service = p.Service.DeepCopy()
			}
			if p.ServiceInterface != nil {
				ports[j].ServiceInterface = p.ServiceInterface.DeepCopy()
				if p.ServiceInterface.Reference != nil {
					ports[j].ServiceInterface.Reference = p.ServiceInterface.Reference.DeepCopy()
					if p.ServiceInterface.Reference.Name != "" {
						ports[j].ServiceInterface.Reference.Name = p.ServiceInterface.Reference.Name + "-" + nodeName
					}
				}
			}
		}
		switches[i].Ports = ports
	}
	sc := &sfcv1.ServiceChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:            scName,
			Namespace:       serviceChainSet.Namespace,
			Labels:          labels,
			Annotations:     serviceChainSet.Annotations,
			OwnerReferences: []metav1.OwnerReference{*owner},
		},
		Spec: sfcv1.ServiceChainSpec{
			Node:     nodeName,
			Switches: switches,
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
