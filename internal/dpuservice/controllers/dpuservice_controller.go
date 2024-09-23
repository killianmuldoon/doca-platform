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
	"strings"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/operator/v1alpha1"
	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/argocd"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/argocd/api/application"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/argocd/api/application/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane/kubeconfig"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane/metadata"
	dpuserviceutils "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/dpuservice/utils"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/operator/utils"

	"github.com/fluxcd/pkg/runtime/patch"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// DPUServiceReconciler reconciles a DPUService object
type DPUServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// pauseDPUServiceReconciler pauses the DPUService Reconciler by doing noop reconciliation loops. This is helpful to
// make tests faster and less complex
var pauseDPUServiceReconciler bool

// +kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservices/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=create
// +kubebuilder:rbac:groups=argoproj.io,resources=appprojects;applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs,verbs=get;list;watch

const (
	dpuServiceControllerName = "dpuservice-manager"

	// TODO: These constants don't belong here and should be moved as they're shared with other packages.
	argoCDSecretLabelKey   = "argocd.argoproj.io/secret-type"
	argoCDSecretLabelValue = "cluster"
	dpuAppProjectName      = "doca-platform-project-dpu"
	hostAppProjectName     = "doca-platform-project-host"

	dpfServiceIDLabelKey = "sfc.nvidia.com/service"
)

// applyPatchOptions contains options which are passed to every `client.Apply` patch.
var applyPatchOptions = []client.PatchOption{
	client.ForceOwnership,
	client.FieldOwner(dpuServiceControllerName),
}

// SetupWithManager sets up the controller with the Manager.
func (r *DPUServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	tenantControlPlane := &metav1.PartialObjectMetadata{}
	tenantControlPlane.SetGroupVersionKind(controlplanemeta.TenantControlPlaneGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpuservicev1.DPUService{}).
		Watches(&argov1.Application{}, handler.EnqueueRequestsFromMapFunc(r.ArgoApplicationToDPUService)).
		// TODO: This doesn't currently work for status updates - need to find a way to increase reconciliation frequency.
		WatchesMetadata(tenantControlPlane, handler.EnqueueRequestsFromMapFunc(r.DPUClusterToDPUService)).
		Complete(r)
}

