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

package dpu

import (
	"context"
	"fmt"
	"reflect"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/allocator"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/state"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// DPUControllerName is used when reporting events
const DPUControllerName = "dpu"

// DPUReconciler reconciles a DPU object
type DPUReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	util.DPUOptions
	Recorder  record.EventRecorder
	Allocator allocator.Allocator
}

//+kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpus,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpus/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpus/finalizers,verbs=update
//+kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpuflavors,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods;pods/exec;nodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=patch;update;delete;create
//+kubebuilder:rbac:groups=maintenance.nvidia.com,resources=nodemaintenances;nodemaintenances/status,verbs=*
//+kubebuilder:rbac:groups="cert-manager.io",resources=*,verbs=*
//+kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpuclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpuclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpuclusters/finalizers,verbs=update

func (r *DPUReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(4).Info("Reconcile", "dpu", req.Name)

	dpu := &provisioningv1.DPU{}
	if err := r.Get(ctx, req.NamespacedName, dpu); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get DPU", "DPU", dpu)
		return ctrl.Result{}, errors.Wrap(err, "failed to get DPU")
	}

	// Add finalizer if not set and DPU is not currently deleting.
	if !controllerutil.ContainsFinalizer(dpu, provisioningv1.DPUFinalizer) && dpu.DeletionTimestamp.IsZero() {
		controllerutil.AddFinalizer(dpu, provisioningv1.DPUFinalizer)
		if err := r.Client.Update(ctx, dpu); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to add DPU finalizer")
		}
		return ctrl.Result{}, nil
	}
	// This is to cache the DPUs that are created with the cluster field set in their manifests, such DPUs will not go through the Allocate() procedure in Initialization phase
	// PS: Users are able to create DPUs without DPUSets, which is not officially supported but also not forbidden. If the cluster field is empty, a DPUCluster will be allocated for it as usual.
	r.Allocator.SaveAssignedDPU(dpu)

	currentState := state.GetDPUState(dpu, r.Allocator)
	nextState, err := currentState.Handle(ctx, r.Client, r.DPUOptions)
	if err != nil {
		logger.Error(err, "Statue handle error", "dpu", err)
	}
	if !reflect.DeepEqual(dpu.Status, nextState) {
		logger.V(3).Info("Update DPU status", "current phase", dpu.Status.Phase, "next phase", nextState.Phase)
		dpu.Status = nextState

		if err := r.Client.Status().Update(ctx, dpu); err != nil {
			logger.Error(err, "Failed to update DPU", "DPU", dpu)
			return ctrl.Result{}, errors.Wrap(err, "failed to update DPU")
		}
	} else if nextState.Phase != provisioningv1.DPUError {
		// TODO: move the state checking in state machine
		return ctrl.Result{RequeueAfter: cutil.RequeueInterval}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DPUReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&provisioningv1.DPU{}).
		Watches(&provisioningv1.DPUCluster{}, handler.EnqueueRequestsFromMapFunc(r.nonInitializedDPU)).
		Complete(r)
}

func (r *DPUReconciler) nonInitializedDPU(ctx context.Context, obj client.Object) []reconcile.Request {
	var ret []reconcile.Request
	dc := obj.(*provisioningv1.DPUCluster)
	if dc.Status.Phase != provisioningv1.PhaseReady {
		return nil
	}
	dpuList := &provisioningv1.DPUList{}
	if err := r.Client.List(ctx, dpuList); err != nil {
		log.FromContext(ctx).Error(fmt.Errorf("failed to list DPUs, err: %v", err), "")
		return nil
	}
	for _, dpu := range dpuList.Items {
		if dpu.Spec.Cluster.Name == "" {
			ret = append(ret, reconcile.Request{NamespacedName: cutil.GetNamespacedName(&dpu)})
		}
	}
	return ret
}
