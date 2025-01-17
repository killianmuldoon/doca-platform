/*
Copyright 2025 NVIDIA

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

	dpuservicev1 "github.com/nvidia/doca-platform/api/dpuservice/v1alpha1"
	"github.com/nvidia/doca-platform/internal/conditions"

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
	dpuServiceTemplateControllerName = "dpuservicetemplatecontroller"
)

// +kubebuilder:rbac:groups=svc.dpu.nvidia.com,resources=dpuservicetemplates,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=svc.dpu.nvidia.com,resources=dpuservicetemplates/finalizers,verbs=update
// +kubebuilder:rbac:groups=svc.dpu.nvidia.com,resources=dpuservicetemplates/status,verbs=get;update;patch

// DPUServiceTemplateReconciler reconciles a DPUServiceTemplate object
type DPUServiceTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// pauseDPUServiceTemplateReconciler pauses the DPUServiceTemplate Reconciler by doing noop reconciliation loops. This
// is helpful to make tests faster and less complex
var pauseDPUServiceTemplateReconciler bool

// SetupWithManager sets up the controller with the Manager.
func (r *DPUServiceTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpuservicev1.DPUServiceTemplate{}).
		Complete(r)
}

// Reconcile reconciles changes in a DPUServiceTemplate object
func (r *DPUServiceTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	if pauseDPUServiceTemplateReconciler {
		log.Info("noop reconciliation")
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling")
	dpuServiceTemplate := &dpuservicev1.DPUServiceTemplate{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpuServiceTemplate); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	patcher := patch.NewSerialPatcher(dpuServiceTemplate, r.Client)

	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {
		log.Info("Patching")
		conditions.SetSummary(dpuServiceTemplate)

		if err := patcher.Patch(ctx, dpuServiceTemplate,
			patch.WithFieldOwner(dpuServiceTemplateControllerName),
			patch.WithStatusObservedGeneration{},
			patch.WithOwnedConditions{Conditions: conditions.TypesAsStrings(dpuservicev1.DPUServiceTemplateConditions)},
		); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	conditions.EnsureConditions(dpuServiceTemplate, dpuservicev1.DPUServiceTemplateConditions)

	// Handle deletion reconciliation loop.
	if !dpuServiceTemplate.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, dpuServiceTemplate)
	}

	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(dpuServiceTemplate, dpuservicev1.DPUServiceTemplateFinalizer) {
		log.Info("Adding finalizer")
		controllerutil.AddFinalizer(dpuServiceTemplate, dpuservicev1.DPUServiceTemplateFinalizer)
		return ctrl.Result{}, nil
	}

	return r.reconcile(ctx, dpuServiceTemplate)
}

// reconcile handles the main reconciliation loop
//
//nolint:unparam
func (r *DPUServiceTemplateReconciler) reconcile(ctx context.Context, dpuServiceTemplate *dpuservicev1.DPUServiceTemplate) (ctrl.Result, error) {
	conditions.AddTrue(dpuServiceTemplate, dpuservicev1.ConditionDPUServiceTemplateReconciled)
	return ctrl.Result{}, nil
}

// reconcile handles the main reconciliation loop
//
//nolint:unparam
func (r *DPUServiceTemplateReconciler) reconcileDelete(ctx context.Context, dpuServiceTemplate *dpuservicev1.DPUServiceTemplate) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling delete")

	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpuServiceTemplate, dpuservicev1.DPUServiceTemplateFinalizer)
	return ctrl.Result{}, nil
}
