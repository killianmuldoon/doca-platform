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
	"strings"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/operator/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/argocd"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/argocd/api/application/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane/kubeconfig"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane/metadata"
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

// +kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservices/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims;events;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=create
// +kubebuilder:rbac:groups=argoproj.io,resources=appprojects;applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=get;list;watch

const (
	// TODO: These constants don't belong here and should be moved as they're shared with other packages.
	argoCDSecretLabelKey   = "argocd.argoproj.io/secret-type"
	argoCDSecretLabelValue = "cluster"
	dpuAppProjectName      = "doca-platform-project-dpu"
	hostAppProjectName     = "doca-platform-project-host"

	dpfServiceIDLabelKey = "sfc.nvidia.com/service"
	// The ArgoCD namespace will always be the same as that where the reconciler is deployed.
	// TODO:Figure out a way to make this dynamic while preserving the e2e tests.
	argoCDNamespace = "dpf-operator-system"

	conditionMessageErroredTemplate             = "Unable to reconcile: %v"
	conditionMessageAppNotReadyTemplate         = "The following applications are not ready: %v"
	conditionMessageAppAwaitingDeletionTemplate = "The following applications are awaiting deletion: %v"
)

const (
	dpuServiceControllerName = "dpuservice-manager"
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
		// TODO: Consider also watching on Application reconcile. This was very noisy when initially tried.
		For(&dpuservicev1.DPUService{}).
		// TODO: This doesn't currently work for status updates - need to find a way to increase reconciliation frequency.
		WatchesMetadata(tenantControlPlane, handler.EnqueueRequestsFromMapFunc(r.DPUClusterToDPUService)).
		Complete(r)
}

