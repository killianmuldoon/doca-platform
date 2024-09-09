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
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"

	"github.com/fluxcd/pkg/runtime/patch"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	DPUServiceCredentialRequestControllerName = "dpuservicecredentialrequestcontroller"
)

// applyDPUServiceCredentialRequestPatchOptions contains options which are passed to every `client.Apply` patch.
var applyDPUServiceCredentialRequestPatchOptions = []client.PatchOption{
	client.ForceOwnership,
	client.FieldOwner(DPUServiceCredentialRequestControllerName),
}

// DPUServiceCredentialRequestReconciler reconciles a DPUServiceCredentialRequest object
type DPUServiceCredentialRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservicecredentialrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservicecredentialrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=svc.dpf.nvidia.com,resources=dpuservicecredentialrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=serviceaccounts;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts/token,verbs=create
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// SetupWithManager sets up the controller with the Manager.
func (r *DPUServiceCredentialRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dpuservicev1.DPUServiceCredentialRequest{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}),
		)).
		Complete(r)
}

func (r *DPUServiceCredentialRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")
	obj := &dpuservicev1.DPUServiceCredentialRequest{}
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	patcher := patch.NewSerialPatcher(obj, r.Client)

	defer func() {
		log.Info("Patching")
		// Set the summary condition for the DPUServiceCredentialRequest.
		conditions.SetSummary(obj)
		if err := patcher.Patch(ctx, obj,
			patch.WithFieldOwner(DPUServiceCredentialRequestControllerName),
			patch.WithStatusObservedGeneration{},
			patch.WithOwnedConditions{Conditions: conditions.TypesAsStrings(dpuservicev1.DPUCredentialRequestConditions)},
		); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deletion reconciliation loop.
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, obj)
	}

	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(obj, dpuservicev1.DPUServiceCredentialRequestFinalizer) {
		controllerutil.AddFinalizer(obj, dpuservicev1.DPUServiceCredentialRequestFinalizer)
		return ctrl.Result{Requeue: true}, nil
	}

	return r.reconcile(ctx, obj)
}

func (r *DPUServiceCredentialRequestReconciler) reconcile(ctx context.Context, obj *dpuservicev1.DPUServiceCredentialRequest) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	targetClient := r.Client
	var cluster *controlplane.DPFCluster
	if obj.Spec.TargetClusterName != nil {
		c, cl, err := r.getCluster(ctx, *obj.Spec.TargetClusterName)
		if err != nil {
			conditions.AddFalse(
				obj,
				dpuservicev1.ConditionServiceAccountReconciled,
				conditions.ReasonError,
				conditions.ConditionMessage(fmt.Sprintf("Error occurred: %v", err)),
			)
			return ctrl.Result{}, err
		}
		targetClient = c
		cluster = cl
	}

	tr, err := reconcileServiceAccount(ctx, obj, targetClient)
	if err != nil {
		conditions.AddFalse(
			obj,
			dpuservicev1.ConditionServiceAccountReconciled,
			conditions.ReasonError,
			conditions.ConditionMessage(fmt.Sprintf("Error occurred: %v", err)),
		)
		return ctrl.Result{}, err
	}

	conditions.AddTrue(obj, dpuservicev1.ConditionServiceAccountReconciled)

	// Update the status of the DPUServiceCredentialRequest.
	obj.Status.ServiceAccount = ptr.To(obj.Spec.ServiceAccount.String())
	obj.Status.TargetClusterName = obj.Spec.TargetClusterName

	var token string
	if tr != nil {
		exp := tr.Status.ExpirationTimestamp.Time
		// calculate the issued at time based on the expiration time in the current
		// generation of the DPUServiceCredentialRequest.
		iat := exp.Add(-1 * time.Duration(*tr.Spec.ExpirationSeconds) * time.Second)
		obj.Status.ExpirationTimestamp = &metav1.Time{Time: exp}
		obj.Status.IssuedAt = &metav1.Time{Time: iat}
		token = tr.Status.Token
	}

	if err = r.reconcileSecret(ctx, obj, cluster, token); err != nil {
		conditions.AddFalse(
			obj,
			dpuservicev1.ConditionSecretReconciled,
			conditions.ReasonError,
			conditions.ConditionMessage(fmt.Sprintf("Error occurred: %v", err)),
		)
		return ctrl.Result{}, err
	}

	conditions.AddTrue(obj, dpuservicev1.ConditionSecretReconciled)

	obj.Status.Secret = ptr.To(obj.Spec.Secret.String())

	// Requeue the object before the token expires.
	requeueAfter, err := calculateNextRequeueTime(obj.Status.ExpirationTimestamp.Time, obj.Status.IssuedAt.Time)
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("Requeueing before token expiration", "requeueAfter", requeueAfter)
	return ctrl.Result{RequeueAfter: time.Until(requeueAfter)}, nil
}

