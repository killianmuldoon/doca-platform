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
	"errors"
	"fmt"
	"time"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"
	nvipamv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/nvipam/api/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// ParentDPUServiceIPAMNameLabel points to the name of the DPUServiceIPAM object that owns a resource in the DPU
	// cluster.
	ParentDPUServiceIPAMNameLabel = "dpf.nvidia.com/dpuserviceipam-name"
	// ParentDPUServiceIPAMNamespaceLabel points to the namespace of the DPUServiceIPAM object that owns a resource in
	// the DPU cluster.
	ParentDPUServiceIPAMNamespaceLabel = "dpf.nvidia.com/dpuserviceipam-namespace"
)

// DPUServiceIPAMReconciler reconciles a DPUServiceIPAM object
type DPUServiceIPAMReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var _ objectsInDPUClustersReconciler = &DPUServiceIPAMReconciler{}

const (
	dpuServiceIPAMControllerName = "dpuserviceipamcontroller"
)

//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuserviceipams,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuserviceipams/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=dpuserviceipams/finalizers,verbs=update
//+kubebuilder:rbac:groups=nv-ipam.nvidia.com,resources=ippools,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles changes in a DPUServiceIPAM object
func (r *DPUServiceIPAMReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")

	dpuServiceIPAM := &sfcv1.DPUServiceIPAM{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpuServiceIPAM); err != nil {
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
		dpuServiceIPAM.SetGroupVersionKind(sfcv1.DPUServiceIPAMGroupVersionKind)
		// Do not include manged fields in the patch call. This does not remove existing fields.
		dpuServiceIPAM.ObjectMeta.ManagedFields = nil
		err := r.Client.Patch(ctx, dpuServiceIPAM, client.Apply, client.ForceOwnership, client.FieldOwner(dpuServiceIPAMControllerName))
		reterr = kerrors.NewAggregate([]error{reterr, err})
	}()

	// Handle deletion reconciliation loop.
	if !dpuServiceIPAM.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, dpuServiceIPAM)
	}

	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(dpuServiceIPAM, sfcv1.DPUServiceIPAMFinalizer) {
		log.Info("Adding finalizer")
		controllerutil.AddFinalizer(dpuServiceIPAM, sfcv1.DPUServiceIPAMFinalizer)
		return ctrl.Result{}, nil
	}

	return r.reconcile(ctx, dpuServiceIPAM)
}

// reconcile handles the main reconciliation loop
//
//nolint:unparam
func (r *DPUServiceIPAMReconciler) reconcile(ctx context.Context, dpuServiceIPAM *sfcv1.DPUServiceIPAM) (ctrl.Result, error) {
	if err := validateDPUServiceIPAM(dpuServiceIPAM); err != nil {
		return ctrl.Result{}, fmt.Errorf("error while validating: %w", err)
	}
	if err := reconcileObjectsInDPUClusters(ctx, r, r.Client, dpuServiceIPAM); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// reconcileDelete handles the delete reconciliation loop
//
//nolint:unparam
func (r *DPUServiceIPAMReconciler) reconcileDelete(ctx context.Context, dpuServiceIPAM *sfcv1.DPUServiceIPAM) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling delete")
	if err := reconcileObjectDeletionInDPUClusters(ctx, r, r.Client, dpuServiceIPAM); err != nil {
		if errors.Is(err, &shouldRequeueError{}) {
			log.Info(fmt.Sprintf("Requeueing because %s", err.Error()))
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, fmt.Errorf("error while reconciling deletion of objects in DPU clusters: %w", err)
	}

	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpuServiceIPAM, sfcv1.DPUServiceIPAMFinalizer)
	return ctrl.Result{}, nil
}

