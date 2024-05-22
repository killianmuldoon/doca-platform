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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ServiceChainReconciler reconciles a ServiceChain object
type ServiceChainReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	ServiceChainNameLabel      = "sfc.dpf.nvidia.com/ServiceChain-name"
	ServiceChainNamespaceLabel = "sfc.dpf.nvidia.com/ServiceChain-namespace"
	ServiceChainControllerName = "service-chain-controller"
)

// Retrieve pod list base on matching labels
func getPodList(ctx context.Context, k8sClient client.Client, matchingLabels map[string]string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	listOptions := client.MatchingLabels(matchingLabels)

	if err := k8sClient.List(ctx, podList, &listOptions); err != nil {
		return nil, err
	}

	return podList, nil
}

//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch;update
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=ServiceChains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=ServiceChains/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=ServiceChains/finalizers,verbs=update
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

func (r *ServiceChainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling")
	sc := &sfcv1.ServiceChain{}
	if err := r.Client.Get(ctx, req.NamespacedName, sc); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			// TODO add ovs-ofctl del-flows
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if !sc.ObjectMeta.DeletionTimestamp.IsZero() {
		// Return early, the object is deleting.
		// TODO add ovs-ofctl del-flows
		return ctrl.Result{}, nil
	}

	// TODO remove this once we have backing logic. This was added in order to pass the linter
	_, err := getPodList(ctx, r.Client, sc.Spec.Switches[0].Ports[0].Service.MatchLabels)

	// TODO add ovs-ofctl add-flows

	// Requeue after xx seconds

	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceChainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// TODO Match current node and go only if we have a match
		For(&sfcv1.ServiceChain{}).
		Complete(r)
}
