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
	"fmt"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"
	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"

	"github.com/fluxcd/pkg/runtime/patch"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	dpuDeploymentControllerName = "dpudeploymentcontroller"

	// ParentDPUDeploymentNameLabel contains the name of the DPUDeployment object that owns the resource
	ParentDPUDeploymentNameLabel = "dpf.nvidia.com/dpudeployment-name"
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

// dpuDeploymentDependencies is a struct that holds the parsed dependencies a DPUDeployment has.
type dpuDeploymentDependencies struct {
	BFB                      *provisioningv1.Bfb
	DPUFlavor                *provisioningv1.DPUFlavor
	DPUServiceConfigurations map[string]*dpuservicev1.DPUServiceConfiguration
	DPUServiceTemplates      map[string]*dpuservicev1.DPUServiceTemplate
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
// TODO: Remove nolint if we ever return different result
//
//nolint:unparam
func (r *DPUDeploymentReconciler) reconcile(ctx context.Context, dpuDeployment *dpuservicev1.DPUDeployment) (ctrl.Result, error) {
	_, err := getDependencies(ctx, r.Client, dpuDeployment)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error while getting the DPUDeployment dependencies: %w", err)
	}

	if err := reconcileDPUSets(ctx, r.Client, dpuDeployment); err != nil {
		return ctrl.Result{}, fmt.Errorf("error while reconciling the DPUSets: %w", err)
	}

	if err := reconcileDPUServices(ctx, r.Client, dpuDeployment); err != nil {
		return ctrl.Result{}, fmt.Errorf("error while reconciling the DPUServices: %w", err)
	}

	if err := reconcileDPUServiceChains(ctx, r.Client, dpuDeployment); err != nil {
		return ctrl.Result{}, fmt.Errorf("error while reconciling the DPUServiceChains: %w", err)
	}

	return ctrl.Result{}, nil
}

// getDependencies gets the DPUDeployment dependencies from the Kubernetes API Server.
func getDependencies(ctx context.Context, c client.Client, dpuDeployment *dpuservicev1.DPUDeployment) (*dpuDeploymentDependencies, error) {
	deps := &dpuDeploymentDependencies{
		DPUServiceConfigurations: make(map[string]*dpuservicev1.DPUServiceConfiguration),
		DPUServiceTemplates:      make(map[string]*dpuservicev1.DPUServiceTemplate),
	}

	bfb := &provisioningv1.Bfb{}
	key := client.ObjectKey{Namespace: dpuDeployment.Namespace, Name: dpuDeployment.Spec.DPUs.BFB}
	if err := c.Get(ctx, key, bfb); err != nil {
		return deps, fmt.Errorf("error while getting %s %s: %w", provisioningv1.BfbGroupVersionKind.String(), key.String(), err)
	}
	deps.BFB = bfb

	dpuFlavor := &provisioningv1.DPUFlavor{}
	key = client.ObjectKey{Namespace: dpuDeployment.Namespace, Name: dpuDeployment.Spec.DPUs.Flavor}
	if err := c.Get(ctx, key, dpuFlavor); err != nil {
		return deps, fmt.Errorf("error while getting %s %s: %w", provisioningv1.DPUFlavorGroupVersionKind.String(), key.String(), err)
	}
	deps.DPUFlavor = dpuFlavor

	for service, config := range dpuDeployment.Spec.Services {
		serviceTemplate := &dpuservicev1.DPUServiceTemplate{}
		key := client.ObjectKey{Namespace: dpuDeployment.Namespace, Name: config.ServiceTemplate}
		if err := c.Get(ctx, key, serviceTemplate); err != nil {
			return deps, fmt.Errorf("error while getting %s %s: %w", dpuservicev1.DPUServiceTemplateGroupVersionKind.String(), key.String(), err)
		}
		deps.DPUServiceTemplates[service] = serviceTemplate

		serviceConfiguration := &dpuservicev1.DPUServiceConfiguration{}
		key = client.ObjectKey{Namespace: dpuDeployment.Namespace, Name: config.ServiceConfiguration}
		if err := c.Get(ctx, key, serviceConfiguration); err != nil {
			return deps, fmt.Errorf("error while getting %s %s: %w", dpuservicev1.DPUServiceConfigurationGroupVersionKind.String(), key.String(), err)
		}
		deps.DPUServiceConfigurations[service] = serviceConfiguration
	}

	return deps, nil
}

// reconcileDPUSets reconciles the DPUSets created by the DPUDeployment
func reconcileDPUSets(ctx context.Context, c client.Client, dpuDeployment *dpuservicev1.DPUDeployment) error {
	owner := metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)
	// TODO: Populate accordingly
	dpuSet := generateDPUSet(client.ObjectKeyFromObject(dpuDeployment), owner, &dpuservicev1.DPUSet{})
	patcher := patch.NewSerialPatcher(dpuSet, c)
	if err := patcher.Patch(ctx, dpuSet, patch.WithFieldOwner(dpuDeploymentControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", dpuSet.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(dpuSet), err)
	}

	return nil
}

// reconcileDPUServices reconciles the DPUServices created by the DPUDeployment
func reconcileDPUServices(ctx context.Context, c client.Client, dpuDeployment *dpuservicev1.DPUDeployment) error {
	return nil
}

// reconcileDPUServices reconciles the DPUServices created by the DPUDeployment
func reconcileDPUServiceChains(ctx context.Context, c client.Client, dpuDeployment *dpuservicev1.DPUDeployment) error {
	return nil
}

// generateDPUSet generates a DPUSet according to the
// TODO: Populate accordingly and remove nolint
//
//nolint:unparam
func generateDPUSet(dpuDeploymentNamespacedName types.NamespacedName, owner *metav1.OwnerReference, dpuSetSettings *dpuservicev1.DPUSet) *provisioningv1.DpuSet {
	dpuSet := &provisioningv1.DpuSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dpuDeploymentNamespacedName.Name,
			Namespace: dpuDeploymentNamespacedName.Namespace,
			Labels: map[string]string{
				ParentDPUDeploymentNameLabel: dpuDeploymentNamespacedName.Name,
			},
		},
	}
	dpuSet.SetOwnerReferences([]metav1.OwnerReference{*owner})
	dpuSet.ObjectMeta.ManagedFields = nil
	dpuSet.SetGroupVersionKind(provisioningv1.DpuSetGroupVersionKind)

	return dpuSet
}

// reconcileDelete handles the deletion reconciliation loop
// TODO: Remove nolint if we ever return different result
//
//nolint:unparam
func (r *DPUDeploymentReconciler) reconcileDelete(ctx context.Context, dpuDeployment *dpuservicev1.DPUDeployment) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling delete")

	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpuDeployment, dpuservicev1.DPUDeploymentFinalizer)
	return ctrl.Result{}, nil
}
