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
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/dpuservice/kubeconfig"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"
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

const (
	// TODO: These constants don't belong here and should be moved as they're shared with other packages.
	dpfClusterNameLabelKey = "dpf.nvidia.com/cluster-name"
	argoCDNamespace        = "default"
	argoCDSecretLabelKey   = "argocd.argoproj.io/secret-type"
	argoCDSecretLabelValue = "cluster"
)

const (
	dpuServiceControllerName = "dpuservice-manager"
)

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

//nolint:unparam
func (r *DPUServiceReconciler) reconcileDelete(ctx context.Context, dpuService *dpuservicev1.DPUService) (ctrl.Result, error) {
	controllerutil.RemoveFinalizer(dpuService, dpuservicev1.DPUServiceFinalizer)
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
	if err := r.reconcileSecrets(ctx, dpuService, clusters); err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return ctrl.Result{}, err
	}

	//  Ensure the ArgoCD project exists and is up-to-date.
	if err := r.reconcileArgoCDAppProject(ctx); err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return ctrl.Result{}, err
	}

	// Update the ArgoApplication for all target clusters.
	if err := r.reconcileArgoApplication(ctx, clusters, dpuService); err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return ctrl.Result{}, err
	}

	// Update the status of the DPUService.
	if err := r.reconcileStatus(ctx); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// reconcileSecrets reconciles a Secret in the format that ArgoCD expects. It uses data from the control plane secret.
func (r *DPUServiceReconciler) reconcileSecrets(ctx context.Context, dpuService *dpuservicev1.DPUService, clusters []types.NamespacedName) error {
	var errs []error
	for _, cluster := range clusters {
		// Get the control plane secret using the naming convention - $CLUSTERNAME-admin-kubeconfig.
		secret := &corev1.Secret{}
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: fmt.Sprintf("%v-admin-kubeconfig", cluster.Name)}
		if err := r.Client.Get(ctx, key, secret); err != nil {
			errs = append(errs, err)
			continue
		}
		// Template an argoSecret using information from the control plane secret.
		argoSecret, err := createArgoSecretFromControlPlaneSecret(secret, dpuService, cluster.Name)
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
func createArgoSecretFromControlPlaneSecret(secret *corev1.Secret, dpuService *dpuservicev1.DPUService, clusterName string) (*corev1.Secret, error) {
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
	return createArgoCDSecret(dpuService, secretConfig, clusterName, clusterConfigName, clusterConfigServer), nil
}

// createArgoCDSecret templates an ArgoCD cluster Secret with the passed values.
func createArgoCDSecret(dpuService *dpuservicev1.DPUService, secretConfig []byte, clusterName, clusterConfigName, clusterConfigServer string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: argoCDNamespace,
			Labels: map[string]string{
				argoCDSecretLabelKey:   argoCDSecretLabelValue,
				dpfClusterNameLabelKey: clusterName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         dpuservicev1.GroupVersion.String(),
					Kind:               dpuservicev1.DPUServiceKind,
					Name:               dpuService.Name,
					UID:                dpuService.UID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
			Annotations: nil,
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

// getClusters returns a list of the Clusters ArgoCD should install to.
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
	}
	return clusters, kerrors.NewAggregate(errs)
}
