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
	"reflect"
	"slices"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"
	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"

	"github.com/fluxcd/pkg/runtime/patch"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	dpuDeploymentControllerName = "dpudeploymentcontroller"

	// ParentDPUDeploymentNameLabel contains the name of the DPUDeployment object that owns the resource
	ParentDPUDeploymentNameLabel = "dpf.nvidia.com/dpudeployment-name"
)

//+kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpudeployments,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpudeployments/finalizers,verbs=update
//+kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuserviceconfigurations,verbs=get;list;watch

// DPUDeploymentReconciler reconciles a DPUDeployment object
type DPUDeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *DPUDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpuservicev1.DPUDeployment{}).
		Watches(&dpuservicev1.DPUServiceConfiguration{}, handler.EnqueueRequestsFromMapFunc(r.DPUServiceConfigurationToDPUDeployment)).
		Complete(r)
}

// DPUServiceConfigurationToDPUDeployment returns the DPUDeployments associated with a DPUServiceConfiguration
func (r *DPUDeploymentReconciler) DPUServiceConfigurationToDPUDeployment(ctx context.Context, o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	dpuDeploymentList := &dpuservicev1.DPUDeploymentList{}
	if err := r.Client.List(ctx, dpuDeploymentList, client.InNamespace(o.GetNamespace())); err != nil {
		return nil
	}

	for _, dpuDeployment := range dpuDeploymentList.Items {
		for _, opt := range dpuDeployment.Spec.Services {
			if opt.ServiceConfiguration == o.GetName() {
				result = append(result, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&dpuDeployment)})
				break
			}
		}
	}

	return result
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
	deps, err := getDependencies(ctx, r.Client, dpuDeployment)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error while getting the DPUDeployment dependencies: %w", err)
	}

	if err := verifyResourceFitting(deps); err != nil {
		return ctrl.Result{}, fmt.Errorf("error while verifying that resources can fit: %w", err)
	}

	if err := reconcileDPUSets(ctx, r.Client, dpuDeployment); err != nil {
		return ctrl.Result{}, fmt.Errorf("error while reconciling the DPUSets: %w", err)
	}

	if err := reconcileDPUServices(ctx, r.Client, dpuDeployment, deps); err != nil {
		return ctrl.Result{}, fmt.Errorf("error while reconciling the DPUServices: %w", err)
	}

	if err := reconcileDPUServiceChain(ctx, r.Client, dpuDeployment); err != nil {
		return ctrl.Result{}, fmt.Errorf("error while reconciling the DPUServiceChain: %w", err)
	}

	return ctrl.Result{}, nil
}

