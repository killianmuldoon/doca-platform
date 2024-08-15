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

package controller

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/operator/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/operator/inventory"

	"github.com/fluxcd/pkg/runtime/patch"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// DefaultDPFOperatorConfigSingletonName is the default single valid name of the DPFOperatorConfig.
	DefaultDPFOperatorConfigSingletonName = "dpfoperatorconfig"
	// DefaultDPFOperatorConfigSingletonNamespace is the default single valid name of the DPFOperatorConfig.
	DefaultDPFOperatorConfigSingletonNamespace = "dpf-operator-system"
)

const (
	dpfOperatorConfigControllerName = "dpfoperatorconfig-controller"
)

var applyPatchOptions = []client.PatchOption{client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)}

// DPFOperatorConfigReconciler reconciles a DPFOperatorConfig object
// TODO: Consider creating a constructor
type DPFOperatorConfigReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Settings  *DPFOperatorConfigReconcilerSettings
	Inventory *inventory.SystemComponents
}

// DPFOperatorConfigReconcilerSettings contains settings related to the DPFOperatorConfig.
type DPFOperatorConfigReconcilerSettings struct {
	// ConfigSingletonNamespaceName restricts reconciliation of the operator to a single DPFOperator Config with a specified namespace and name.
	ConfigSingletonNamespaceName *types.NamespacedName

	// SkipWebhook skips ValidatingWebhookConfiguration/MutatingWebhookConfiguration installing.
	// SkipWebhook is true iff we are running tests in an environment that doesn't have functioning webhook server.
	SkipWebhook bool
}

// This Operator deploys ArgoCD and needs RBAC for all groups, resources and verbs.
// TODO: Revisit this blanket RBAC.
//+kubebuilder:rbac:groups=*,resources=*,verbs=*

// SetupWithManager sets up the controller with the Manager.
// TODO: consider watching other objects this controller interacts with e.g. pods, secrets with a label selector to speed up reconciliation.
func (r *DPFOperatorConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.DPFOperatorConfig{}).
		Complete(r)
}

