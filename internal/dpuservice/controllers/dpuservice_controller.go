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

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application/v1alpha1"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane/metadata"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/dpuservice/kubeconfig"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"
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
// +kubebuilder:rbac:groups=argoproj.io,resources=appprojects;applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=get;list;watch

const (
	// TODO: These constants don't belong here and should be moved as they're shared with other packages.
	argoCDNamespace        = "default"
	argoCDSecretLabelKey   = "argocd.argoproj.io/secret-type"
	argoCDSecretLabelValue = "cluster"
	appProjectName         = "doca-platform-project"
	dpfServiceIDLabelKey   = "dpf.nvidia.com/service-id"
)

const (
	dpuServiceControllerName = "dpuservice-manager"
)

// dpfCluster represents a single Kubernetes cluster in DPF.
// TODO: Consider if this should be a more complex type carrying more data about the cluster.
// TODO: Consider making this part of a shared metadata package.
type dpfCluster types.NamespacedName

func (c *dpfCluster) String() string {
	return fmt.Sprintf("%s-%s", c.Namespace, c.Name)
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

	//original := dpuService.DeepCopy()
	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {
		// TODO: Make this a generic patcher.
		// TODO: There is an issue patching status here with SSA - the finalizer managed field becomes broken and the finalizer can not be removed. Investigate.
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

//nolint:unparam
func (r *DPUServiceReconciler) reconcileDelete(ctx context.Context, dpuService *dpuservicev1.DPUService) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpuService, dpuservicev1.DPUServiceFinalizer)
	// We should have an ownerReference chain in order to delete subordinate objects.
	return ctrl.Result{}, nil
}

func (r *DPUServiceReconciler) reconcile(ctx context.Context, dpuService *dpuservicev1.DPUService) (ctrl.Result, error) {
	var res ctrl.Result
	// Get the list of clusters this DPUService targets.
	// TODO: Add some way to check if the clusters are healthy. Reconciler should retry clusters if they're unready.
	clusters, err := getClusters(ctx, r.Client)
	if err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return ctrl.Result{}, err
	}

	// Ensure the Argo secret for each cluster is up-to-date.
	if err = r.reconcileSecrets(ctx, clusters); err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return ctrl.Result{}, err
	}

	//  Ensure the ArgoCD AppProject exists and is up-to-date.
	if err = r.reconcileAppProject(ctx, clusters); err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return ctrl.Result{}, err
	}

	// Update the ArgoApplication for all target clusters.
	if err = r.reconcileApplication(ctx, clusters, dpuService); err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return ctrl.Result{}, err
	}

	// Update the status of the DPUService.
	if res, err = r.reconcileStatus(ctx, dpuService, clusters); err != nil {
		return ctrl.Result{}, err
	}
	return res, nil
}

// reconcileSecrets reconciles a Secret in the format that ArgoCD expects. It uses data from the control plane secret.
func (r *DPUServiceReconciler) reconcileSecrets(ctx context.Context, clusters []dpfCluster) error {
	var errs []error
	for i := range clusters {
		cluster := clusters[i]
		// Get the control plane secret using the naming convention - $CLUSTERNAME-admin-kubeconfig.
		secret := &corev1.Secret{}
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: fmt.Sprintf("%v-admin-kubeconfig", cluster.Name)}
		if err := r.Client.Get(ctx, key, secret); err != nil {
			errs = append(errs, err)
			continue
		}
		// Template an argoSecret using information from the control plane secret.
		argoSecret, err := createArgoSecretFromControlPlaneSecret(secret, cluster)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		// Create or patch
		if err := r.Client.Patch(ctx, argoSecret, client.Apply, client.ForceOwnership, client.FieldOwner(dpuServiceControllerName)); err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return kerrors.NewAggregate(errs)
}

// createArgoSecretFromControlPlaneSecret generates an ArgoCD cluster secret from the control plane secret. This control plane secret is strictly tied
// to the Kamaji implementation.
func createArgoSecretFromControlPlaneSecret(secret *corev1.Secret, cluster dpfCluster) (*corev1.Secret, error) {
	adminConfig, err := getKubeconfigFromControlPlaneSecret(secret)
	if err != nil {
		return nil, err
	}
	clusterConfigName := adminConfig.Clusters[0].Name
	clusterConfigServer := adminConfig.Clusters[0].Cluster.Server
	secretConfig, err := json.Marshal(config{TlsClientConfig: tlsClientConfig{
		CaData:   adminConfig.Clusters[0].Cluster.CertificateAuthorityData,
		KeyData:  adminConfig.Users[0].User.ClientKeyData,
		CertData: adminConfig.Users[0].User.ClientCertificateData,
	}})
	if err != nil {
		return nil, err
	}
	return createArgoCDSecret(secretConfig, cluster, clusterConfigName, clusterConfigServer), nil
}

