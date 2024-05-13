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

	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/inventory"

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

// DPFOperatorConfigReconciler reconciles a DPFOperatorConfig object
// TODO: Consider creating a constructor
type DPFOperatorConfigReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Settings  *DPFOperatorConfigReconcilerSettings
	Inventory *inventory.Manifests
}

// DPFOperatorConfigReconcilerSettings contains settings related to the DPFOperatorConfig.
type DPFOperatorConfigReconcilerSettings struct {
	// CustomOVNKubernetesDPUImage the OVN Kubernetes image deployed by the operator to the DPU enabled nodes (workers).
	CustomOVNKubernetesDPUImage string

	// CustomOVNKubernetesNonDPUImage the OVN Kubernetes image deployed by the operator to the non DPU enabled nodes
	// (control plane)
	CustomOVNKubernetesNonDPUImage string

	// ConfigSingletonNamespaceName restricts reconciliation of the operator to a single DPFOperator Config with a specified namespace and name.
	ConfigSingletonNamespaceName *types.NamespacedName

	// ReconcileOVNKubernetes describes whether or not special steps should be taken to reconcile OVN-Kubernetes.
	// This is a requirement for enabling OVN-Kubernetes offloads to the DPU.
	ReconcileOVNKubernetes bool
}

//+kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=nodes;pods;secrets;services,verbs=create;get;list;watch;patch;delete;update
//+kubebuilder:rbac:groups=core,resources=serviceaccounts;configmaps,verbs=get;list;watch;create;patch;update;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments;daemonsets,verbs=get;list;watch;create;patch;delete
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch;update;get;list;delete;watch
//+kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservices/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims;events;serviceaccounts;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=argoproj.io,resources=appprojects;applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=get;list;watch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains/finalizers,verbs=update
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfaces/finalizers,verbs=update
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuservicechains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuservicechains/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuservicechains/finalizers,verbs=update
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuserviceinterfaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuserviceinterfaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuserviceinterfaces/finalizers,verbs=update
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuserviceipams,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuserviceipams/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuserviceipams/finalizers,verbs=update
//+kubebuilder:rbac:groups=cert-manager.io,resources=issuers;certificates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations;mutatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=dpusets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=dpusets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=dpusets/finalizers,verbs=update
//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=bfbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=bfbs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=bfbs/finalizers,verbs=update
//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=dpus,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=dpus/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=dpus/finalizers,verbs=update
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=get;update;patch

const (
	dpfOperatorConfigControllerName = "dpfoperatorconfig-controller"
)

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
func (r *DPFOperatorConfigReconciler) reconcile(ctx context.Context, dpfOperatorConfig *operatorv1.DPFOperatorConfig) (ctrl.Result, error) {
	if err := r.reconcileSystemComponents(ctx, dpfOperatorConfig); err != nil {
		return ctrl.Result{}, err
	}

	if r.Settings.ReconcileOVNKubernetes {
		// Ensure Custom OVN Kubernetes Deployment is done.
		if err := r.reconcileCustomOVNKubernetesDeployment(ctx, dpfOperatorConfig); err != nil {
			// TODO: In future we should tolerate this error, but only when we have status reporting.
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
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
			BFBPersistentVolumeClaimName: config.Spec.ProvisioningConfiguration.BFBPersistentVolumeClaimName,
			ImagePullSecret:              config.Spec.ProvisioningConfiguration.ImagePullSecret,
		},
		DisableSystemComponents: disableComponents,
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
func (r *DPFOperatorConfigReconciler) reconcileSystemComponents(ctx context.Context, config *operatorv1.DPFOperatorConfig) error {
	var errs []error
	vars := getVariablesFromConfig(config)
	// TODO: Handle deletion of objects on version upgrade.
	// Create objects for components deployed to the management cluster.
	errs = append(errs, r.generateAndPatchObjects(ctx, r.Inventory.DPUService, vars))
	errs = append(errs, r.generateAndPatchObjects(ctx, r.Inventory.DPFProvisioning, vars))

	// Create DPUServices for system components deployed to the DPU cluster.
	errs = append(errs, r.generateAndPatchObjects(ctx, r.Inventory.ServiceFunctionChainSet, vars))
	errs = append(errs, r.generateAndPatchObjects(ctx, r.Inventory.SRIOVDevicePlugin, vars))
	errs = append(errs, r.generateAndPatchObjects(ctx, r.Inventory.Multus, vars))
	errs = append(errs, r.generateAndPatchObjects(ctx, r.Inventory.Flannel, vars))
	errs = append(errs, r.generateAndPatchObjects(ctx, r.Inventory.NvIPAM, vars))

	return kerrors.NewAggregate(errs)
}

func (r *DPFOperatorConfigReconciler) generateAndPatchObjects(ctx context.Context, manifests inventory.Component, vars inventory.Variables) error {
	objs, err := manifests.GenerateManifests(vars)
	if err != nil {
		return fmt.Errorf("error while generating manifests for flannel, err: %v", err)
	}
	var errs []error
	for _, obj := range objs {
		err := r.Client.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName))
		if err != nil {
			errs = append(errs, fmt.Errorf("error patching %v %v: %w",
				obj.GetObjectKind().GroupVersionKind().Kind,
				klog.KObj(obj),
				err))
		}
	}
	return kerrors.NewAggregate(errs)
}
