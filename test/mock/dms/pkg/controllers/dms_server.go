/*
Copyright 2025 NVIDIA

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

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/dpucluster"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/test/mock/dms/pkg/server"

	"github.com/fluxcd/pkg/runtime/patch"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// DMSServerReconciler reconciles a DPU object
type DMSServerReconciler struct {
	Client client.Client
	Server *server.DMSServerMux
	// PodName is used to advertise this pod as the DMS server for DPUs.
	PodName string
}

// +kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpus,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpus/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpus/finalizers,verbs=update
// +kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpuclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpuclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=provisioning.dpu.nvidia.com,resources=dpuclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods;pods/exec;nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=patch;update;delete;create

func (r *DMSServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")
	dpu := &provisioningv1.DPU{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpu); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	patcher := patch.NewSerialPatcher(dpu, r.Client)

	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {
		log.Info("Patching")
		if err := patcher.Patch(ctx, dpu,
			patch.WithFieldOwner("mock-dms-controller"),
		); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	if dpu.Annotations == nil {
		dpu.Annotations = map[string]string{}
	}

	// Set the mock dms pod as both the DMS and hostnetwork pod using override annotations on the DPU.
	// This enables the DPU To pass checks related to deploying the DMS pod and waiting for the hostnetwork pod to become ready.
	dpu.Annotations[cutil.OverrideDMSPodNameAnnotationKey] = r.PodName
	dpu.Annotations[cutil.OverrideHostNetworkAnnotationKey] = r.PodName

	// Add the annotation and patch the DPU.
	if err := r.Server.EnsureListenerForDPU(dpu); err != nil {
		return ctrl.Result{}, err
	}

	// Create a node for the DPU in the DPUCluster. This enables the DPU to pass checks in the DPU Cluster Config phase.
	if err := r.createNodeForDPU(ctx, dpu); err != nil {
		return ctrl.Result{}, err
	}
	// If we have an error we have to requeue the DPU and let controller-runtime handle the error.
	return ctrl.Result{}, nil
}

// createNodeForDPU creates a Kubernetes node with a Ready conditions for the DPU.
func (r *DMSServerReconciler) createNodeForDPU(ctx context.Context, dpu *provisioningv1.DPU) error {
	// Only create the node in the DPUClusterConfig phase.
	if dpu.Status.Phase != provisioningv1.DPUClusterConfig {
		return nil
	}
	log := ctrllog.FromContext(ctx)
	log.Info("Ensuring node is up to date for DPU")
	dpuCluster := &provisioningv1.DPUCluster{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: dpu.Spec.Cluster.Namespace, Name: dpu.Spec.Cluster.Name}, dpuCluster)
	if err != nil {
		return err
	}

	dpuClient, err := dpucluster.NewConfig(r.Client, dpuCluster).Client(ctx)
	if err != nil {
		return err
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			// The Node should have the same name as the DPU.
			Name:      dpu.Name,
			Namespace: dpu.Namespace,
			Annotations: map[string]string{
				"kwok.x-k8s.io/node": "fake",
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		Spec: corev1.NodeSpec{
			PodCIDR:    "",
			PodCIDRs:   nil,
			ProviderID: "",
		},
	}
	// Return early if the node already exists. Do not repeatedly reconcile the object to avoid conflicts with kwok.
	if err := dpuClient.Get(ctx, client.ObjectKeyFromObject(node), &corev1.Node{}); err == nil {
		return nil
	}
	node.ManagedFields = nil
	if err := dpuClient.Patch(ctx, node, client.Apply, client.ForceOwnership, client.FieldOwner("mock-dms")); err != nil {
		return err
	}

	node.Status = corev1.NodeStatus{
		NodeInfo: corev1.NodeSystemInfo{
			Architecture: "arm64",
		},
		Allocatable: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000"),
			corev1.ResourceMemory: resource.MustParse("2Ti"),
			corev1.ResourcePods:   resource.MustParse("1000"),
		},
		Capacity: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000"),
			corev1.ResourceMemory: resource.MustParse("2Ti"),
			corev1.ResourcePods:   resource.MustParse("1000"),
		},
	}

	node.ManagedFields = nil
	if err := dpuClient.Status().Patch(ctx, node, client.Apply, client.ForceOwnership, client.FieldOwner("mock-dms")); err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DMSServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&provisioningv1.DPU{}).
		Named("dms-server").
		Complete(r)
}