// getKubeconfigFromControlPlaneSecret returns an admin Kubeconfig stored in a control plane secret.
func getKubeconfigFromControlPlaneSecret(secret *corev1.Secret) (*kubeconfig.Type, error) {
	adminSecret, ok := secret.Data["admin.conf"]
	if !ok {
		return nil, fmt.Errorf("secret %v/%v not in the expected format: data.admin.conf not found", secret.Namespace, secret.Name)
	}
	var adminConfig kubeconfig.Type
	if err := yaml.Unmarshal(adminSecret, &adminConfig); err != nil {
		return nil, err
	}
	if len(adminConfig.Users) != 1 {
		return nil, fmt.Errorf("secret %v/%v not in the expected format: user list should have one member", secret.Namespace, secret.Name)
	}
	if len(adminConfig.Clusters) != 1 {
		return nil, fmt.Errorf("secret %v/%v not in the expected format: cluster list should have one member", secret.Namespace, secret.Name)
	}
	return &adminConfig, nil
}

// createArgoCDSecret templates an ArgoCD cluster Secret with the passed values.
func createArgoCDSecret(secretConfig []byte, cluster dpfCluster, clusterConfigName, clusterConfigServer string) *corev1.Secret {
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

// reconcileStatus returns a reconcile result representing whether ArgoCD Applications have been fully synced.
// TODO: This function should update status with errors representing the state of the application.
func (r *DPUServiceReconciler) reconcileStatus(ctx context.Context, dpuService *dpuservicev1.DPUService, clusters []dpfCluster) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	applicationList := &argov1.ApplicationList{}
	var errs []error
	// List all of the applications matching this DPUService.
	// TODO: Optimize list calls by adding indexes to the reconciler where needed.
	if err := r.Client.List(ctx, applicationList, client.MatchingLabels{
		dpuservicev1.DPUServiceNameLabelKey:      dpuService.Name,
		dpuservicev1.DPUServiceNamespaceLabelKey: dpuService.Namespace,
	}); err != nil {
		return ctrl.Result{}, err
	}

	// Get a summarized status for each application linked to the DPUService.
	for i := range applicationList.Items {
		app := applicationList.Items[i]
		if app.Status.Sync.Status != argov1.SyncStatusCodeSynced {
			errs = append(errs, fmt.Errorf("application %s/%s not yet ready", app.Name, app.Namespace))
		}
	}

	// Requeue if there are any errors, or if there are fewer applications than we have clusters.
	if len(errs) > 0 || len(applicationList.Items) != len(clusters) {
		log.Info("Applications not ready. Requeuing")
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

func (r *DPUServiceReconciler) reconcileAppProject(ctx context.Context, clusters []dpfCluster) error {
	clusterKeys := []types.NamespacedName{}
	for i := range clusters {
		clusterKeys = append(clusterKeys, types.NamespacedName{Namespace: clusters[i].Namespace, Name: clusters[i].Name})
	}
	appProject := argocd.NewAppProject(appProjectName, clusterKeys)
	if err := r.Client.Patch(ctx, appProject, client.Apply, client.ForceOwnership, client.FieldOwner(dpuServiceControllerName)); err != nil {
		return err
	}
	return nil
}

func (r *DPUServiceReconciler) reconcileApplication(ctx context.Context, clusters []dpfCluster, dpuService *dpuservicev1.DPUService) error {
	var errs []error
	values, err := argoCDValuesFromDPUService(dpuService)
	if err != nil {
		return err
	}
	for _, cluster := range clusters {
		clusterKey := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name}
		argoApplication := argocd.NewApplication(appProjectName, clusterKey, dpuService, values)
		argoApplication.SetOwnerReferences(
			ensureOwnerRef(argoApplication.GetOwnerReferences(), metav1.NewControllerRef(dpuService, dpuservicev1.DPUServiceGroupVersionKind)),
		)
		if err := r.Client.Patch(ctx, argoApplication, client.Apply, client.ForceOwnership, client.FieldOwner(dpuServiceControllerName)); err != nil {
			return err
		}
	}
	return kerrors.NewAggregate(errs)

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

// getClusters returns a list of the Clusters ArgoCD should install to.
func getClusters(ctx context.Context, c client.Client) ([]dpfCluster, error) {
	var errs []error
	secrets := &corev1.SecretList{}
	err := c.List(ctx, secrets, client.MatchingLabels(controlplanemeta.DPFClusterSecretLabels))
	if err != nil {
		return nil, err
	}
	clusters := []dpfCluster{}
	for _, secret := range secrets.Items {
		clusterName, found := secret.GetLabels()[controlplanemeta.DPFClusterSecretClusterNameLabelKey]
		if !found {
			errs = append(errs, fmt.Errorf("could not identify cluster name for secret %v/%v", secret.Namespace, secret.Name))
			continue
		}
		clusters = append(clusters, dpfCluster{
			Namespace: secret.Namespace,
			Name:      clusterName,
		})
	}
	return clusters, kerrors.NewAggregate(errs)
}

// ensureOwnerRef makes sure the slice contains the passed OwnerReference.
func ensureOwnerRef(ownerReferences []metav1.OwnerReference, owner *metav1.OwnerReference) []metav1.OwnerReference {
	for index, r := range ownerReferences {
		if r.UID == owner.UID {
			// Update the ownerReference in place to ensure apiVersion is up-to-date.
			ownerReferences[index] = *owner
			return ownerReferences
		}
	}
	return append(ownerReferences, *owner)
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
