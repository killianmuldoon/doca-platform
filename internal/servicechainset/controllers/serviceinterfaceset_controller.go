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
	"maps"

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

const (
	ServiceInterfaceSetNameLabel      = "sfc.dpf.nvidia.com/serviceinterfaceset-name"
	ServiceInterfaceSetNamespaceLabel = "sfc.dpf.nvidia.com/serviceinterfaceset-namespace"
	serviceInterfaceSetControllerName = "service-interface-set-controller"
)

// ServiceInterfaceSetReconciler reconciles a ServiceInterfaceSet object
type ServiceInterfaceSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfacesets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfacesets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfacesets/finalizers,verbs=update
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfaces/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ServiceInterfaceSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling")

	sis := &sfcv1.ServiceInterfaceSet{}
	if err := r.Client.Get(ctx, req.NamespacedName, sis); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	return r.reconcile(ctx, sis)
}

func (r *ServiceInterfaceSetReconciler) reconcile(ctx context.Context, sis *sfcv1.ServiceInterfaceSet) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	// Get node list by nodeSelector
	nodeList, err := r.getNodeList(ctx, sis.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get Node list: %w", err)
	}
	// Get ServiceInterface map which are owned by serviceInterfaceSet
	serviceInterfaceMap, err := r.getServiceInterfaceMap(ctx, sis)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get ServiceInterface list: %w", err)
	}
	// create or update ServiceInterface for the node
	for _, node := range nodeList.Items {
		if err = r.createOrUpdateServiceInterface(ctx, sis, node.Name); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create or update ServiceInterface: %w", err)
		}
		delete(serviceInterfaceMap, node.Name)
	}
	// delete ServiceInterface if node does not exist
	for nodeName, sc := range serviceInterfaceMap {
		if err := r.Delete(ctx, sc); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete ServiceInterface: %w", err)
		}
		log.Info("ServiceInterface is deleted", "nodename", nodeName, "serviceInterface", sc)
	}

	return ctrl.Result{}, nil
}

func (r *ServiceInterfaceSetReconciler) getNodeList(ctx context.Context, selector *metav1.LabelSelector) (*corev1.NodeList, error) {
	nodeList := &corev1.NodeList{}
	nodeSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}
	listOptions := client.ListOptions{
		LabelSelector: nodeSelector,
	}

	if err := r.List(ctx, nodeList, &listOptions); err != nil {
		return nil, err
	}

	return nodeList, nil
}

func (r *ServiceInterfaceSetReconciler) getServiceInterfaceMap(ctx context.Context, sis *sfcv1.ServiceInterfaceSet) (map[string]*sfcv1.ServiceInterface, error) {
	serviceInterfaceMap := make(map[string]*sfcv1.ServiceInterface)
	serviceInterfaceList := &sfcv1.ServiceInterfaceList{}
	if err := r.List(ctx, serviceInterfaceList, client.MatchingLabels{
		ServiceInterfaceSetNameLabel:      sis.Name,
		ServiceInterfaceSetNamespaceLabel: sis.Namespace,
	}); err != nil {
		return serviceInterfaceMap, err
	}
	for i := range serviceInterfaceList.Items {
		sc := serviceInterfaceList.Items[i]
		serviceInterfaceMap[sc.Spec.Node] = &sc
	}
	return serviceInterfaceMap, nil
}

func (r *ServiceInterfaceSetReconciler) createOrUpdateServiceInterface(ctx context.Context, serviceInterfaceSet *sfcv1.ServiceInterfaceSet,
	nodeName string) error {
	log := log.FromContext(ctx)
	labels := map[string]string{ServiceInterfaceSetNameLabel: serviceInterfaceSet.Name,
		ServiceInterfaceSetNamespaceLabel: serviceInterfaceSet.Namespace}
	maps.Copy(labels, serviceInterfaceSet.Spec.Labels)
	scName := serviceInterfaceSet.Name + "-" + nodeName
	owner := metav1.NewControllerRef(serviceInterfaceSet, sfcv1.GroupVersion.WithKind("ServiceInterfaceSet"))

	sc := &sfcv1.ServiceInterface{
		ObjectMeta: metav1.ObjectMeta{
			Name:            scName,
			Namespace:       serviceInterfaceSet.Namespace,
			Labels:          labels,
			Annotations:     serviceInterfaceSet.Annotations,
			OwnerReferences: []metav1.OwnerReference{*owner},
		},
		Spec: sfcv1.ServiceInterfaceSpec{
			Node:          nodeName,
			InterfaceType: serviceInterfaceSet.Spec.TemplateSpec.InterfaceType,
			InterfaceName: serviceInterfaceSet.Spec.TemplateSpec.InterfaceName,
			BridgeName:    serviceInterfaceSet.Spec.TemplateSpec.BridgeName,
		},
	}
	if serviceInterfaceSet.Spec.TemplateSpec.Vlan != nil {
		sc.Spec.Vlan = &sfcv1.VLAN{
			VlanID:             serviceInterfaceSet.Spec.TemplateSpec.Vlan.VlanID,
			ParentInterfaceRef: serviceInterfaceSet.Spec.TemplateSpec.Vlan.ParentInterfaceRef + "-" + nodeName,
		}
	}
	if serviceInterfaceSet.Spec.TemplateSpec.VF != nil {
		sc.Spec.VF = &sfcv1.VF{
			VFID:               serviceInterfaceSet.Spec.TemplateSpec.VF.VFID,
			PFID:               serviceInterfaceSet.Spec.TemplateSpec.VF.PFID,
			ParentInterfaceRef: serviceInterfaceSet.Spec.TemplateSpec.VF.ParentInterfaceRef + "-" + nodeName,
		}
	}
	if serviceInterfaceSet.Spec.TemplateSpec.PF != nil {
		sc.Spec.PF = &sfcv1.PF{
			PFID: serviceInterfaceSet.Spec.TemplateSpec.PF.PFID,
		}
	}
	sc.ObjectMeta.ManagedFields = nil
	sc.SetGroupVersionKind(sfcv1.GroupVersion.WithKind("ServiceInterface"))
	if err := r.Client.Patch(ctx, sc, client.Apply, client.ForceOwnership, client.FieldOwner(serviceInterfaceSetControllerName)); err != nil {
		return err
	}
	log.Info("ServiceInterface is updated/created", "ServiceInterface", sc)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceInterfaceSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfcv1.ServiceInterfaceSet{}).
		Owns(&sfcv1.ServiceInterface{}).
		Watches(&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.nodeToServiceInterfaceSetReq),
			builder.WithPredicates(predicate.LabelChangedPredicate{})).
		Complete(r)
}

func (r *ServiceInterfaceSetReconciler) nodeToServiceInterfaceSetReq(ctx context.Context, resource client.Object) []reconcile.Request {
	requests := make([]reconcile.Request, 0)
	serviceInterfaceSetList := &sfcv1.ServiceInterfaceSetList{}
	if err := r.List(ctx, serviceInterfaceSetList); err == nil {
		for _, item := range serviceInterfaceSetList.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				}})
		}
	}
	return requests
}
