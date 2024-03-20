/*
Copyright 2024.

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

	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// DPFOperatorConfigReconciler reconciles a DPFOperatorConfig object
type DPFOperatorConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs/finalizers,verbs=update

const (
	dpfOperatorConfigControllerName = "dpfoperatorconfig-controller"
)

// Reconcile reconciles changes in a DPFOperatorConfig.
func (r *DPFOperatorConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")
	dpfOperatorConfig := &operatorv1.DPFOperatorConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpfOperatorConfig); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	//original := dpfOperatorConfig.DeepCopy()
	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {
		log.Info("Calling defer")
		// TODO: Make this a generic patcher.
		// TODO: There is an issue patching status here with SSA - the finalizer managed field becomes broken and the finalizer can not be removed. Investigate.
		// Set the GVK explicitly for the patch.
		dpfOperatorConfig.SetGroupVersionKind(operatorv1.DPFOperatorConfigGroupVersionKind)
		// Do not include manged fields in the patch call. This does not remove existing fields.
		dpfOperatorConfig.ObjectMeta.ManagedFields = nil
		err := r.Client.Patch(ctx, dpfOperatorConfig, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName))
		reterr = kerrors.NewAggregate([]error{reterr, err})
	}()

	// Handle deletion reconciliation loop.
	if !dpfOperatorConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("Removing")
		return r.reconcileDelete(ctx, dpfOperatorConfig)
	}

	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(dpfOperatorConfig, operatorv1.DPFOperatorConfigFinalizer) {
		log.Info("Adding finalizer")
		controllerutil.AddFinalizer(dpfOperatorConfig, operatorv1.DPFOperatorConfigFinalizer)
		return ctrl.Result{}, nil
	}
	return r.reconcile(ctx, dpfOperatorConfig)
}

//nolint:unparam
func (r *DPFOperatorConfigReconciler) reconcileDelete(ctx context.Context, dpfOperatorConfig *operatorv1.DPFOperatorConfig) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpfOperatorConfig, operatorv1.DPFOperatorConfigFinalizer)
	// We should have an ownerReference chain in order to delete subordinate objects.
	return ctrl.Result{}, nil
}

//nolint:unparam
func (r *DPFOperatorConfigReconciler) reconcile(ctx context.Context, dpfOperatorConfig *operatorv1.DPFOperatorConfig) (ctrl.Result, error) {
	// Ensure Custom OVN Kubernetes Deployment is done
	if err := r.reconcileCustomOVNKubernetesDeployment(ctx); err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileCustomOVNKubernetesDeployment ensures that custom OVN Kubernetes is deployed
func (r *DPFOperatorConfigReconciler) reconcileCustomOVNKubernetesDeployment(ctx context.Context) error {
	// Phase 1
	// - ensure cluster version operator is scaled down
	// - ensure network operator is scaled down
	// - ensure webhook is removed
	// - ensure OVN Kubernetes daemonset has different nodeSelector (i.e. point to control plane only)
	// Phase 2
	// - ensure no original OVN Kubernetes pods runs on worker
	// - remove node annotation k8s.ovn.org/node-chassis-id (avoid removing again on next reconciliation loop, needs status)
	// - ensure DPU CNI Provisioner is deployed
	// - ensure Host CNI provisioner is deployed
	// - ensure both provisioners are ready and have more than 1 pods
	// Phase 3
	// - deploy custom OVN Kubernetes
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DPFOperatorConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.DPFOperatorConfig{}).
		Complete(r)
}
