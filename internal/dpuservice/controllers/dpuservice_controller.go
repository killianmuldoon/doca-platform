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

package controllers

import (
	"context"
	"fmt"

	controlplanev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/controlplane/v1alpha1"
	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DPUServiceReconciler reconciles a DPUService object
type DPUServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservices/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims;events;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete

const dpuServiceControllerName = "dpuservice-manager"

// SetupWithManager sets up the controller with the Manager.
func (r *DPUServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpuservicev1.DPUService{}).
		Complete(r)
}

// Reconcile reconciles changes in a DPUService.
func (r *DPUServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	_ = log.FromContext(ctx)

	dpuService := &dpuservicev1.DPUService{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpuService); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {
		// TODO: Make this a generic patcher for all reconcilers.
		// Set the GVK explicitly for the patch.
		dpuService.SetGroupVersionKind(dpuservicev1.DPUServiceGroupVersionKind)
		// Do not include manged fields in the patch call. This does not remove existing fields.
		dpuService.ObjectMeta.ManagedFields = nil
		err := r.Client.Patch(ctx, dpuService, client.Apply, client.ForceOwnership, client.FieldOwner(dpuServiceControllerName))
		reterr = kerrors.NewAggregate([]error{reterr, err})
	}()

	// Handle deletion reconciliation loop.
	if !dpuService.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, dpuService)
	}

	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(dpuService, dpuservicev1.DPUServiceFinalizer) {
		controllerutil.AddFinalizer(dpuService, dpuservicev1.DPUServiceFinalizer)
		return ctrl.Result{}, nil
	}
	return r.reconcile(ctx, dpuService)

}
func (r *DPUServiceReconciler) reconcileDelete(ctx context.Context, service *dpuservicev1.DPUService) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

//nolint:unparam //TODO: remove once function is implemented.
func (r *DPUServiceReconciler) reconcile(ctx context.Context, dpuService *dpuservicev1.DPUService) (ctrl.Result, error) {
	// Get the list of clusters this DPUService targets.
	clusters, err := getClusters(ctx, r.Client)
	if err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return ctrl.Result{}, err
	}

	// Ensure the Argo secret for each cluster is up-to-date.
	if err := r.reconcileSecrets(ctx, clusters); err != nil {
		return ctrl.Result{}, err
	}

	//  Ensure the ArgoCD project exists and is up-to-date.
	if err := r.reconcileArgoCDAppProject(ctx); err != nil {
		return ctrl.Result{}, err
	}

	// Update the ArgoApplication for all target clusters.
	if err := r.reconcileArgoApplication(ctx, clusters, dpuService); err != nil {
		return ctrl.Result{}, err
	}

	// Update the status of the DPUService.
	if err := r.reconcileStatus(ctx); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *DPUServiceReconciler) reconcileSecrets(ctx context.Context, clusters []types.NamespacedName) error {
	return nil
}
func (r *DPUServiceReconciler) reconcileArgoCDAppProject(ctx context.Context) error {
	return nil
}

func (r *DPUServiceReconciler) reconcileArgoApplication(
	ctx context.Context, clusters []types.NamespacedName, dpuService *dpuservicev1.DPUService) error {
	return nil

}

func (r *DPUServiceReconciler) reconcileStatus(ctx context.Context) error {
	return nil
}

func getClusters(ctx context.Context, c client.Client) ([]types.NamespacedName, error) {
	var errs []error
	secrets := &corev1.SecretList{}
	err := c.List(ctx, secrets, client.MatchingLabels(controlplanev1.DPFClusterSecretLabels))
	if err != nil {
		return nil, err
	}
	clusters := []types.NamespacedName{}
	for _, secret := range secrets.Items {
		clusterName, found := secret.GetLabels()[controlplanev1.DPFClusterSecretClusterNameLabelKey]
		if !found {
			errs = append(errs, fmt.Errorf("could not identify cluster name for secret %v/%v", secret.Namespace, secret.Name))
			continue
		}
		clusters = append(clusters, types.NamespacedName{
			Namespace: secret.Namespace,
			Name:      clusterName,
		})
		fmt.Printf("%v", clusters)
	}
	return clusters, kerrors.NewAggregate(errs)
}