// getObjectsInDPUCluster is the method called by the reconcileObjectDeletionInDPUClusters function which deletes
// objects in the DPU cluster related to the given parentObject. The implementation should get the created objects
// in the DPU cluster.
func (r *DPUServiceIPAMReconciler) getObjectsInDPUCluster(ctx context.Context, c client.Client, dpuObject client.Object) ([]unstructured.Unstructured, error) {
	pools := []unstructured.Unstructured{}
	for _, poolListType := range []string{nvipamv1.IPPoolListKind, nvipamv1.CIDRPoolListKind} {
		p := &unstructured.UnstructuredList{}
		p.SetGroupVersionKind(nvipamv1.GroupVersion.WithKind(poolListType))
		if err := c.List(ctx, p, client.InNamespace(dpuObject.GetNamespace()), client.MatchingLabels{
			ParentDPUServiceIPAMNameLabel:      dpuObject.GetName(),
			ParentDPUServiceIPAMNamespaceLabel: dpuObject.GetNamespace(),
		}); err != nil {
			return nil, fmt.Errorf("error while listing %s as unstructured: %w", p.GetObjectKind().GroupVersionKind().String(), err)
		}

		pools = append(pools, p.Items...)
	}

	return pools, nil
}

// createOrUpdateChild is the method called by the reconcileObjectsInDPUClusters function which applies changes to the
// DPU clusters on DPUServiceIPAM object updates.
func (r *DPUServiceIPAMReconciler) createOrUpdateObjectsInDPUCluster(ctx context.Context, c client.Client, dpuObject client.Object) error {
	dpuServiceIPAM, ok := dpuObject.(*sfcv1.DPUServiceIPAM)
	if !ok {
		return errors.New("error converting input object to DPUServiceIPAM")
	}

	if dpuServiceIPAM.Spec.IPV4Subnet != nil {
		return reconcileIPPoolMode(ctx, c, dpuServiceIPAM)
	}

	return reconcileCIDRPoolMode(ctx, c, dpuServiceIPAM)
}

// deleteObjectsInDPUCluster is the method called by the reconcileObjectDeletionInDPUClusters function which deletes
// objects in the DPU cluster related to the deleted DPUServiceIPAM object.
func (r *DPUServiceIPAMReconciler) deleteObjectsInDPUCluster(ctx context.Context, c client.Client, dpuObject client.Object) error {
	dpuServiceIPAM, ok := dpuObject.(*sfcv1.DPUServiceIPAM)
	if !ok {
		return errors.New("error converting input object to DPUServiceIPAM")
	}

	for _, poolType := range []string{nvipamv1.IPPoolKind, nvipamv1.CIDRPoolKind} {
		if err := deleteDPUServiceOwnedPoolsOfType(ctx, c, dpuServiceIPAM, poolType); err != nil {
			return err
		}
	}

	return nil
}

// deleteDPUServiceOwnedPoolsOfType deletes all the objects owned by the given DPUServiceIPAM object
func deleteDPUServiceOwnedPoolsOfType(ctx context.Context, c client.Client, dpuServiceIPAM *sfcv1.DPUServiceIPAM, poolType string) error {
	p := &unstructured.Unstructured{}
	p.SetGroupVersionKind(nvipamv1.GroupVersion.WithKind(poolType))
	if err := c.DeleteAllOf(ctx, p, client.InNamespace(dpuServiceIPAM.Namespace), client.MatchingLabels{
		ParentDPUServiceIPAMNameLabel:      dpuServiceIPAM.Name,
		ParentDPUServiceIPAMNamespaceLabel: dpuServiceIPAM.Namespace,
	}); err != nil {
		return fmt.Errorf("error while removing all %s: %w", p.GetObjectKind().GroupVersionKind().String(), err)
	}

	return nil
}