// reconcileServiceAccount reconciles the ServiceAccount for the DPUServiceCredentialRequest.
// It returns the TokenRequest if the reconciliation was successful.
func reconcileServiceAccount(ctx context.Context, obj *dpuservicev1.DPUServiceCredentialRequest, targetClient client.Client) (*authenticationv1.TokenRequest, error) {
	log := ctrllog.FromContext(ctx)

	// The ServiceAccount name and/or namespace diverges or the DPUServiceCredentialRequest
	// is being deleted, delete the ServiceAccount.
	if (obj.Status.ServiceAccount != nil && *obj.Status.ServiceAccount != obj.Spec.ServiceAccount.String()) || !obj.DeletionTimestamp.IsZero() {
		// If the DPUServiceCredentialRequest is being deleted, we need to short-circuit
		// to avoid recreating the ServiceAccount.
		if err := deleteServiceAccount(ctx, obj, targetClient); err != nil || !obj.DeletionTimestamp.IsZero() {
			return nil, err
		}
	}

	// The target cluster name diverges, delete the ServiceAccount if it exists.
	if !equalStrings(obj.Status.TargetClusterName, obj.Spec.TargetClusterName) && obj.Status.ServiceAccount != nil {
		if err := deleteServiceAccount(ctx, obj, targetClient); err != nil {
			return nil, err
		}
	}

	// If a valid ExpirationTimestamp exists and the token is not expiring, return early.
	if obj.Status.ExpirationTimestamp != nil && !requiresRefresh(obj.Status.ExpirationTimestamp.Time, obj.Status.IssuedAt.Time) {
		if obj.Spec.Duration == nil {
			log.Info("ServiceAccount already exists with a valid token", "namespace", obj.Spec.ServiceAccount.GetNamespace(), "name", obj.Spec.ServiceAccount.Name)
			return nil, nil
		}
		// we should requeue the object before the token expires if the duration has not changed
		if expirationSeconds := getExpirationSeconds(obj.Status.ExpirationTimestamp.Time, obj.Status.IssuedAt.Time); expirationSeconds == int64(obj.Spec.Duration.Duration/time.Second) {
			log.Info("ServiceAccount already exists with a valid token", "namespace", obj.Spec.ServiceAccount.GetNamespace(), "name", obj.Spec.ServiceAccount.Name)
			return nil, nil
		}
	}

	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.Spec.ServiceAccount.Name,
			Namespace: obj.Spec.ServiceAccount.GetNamespace(),
		},
		// Disable automounting of the ServiceAccount token as this SA is not
		// intended to be used by pods.
		AutomountServiceAccountToken: ptr.To(false),
	}

	tr := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences: []string{"https://kubernetes.default.svc"},
		},
	}
	if obj.Spec.Duration != nil {
		tr.Spec.ExpirationSeconds = ptr.To(int64(obj.Spec.Duration.Duration / time.Second))
	}

	if err := targetClient.Patch(ctx, sa, client.Apply, applyDPUServiceCredentialRequestPatchOptions...); err != nil {
		return nil, fmt.Errorf("error while patching ServiceAccount on target: %w", err)
	}

	err := targetClient.SubResource("token").Create(ctx, sa, tr, &client.SubResourceCreateOptions{CreateOptions: client.CreateOptions{FieldManager: DPUServiceCredentialRequestControllerName}})
	if err != nil {
		return nil, err
	}

	log.Info("ServiceAccount created", "namespace", sa.GetNamespace(), "name", sa.GetName())

	return tr, nil
}

func (r *DPUServiceCredentialRequestReconciler) reconcileSecret(ctx context.Context, obj *dpuservicev1.DPUServiceCredentialRequest, cluster *controlplane.DPFCluster, token string) error {
	var config map[string][]byte
	// The Secret name and/or namespace diverges or the DPUServiceCredentialRequest
	// is being deleted, delete the Secret.
	if (obj.Status.Secret != nil && *obj.Status.Secret != obj.Spec.Secret.String()) || !obj.DeletionTimestamp.IsZero() {
		data, err := r.deleteSecret(ctx, obj)
		// If the DPUServiceCredentialRequest is being deleted, we need to short-circuit
		// to avoid recreating the Secret.
		if err != nil || !obj.DeletionTimestamp.IsZero() {
			return err
		}
		// if the secret was deleted but the DPUServiceCredentialRequest is not being deleted,
		// we have to recreate the secret with a new name but same data
		config = data
	}

	// if we have a token, create a kubeconfig from it
	if token != "" {
		data, err := r.createKubeconfigWithToken(ctx, obj, cluster, token)
		if err != nil {
			return err
		}
		config = map[string][]byte{
			"kubeconfig": data,
		}
	}

	return r.patchSecret(ctx, obj, config)
}

