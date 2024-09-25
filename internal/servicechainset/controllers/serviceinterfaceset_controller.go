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

	"github.com/fluxcd/pkg/runtime/patch"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

var _ serviceSetReconciler = &ServiceInterfaceSetReconciler{}

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
//
//nolint:dupl
func (r *ServiceInterfaceSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
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

	patcher := patch.NewSerialPatcher(sis, r.Client)

	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {

		log.Info("Patching")
		if err := patcher.Patch(ctx, sis,
			patch.WithFieldOwner(serviceInterfaceSetControllerName),
		); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	if !sis.ObjectMeta.DeletionTimestamp.IsZero() {
		return reconcileDelete(ctx, sis, r.Client, r, sfcv1.ServiceInterfaceSetFinalizer)
	}

	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(sis, sfcv1.ServiceInterfaceSetFinalizer) {
		log.Info("Adding finalizer")
		controllerutil.AddFinalizer(sis, sfcv1.ServiceInterfaceSetFinalizer)
		return ctrl.Result{}, nil
	}

	return reconcileSet(ctx, sis, r.Client, sis.Spec.NodeSelector, r)
}

func (r *ServiceInterfaceSetReconciler) getChildMap(ctx context.Context, set client.Object) (map[string]client.Object, error) {
	serviceInterfaceMap := make(map[string]client.Object)
	serviceInterfaceList := &sfcv1.ServiceInterfaceList{}
	if err := r.List(ctx, serviceInterfaceList, client.MatchingLabels{
		ServiceInterfaceSetNameLabel:      set.GetName(),
		ServiceInterfaceSetNamespaceLabel: set.GetNamespace(),
	}); err != nil {
		return serviceInterfaceMap, err
	}
	for i := range serviceInterfaceList.Items {
		si := serviceInterfaceList.Items[i]
		serviceInterfaceMap[*si.Spec.Node] = &si
	}
	return serviceInterfaceMap, nil
}

func (r *ServiceInterfaceSetReconciler) createOrUpdateChild(ctx context.Context, set client.Object, nodeName string) error {
	log := log.FromContext(ctx)
	serviceInterfaceSet := set.(*sfcv1.ServiceInterfaceSet)
	labels := map[string]string{ServiceInterfaceSetNameLabel: serviceInterfaceSet.Name,
		ServiceInterfaceSetNamespaceLabel: serviceInterfaceSet.Namespace}
	maps.Copy(labels, serviceInterfaceSet.Spec.Template.ObjectMeta.Labels)
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
			Node:          ptr.To(nodeName),
			InterfaceType: serviceInterfaceSet.Spec.Template.Spec.InterfaceType,
			InterfaceName: serviceInterfaceSet.Spec.Template.Spec.InterfaceName,
		},
	}
	if serviceInterfaceSet.Spec.Template.Spec.Vlan != nil {
		sc.Spec.Vlan = &sfcv1.VLAN{
			VlanID:             serviceInterfaceSet.Spec.Template.Spec.Vlan.VlanID,
			ParentInterfaceRef: serviceInterfaceSet.Spec.Template.Spec.Vlan.ParentInterfaceRef + "-" + nodeName,
		}
	}
	if serviceInterfaceSet.Spec.Template.Spec.VF != nil {
		sc.Spec.VF = &sfcv1.VF{
			VFID:               serviceInterfaceSet.Spec.Template.Spec.VF.VFID,
			PFID:               serviceInterfaceSet.Spec.Template.Spec.VF.PFID,
			ParentInterfaceRef: serviceInterfaceSet.Spec.Template.Spec.VF.ParentInterfaceRef + "-" + nodeName,
		}
	}
	if serviceInterfaceSet.Spec.Template.Spec.PF != nil {
		sc.Spec.PF = &sfcv1.PF{
			ID: serviceInterfaceSet.Spec.Template.Spec.PF.ID,
		}
	}
	if serviceInterfaceSet.Spec.Template.Spec.Service != nil {
		sc.Spec.Service = &sfcv1.ServiceDef{
			ServiceID:   serviceInterfaceSet.Spec.Template.Spec.Service.ServiceID,
			NetworkName: serviceInterfaceSet.Spec.Template.Spec.Service.NetworkName,
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

// TODO not implemented
//
//nolint:unused
func (r *ServiceInterfaceSetReconciler) getObjectsInDPUCluster(ctx context.Context, k8sClient client.Client, dpuObject client.Object) ([]unstructured.Unstructured, error) {
	return nil, nil
}

// TODO not implemented
//
//nolint:unused
func (r *ServiceInterfaceSetReconciler) getUnreadyObjects(objects []unstructured.Unstructured) ([]types.NamespacedName, error) {
	return nil, nil
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
