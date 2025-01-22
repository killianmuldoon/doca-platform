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

package dpu

import (
	"context"
	"fmt"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/test/mock/dms/pkg/server"

	"github.com/fluxcd/pkg/runtime/patch"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// DMSServerReconciler reconciles a DPU object
type DMSServerReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	Server server.DMSServerMux
}

// +kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpus,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpus/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpus/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods;pods/exec;nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=patch;update;delete;create

const DMSAddressAnnotationKey = "dpu.nvidia.com/dms-address-override"

func (r *DMSServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")
	dpu := &provisioningv1.DPU{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpu); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	patcher := patch.NewSerialPatcher(dpu, r.Client)

	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {
		log.Info("Patching")
		if err := patcher.Patch(ctx, dpu,
			patch.WithFieldOwner("mock-dms-controller"),
		); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Add finalizer if not set and DPU is not currently deleting.
	if !controllerutil.ContainsFinalizer(dpu, provisioningv1.DPUFinalizer) && dpu.DeletionTimestamp.IsZero() {
		controllerutil.AddFinalizer(dpu, provisioningv1.DPUFinalizer)
		if err := r.Client.Update(ctx, dpu); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add DPU finalizer %w", err)
		}
		return ctrl.Result{}, nil
	}

	// If the address annotation isn't set create a server, add the annotation and patch the DPU.
	_, ok := dpu.Annotations[DMSAddressAnnotationKey]
	if !ok {
		err := r.AddDMSListenerForDPU(dpu)
		return ctrl.Result{}, err
	}

	// If we have an error we have to requeue the DPU and let controller-runtime handle the error.
	return ctrl.Result{}, nil
}

// AddDMSListenerForDPU adds a new listener.
// Find free  port.
// Create new listener in the mux.
// Add the annotation.
func (r *DMSServerReconciler) AddDMSListenerForDPU(dpu *provisioningv1.DPU) error {
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DMSServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&provisioningv1.DPU{}).
		Complete(r)
}