func (r *DPUServiceCredentialRequestReconciler) getCluster(ctx context.Context, clusterName string) (client.Client, *controlplane.DPFCluster, error) {
	clusters, err := controlplane.GetDPFClusters(ctx, r.Client)
	if err != nil {
		return nil, nil, fmt.Errorf("error while getting DPF clusters: %w", err)
	}
	if len(clusters) == 0 {
		return nil, nil, fmt.Errorf("no cluster found")
	}

	var cluster controlplane.DPFCluster
	for _, cl := range clusters {
		if cl.Name == clusterName {
			cluster = cl
			break
		}
	}
	if cluster.Name == "" {
		return nil, nil, fmt.Errorf("cluster %v not found", clusterName)
	}

	client, err := cluster.NewClient(ctx, r.Client)
	if err != nil {
		return nil, nil, fmt.Errorf("error while getting client for cluster %v: %w", cluster.Name, err)
	}

	return client, &cluster, nil
}

// createKubeconfigWithToken creates a kubeconfig with the given token.
func (r *DPUServiceCredentialRequestReconciler) createKubeconfigWithToken(ctx context.Context, obj *dpuservicev1.DPUServiceCredentialRequest, cluster *controlplane.DPFCluster, token string) ([]byte, error) {
	var (
		clusterName string
		server      string
		caData      []byte
	)
	if cluster != nil {
		kubeConfig, err := cluster.GetKubeconfig(ctx, r.Client)
		if err != nil {
			return nil, fmt.Errorf("error while getting kubeconfig for cluster %v: %w", cluster.Name, err)
		}
		// retrieve cluster related info from kubeConfig
		clusterName = kubeConfig.Clusters[0].Name
		server = kubeConfig.Clusters[0].Cluster.Server
		caData = kubeConfig.Clusters[0].Cluster.CertificateAuthorityData
	} else {
		// retrieve CA from the current pod SA
		caBytes, url, err := getClusterAccessData()
		if err != nil {
			return nil, fmt.Errorf("error while getting CA from the current pod: %w", err)
		}
		clusterName = "incluster"
		server = url
		caData = caBytes
	}

	// context and user are the same as the ServiceAccount name
	context := obj.Spec.ServiceAccount.Name
	user := obj.Spec.ServiceAccount.Name

	config := clientcmdapi.NewConfig()
	config.Clusters[clusterName] = &clientcmdapi.Cluster{
		Server:                   server,
		CertificateAuthorityData: caData,
	}

	config.Contexts[context] = &clientcmdapi.Context{
		Cluster:   clusterName,
		Namespace: obj.Spec.ServiceAccount.GetNamespace(),
		AuthInfo:  user,
	}
	config.CurrentContext = context
	config.AuthInfos[user] = &clientcmdapi.AuthInfo{
		Token: token,
	}

	data, err := clientcmd.Write(*config)
	if err != nil {
		return nil, fmt.Errorf("error while writing kubeconfig: %w", err)
	}

	return data, nil
}

// patchSecret creates a secret with the given config.
func (r *DPUServiceCredentialRequestReconciler) patchSecret(ctx context.Context, obj *dpuservicev1.DPUServiceCredentialRequest, config map[string][]byte) error {
	log := ctrllog.FromContext(ctx)

	var previousData map[string][]byte
	if obj.Status.Secret != nil {
		// If the Secret already exists, get the current data to avoid overwriting it.
		secret := corev1.Secret{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: obj.Spec.Secret.Name, Namespace: obj.Spec.Secret.GetNamespace()}, &secret)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error while getting Secret %s: %w", *obj.Status.Secret, err)
		}
		if secret.Data != nil {
			previousData = secret.Data
		}
	}

	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.Spec.Secret.Name,
			Namespace: obj.Spec.Secret.GetNamespace(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: previousData,
	}

	if config != nil {
		secret.Data = config
	}

	if obj.Spec.ObjectMeta != nil {
		secret.ObjectMeta.Labels = obj.Spec.ObjectMeta.Labels
		secret.ObjectMeta.Annotations = obj.Spec.ObjectMeta.Annotations
	}

	if err := r.Client.Patch(ctx, &secret, client.Apply, applyDPUServiceCredentialRequestPatchOptions...); err != nil {
		return err
	}
	log.Info("Secret patched", "namespace", obj.Spec.Secret.GetNamespace(), "name", obj.Spec.Secret.Name)
	return nil
}

