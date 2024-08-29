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

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"

	"github.com/fluxcd/pkg/runtime/patch"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	dpuDeploymentControllerName = "dpudeploymentcontroller"
)

//+kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpudeployments,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpudeployments/finalizers,verbs=update

// DPUDeploymentReconciler reconciles a DPUDeployment object
type DPUDeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *DPUDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpuservicev1.DPUDeployment{}).
		Complete(r)
}

// Reconcile reconciles changes in a DPUDeployment object
func (r *DPUDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")

	dpuDeployment := &dpuservicev1.DPUDeployment{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpuDeployment); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	patcher := patch.NewSerialPatcher(dpuDeployment, r.Client)

	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {
		log.Info("Patching")
		if err := patcher.Patch(ctx, dpuDeployment,
			patch.WithFieldOwner(dpuDeploymentControllerName),
		); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deletion reconciliation loop.
	if !dpuDeployment.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, dpuDeployment)
	}

	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(dpuDeployment, dpuservicev1.DPUDeploymentFinalizer) {
		log.Info("Adding finalizer")
		controllerutil.AddFinalizer(dpuDeployment, dpuservicev1.DPUDeploymentFinalizer)
		return ctrl.Result{}, nil
	}

	return r.reconcile(ctx, dpuDeployment)
}

// reconcile handles the main reconciliation loop
func (r *DPUDeploymentReconciler) reconcile(ctx context.Context, dpuDeployment *dpuservicev1.DPUDeployment) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// reconcileDelete handles the deletion reconciliation loop
// TODO: Remove nolint if we even return different result
//
//nolint:unparam
func (r *DPUDeploymentReconciler) reconcileDelete(ctx context.Context, dpuDeployment *dpuservicev1.DPUDeployment) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling delete")

	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpuDeployment, dpuservicev1.DPUDeploymentFinalizer)
	return ctrl.Result{}, nil
}
