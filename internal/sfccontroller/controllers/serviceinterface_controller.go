/*
COPYRIGHT 2024 NVIDIA

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

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ServiceInterfaceNameLabel      = "sfc.dpf.nvidia.com/ServiceInterface-name"
	ServiceInterfaceNamespaceLabel = "sfc.dpf.nvidia.com/ServiceInterface-namespace"
	ServiceInterfaceControllerName = "service-interface-controller"
)

// ServiceInterfaceReconciler reconciles a ServiceInterface object
type ServiceInterfaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=ServiceInterfaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=ServiceInterfaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=ServiceInterfaces/finalizers,verbs=update
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=serviceinterfaces/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ServiceInterfaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling")
	sis := &sfcv1.ServiceInterface{}
	if err := r.Client.Get(ctx, req.NamespacedName, sis); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			// TODO add ovs-vsctl del-port
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if !sis.ObjectMeta.DeletionTimestamp.IsZero() {
		// Return early, the object is deleting.
		// TODO add ovs-vsctl del-port
		return ctrl.Result{}, nil
	}

	// TODO add ovs-vsctl add-port
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceInterfaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// TODO Match current node and go only if we have a match
		For(&sfcv1.ServiceInterface{}).
		Complete(r)
}
