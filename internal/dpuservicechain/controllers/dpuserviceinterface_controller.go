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

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane/metadata"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

//nolint:dupl
var _ dpuDuplicateReconciler = &DPUServiceInterfaceReconciler{}

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
	// Handle deletion reconciliation loop.
	if !dpuServiceInterface.ObjectMeta.DeletionTimestamp.IsZero() {
		sis := &sfcv1.ServiceInterfaceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dpuServiceInterface.Name,
				Namespace: dpuServiceInterface.Namespace,
			},
		}
		err := reconcileDelete(ctx, r.Client, sis, dpuServiceInterface, sfcv1.DPUServiceInterfaceFinalizer)
		return ctrl.Result{}, err
	}
	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(dpuServiceInterface, sfcv1.DPUServiceInterfaceFinalizer) {
		controllerutil.AddFinalizer(dpuServiceInterface, sfcv1.DPUServiceInterfaceFinalizer)
		if err := r.Update(ctx, dpuServiceInterface); err != nil {
			return ctrl.Result{}, err
		}
	}
	return reconcile(ctx, r.Client, dpuServiceInterface, r)
}

func (r *DPUServiceInterfaceReconciler) createOrUpdateChild(ctx context.Context, k8sClient client.Client, dpuObject client.Object) error {
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
