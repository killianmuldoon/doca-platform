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

package dpucluster

import (
	"context"
	"fmt"
	"os"
	"sync"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/allocator"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DPUClusterControllerName is the controllers name that will be used when reporting events
const DPUClusterControllerName = "dpucluster"

// DPUClusterReconciler reconciles a DPUCluster object
type DPUClusterReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Recorder     record.EventRecorder
	adminClients sync.Map
	Allocator    allocator.Allocator
}

// +kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpuclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpuclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpuclusters/finalizers,verbs=update

func (r *DPUClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	dc := &provisioningv1.DPUCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, dc); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !dc.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(dc, provisioningv1.FinalizerInternalCleanUp) {
			return ctrl.Result{}, nil
		}
		r.Allocator.RemoveCluster(dc)
		if err := r.deleteAdminClient(dc); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete admin client, err: %v", err)
		}
		controllerutil.RemoveFinalizer(dc, provisioningv1.FinalizerInternalCleanUp)
		if err := r.Client.Update(ctx, dc); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer, err: %v", err)
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(dc, provisioningv1.FinalizerInternalCleanUp) {
		logger.V(3).Info(fmt.Sprintf("add finalizer %s", provisioningv1.FinalizerInternalCleanUp))
		controllerutil.AddFinalizer(dc, provisioningv1.FinalizerInternalCleanUp)
		if err := r.Client.Update(ctx, dc); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer, err: %v", err)
		}
	}
	r.Allocator.SaveCluster(dc)

	if dc.Status.Phase == provisioningv1.PhasePending {
		dc.Status.Phase = provisioningv1.PhaseCreating
		if err := r.Client.Status().Update(ctx, dc); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to set Creating phase, err: %v", err)
		}
		return ctrl.Result{}, nil
	}

	allTrue, isCreated := true, false
	for _, cond := range dc.Status.Conditions {
		if cond.Type == string(provisioningv1.ConditionReady) {
			continue
		}
		allTrue = allTrue && (cond.Status == metav1.ConditionTrue)
		if cond.Type == string(provisioningv1.ConditionCreated) {
			isCreated = cond.Status == metav1.ConditionTrue
		}
	}
	allTrue = allTrue && isCreated

	var errList []error
	if allTrue {
		adminClient, err := r.getOrCreateClient(ctx, dc)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get admin client, err: %v", err)
		}
		if _, err := adminClient.ServerVersion(); err != nil {
			logger.Error(fmt.Errorf("health check failed, err: %v", err), "")
			cond := cutil.NewCondition(string(provisioningv1.ConditionReady), err, "HealthCheckFailed", "")
			meta.SetStatusCondition(&dc.Status.Conditions, *cond)
			dc.Status.Phase = provisioningv1.PhaseNotReady
			errList = append(errList, err)
		} else {
			cond := cutil.NewCondition(string(provisioningv1.ConditionReady), nil, "HealthCheckPassed", "")
			meta.SetStatusCondition(&dc.Status.Conditions, *cond)
			dc.Status.Phase = provisioningv1.PhaseReady
		}
	} else {
		switch dc.Status.Phase {
		case provisioningv1.PhaseReady, provisioningv1.PhaseNotReady:
			dc.Status.Phase = provisioningv1.PhaseNotReady
		default: // no-op
		}
	}
	if err := r.Client.Status().Update(ctx, dc); err != nil {
		errList = append(errList, fmt.Errorf("failed to update status, err: %v", err))
		return ctrl.Result{}, kerrors.NewAggregate(errList)
	}
	return ctrl.Result{}, kerrors.NewAggregate(errList)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DPUClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&provisioningv1.DPUCluster{}).
		Complete(r)
}

func (r *DPUClusterReconciler) getOrCreateClient(ctx context.Context, dc *provisioningv1.DPUCluster) (*kubernetes.Clientset, error) {
	if dc == nil {
		return nil, fmt.Errorf("dc is nil")
	}
	if value, ok := r.adminClients.Load(dc.UID); ok {
		return value.(*kubernetes.Clientset), nil
	}

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: dc.Spec.Kubeconfig, Namespace: dc.Namespace}, secret); err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig, err: %v", err)
	}
	data, ok := secret.Data["admin.conf"]
	if !ok {
		return nil, fmt.Errorf("admin.conf not found")
	}
	cfg, err := clientcmd.NewClientConfigFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to build client config from bytes, err: %v", err)
	}
	clientCfg, err := cfg.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build client config, err: %v", err)
	}
	clientSet, err := kubernetes.NewForConfig(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build client from client config, err: %v", err)
	}
	fp := cutil.AdminKubeConfigPath(*dc)
	if err := os.WriteFile(fp, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write kubeconfig file, err: %v", err)
	}
	r.adminClients.Store(dc.UID, clientSet)
	return clientSet, nil
}

func (r *DPUClusterReconciler) deleteAdminClient(dc *provisioningv1.DPUCluster) error {
	if dc == nil {
		return nil
	}
	fp := cutil.AdminKubeConfigPath(*dc)
	err := os.Remove(fp)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove kubeconfig file, path: %s, err: %v", fp, err)
	}
	r.adminClients.Delete(dc.UID)
	return nil
}