// reconcileIPPoolMode reconciles NVIPAM IPPool object and removes any leftover CIDRPool
func reconcileIPPoolMode(ctx context.Context, c client.Client, dpuServiceIPAM *sfcv1.DPUServiceIPAM) error {
	pool := generateIPPool(dpuServiceIPAM)
	if err := c.Patch(ctx, pool, client.Apply, client.ForceOwnership, client.FieldOwner(dpuServiceChainControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", pool.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(pool), err)
	}

	// Delete any leftover CIDRPool in case the configuration has changed from specifying `.Spec.IPV4Network` to
	// specifying `.Spec.IPV4Subnet`.
	if err := deleteDPUServiceOwnedPoolsOfType(ctx, c, dpuServiceIPAM, nvipamv1.CIDRPoolKind); err != nil {
		return fmt.Errorf("error while removing potential leftover NVIPAM CRs: %w", err)
	}

	return nil
}

// reconcileCIDRPoolMode reconciles NVIPAM CIDRPool object and removes any leftover IPPool
func reconcileCIDRPoolMode(ctx context.Context, c client.Client, dpuServiceIPAM *sfcv1.DPUServiceIPAM) error {
	pool := generateCIDRPool(dpuServiceIPAM)
	if err := c.Patch(ctx, pool, client.Apply, client.ForceOwnership, client.FieldOwner(dpuServiceChainControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", pool.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(pool), err)
	}

	// Delete any leftover IPPool in case the configuration has changed from specifying `.Spec.IPV4Subnet` to
	// specifying `.Spec.IPV4Network`.
	if err := deleteDPUServiceOwnedPoolsOfType(ctx, c, dpuServiceIPAM, nvipamv1.IPPoolKind); err != nil {
		return fmt.Errorf("error while removing potential leftover NVIPAM CRs: %w", err)
	}

	return nil
}

// generateIPPool generates an IPPool object for the given dpuServiceIPAM
func generateIPPool(dpuServiceIPAM *sfcv1.DPUServiceIPAM) *nvipamv1.IPPool {
	pool := &nvipamv1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dpuServiceIPAM.Name,
			Namespace: dpuServiceIPAM.Namespace,
			Labels: map[string]string{
				ParentDPUServiceIPAMNameLabel:      dpuServiceIPAM.Name,
				ParentDPUServiceIPAMNamespaceLabel: dpuServiceIPAM.Namespace,
			},
		},
		Spec: nvipamv1.IPPoolSpec{
			Subnet:           dpuServiceIPAM.Spec.IPV4Subnet.Subnet,
			PerNodeBlockSize: dpuServiceIPAM.Spec.IPV4Subnet.PerNodeIPCount,
			Gateway:          dpuServiceIPAM.Spec.IPV4Subnet.Gateway,
			NodeSelector:     dpuServiceIPAM.Spec.NodeSelector,
		},
	}
	pool.ObjectMeta.ManagedFields = nil
	pool.SetGroupVersionKind(nvipamv1.GroupVersion.WithKind("IPPool"))
	return pool
}

// generateCIDRPool generates a CIDRPool object for the given dpuServiceIPAM
func generateCIDRPool(dpuServiceIPAM *sfcv1.DPUServiceIPAM) *nvipamv1.CIDRPool {
	pool := &nvipamv1.CIDRPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dpuServiceIPAM.Name,
			Namespace: dpuServiceIPAM.Namespace,
			Labels: map[string]string{
				ParentDPUServiceIPAMNameLabel:      dpuServiceIPAM.Name,
				ParentDPUServiceIPAMNamespaceLabel: dpuServiceIPAM.Namespace,
			},
		},
		Spec: nvipamv1.CIDRPoolSpec{
			CIDR:                 dpuServiceIPAM.Spec.IPV4Network.Network,
			GatewayIndex:         ptr.To[uint](dpuServiceIPAM.Spec.IPV4Network.GatewayIndex),
			PerNodeNetworkPrefix: dpuServiceIPAM.Spec.IPV4Network.PrefixSize,
			NodeSelector:         dpuServiceIPAM.Spec.NodeSelector,
		},
	}
	pool.ObjectMeta.ManagedFields = nil
	pool.SetGroupVersionKind(nvipamv1.GroupVersion.WithKind("CIDRPool"))
	return pool
}

func validateDPUServiceIPAM(dpuServiceIPAM *sfcv1.DPUServiceIPAM) error {
	// TODO: Move validation to webhook
	if dpuServiceIPAM.Spec.IPV4Network != nil && dpuServiceIPAM.Spec.IPV4Subnet != nil {
		return errors.New("DPUServiceIPAM should not specify both ipv4Network and ipv4Subnet")
	}

	// TODO: Move validation to webhook
	if dpuServiceIPAM.Spec.IPV4Network == nil && dpuServiceIPAM.Spec.IPV4Subnet == nil {
		return errors.New("DPUServiceIPAM should specify either ipv4Network or ipv4Subnet")
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DPUServiceIPAMReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sfcv1.DPUServiceIPAM{}).
		Complete(r)
}