// Reconcile reconciles changes in a DPFOperatorConfig.
func (r *DPFOperatorConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")
	// TODO: Validate that the DPF Operator Config only exists with the correct name and namespace at creation time.
	if r.Settings.ConfigSingletonNamespaceName != nil {
		if req.Namespace != r.Settings.ConfigSingletonNamespaceName.Name && req.Name != r.Settings.ConfigSingletonNamespaceName.Name {
			return ctrl.Result{}, fmt.Errorf("invalid config: only one object with name %s in namespace %s is allowed",
				r.Settings.ConfigSingletonNamespaceName.Namespace, r.Settings.ConfigSingletonNamespaceName.Name)
		}
	}

	dpfOperatorConfig := &operatorv1.DPFOperatorConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpfOperatorConfig); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	conditions.EnsureConditions(dpfOperatorConfig, operatorv1.Conditions)

	// If the DPFOperatorConfig is paused take no further action.
	if isPaused(dpfOperatorConfig) {
		log.Info("Reconciling Paused")
		return ctrl.Result{}, nil
	}

	patcher := patch.NewSerialPatcher(dpfOperatorConfig, r.Client)

	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {
		// Update the ready condition for the system components.
		r.updateSystemComponentStatus(ctx, dpfOperatorConfig)
		// Set the summary condition for the DPFOperatorConfig.
		conditions.SetSummary(dpfOperatorConfig)

		log.Info("Patching")
		if err := patcher.Patch(ctx, dpfOperatorConfig, patch.WithFieldOwner(dpfOperatorConfigControllerName)); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deletion reconciliation loop.
	if !dpfOperatorConfig.ObjectMeta.DeletionTimestamp.IsZero() {
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

func isPaused(config *operatorv1.DPFOperatorConfig) bool {
	if config.Spec.Overrides == nil {
		return false
	}
	return *config.Spec.Overrides.Paused
}

//nolint:unparam
func (r *DPFOperatorConfigReconciler) reconcile(ctx context.Context, dpfOperatorConfig *operatorv1.DPFOperatorConfig) (ctrl.Result, error) {
	if err := r.reconcileImagePullSecrets(ctx, dpfOperatorConfig); err != nil {
		conditions.AddFalse(
			dpfOperatorConfig,
			operatorv1.ImagePullSecretsReconciledCondition,
			conditions.ReasonError,
			conditions.ConditionMessage(fmt.Sprintf("Image pull secrets must be reconciled for DPF Operator to continue: %s", err.Error())))
		return ctrl.Result{}, err
	}
	conditions.AddTrue(dpfOperatorConfig, operatorv1.ImagePullSecretsReconciledCondition)

	if err := r.reconcileSystemComponents(ctx, dpfOperatorConfig); err != nil {
		conditions.AddFalse(
			dpfOperatorConfig,
			operatorv1.SystemComponentsReconciledCondition,
			conditions.ReasonError,
			conditions.ConditionMessage(err.Error()))
		return ctrl.Result{}, err
	}
	conditions.AddTrue(dpfOperatorConfig, operatorv1.SystemComponentsReconciledCondition)

	return ctrl.Result{}, nil
}

func (r *DPFOperatorConfigReconciler) updateSystemComponentStatus(ctx context.Context, config *operatorv1.DPFOperatorConfig) {
	unreadyMessages := []string{}
	log := ctrllog.FromContext(ctx)
	log.Info("Updating status of DPF OperatorConfig")
	baseMessage := "System components are not ready: "
	for _, component := range r.Inventory.AllComponents() {
		log.Info("Component", "component", component)
		err := component.IsReady(ctx, r.Client, config.Namespace)
		if err != nil {
			log.Error(err, "Component not ready", "component", component)
			unreadyMessages = append(unreadyMessages, fmt.Sprintf("%s: %s", component.Name(), err.Error()))
		}
	}

	if len(unreadyMessages) > 0 {
		conditions.AddFalse(
			config,
			operatorv1.SystemComponentsReadyCondition,
			conditions.ReasonError,
			conditions.ConditionMessage(fmt.Sprintf("%s: %s", baseMessage, strings.Join(unreadyMessages, ", "))))
		return
	}

	conditions.AddTrue(config, operatorv1.SystemComponentsReadyCondition)
}

func getVariablesFromConfig(config *operatorv1.DPFOperatorConfig) inventory.Variables {
	disableComponents := make(map[string]bool)
	if config.Spec.Overrides != nil {
		for _, item := range config.Spec.Overrides.DisableSystemComponents {
			disableComponents[item] = true
		}
	}
	return inventory.Variables{
		Namespace: config.Namespace,
		DPFProvisioningController: inventory.DPFProvisioningVariables{
			BFBPersistentVolumeClaimName:        config.Spec.ProvisioningConfiguration.BFBPersistentVolumeClaimName,
			ImagePullSecretForDMSAndHostNetwork: config.Spec.ProvisioningConfiguration.ImagePullSecretForDMSAndHostNetwork,
			DHCP:                                config.Spec.ProvisioningConfiguration.DHCPServerAddress,
			DMSTimeout:                          config.Spec.ProvisioningConfiguration.DMSTimeout,
		},
		DisableSystemComponents: disableComponents,
		ImagePullSecrets:        config.Spec.ImagePullSecrets,
	}
}

// reconcileSystemComponents applies manifests for components which form the DPF system.
// It deploys the following:
// 1. DPUService controller
// 2. DPU provisioning controller
// 3. ServiceFunctionChainSet controller DPUService
// 4. SR-IOV device plugin DPUService
// 5. Multus DPUService
// 6. Flannel DPUService
// 7. NVIDIA Kubernetes IPAM
// 8. OVS CNI
// 8. SFC Controller
func (r *DPFOperatorConfigReconciler) reconcileSystemComponents(ctx context.Context, config *operatorv1.DPFOperatorConfig) error {
	vars := getVariablesFromConfig(config)
	// TODO: Handle deletion of objects on version upgrade.

	// Create objects for system components.
	for _, component := range r.Inventory.AllComponents() {
		if err := r.generateAndPatchObjects(ctx, component, vars); err != nil {
			return err
		}
	}

	return nil
}

func (r *DPFOperatorConfigReconciler) generateAndPatchObjects(ctx context.Context, manifests inventory.Component, vars inventory.Variables) error {
	objs, err := manifests.GenerateManifests(vars)
	if err != nil {
		return fmt.Errorf("error while generating manifests for object, err: %v", err)
	}
	var errs []error
	for _, obj := range objs {
		if kind := obj.GetObjectKind().GroupVersionKind().Kind; r.Settings.SkipWebhook &&
			(kind == "ValidatingWebhookConfiguration" || kind == "MutatingWebhookConfiguration") {
			continue
		}
		err := r.Client.Patch(ctx, obj, client.Apply, applyPatchOptions...)
		if err != nil {
			errs = append(errs, fmt.Errorf("error patching %v %v: %w",
				obj.GetObjectKind().GroupVersionKind().Kind,
				klog.KObj(obj),
				err))
		}
	}
	return kerrors.NewAggregate(errs)
}

func (r *DPFOperatorConfigReconciler) reconcileImagePullSecrets(ctx context.Context, config *operatorv1.DPFOperatorConfig) error {
	for _, name := range config.Spec.ImagePullSecrets {
		secret := &corev1.Secret{}
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: config.Namespace, Name: name}, secret); err != nil {
			return err
		}
		labels := secret.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		secret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
		secret.SetManagedFields(nil)
		labels[dpuservicev1.DPFImagePullSecretLabelKey] = ""
		secret.SetLabels(labels)
		if err := r.Client.Patch(ctx, secret, client.Apply, applyPatchOptions...); err != nil {
			return err
		}
	}
	return nil
}
