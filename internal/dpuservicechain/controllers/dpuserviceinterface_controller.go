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

//nolint:dupl
package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane/metadata"
	ssa "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/serversideapply"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

//nolint:dupl
var _ objectsInDPUClustersReconciler = &DPUServiceInterfaceReconciler{}

// DPUServiceInterfaceReconciler reconciles a DPUServiceInterface object
type DPUServiceInterfaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	dpuServiceInterfaceControllerName = "dpuserviceinterfacecontroller"
)

// +kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuserviceinterfaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuserviceinterfaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuserviceinterfaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=get;list;watch

// Reconcile reconciles changes in a DPUServiceInterface.
//
//nolint:dupl
func (r *DPUServiceInterfaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")

	dpuServiceInterface := &sfcv1.DPUServiceInterface{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpuServiceInterface); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {
		log.Info("Calling defer")

		if err := updateSummary(ctx, r, r.Client, sfcv1.ConditionServiceInterfaceSetReady, dpuServiceInterface); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
		if err := ssa.Patch(ctx, r.Client, dpuServiceInterfaceControllerName, dpuServiceInterface); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	conditions.EnsureConditions(dpuServiceInterface, sfcv1.DPUServiceInterfaceConditions)

	// Handle deletion reconciliation loop.
	if !dpuServiceInterface.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, dpuServiceInterface)
	}

	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(dpuServiceInterface, sfcv1.DPUServiceInterfaceFinalizer) {
		log.Info("Adding finalizer")
		controllerutil.AddFinalizer(dpuServiceInterface, sfcv1.DPUServiceInterfaceFinalizer)
		return ctrl.Result{}, nil
	}

	return r.reconcile(ctx, dpuServiceInterface)
}

// reconcile handles the main reconciliation loop
//
//nolint:unparam
func (r *DPUServiceInterfaceReconciler) reconcile(ctx context.Context, dpuServiceInterface *sfcv1.DPUServiceInterface) (ctrl.Result, error) {
	if err := reconcileObjectsInDPUClusters(ctx, r, r.Client, dpuServiceInterface); err != nil {
		conditions.AddFalse(
			dpuServiceInterface,
			sfcv1.ConditionServiceInterfaceSetReconciled,
			conditions.ReasonError,
			conditions.ConditionMessage(fmt.Sprintf("Error occurred: %s", err.Error())),
		)
		return ctrl.Result{}, err
	}
	conditions.AddTrue(
		dpuServiceInterface,
		sfcv1.ConditionServiceInterfaceSetReconciled,
	)

	return ctrl.Result{}, nil
}

func (r *DPUServiceInterfaceReconciler) getObjectsInDPUCluster(ctx context.Context, k8sClient client.Client, dpuObject client.Object) ([]unstructured.Unstructured, error) {
	sis := &unstructured.Unstructured{}
	sis.SetGroupVersionKind(sfcv1.ServiceInterfaceSetGroupVersionKind)
	key := client.ObjectKey{Namespace: dpuObject.GetNamespace(), Name: dpuObject.GetName()}
	err := k8sClient.Get(ctx, key, sis)
	if err != nil {
		return nil, fmt.Errorf("error while getting %s %s: %w", sis.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}

	return []unstructured.Unstructured{*sis}, nil
}

func (r *DPUServiceInterfaceReconciler) createOrUpdateObjectsInDPUCluster(ctx context.Context, k8sClient client.Client, dpuObject client.Object) error {
	dpuServiceInterface := dpuObject.(*sfcv1.DPUServiceInterface)
	sis := &sfcv1.ServiceInterfaceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dpuServiceInterface.Name,
			Namespace:   dpuServiceInterface.Namespace,
			Labels:      dpuServiceInterface.Spec.Template.Labels,
			Annotations: dpuServiceInterface.Spec.Template.Annotations,
		},
		Spec: *dpuServiceInterface.Spec.Template.Spec.DeepCopy(),
	}
	sis.ObjectMeta.ManagedFields = nil
	sis.SetGroupVersionKind(sfcv1.GroupVersion.WithKind("ServiceInterfaceSet"))
	return k8sClient.Patch(ctx, sis, client.Apply, client.ForceOwnership, client.FieldOwner(dpuServiceInterfaceControllerName))
}

func (r *DPUServiceInterfaceReconciler) reconcileDelete(ctx context.Context, dpuServiceInterface *sfcv1.DPUServiceInterface) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling delete")
	if err := reconcileObjectDeletionInDPUClusters(ctx, r, r.Client, dpuServiceInterface); err != nil {
		e := &shouldRequeueError{}
		if errors.As(err, &e) {
			log.Info(fmt.Sprintf("Requeueing because %s", err.Error()))
			conditions.AddFalse(
				dpuServiceInterface,
				sfcv1.ConditionServiceInterfaceSetReconciled,
				conditions.ReasonAwaitingDeletion,
				conditions.ConditionMessage(err.Error()),
			)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpuServiceInterface, sfcv1.DPUServiceInterfaceFinalizer)
	return ctrl.Result{}, nil
}

func (r *DPUServiceInterfaceReconciler) deleteObjectsInDPUCluster(ctx context.Context, k8sClient client.Client, dpuObject client.Object) error {
	dpuServiceInterface := dpuObject.(*sfcv1.DPUServiceInterface)
	sis := &sfcv1.ServiceInterfaceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dpuServiceInterface.Name,
			Namespace: dpuServiceInterface.Namespace,
		},
	}
	return k8sClient.Delete(ctx, sis)
}

// getUnreadyObjects is the method called by reconcileReadinessOfObjectsInDPUClusters function which returns whether
// objects in the DPU cluster are ready. The input to the function is a list of objects that exist in a particular
// cluster.
func (r *DPUServiceInterfaceReconciler) getUnreadyObjects(objects []unstructured.Unstructured) ([]types.NamespacedName, error) {
	unreadyObjs := []types.NamespacedName{}
	for _, o := range objects {
		// TODO: Convert to ServiceInterfaceSet when we implement status for this controller
		conditions, exists, err := unstructured.NestedSlice(o.Object, "status", "conditions")
		if err != nil {
			return nil, err
		}
		// TODO: Check on condition ready when we implement status for this controller
		if len(conditions) == 0 || !exists {
			unreadyObjs = append(unreadyObjs, types.NamespacedName{Name: o.GetName(), Namespace: o.GetNamespace()})
			continue
		}
	}
	return unreadyObjs, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DPUServiceInterfaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	tenantControlPlane := &metav1.PartialObjectMetadata{}
	tenantControlPlane.SetGroupVersionKind(controlplanemeta.TenantControlPlaneGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfcv1.DPUServiceInterface{}).
		// TODO: This doesn't currently work for status updates - need to find a way to increase reconciliation frequency.
		WatchesMetadata(tenantControlPlane, handler.EnqueueRequestsFromMapFunc(r.DPUClusterToDPUServiceInterface)).
		Complete(r)
}

// DPUClusterToDPUServiceInterface ensures all DPUServiceInterfaces are updated each time there is an update to a DPUCluster.
func (r *DPUServiceInterfaceReconciler) DPUClusterToDPUServiceInterface(ctx context.Context, o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	dpuServiceList := &sfcv1.DPUServiceInterfaceList{}
	if err := r.Client.List(ctx, dpuServiceList); err != nil {
		return nil
	}
	for _, m := range dpuServiceList.Items {
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}
	return result
}