func (r *DPUServiceCredentialRequestReconciler) reconcileDelete(ctx context.Context, obj *dpuservicev1.DPUServiceCredentialRequest) (ctrl.Result, error) {
	targetClient := r.Client
	if obj.Status.TargetClusterName != nil {
		c, _, err := r.getCluster(ctx, *obj.Status.TargetClusterName)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error while getting client for cluster %v: %w", *obj.Status.TargetClusterName, err)
		}
		targetClient = c
	}

	if _, err := reconcileServiceAccount(ctx, obj, targetClient); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileSecret(ctx, obj, nil, ""); err != nil {
		return ctrl.Result{}, err
	}

	if !obj.DeletionTimestamp.IsZero() {
		// Remove our finalizer from the object.
		controllerutil.RemoveFinalizer(obj, dpuservicev1.DPUServiceCredentialRequestFinalizer)

		// Stop reconciliation as the object is being deleted.
		return ctrl.Result{}, nil
	}

	// Requeue here to reconcile dependencies.
	return ctrl.Result{Requeue: true}, nil
}

func deleteServiceAccount(ctx context.Context, obj *dpuservicev1.DPUServiceCredentialRequest, targetClient client.Client) error {
	log := ctrllog.FromContext(ctx)
	if obj.Status.ServiceAccount != nil {
		ns, name := obj.Status.GetServiceAccount()
		namespacedName := types.NamespacedName{Namespace: ns, Name: name}
		var sa corev1.ServiceAccount
		err := targetClient.Get(ctx, namespacedName, &sa)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error while getting ServiceAccount %s: %w", namespacedName, err)
		}
		if err == nil {
			if err := targetClient.Delete(ctx, &sa); err != nil {
				return fmt.Errorf("error while deleting ServiceAccount %s: %w", namespacedName, err)
			}
		}
		log.Info("ServiceAccount deleted", "namespace", ns, "name", name)

		// Clear the ServiceAccount related fields in the status.
		obj.Status.ServiceAccount = nil
		obj.Status.TargetClusterName = nil
		obj.Status.ExpirationTimestamp = nil
		obj.Status.IssuedAt = nil
	}

	return nil
}

// deleteSecret deletes the Secret referenced in the status.
// It returns the data of the deleted Secret if it exists.
func (r *DPUServiceCredentialRequestReconciler) deleteSecret(ctx context.Context, obj *dpuservicev1.DPUServiceCredentialRequest) (map[string][]byte, error) {
	log := ctrllog.FromContext(ctx)

	var data map[string][]byte
	if obj.Status.Secret != nil {
		ns, name := obj.Status.GetSecret()
		namespacedName := types.NamespacedName{Namespace: ns, Name: name}
		var secret corev1.Secret
		err := r.Client.Get(ctx, namespacedName, &secret)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("error while getting Sercret %s: %w", namespacedName, err)
		}
		data = secret.Data
		if err == nil {
			if err := r.Client.Delete(ctx, &secret); err != nil {
				return nil, fmt.Errorf("error while deleting Secret %s: %w", namespacedName, err)
			}
		}
		log.Info("Secret deleted", "namespace", ns, "name", name)

		// Clear the Secret reference in the status.
		obj.Status.Secret = nil
	}

	return data, nil
}

// getClusterAccessData returns the CA data and the server URL for the current cluster.
// It relies on the CA being mounted in the pod at /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
// and the KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT environment variables being set.
func getClusterAccessData() ([]byte, string, error) {
	rootCAFile := "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	rootCAs := x509.NewCertPool()
	caBytes, err := os.ReadFile(rootCAFile)
	if err != nil {
		return nil, "", fmt.Errorf("error while reading root CA from %s: %w", rootCAFile, err)
	}
	if ok := rootCAs.AppendCertsFromPEM(caBytes); !ok {
		return nil, "", fmt.Errorf("error while appending root CA from %s: %w", rootCAFile, err)
	}

	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, "", fmt.Errorf("KUBERNETES_SERVICE_HOST or KUBERNETES_SERVICE_PORT not set")
	}
	return caBytes, "https://" + net.JoinHostPort(host, port), nil
}

// equalStrings compares two strings pointers and returns true if they are equal.
func equalStrings(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}