// verifyResourceFitting verifies that the user provided resources for DPUServices can fit the resources defined in the
// DPUFlavor
func verifyResourceFitting(dependencies *dpuDeploymentDependencies) error {
	totalResources := make(corev1.ResourceList)
	for k, v := range dependencies.DPUFlavor.Spec.DPUDeploymentResources {
		totalResources[k] = v
	}

	for _, dpuServiceTemplate := range dependencies.DPUServiceTemplates {
		for resourceName, requiredQuantity := range dpuServiceTemplate.Spec.ResourceRequirements {
			totalResource := resource.Quantity{}
			if resource, ok := totalResources[resourceName]; ok {
				totalResource = resource
			}

			totalResource.Sub(requiredQuantity)
			totalResources[resourceName] = totalResource
		}
	}

	additionalResourcesRequired := []string{}
	for resourceName, quantity := range totalResources {
		if quantity.Sign() < 0 {
			quantity.Neg()
			additionalResourcesRequired = append(additionalResourcesRequired, fmt.Sprintf("%s: %s", resourceName, quantity.String()))
		}
	}

	if len(additionalResourcesRequired) > 0 {
		return fmt.Errorf("there are not enough resources for DPUServices to fit in this DPUDeployment. Additional resources needed: %v", additionalResourcesRequired)
	}

	return nil
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
// As part of this flow, we try to find existing DPUSets and update them to match the DPUDeployment spec instead of
// creating new ones. The reason behind that is so that we can minimize the mutations on DPU objects that impose infra
// changes (provisioning of BFB) that may be disruptive and take a lot of time.
func reconcileDPUSets(ctx context.Context, c client.Client, dpuDeployment *dpuservicev1.DPUDeployment) error {
	owner := metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)

	// Grab existing DPUSets
	existingDPUSets := &provisioningv1.DpuSetList{}
	if err := c.List(ctx,
		existingDPUSets,
		client.MatchingLabels{
			ParentDPUDeploymentNameLabel: dpuDeployment.Name,
		},
		client.InNamespace(dpuDeployment.Namespace)); err != nil {
		return fmt.Errorf("error while listing dpusets : %w", err)
	}

	// Ignore DPUSets that match already what is defined in the DPUDeployment
	dpuSetsToBeCreated := make([]dpuservicev1.DPUSet, len(dpuDeployment.Spec.DPUs.DPUSets))
	copy(dpuSetsToBeCreated, dpuDeployment.Spec.DPUs.DPUSets)
	for index, dpuSetOption := range dpuDeployment.Spec.DPUs.DPUSets {
		dpuSet := generateDPUSet(client.ObjectKeyFromObject(dpuDeployment),
			owner,
			&dpuSetOption,
			dpuDeployment.Spec.DPUs.BFB,
			dpuDeployment.Spec.DPUs.Flavor)

		// In case we found a matching DPUSet, remove the items from the lists
		if i := equalDPUSetIndex(dpuSet, existingDPUSets.Items); i >= 0 {
			existingDPUSets.Items = deleteElementOrNil[[]provisioningv1.DpuSet](existingDPUSets.Items, i, i+1)
			dpuSetsToBeCreated = deleteElementOrNil[[]dpuservicev1.DPUSet](dpuSetsToBeCreated, index, index+1)
		}
	}

	// Create or update DPUSets to match what is defined in the DPUDeployment
	for _, dpuSetOption := range dpuSetsToBeCreated {
		dpuSet := generateDPUSet(client.ObjectKeyFromObject(dpuDeployment),
			owner,
			&dpuSetOption,
			dpuDeployment.Spec.DPUs.BFB,
			dpuDeployment.Spec.DPUs.Flavor)

		// Use existing DPUSet instead of creating a new one if it exists.
		if len(existingDPUSets.Items) > 0 {
			existingDPUSet := existingDPUSets.Items[0]
			dpuSet.Name = existingDPUSet.Name

			existingDPUSets.Items = deleteElementOrNil[[]provisioningv1.DpuSet](existingDPUSets.Items, 0, 1)
		}

		if err := c.Patch(ctx, dpuSet, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName)); err != nil {
			return fmt.Errorf("error while patching %s %s: %w", dpuSet.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(dpuSet), err)
		}
	}

	// Cleanup the remaining stale DPUSets
	for _, dpuSet := range existingDPUSets.Items {
		if err := c.Delete(ctx, &dpuSet); err != nil {
			return fmt.Errorf("error while deleting %s %s: %w", dpuSet.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&dpuSet), err)
		}
	}

	return nil
}

// equalDPUSetIndex tries to find a DpuSet that matches the expected input from a list and returns the index
func equalDPUSetIndex(expected *provisioningv1.DpuSet, existing []provisioningv1.DpuSet) int {
	for i, existingDpuSet := range existing {
		if reflect.DeepEqual(expected.Spec, existingDpuSet.Spec) &&
			isMapSubset[string, string](existingDpuSet.Labels, expected.Labels) {
			return i
		}
	}
	return -1
}

