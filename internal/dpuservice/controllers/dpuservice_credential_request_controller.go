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
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"

	"github.com/fluxcd/pkg/runtime/patch"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	DPUServiceCredentialRequestControllerName = "dpuservicecredentialrequestcontroller"
)

// DPUServiceCredentialRequestReconciler reconciles a DPUServiceCredentialRequest object
type DPUServiceCredentialRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservicecredentialrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservicecredentialrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservicecredentialrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=serviceaccounts;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts/token,verbs=create
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// SetupWithManager sets up the controller with the Manager.
func (r *DPUServiceCredentialRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpuservicev1.DPUServiceCredentialRequest{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}),
		)).
		Complete(r)
}

func (r *DPUServiceCredentialRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")
	obj := &dpuservicev1.DPUServiceCredentialRequest{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	patcher := patch.NewSerialPatcher(obj, r.Client)

	defer func() {
		log.Info("Patching")
		// Set the summary condition for the DPUServiceCredentialRequest.
		conditions.SetSummary(obj)
		if err := patcher.Patch(ctx, obj,
			patch.WithFieldOwner(DPUServiceCredentialRequestControllerName),
			patch.WithStatusObservedGeneration{},
			patch.WithOwnedConditions{Conditions: conditions.TypesAsStrings(dpuservicev1.DPUCredentialRequestConditions)},
		); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	return ctrl.Result{}, nil
}
