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

package controller

import (
	"context"
	"fmt"

	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ovnKubernetesNamespace               = "openshift-ovn-kubernetes"
	ovnKubernetesDaemonsetName           = "ovnkube-node"
	networkOperatorNamespace             = "openshift-network-operator"
	networkOperatorDeploymentName        = "network-operator"
	clusterVersionOperatorNamespace      = "openshift-cluster-version"
	clusterVersionOperatorDeploymentName = "cluster-version-operator"
	nodeIdentityWebhookConfigurationName = "network-node-identity.openshift.io"
	controlPlaneNodeLabel                = "node-role.kubernetes.io/control-plane"
)

// DPFOperatorConfigReconciler reconciles a DPFOperatorConfig object
type DPFOperatorConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs/finalizers,verbs=update

const (
	dpfOperatorConfigControllerName = "dpfoperatorconfig-controller"
)

// Reconcile reconciles changes in a DPFOperatorConfig.
func (r *DPFOperatorConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")
	dpfOperatorConfig := &operatorv1.DPFOperatorConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpfOperatorConfig); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	//original := dpfOperatorConfig.DeepCopy()
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
		log.Info("Removing")
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
func (r *DPFOperatorConfigReconciler) reconcileDelete(ctx context.Context, dpfOperatorConfig *operatorv1.DPFOperatorConfig) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpfOperatorConfig, operatorv1.DPFOperatorConfigFinalizer)
	// We should have an ownerReference chain in order to delete subordinate objects.
	return ctrl.Result{}, nil
}

//nolint:unparam
func (r *DPFOperatorConfigReconciler) reconcile(ctx context.Context, dpfOperatorConfig *operatorv1.DPFOperatorConfig) (ctrl.Result, error) {
	// Ensure Custom OVN Kubernetes Deployment is done
	if err := r.reconcileCustomOVNKubernetesDeployment(ctx); err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileCustomOVNKubernetesDeployment ensures that custom OVN Kubernetes is deployed
func (r *DPFOperatorConfigReconciler) reconcileCustomOVNKubernetesDeployment(ctx context.Context) error {
	// Phase 1
	// - ensure cluster version operator is scaled down
	// - ensure network operator is scaled down
	// - ensure webhook is removed
	// - ensure OVN Kubernetes daemonset has different nodeSelector (i.e. point to control plane only)
	if err := r.ScaleDownOVNKubernetesComponents(ctx); err != nil {
		return fmt.Errorf("error while scaling down OVN Kubernetes components: %w", err)
	}
	// Phase 2
	// - ensure no original OVN Kubernetes pods runs on worker
	// - remove node annotation k8s.ovn.org/node-chassis-id (avoid removing again on next reconciliation loop, needs status)
	// - ensure DPU CNI Provisioner is deployed
	// - ensure Host CNI provisioner is deployed
	// - ensure both provisioners are ready and have more than 1 pods
	// Phase 3
	// - deploy custom OVN Kubernetes
	return nil
}

func (r *DPFOperatorConfigReconciler) ScaleDownOVNKubernetesComponents(ctx context.Context) error {
	var errs []error

	// Ensure cluster version operator is scaled down
	clusterVersionOperatorDeployment := &appsv1.Deployment{}
	key := client.ObjectKey{Namespace: clusterVersionOperatorNamespace, Name: clusterVersionOperatorDeploymentName}
	err := r.Client.Get(ctx, key, clusterVersionOperatorDeployment)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("error while getting %s: %w", key.String(), err))
		}
	} else {
		clusterVersionOperatorDeployment.Spec.Replicas = ptr.To[int32](0)
		clusterVersionOperatorDeployment.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		clusterVersionOperatorDeployment.ObjectMeta.ManagedFields = nil
		if err := r.Client.Patch(ctx, clusterVersionOperatorDeployment, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
			errs = append(errs, fmt.Errorf("error while patching %s: %w", key.String(), err))
		}
	}

	// Ensure network operator is scaled down
	networkOperatorDeployment := &appsv1.Deployment{}
	key = client.ObjectKey{Namespace: networkOperatorNamespace, Name: networkOperatorDeploymentName}
	err = r.Client.Get(ctx, key, networkOperatorDeployment)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("error while getting %s: %w", key.String(), err))
		}
	} else {
		networkOperatorDeployment.Spec.Replicas = ptr.To[int32](0)
		networkOperatorDeployment.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		networkOperatorDeployment.ObjectMeta.ManagedFields = nil
		if err := r.Client.Patch(ctx, networkOperatorDeployment, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
			errs = append(errs, fmt.Errorf("error while patching %s: %w", key.String(), err))
		}
	}

	// Ensure node identity webhook is removed
	nodeIdentityWebhookConfiguration := &admissionregistrationv1.ValidatingWebhookConfiguration{}
	key = client.ObjectKey{Name: nodeIdentityWebhookConfigurationName}
	err = r.Client.Get(ctx, key, nodeIdentityWebhookConfiguration)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("error while getting %s: %w", key.String(), err))
		}
	} else {
		if err := r.Client.Delete(ctx, nodeIdentityWebhookConfiguration); err != nil {
			errs = append(errs, fmt.Errorf("error while deleting %s: %w", key.String(), err))
		}
	}

	// Ensure OVN Kubernetes daemonset has different nodeSelector (i.e. point to control plane only)
	ovnKubernetesDaemonset := &appsv1.DaemonSet{}
	key = client.ObjectKey{Namespace: ovnKubernetesNamespace, Name: ovnKubernetesDaemonsetName}
	err = r.Client.Get(ctx, key, ovnKubernetesDaemonset)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("error while getting %s: %w", key.String(), err))
		}
	} else {
		affinity := &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      controlPlaneNodeLabel,
									Operator: corev1.NodeSelectorOpExists,
								},
							},
						},
					},
				},
			},
		}
		ovnKubernetesDaemonset.Spec.Template.Spec.Affinity = affinity
		ovnKubernetesDaemonset.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DaemonSet"))
		ovnKubernetesDaemonset.ObjectMeta.ManagedFields = nil
		if err := r.Client.Patch(ctx, ovnKubernetesDaemonset, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
			errs = append(errs, fmt.Errorf("error while patching %s: %w", key.String(), err))
		}
	}

	return kerrors.NewAggregate(errs)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DPFOperatorConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.DPFOperatorConfig{}).
		Complete(r)
}