// reconcileDPUServices reconciles the DPUServices created by the DPUDeployment
func reconcileDPUServices(ctx context.Context, c client.Client, dpuDeployment *dpuservicev1.DPUDeployment, dependencies *dpuDeploymentDependencies) error {
	owner := metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)

	// Grab existing DPUServices
	existingDPUServices := &dpuservicev1.DPUServiceList{}
	if err := c.List(ctx,
		existingDPUServices,
		client.MatchingLabels{
			ParentDPUDeploymentNameLabel: dpuDeployment.Name,
		},
		client.InNamespace(dpuDeployment.Namespace)); err != nil {
		return fmt.Errorf("error while listing dpuservices: %w", err)
	}

	existingDPUServicesMap := make(map[string]dpuservicev1.DPUService)
	for _, dpuService := range existingDPUServices.Items {
		existingDPUServicesMap[dpuService.Name] = dpuService
	}

	// Create or update DPUServices to match what is defined in the DPUDeployment
	for dpuServiceName := range dpuDeployment.Spec.Services {
		dpuService := generateDPUService(client.ObjectKeyFromObject(dpuDeployment),
			owner,
			dpuServiceName,
			dependencies.DPUServiceConfigurations[dpuServiceName],
			dependencies.DPUServiceTemplates[dpuServiceName],
		)

		if err := c.Patch(ctx, dpuService, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName)); err != nil {
			return fmt.Errorf("error while patching %s %s: %w", dpuService.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(dpuService), err)
		}

		delete(existingDPUServicesMap, dpuService.Name)
	}

	// Cleanup the remaining stale DPUServices
	for _, dpuService := range existingDPUServicesMap {
		if err := c.Delete(ctx, &dpuService); err != nil {
			return fmt.Errorf("error while deleting %s %s: %w", dpuService.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&dpuService), err)
		}
	}

	return nil
}

// reconcileDPUServiceChain reconciles the DPUServiceChain object created by the DPUDeployment
func reconcileDPUServiceChain(ctx context.Context, c client.Client, dpuDeployment *dpuservicev1.DPUDeployment) error {
	owner := metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)

	dpuServiceChain := generateDPUServiceChain(client.ObjectKeyFromObject(dpuDeployment), owner, dpuDeployment.Spec.ServiceChains)

	if err := c.Patch(ctx, dpuServiceChain, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", dpuServiceChain.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(dpuServiceChain), err)
	}

	return nil
}

// generateDPUSet generates a DPUSet according to the DPUDeployment
func generateDPUSet(dpuDeploymentNamespacedName types.NamespacedName,
	owner *metav1.OwnerReference,
	dpuSetSettings *dpuservicev1.DPUSet,
	bfb string,
	dpuFlavor string,
) *provisioningv1.DpuSet {
	dpuSet := &provisioningv1.DpuSet{
		ObjectMeta: metav1.ObjectMeta{
			// Extremely simplified version of:
			// https://github.com/kubernetes/apiserver/blob/master/pkg/storage/names/generate.go
			Name:      fmt.Sprintf("%s-%s", dpuDeploymentNamespacedName.Name, utilrand.String(5)),
			Namespace: dpuDeploymentNamespacedName.Namespace,
			Labels: map[string]string{
				ParentDPUDeploymentNameLabel: dpuDeploymentNamespacedName.Name,
			},
		},
		Spec: provisioningv1.DpuSetSpec{
			NodeSelector: dpuSetSettings.NodeSelector,
			DpuSelector:  dpuSetSettings.DPUSelector,
			Strategy: &provisioningv1.DpuSetStrategy{
				// TODO: Update to OnDelete when this is implemented
				Type: provisioningv1.RollingUpdateStrategyType,
			},
			DpuTemplate: provisioningv1.DpuTemplate{
				Annotations: dpuSetSettings.DPUAnnotations,
				Spec: provisioningv1.DPUSpec{
					Bfb: provisioningv1.BFBSpec{
						BFBName: bfb,
					},
					DPUFlavor: dpuFlavor,
				},
				// TODO: Derive and add k8s_cluster
				// TODO: Add nodeEffect
			},
		},
	}
	dpuSet.SetOwnerReferences([]metav1.OwnerReference{*owner})
	dpuSet.ObjectMeta.ManagedFields = nil
	dpuSet.SetGroupVersionKind(provisioningv1.DpuSetGroupVersionKind)

	return dpuSet
}