// Reconcile reconciles changes in a DPUService.
func (r *DPUServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	if pauseDPUServiceReconciler {
		log.Info("noop reconciliation")
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling")
	dpuService := &dpuservicev1.DPUService{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpuService); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	patcher := patch.NewSerialPatcher(dpuService, r.Client)

	conditions.EnsureConditions(dpuService, dpuservicev1.Conditions)
	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {
		log.Info("Patching")
		if err := r.summary(ctx, dpuService); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
		if err := patcher.Patch(ctx, dpuService,
			patch.WithFieldOwner(dpuServiceControllerName),
			patch.WithStatusObservedGeneration{},
			patch.WithOwnedConditions{Conditions: conditions.TypesAsStrings(dpuservicev1.Conditions)},
		); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
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
	if err := r.reconcile(ctx, dpuService); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

//nolint:unparam
func (r *DPUServiceReconciler) reconcileDelete(ctx context.Context, dpuService *dpuservicev1.DPUService) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Handling DPUService deletion")
	res, err := r.reconcileDeleteApplications(ctx, dpuService)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !res.IsZero() {
		return res, nil
	}

	if err := r.reconcileDeleteImagePullSecrets(ctx, dpuService); err != nil {
		return ctrl.Result{}, err
	}

	// Add AwaitingDeletion condition to true. Probably this will never going to apply on the DPUService as it will be
	// deleted in the next step anyway.
	conditions.AddTrue(dpuService, dpuservicev1.ConditionApplicationsReconciled)

	// If there are no associated applications remove the finalizer.
	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpuService, dpuservicev1.DPUServiceFinalizer)

	return ctrl.Result{}, nil
}

func (r *DPUServiceReconciler) reconcileDeleteApplications(ctx context.Context, dpuService *dpuservicev1.DPUService) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	applications := &argov1.ApplicationList{}
	if err := r.Client.List(ctx, applications, client.MatchingLabels{
		dpuservicev1.DPUServiceNameLabelKey:      dpuService.Name,
		dpuservicev1.DPUServiceNamespaceLabelKey: dpuService.Namespace,
	}); err != nil {
		return ctrl.Result{}, err
	}

	for _, app := range applications.Items {
		if err := r.Client.Delete(ctx, &app); err != nil {
			// Tolerate if the application is not found and already deleted.
			if !apierrors.IsNotFound(err) {
				conditions.AddFalse(
					dpuService,
					dpuservicev1.ConditionApplicationsReconciled,
					conditions.ReasonAwaitingDeletion,
					conditions.ConditionMessage(fmt.Sprintf("Error occurred: %v", err)),
				)
				return ctrl.Result{}, err
			}
		}
	}

	// List Applications again to verify if it is in deletion.
	// Already deleted Applications will not be listed.
	if err := r.Client.List(ctx, applications, client.MatchingLabels{
		dpuservicev1.DPUServiceNameLabelKey:      dpuService.Name,
		dpuservicev1.DPUServiceNamespaceLabelKey: dpuService.Namespace,
	}); err != nil {
		return ctrl.Result{}, err
	}

	appsInDeletion := []string{}
	for _, app := range applications.Items {
		if !app.GetDeletionTimestamp().IsZero() {
			argoApp := fmt.Sprintf("%s/%s.%s", app.Namespace, app.Name, app.GroupVersionKind().Group)
			appsInDeletion = append(appsInDeletion, argoApp)
		}
	}

	if len(appsInDeletion) > 0 {
		message := fmt.Sprintf("The following applications are awaiting deletion: %v", strings.Join(appsInDeletion, ", "))
		conditions.AddFalse(
			dpuService,
			dpuservicev1.ConditionApplicationsReconciled,
			conditions.ReasonAwaitingDeletion,
			conditions.ConditionMessage(message),
		)
	}

	if len(applications.Items) > 0 {
		log.Info(fmt.Sprintf("Requeueing: %d applications still managed by DPUService", len(applications.Items)))
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// reconcileDeleteImagePullSecrets ensures imagePullSecrets are removed from DPUClusters where no more DPUServices with that namespace are deployed.
func (r *DPUServiceReconciler) reconcileDeleteImagePullSecrets(ctx context.Context, dpuService *dpuservicev1.DPUService) error {
	log := ctrllog.FromContext(ctx)

	// List all DPUServices.
	dpuServices := &dpuservicev1.DPUServiceList{}
	if err := r.List(ctx, dpuServices, client.InNamespace(dpuService.GetNamespace())); err != nil {
		return fmt.Errorf("list DPUServices: %w", err)
	}

	// If at least 1 DPUService is not in deleting we cannot clean up the ImagePullSecrets.
	for _, dpuService := range dpuServices.Items {
		if dpuService.DeletionTimestamp.IsZero() {
			return nil
		}
	}

	log.Info("Cleaning up ImagePullSecrets in all DPU Clusters")
	clusters, err := controlplane.GetDPFClusters(ctx, r.Client)
	if err != nil {
		return err
	}

	// Delete the secrets in every DPUCluster.
	var errs []error
	for _, cluster := range clusters {
		dpuClusterClient, err := cluster.NewClient(ctx, r.Client)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// First get the secrets with the correct label.
		secrets := &corev1.SecretList{}
		err = dpuClusterClient.List(ctx, secrets, client.HasLabels{dpuservicev1.DPFImagePullSecretLabelKey}, client.InNamespace(dpuService.GetNamespace()))
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// Create list for better logging.
		var secretsNameList []string
		for _, secret := range secrets.Items {
			secretNamespaceName := fmt.Sprintf("%s/%s", secret.GetNamespace(), secret.GetName())
			secretsNameList = append(secretsNameList, secretNamespaceName)
		}

		log.Info("Deleting ImagePullSecrets", "cluster", cluster.Name, "secrets", secretsNameList)
		for _, secret := range secrets.Items {
			if err := dpuClusterClient.Delete(ctx, secret.DeepCopy()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return kerrors.NewAggregate(errs)
}

func (r *DPUServiceReconciler) reconcile(ctx context.Context, dpuService *dpuservicev1.DPUService) error {
	// Get the list of clusters this DPUService targets.
	// TODO: Add some way to check if the clusters are healthy. Reconciler should retry clusters if they're unready.
	clusters, err := controlplane.GetDPFClusters(ctx, r.Client)
	if err != nil {
		return err
	}

	dpfOperatorConfigList := operatorv1.DPFOperatorConfigList{}
	if err := r.Client.List(ctx, &dpfOperatorConfigList, &client.ListOptions{}); err != nil {
		return fmt.Errorf("list DPFOperatorConfigs: %w", err)
	}
	if len(dpfOperatorConfigList.Items) == 0 || len(dpfOperatorConfigList.Items) > 1 {
		return fmt.Errorf("exactly one DPFOperatorConfig necessary")
	}
	dpfOperatorConfig := &dpfOperatorConfigList.Items[0]

	if err := r.reconcileInterfaces(ctx, dpuService); err != nil {
		return err
	}

	// TODO: Add the condition to the DPUService only if there are annotations to add to the application serviceDeamonSet.
	conditions.AddTrue(dpuService, dpuservicev1.ConditionDPUServiceInterfaceReconciled)

	if err := r.reconcileApplicationPrereqs(ctx, dpuService, clusters, dpfOperatorConfig); err != nil {
		message := fmt.Sprintf("Unable to reconcile application prereq for %s", err.Error())
		conditions.AddFalse(
			dpuService,
			dpuservicev1.ConditionApplicationPrereqsReconciled,
			conditions.ReasonError,
			conditions.ConditionMessage(message),
		)
		return err
	}
	conditions.AddTrue(dpuService, dpuservicev1.ConditionApplicationPrereqsReconciled)

	// Update the ArgoApplication for all target clusters.
	if err = r.reconcileApplication(ctx, clusters, dpuService, dpfOperatorConfig.GetNamespace()); err != nil {
		message := fmt.Sprintf("Unable to reconcile Applications: %v", err)
		conditions.AddFalse(
			dpuService,
			dpuservicev1.ConditionApplicationsReconciled,
			conditions.ReasonError,
			conditions.ConditionMessage(message),
		)
		return err
	}
	conditions.AddTrue(dpuService, dpuservicev1.ConditionApplicationsReconciled)

	return nil
}

func (r *DPUServiceReconciler) reconcileApplicationPrereqs(ctx context.Context, dpuService *dpuservicev1.DPUService, clusters []controlplane.DPFCluster, dpfOperatorConfig *operatorv1.DPFOperatorConfig) error {
	// Ensure the DPUService namespace exists in target clusters.
	project := getProjectName(dpuService)
	// TODO: think about how to cleanup the namespace in the DPU.
	if err := r.ensureNamespaces(ctx, clusters, project, dpuService.GetNamespace()); err != nil {
		return fmt.Errorf("cluster namespaces: %v", err)
	}

	// Ensure the Argo secret for each cluster is up-to-date.
	if err := r.reconcileArgoSecrets(ctx, clusters, dpfOperatorConfig); err != nil {
		return fmt.Errorf("ArgoSecrets: %v", err)
	}

	//  Ensure the ArgoCD AppProject exists and is up-to-date.
	if err := r.reconcileAppProject(ctx, clusters, dpfOperatorConfig); err != nil {
		return fmt.Errorf("AppProject: %v", err)
	}

	// Reconcile the ImagePullSecrets.
	if err := r.reconcileImagePullSecrets(ctx, clusters, dpuService); err != nil {
		return fmt.Errorf("ImagePullSecrets: %v", err)
	}

	return nil
}

// reconcileInterfaces reconciles the DPUServiceInterfaces associated with the DPUService.
// TODO: Process the DPUServiceInterfaces and return annotations when required.
func (r *DPUServiceReconciler) reconcileInterfaces(ctx context.Context, dpuService *dpuservicev1.DPUService) error {
	log := ctrllog.FromContext(ctx)
	if dpuService.Spec.Interfaces == nil {
		return nil
	}

	for _, interfaceName := range dpuService.Spec.Interfaces {
		dpuServiceInterface := &sfcv1.DPUServiceInterface{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: interfaceName, Namespace: dpuService.Namespace}, dpuServiceInterface); err != nil {
			// Requeue until the DPUServiceInterface is available.
			conditions.AddFalse(dpuService,
				dpuservicev1.ConditionDPUServiceInterfaceReconciled,
				conditions.ReasonError,
				conditions.ConditionMessage(fmt.Sprintf("failed to get DPUServiceInterface %s: %v", interfaceName, err)))
			return err
		}

		// report conflicting ServiceID in DPUServiceInterface
		if dpuServiceInterface.Spec.GetTemplateSpec().GetTemplateSpec().Service != nil && dpuService.Spec.ServiceID != nil {
			if dpuServiceInterface.Spec.GetTemplateSpec().GetTemplateSpec().Service.ServiceID != *dpuService.Spec.ServiceID {
				conditions.AddFalse(dpuService,
					dpuservicev1.ConditionDPUServiceInterfaceReconciled,
					conditions.ReasonFailure,
					conditions.ConditionMessage(fmt.Sprintf("conflicting ServiceID in DPUServiceInterface %s", interfaceName)))
				return fmt.Errorf("conflicting ServiceID in DPUServiceInterface %s", interfaceName)
			}
		}

		inUse, exist := isDPUServiceInterfaceInUse(dpuServiceInterface, dpuService)
		if inUse {
			conditions.AddFalse(dpuService,
				dpuservicev1.ConditionDPUServiceInterfaceReconciled,
				conditions.ReasonFailure,
				conditions.ConditionMessage(fmt.Sprintf("dpuServiceInterface %s is in use by another DPUService", interfaceName)))
			return fmt.Errorf("dpuServiceInterface %s is in use by another DPUService", interfaceName)
		}

		if !exist {
			dpuServiceInterface.SetAnnotations(map[string]string{
				dpuservicev1.DPUServiceInterfaceAnnotationKey: dpuService.Name,
			})
		}

		// set serviceID on the DPUServiceInterface if it is not set.
		if dpuServiceInterface.Spec.GetTemplateSpec().GetTemplateSpec().Service == nil {
			dpuServiceInterface.Spec.GetTemplateSpec().GetTemplateSpec().Service = &sfcv1.ServiceDef{
				ServiceID: *dpuService.Spec.ServiceID,
			}
		}

		// remove managed fields from the DPUServiceInterface to avoid conflicts with the DPUService.
		dpuServiceInterface.SetManagedFields(nil)
		// Set the GroupVersionKind to the DPUServiceInterface.
		dpuServiceInterface.SetGroupVersionKind(sfcv1.DPUServiceInterfaceGroupVersionKind)
		if err := r.Client.Patch(ctx, dpuServiceInterface, client.Apply, applyPatchOptions...); err != nil {
			conditions.AddFalse(dpuService,
				dpuservicev1.ConditionDPUServiceInterfaceReconciled,
				conditions.ReasonError,
				conditions.ConditionMessage(fmt.Sprintf("failed to patch DPUServiceInterface %s: %v", interfaceName, err)))
			return err
		}
	}
	log.Info("Reconciled DPUServiceInterfaces")
	return nil
}

func (r *DPUServiceReconciler) ensureNamespaces(ctx context.Context, clusters []controlplane.DPFCluster, project, dpuNamespace string) error {
	var errs []error
	if project == dpuAppProjectName {
		for _, cluster := range clusters {
			dpuClusterClient, err := cluster.NewClient(ctx, r.Client)
			if err != nil {
				errs = append(errs, fmt.Errorf("get cluster %s: %v", cluster.Name, err))
			}
			if err := utils.EnsureNamespace(ctx, dpuClusterClient, dpuNamespace); err != nil {
				errs = append(errs, fmt.Errorf("namespace for cluster %s: %v", cluster.Name, err))
			}
		}
	} else {
		if err := utils.EnsureNamespace(ctx, r.Client, dpuNamespace); err != nil {
			return fmt.Errorf("in-cluster namespace: %v", err)
		}
	}
	return kerrors.NewAggregate(errs)
}

// reconcileArgoSecrets reconciles a Secret in the format that ArgoCD expects. It uses data from the control plane secret.
func (r *DPUServiceReconciler) reconcileArgoSecrets(ctx context.Context, clusters []controlplane.DPFCluster, dpfOperatorConfig *operatorv1.DPFOperatorConfig) error {
	log := ctrllog.FromContext(ctx)

	var errs []error
	for _, cluster := range clusters {
		// Get the control plane kubeconfig
		adminConfig, err := cluster.GetKubeconfig(ctx, r.Client)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// Template an argoSecret using information from the control plane secret.
		argoSecret, err := createArgoSecretFromKubeconfig(adminConfig, dpfOperatorConfig.GetNamespace(), cluster.String())
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// Add owner reference ArgoSecret->DPFOperatorConfig to ensure that the Secret will be deleted with the DPFOperatorConfig.
		// This ensures that we should have no orphaned ArgoCD Secrets.
		owner := metav1.NewControllerRef(dpfOperatorConfig, operatorv1.DPFOperatorConfigGroupVersionKind)
		argoSecret.SetOwnerReferences([]metav1.OwnerReference{*owner})

		// Create or patch
		log.Info("Patching Secrets for DPF clusters")
		if err := r.Client.Patch(ctx, argoSecret, client.Apply, applyPatchOptions...); err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return kerrors.NewAggregate(errs)
}

// summary sets the conditions for the sync status of the ArgoCD applications as well as the overall conditions.
func (r *DPUServiceReconciler) summary(ctx context.Context, dpuService *dpuservicev1.DPUService) error {
	defer conditions.SetSummary(dpuService)

	// Get the list of clusters this DPUService targets.
	clusters, err := controlplane.GetDPFClusters(ctx, r.Client)
	if err != nil {
		return err
	}

	applicationList := &argov1.ApplicationList{}
	// List all of the applications matching this DPUService.
	// TODO: Optimize list calls by adding indexes to the reconciler where needed.
	if err := r.Client.List(ctx, applicationList, client.MatchingLabels{
		dpuservicev1.DPUServiceNameLabelKey:      dpuService.Name,
		dpuservicev1.DPUServiceNamespaceLabelKey: dpuService.Namespace,
	}); err != nil {
		return err
	}

	unreadyApplications := []string{}
	// Get a summarized error for each application linked to the DPUService and set it as the condition message.
	for _, app := range applicationList.Items {
		argoApp := fmt.Sprintf("%s/%s.%s", app.Namespace, app.Name, application.ApplicationFullName)
		if app.Status.Sync.Status != argov1.SyncStatusCodeSynced || app.Status.Health.Status != argov1.HealthStatusHealthy {
			appWithStatus := fmt.Sprintf("%s (Sync=%s, Health=%s)", argoApp, app.Status.Sync.Status, app.Status.Health.Status)
			unreadyApplications = append(unreadyApplications, appWithStatus)
		}
	}

	// Update condition and requeue if there are any errors, or if there are fewer applications than we have clusters.
	if len(unreadyApplications) > 0 || len(applicationList.Items) != len(clusters) {
		message := fmt.Sprintf("Applications are not ready: %v", strings.Join(unreadyApplications, ", "))
		conditions.AddFalse(
			dpuService,
			dpuservicev1.ConditionApplicationsReady,
			conditions.ReasonPending,
			conditions.ConditionMessage(message),
		)
		return nil
	}
	conditions.AddTrue(dpuService, dpuservicev1.ConditionApplicationsReady)
	return nil
}

func (r *DPUServiceReconciler) reconcileImagePullSecrets(ctx context.Context, clusters []controlplane.DPFCluster, service *dpuservicev1.DPUService) error {
	log := ctrllog.FromContext(ctx)
	log.Info("Patching ImagePullSecrets for DPU clusters")

	// First get the secrets with the correct label.
	secrets := &corev1.SecretList{}
	err := r.List(ctx, secrets, client.HasLabels{dpuservicev1.DPFImagePullSecretLabelKey})
	if err != nil {
		return err
	}

	secretsToPatch := []*corev1.Secret{}
	existingSecrets := map[string]bool{}
	// Copy the spec of the secret to a new secret and set the namespace.
	for _, secret := range secrets.Items {
		secretsToPatch = append(secretsToPatch, &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.Name,
				Namespace: service.Namespace,
				Labels:    secret.Labels,
			},
			Immutable: secret.Immutable,
			Data:      secret.Data,
			Type:      secret.Type,
		})
		existingSecrets[secret.GetName()] = true
	}

	// Apply the new secret to every DPUCluster.
	var errs []error
	for _, cluster := range clusters {
		dpuClusterClient, err := cluster.NewClient(ctx, r.Client)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, secret := range secretsToPatch {
			if err := dpuClusterClient.Patch(ctx, secret, client.Apply, applyPatchOptions...); err != nil {
				errs = append(errs, err)
			}
		}

		// Cleanup orphaned secrets.
		inClusterSecrets := &corev1.SecretList{}
		if err := dpuClusterClient.List(ctx, inClusterSecrets, client.HasLabels{dpuservicev1.DPFImagePullSecretLabelKey}); err != nil {
			return err
		}
		for _, secret := range inClusterSecrets.Items {
			if _, ok := existingSecrets[secret.GetName()]; ok {
				log.Info("ImagePullSecret still present in cluster", "secret", secret.Name, "cluster", cluster.Name)
				continue
			}

			log.Info("Deleting secret in cluster", "secret", secret.Name, "cluster", cluster.Name)
			if err := dpuClusterClient.Delete(ctx, &secret); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return kerrors.NewAggregate(errs)
}

func (r *DPUServiceReconciler) reconcileAppProject(ctx context.Context, clusters []controlplane.DPFCluster, dpfOperatorConfig *operatorv1.DPFOperatorConfig) error {
	log := ctrllog.FromContext(ctx)

	clusterKeys := []types.NamespacedName{}
	for i := range clusters {
		clusterKeys = append(clusterKeys, types.NamespacedName{Namespace: clusters[i].Namespace, Name: clusters[i].Name})
	}
	dpuAppProject := argocd.NewAppProject(dpfOperatorConfig.Namespace, dpuAppProjectName, clusterKeys)
	// Add owner reference AppProject->DPFOperatorConfig to ensure that the AppProject will be deleted with the DPFOperatorConfig.
	// This ensures that we should have no orphaned ArgoCD AppProjects.
	owner := metav1.NewControllerRef(dpfOperatorConfig, operatorv1.DPFOperatorConfigGroupVersionKind)
	dpuAppProject.SetOwnerReferences([]metav1.OwnerReference{*owner})

	log.Info("Patching AppProject for DPU clusters")
	if err := r.Client.Patch(ctx, dpuAppProject, client.Apply, applyPatchOptions...); err != nil {
		return err
	}

	inClusterKey := []types.NamespacedName{{Namespace: "*", Name: "in-cluster"}}
	hostAppProject := argocd.NewAppProject(dpfOperatorConfig.Namespace, hostAppProjectName, inClusterKey)
	// Add owner reference, same as above.
	hostAppProject.SetOwnerReferences([]metav1.OwnerReference{*owner})

	log.Info("Patching AppProject for Host cluster")
	if err := r.Client.Patch(ctx, hostAppProject, client.Apply, applyPatchOptions...); err != nil {
		return err
	}

	return nil
}

func (r *DPUServiceReconciler) reconcileApplication(ctx context.Context, clusters []controlplane.DPFCluster, dpuService *dpuservicev1.DPUService, dpfOperatorConfigNamespace string) error {
	project := getProjectName(dpuService)
	if project == dpuAppProjectName {
		for _, cluster := range clusters {
			if err := r.ensureApplication(ctx, dpuService, cluster.Name, dpfOperatorConfigNamespace); err != nil {
				return err
			}
		}
	} else {
		return r.ensureApplication(ctx, dpuService, "in-cluster", dpfOperatorConfigNamespace)
	}
	return nil
}

func (r *DPUServiceReconciler) ensureApplication(ctx context.Context, dpuService *dpuservicev1.DPUService, applicationName, dpfOperatorConfigNamespace string) error {
	log := ctrllog.FromContext(ctx)
	project := getProjectName(dpuService)
	values, err := argoCDValuesFromDPUService(dpuService)
	if err != nil {
		return err
	}

	argoApplication := argocd.NewApplication(dpfOperatorConfigNamespace, project, dpuService, values, applicationName)
	gotArgoApplication := &argov1.Application{}

	// If Application does not exist, create it. Otherwise, patch the object.
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(argoApplication), gotArgoApplication); apierrors.IsNotFound(err) {
		log.Info("Creating Application", "Application", klog.KObj(argoApplication))
		if err := r.Client.Create(ctx, argoApplication); err != nil {
			return err
		}
	} else {
		// Return early if the spec has not changed.
		if reflect.DeepEqual(gotArgoApplication.Spec, argoApplication.Spec) {
			return nil
		}

		// Copy spec to the original object and update it.
		gotArgoApplication.Spec = argoApplication.Spec

		log.Info("Updating Application", "Application", klog.KObj(gotArgoApplication))
		if err := r.Client.Update(ctx, gotArgoApplication); err != nil {
			return err
		}
	}
	return nil
}

// config is used to marshal the config section of the argoCD secret data.
type config struct {
	TlsClientConfig tlsClientConfig `json:"tlsClientConfig"`
}

// tlsClientConfig is used to marshal the tlsClientConfig section of the argoCD secret data.config.
type tlsClientConfig struct {
	CaData   []byte `json:"caData,omitempty"`
	KeyData  []byte `json:"keyData,omitempty"`
	CertData []byte `json:"certData,omitempty"`
}

// createArgoSecretFromKubeconfig generates an ArgoCD cluster secret from the given kubeconfig.
func createArgoSecretFromKubeconfig(kubeconfig *kubeconfig.Type, dpfOperatorConfigNamespace, clusterName string) (*corev1.Secret, error) {
	clusterConfigName := kubeconfig.Clusters[0].Name
	clusterConfigServer := kubeconfig.Clusters[0].Cluster.Server
	secretConfig, err := json.Marshal(config{TlsClientConfig: tlsClientConfig{
		CaData:   kubeconfig.Clusters[0].Cluster.CertificateAuthorityData,
		KeyData:  kubeconfig.Users[0].User.ClientKeyData,
		CertData: kubeconfig.Users[0].User.ClientCertificateData,
	}})
	if err != nil {
		return nil, err
	}
	return createArgoCDSecret(secretConfig, dpfOperatorConfigNamespace, clusterName, clusterConfigName, clusterConfigServer), nil
}

// createArgoCDSecret templates an ArgoCD cluster Secret with the passed values.
func createArgoCDSecret(secretConfig []byte, dpfOperatorConfigNamespace, clusterName, clusterConfigName, clusterConfigServer string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			// The secret name is the cluster name. DPUClusters must have unique names.
			Name:      clusterName,
			Namespace: dpfOperatorConfigNamespace,
			Labels: map[string]string{
				argoCDSecretLabelKey:                argoCDSecretLabelValue,
				controlplanemeta.DPFClusterLabelKey: clusterName,
				operatorv1.DPFComponentLabelKey:     dpuServiceControllerName,
			},
			OwnerReferences: nil,
			Annotations:     nil,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"name":   []byte(clusterConfigName),
			"server": []byte(clusterConfigServer),
			"config": secretConfig,
		},
	}
}

// getProjectName returns the correct project name for the DPUService depending on the cluster it's destined for.
func getProjectName(dpuService *dpuservicev1.DPUService) string {
	if dpuService.Spec.DeployInCluster != nil {
		if *dpuService.Spec.DeployInCluster {
			return hostAppProjectName
		}
	}
	return dpuAppProjectName
}

func argoCDValuesFromDPUService(dpuService *dpuservicev1.DPUService) (*runtime.RawExtension, error) {
	service := dpuService.DeepCopy()
	if service.Spec.ServiceDaemonSet == nil {
		service.Spec.ServiceDaemonSet = &dpuservicev1.ServiceDaemonSetValues{}
	}
	if service.Spec.ServiceDaemonSet.Labels == nil {
		service.Spec.ServiceDaemonSet.Labels = map[string]string{}
	}
	if dpuService.Spec.ServiceID != nil {
		service.Spec.ServiceDaemonSet.Labels[dpfServiceIDLabelKey] = *dpuService.Spec.ServiceID
	}

	// Marshal the ServiceDaemonSet and other values to map[string]interface to combine them.
	var otherValues, serviceDaemonSetValues map[string]interface{}
	if service.Spec.HelmChart.Values != nil {
		if err := json.Unmarshal(service.Spec.HelmChart.Values.Raw, &otherValues); err != nil {
			return nil, err
		}
	}

	// Unmarshal the ServiceDaemonSet to get the byte representation.
	dsValuesData, err := json.Marshal(service.Spec.ServiceDaemonSet)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(dsValuesData, &serviceDaemonSetValues); err != nil {
		return nil, err
	}

	serviceDaemonSetValuesWithKey := map[string]interface{}{
		"serviceDaemonSet": serviceDaemonSetValues,
	}

	// Combine values
	combinedValues := dpuserviceutils.MergeMaps(serviceDaemonSetValuesWithKey, otherValues)

	data, err := json.Marshal(combinedValues)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{Raw: data}, nil
}

// DPUClusterToDPUService ensures all DPUServices are updated each time there is an update to a DPUCluster.
func (r *DPUServiceReconciler) DPUClusterToDPUService(ctx context.Context, o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	dpuServiceList := &dpuservicev1.DPUServiceList{}
	if err := r.Client.List(ctx, dpuServiceList); err != nil {
		return nil
	}
	for _, m := range dpuServiceList.Items {
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}
	return result
}

// ArgoApplicationToDPUService ensures a DPUService is updated each time there is an update to the linked Argo Application.
func (r *DPUServiceReconciler) ArgoApplicationToDPUService(ctx context.Context, o client.Object) []ctrl.Request {
	namespace, ok := o.GetLabels()[dpuservicev1.DPUServiceNamespaceLabelKey]
	if !ok {
		return nil
	}
	name, ok := o.GetLabels()[dpuservicev1.DPUServiceNameLabelKey]
	if !ok {
		return nil
	}
	return []ctrl.Request{{
		NamespacedName: client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		},
	}}
}

// isDPUServiceInterfaceInUse checks if the DPUServiceInterface is in use by another DPUService.
// Returns true if the DPUServiceInterface is in use, and true if the DPUServiceInterface is found.
func isDPUServiceInterfaceInUse(dpuServiceInterface *sfcv1.DPUServiceInterface, dpuService *dpuservicev1.DPUService) (inUse bool, exist bool) {
	annotations := dpuServiceInterface.GetAnnotations()
	if annotations != nil {
		_, ok := annotations[dpuservicev1.DPUServiceInterfaceAnnotationKey]
		if ok {
			exist = true
		}
		if ok && annotations[dpuservicev1.DPUServiceInterfaceAnnotationKey] != dpuService.Name {
			inUse = true
			return
		}
	}
	return
}
