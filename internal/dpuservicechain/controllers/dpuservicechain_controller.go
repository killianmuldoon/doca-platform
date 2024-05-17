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

//nolint:dupl
package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane/metadata"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ objectsInDPUClustersReconciler = &DPUServiceChainReconciler{}

// DPUServiceChainReconciler reconciles a DPUServiceChain object
type DPUServiceChainReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	dpuServiceChainControllerName = "dpuservicechaincontroller"
)

// +kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuservicechains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuservicechains/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuservicechains/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events;secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=get;list;watch

// Reconcile reconciles changes in a DPUServiceChain.
func (r *DPUServiceChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")
	dpuServiceChain := &sfcv1.DPUServiceChain{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpuServiceChain); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// Handle deletion reconciliation loop.
	if !dpuServiceChain.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, dpuServiceChain)
	}
	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(dpuServiceChain, sfcv1.DPUServiceChainFinalizer) {
		controllerutil.AddFinalizer(dpuServiceChain, sfcv1.DPUServiceChainFinalizer)
		if err := r.Update(ctx, dpuServiceChain); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := reconcileObjectsInDPUClusters(ctx, r, r.Client, dpuServiceChain); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *DPUServiceChainReconciler) reconcileDelete(ctx context.Context, dpuServiceChain *sfcv1.DPUServiceChain) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling delete")
	if err := reconcileObjectDeletionInDPUClusters(ctx, r, r.Client, dpuServiceChain); err != nil {
		if errors.Is(err, &shouldRequeueError{}) {
			log.Info(fmt.Sprintf("Requeueing because %s", err.Error()))
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpuServiceChain, sfcv1.DPUServiceChainFinalizer)
	if err := r.Client.Update(ctx, dpuServiceChain); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DPUServiceChainReconciler) getObjectsInDPUCluster(ctx context.Context, k8sClient client.Client, dpuObject client.Object) ([]unstructured.Unstructured, error) {
	scs := &unstructured.Unstructured{}
	scs.SetGroupVersionKind(sfcv1.ServiceChainSetGroupVersionKind)
	key := client.ObjectKey{Namespace: dpuObject.GetNamespace(), Name: dpuObject.GetName()}
	err := k8sClient.Get(ctx, key, scs)
	if err != nil {
		return nil, fmt.Errorf("error while getting %s %s: %w", scs.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}

	return []unstructured.Unstructured{*scs}, nil
}

func (r *DPUServiceChainReconciler) createOrUpdateObjectsInDPUCluster(ctx context.Context, k8sClient client.Client, dpuObject client.Object) error {
	dpuServiceChain := dpuObject.(*sfcv1.DPUServiceChain)
	scs := &sfcv1.ServiceChainSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dpuServiceChain.Name,
			Namespace:   dpuServiceChain.Namespace,
			Labels:      dpuServiceChain.Spec.Template.Labels,
			Annotations: dpuServiceChain.Spec.Template.Annotations,
		},
		Spec: *dpuServiceChain.Spec.Template.Spec.DeepCopy(),
	}
	scs.ObjectMeta.ManagedFields = nil
	scs.SetGroupVersionKind(sfcv1.GroupVersion.WithKind("ServiceChainSet"))
	return k8sClient.Patch(ctx, scs, client.Apply, client.ForceOwnership, client.FieldOwner(dpuServiceChainControllerName))
}

func (r *DPUServiceChainReconciler) deleteObjectsInDPUCluster(ctx context.Context, k8sClient client.Client, dpuObject client.Object) error {
	dpuServiceChain := dpuObject.(*sfcv1.DPUServiceChain)
	scs := &sfcv1.ServiceChainSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dpuServiceChain.Name,
			Namespace: dpuServiceChain.Namespace,
		},
	}
	return k8sClient.Delete(ctx, scs)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DPUServiceChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	tenantControlPlane := &metav1.PartialObjectMetadata{}
	tenantControlPlane.SetGroupVersionKind(controlplanemeta.TenantControlPlaneGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfcv1.DPUServiceChain{}).
		// TODO: This doesn't currently work for status updates - need to find a way to increase reconciliation frequency.
		WatchesMetadata(tenantControlPlane, handler.EnqueueRequestsFromMapFunc(r.DPUClusterToDPUServiceChain)).
		Complete(r)
}

// DPUClusterToDPUServiceChain ensures all DPUServiceChains are updated each time there is an update to a DPUCluster.
func (r *DPUServiceChainReconciler) DPUClusterToDPUServiceChain(ctx context.Context, o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	dpuServiceList := &sfcv1.DPUServiceChainList{}
	if err := r.Client.List(ctx, dpuServiceList); err != nil {
		return nil
	}
	for _, m := range dpuServiceList.Items {
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}
	return result
}