// generateDPUService generates a DPUService according to the DPUDeployment
func generateDPUService(dpuDeploymentNamespacedName types.NamespacedName,
	owner *metav1.OwnerReference,
	name string,
	serviceConfig *dpuservicev1.DPUServiceConfiguration,
	serviceTemplate *dpuservicev1.DPUServiceTemplate,
) *dpuservicev1.DPUService {
	dpuService := &dpuservicev1.DPUService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", dpuDeploymentNamespacedName.Name, name),
			Namespace: dpuDeploymentNamespacedName.Namespace,
			Labels: map[string]string{
				ParentDPUDeploymentNameLabel: dpuDeploymentNamespacedName.Name,
			},
		},
		Spec: dpuservicev1.DPUServiceSpec{
			HelmChart:       serviceConfig.Spec.ServiceConfiguration.HelmChart,
			ServiceID:       ptr.To[string](name),
			DeployInCluster: serviceConfig.Spec.ServiceConfiguration.DeployInCluster,
		},
	}

	if serviceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.Labels != nil ||
		serviceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations != nil ||
		serviceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.UpdateStrategy != nil ||
		serviceTemplate.Spec.ServiceDaemonSet.Resources != nil {
		dpuService.Spec.ServiceDaemonSet = &dpuservicev1.ServiceDaemonSetValues{
			Labels:         serviceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.Labels,
			Annotations:    serviceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations,
			UpdateStrategy: serviceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.UpdateStrategy,
			Resources:      serviceTemplate.Spec.ServiceDaemonSet.Resources,
			// TODO: Figure out what to do with NodeSelector
		}
	}

	dpuService.SetOwnerReferences([]metav1.OwnerReference{*owner})
	dpuService.ObjectMeta.ManagedFields = nil
	dpuService.SetGroupVersionKind(dpuservicev1.DPUServiceGroupVersionKind)

	return dpuService
}

// generateDPUServiceChain generates a DPUServiceChain according to the DPUDeployment
func generateDPUServiceChain(dpuDeploymentNamespacedName types.NamespacedName, owner *metav1.OwnerReference, switches []sfcv1.Switch) *sfcv1.DPUServiceChain {
	dpuServiceChain := &sfcv1.DPUServiceChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dpuDeploymentNamespacedName.Name,
			Namespace: dpuDeploymentNamespacedName.Namespace,
			Labels: map[string]string{
				ParentDPUDeploymentNameLabel: dpuDeploymentNamespacedName.Name,
			},
		},
		Spec: sfcv1.DPUServiceChainSpec{
			// TODO: Derive and add cluster selector
			Template: sfcv1.ServiceChainSetSpecTemplate{
				Spec: sfcv1.ServiceChainSetSpec{
					// TODO: Figure out what to do with NodeSelector
					Template: sfcv1.ServiceChainSpecTemplate{
						Spec: sfcv1.ServiceChainSpec{
							Switches: switches,
						},
					},
				},
			},
		},
	}
	dpuServiceChain.SetOwnerReferences([]metav1.OwnerReference{*owner})
	dpuServiceChain.ObjectMeta.ManagedFields = nil
	dpuServiceChain.SetGroupVersionKind(sfcv1.DPUServiceChainGroupVersionKind)

	return dpuServiceChain
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

// deleteElementOrNil deletes an element from a slice or returns nil if this is the last element in the slice
func deleteElementOrNil[S ~[]E, E any](s S, i, j int) S {
	if len(s) == 1 {
		s = nil
	} else {
		s = slices.Delete[S](s, i, j)
	}
	return s
}

// isMapSubset returns whether the keys and values of the second map are included in the first map
func isMapSubset[K, V comparable](one map[K]V, two map[K]V) bool {
	if len(two) > len(one) {
		return false
	}
	for key, valueTwo := range two {
		if valueOne, found := one[key]; !found || valueOne != valueTwo {
			return false
		}
	}
	return true
}