// Reconcile reconciles changes in a DPUService.
func (r *DPUServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
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

	conditions.EnsureConditions(dpuService, dpuservicev1.AllConditions)
	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {
		log.Info("Patching")
		if err := r.summary(ctx, dpuService); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
		if err := patcher.Patch(ctx, dpuService, patch.WithFieldOwner(dpuServiceControllerName)); err != nil {
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
	// TODO: add deletion for Argo Secrets, AppProject and ImagePullSecrets. Also add AwaitingDeletion conditions.
	log.Info("handling DPUService deletion")

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
		message := fmt.Sprintf(conditionMessageAppAwaitingDeletionTemplate, strings.Join(appsInDeletion, ", "))
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

	// Add AwaitingDeletion condition to true. Probably this will never going to apply on the DPUService as it will be
	// deleted in the next step anyway.
	conditions.AddTrue(dpuService, dpuservicev1.ConditionApplicationsReconciled)

	// If there are no associated applications remove the finalizer.
	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpuService, dpuservicev1.DPUServiceFinalizer)
	return ctrl.Result{}, nil
}

func (r *DPUServiceReconciler) reconcile(ctx context.Context, dpuService *dpuservicev1.DPUService) error {
	// Get the list of clusters this DPUService targets.
	// TODO: Add some way to check if the clusters are healthy. Reconciler should retry clusters if they're unready.
	clusters, err := controlplane.GetDPFClusters(ctx, r.Client)
	if err != nil {
		return err
	}

	if err := r.reconcileApplicationPrereqs(ctx, dpuService, clusters); err != nil {
		message := fmt.Sprintf(conditionMessageErroredTemplate, err.Error())
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
	if err = r.reconcileApplication(ctx, argoCDNamespace, clusters, dpuService); err != nil {
		err = fmt.Errorf("Application: %w", err)
		message := fmt.Sprintf(conditionMessageErroredTemplate, err)
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

func (r *DPUServiceReconciler) reconcileApplicationPrereqs(ctx context.Context, dpuService *dpuservicev1.DPUService, clusters []controlplane.DPFCluster) error {
	// Ensure the DPUService namespace exists in target clusters.
	project := getProjectName(dpuService)
	if err := r.ensureNamespaces(ctx, clusters, project); err != nil {
		return fmt.Errorf("Cluster namespaces: %v", err)
	}

	// Ensure the Argo secret for each cluster is up-to-date.
	if err := r.reconcileArgoSecrets(ctx, clusters); err != nil {
		return fmt.Errorf("ArgoSecrets: %v", err)
	}

	//  Ensure the ArgoCD AppProject exists and is up-to-date.
	if err := r.reconcileAppProject(ctx, argoCDNamespace, clusters); err != nil {
		return fmt.Errorf("AppProject: %v", err)
	}

	// Reconcile the ImagePullSecrets.
	if err := r.reconcileImagePullSecrets(ctx, clusters, dpuService); err != nil {
		return fmt.Errorf("ImagePullSecrets: %v", err)
	}

	return nil
}

func (r *DPUServiceReconciler) ensureNamespaces(ctx context.Context, clusters []controlplane.DPFCluster, project string) error {
	var errs []error
	if project == dpuAppProjectName {
		for _, cluster := range clusters {
			dpuClusterClient, err := cluster.NewClient(ctx, r.Client)
			if err != nil {
				errs = append(errs, err)
			}
			if err := utils.EnsureNamespace(ctx, dpuClusterClient, argoCDNamespace); err != nil {
				errs = append(errs, err)
			}
		}
	} else {
		if err := utils.EnsureNamespace(ctx, r.Client, argoCDNamespace); err != nil {
			return fmt.Errorf("In-Cluster namespace: %v", err)
		}
	}
	return kerrors.NewAggregate(errs)
}

// reconcileArgoSecrets reconciles a Secret in the format that ArgoCD expects. It uses data from the control plane secret.
func (r *DPUServiceReconciler) reconcileArgoSecrets(ctx context.Context, clusters []controlplane.DPFCluster) error {
	log := ctrllog.FromContext(ctx)

	var errs []error
	for i := range clusters {
		cluster := clusters[i]
		// Get the control plane kubeconfig
		adminConfig, err := cluster.GetKubeconfig(ctx, r.Client)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// Template an argoSecret using information from the control plane secret.
		argoSecret, err := createArgoSecretFromKubeconfig(argoCDNamespace, cluster, adminConfig)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// Create or patch
		log.Info("Patching Secrets for DPF clusters")
		if err := r.Client.Patch(ctx, argoSecret, client.Apply, applyPatchOptions...); err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return kerrors.NewAggregate(errs)
}

// createArgoSecretFromKubeconfig generates an ArgoCD cluster secret from the given kubeconfig.
func createArgoSecretFromKubeconfig(argoCDNamespace string, cluster controlplane.DPFCluster, kubeconfig *kubeconfig.Type) (*corev1.Secret, error) {
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
	return createArgoCDSecret(argoCDNamespace, secretConfig, cluster, clusterConfigName, clusterConfigServer), nil
}

// createArgoCDSecret templates an ArgoCD cluster Secret with the passed values.
func createArgoCDSecret(argoCDNamespace string, secretConfig []byte, cluster controlplane.DPFCluster, clusterConfigName, clusterConfigServer string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			// The secret name is the cluster name. DPUClusters must have unique names.
			Name:      cluster.String(),
			Namespace: argoCDNamespace,
			Labels: map[string]string{
				argoCDSecretLabelKey:                argoCDSecretLabelValue,
				controlplanemeta.DPFClusterLabelKey: cluster.String(),
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

	outOfSyncApplications := []string{}
	// Get a summarized error for each application linked to the DPUService and set it as the condition message.
	for i := range applicationList.Items {
		app := applicationList.Items[i]
		if app.Status.Sync.Status != argov1.SyncStatusCodeSynced {
			argoApp := fmt.Sprintf("%s/%s.%s", app.Namespace, app.Name, app.GroupVersionKind().Group)
			outOfSyncApplications = append(outOfSyncApplications, argoApp)
		}
	}

	// Update condition and requeue if there are any errors, or if there are fewer applications than we have clusters.
	if len(outOfSyncApplications) > 0 || len(applicationList.Items) != len(clusters) {
		message := fmt.Sprintf(conditionMessageAppNotReadyTemplate, strings.Join(outOfSyncApplications, ", "))
		conditions.AddFalse(
			dpuService,
			dpuservicev1.ConditionApplicationsReady,
			conditions.ReasonPending,
			conditions.ConditionMessage(message),
		)
		// TODO: Make the DPUService controller react to changes in Applications
		return fmt.Errorf("Applications not ready. Requeuing")
	}
	conditions.AddTrue(dpuService, dpuservicev1.ConditionApplicationsReady)
	return nil
}

func (r *DPUServiceReconciler) reconcileAppProject(ctx context.Context, argoCDNamespace string, clusters []controlplane.DPFCluster) error {
	log := ctrllog.FromContext(ctx)

	clusterKeys := []types.NamespacedName{}
	for i := range clusters {
		clusterKeys = append(clusterKeys, types.NamespacedName{Namespace: clusters[i].Namespace, Name: clusters[i].Name})
	}
	dpuAppProject := argocd.NewAppProject(argoCDNamespace, dpuAppProjectName, clusterKeys)

	log.Info("Patching AppProject for DPU clusters")
	if err := r.Client.Patch(ctx, dpuAppProject, client.Apply, applyPatchOptions...); err != nil {
		return err
	}

	inClusterKey := []types.NamespacedName{{Namespace: "*", Name: "in-cluster"}}
	hostAppProject := argocd.NewAppProject(argoCDNamespace, hostAppProjectName, inClusterKey)

	log.Info("Patching AppProject for Host cluster")
	if err := r.Client.Patch(ctx, hostAppProject, client.Apply, applyPatchOptions...); err != nil {
		return err
	}

	return nil
}

func (r *DPUServiceReconciler) reconcileApplication(ctx context.Context, argoCDNamespace string, clusters []controlplane.DPFCluster, dpuService *dpuservicev1.DPUService) error {
	log := ctrllog.FromContext(ctx)

	project := getProjectName(dpuService)
	values, err := argoCDValuesFromDPUService(dpuService)
	if err != nil {
		return err
	}
	if project == dpuAppProjectName {
		for _, cluster := range clusters {
			clusterKey := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name}
			argoApplication := argocd.NewApplication(argoCDNamespace, project, clusterKey, dpuService, values)

			log.Info("Patching Application", "Application", klog.KObj(argoApplication))
			if err := r.Client.Patch(ctx, argoApplication, client.Apply, applyPatchOptions...); err != nil {
				return err
			}
		}
	} else {
		argoApplication := argocd.NewApplication(argoCDNamespace, project, types.NamespacedName{Namespace: "*", Name: "in-cluster"}, dpuService, values)
		log.Info("Patching Application", "Application", klog.KObj(argoApplication))
		if err := r.Client.Patch(ctx, argoApplication, client.Apply, applyPatchOptions...); err != nil {
			return err
		}
	}
	return nil
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
	if service.Spec.Values != nil {
		if err := json.Unmarshal(service.Spec.Values.Raw, &otherValues); err != nil {
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
	// Set the serviceDaemonSet values in the combined values.
	combinedValues := map[string]interface{}{}
	combinedValues["serviceDaemonSet"] = serviceDaemonSetValues
	// Add all keys from other values to the ServiceDaemonSet values.
	for k, v := range otherValues {
		combinedValues[k] = v
	}

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

func (r *DPUServiceReconciler) reconcileImagePullSecrets(ctx context.Context, clusters []controlplane.DPFCluster, service *dpuservicev1.DPUService) error {
	// First get the secrets with the correct label.
	secrets := &corev1.SecretList{}
	err := r.List(ctx, secrets, client.HasLabels{dpuservicev1.DPFImagePullSecretLabelKey})
	if err != nil {
		return err
	}
	secretsToPatch := []*corev1.Secret{}
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
	}
	// Apply the new secret to every DPUCluster.
	var errs []error
	for _, cluster := range clusters {
		dpuClusterClient, err := cluster.NewClient(ctx, r.Client)
		if err != nil {
			errs = append(errs, err)
		}
		for _, secret := range secretsToPatch {
			if err := dpuClusterClient.Patch(ctx, secret, client.Apply, applyPatchOptions...); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return kerrors.NewAggregate(errs)
}
