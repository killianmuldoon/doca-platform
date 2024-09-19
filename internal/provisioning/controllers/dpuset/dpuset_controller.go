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

package dpuset

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"

	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util/reboot"

	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// controller name that will be used when
	DpuSetControllerName = "dpuset"
)

// DpuSetReconciler reconciles a DpuSet object
type DpuSetReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=dpusets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=dpusets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=dpusets/finalizers,verbs=update
//+kubebuilder:rbac:groups=provisioning.dpf.nvidia.com,resources=dpuflavors,verbs=get;list;watch

func (r *DpuSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(4).Info("Reconcile", "dpuset", req.Name)

	dpuSet := &provisioningv1.DpuSet{}
	if err := r.Get(ctx, req.NamespacedName, dpuSet); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get DPUSet", "DPUSet", dpuSet)
		return ctrl.Result{}, errors.Wrap(err, "failed to get DpuSet")
	}

	if !dpuSet.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, dpuSet)
	}

	return r.reconcile(ctx, dpuSet)
}

func (r *DpuSetReconciler) reconcileDelete(ctx context.Context, dpuSet *provisioningv1.DpuSet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if err := r.finalizeDPUSet(ctx, dpuSet); err != nil {
		logger.Error(err, "Failed to finalize DPUSet", "DPUSet", dpuSet)
		return ctrl.Result{}, errors.Wrap(err, "failed to finalize DPUSet")
	}

	return ctrl.Result{}, nil
}

func (r *DpuSetReconciler) reconcile(ctx context.Context, dpuSet *provisioningv1.DpuSet) (ctrl.Result, error) {
	var err error
	logger := log.FromContext(ctx)

	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(dpuSet, provisioningv1.DpuSetFinalizer) {
		controllerutil.AddFinalizer(dpuSet, provisioningv1.DpuSetFinalizer)
		if err := r.Client.Update(ctx, dpuSet); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to add DPUSet finalizer")
		}
		return ctrl.Result{}, nil
	}

	if err := r.createDPUClusterKubeConfig(ctx, dpuSet); err != nil {
		logger.Error(err, "Failed to create DPU cluster kubeconfig file", "DPUSet", dpuSet)
		return ctrl.Result{}, errors.Wrap(err, "failed to create DPU cluster kubeconfig file")
	}

	// Get node map by nodeSelector
	nodeMap, err := r.getNodeMap(ctx, dpuSet.Spec.NodeSelector, dpuSet.Spec.DpuSelector)
	if err != nil {
		logger.Error(err, "Failed to get Node list", "DPUSet", dpuSet)
		return ctrl.Result{}, errors.Wrap(err, "failed to get Node list")
	}

	// Get dpu map which are owned by dpuset
	dpuMap, err := r.getDpusMap(ctx, dpuSet)
	if err != nil {
		logger.Error(err, "Failed to get Dpu list", "DPUSet", dpuSet)
		return ctrl.Result{}, errors.Wrap(err, "failed to get Dpu list")
	}

	// create dpu for the node
	for nodeName, node := range nodeMap {
		if _, ok := dpuMap[nodeName]; !ok {
			if _, err = r.createDpu(ctx, dpuSet, node); err != nil {
				logger.Error(err, "Failed to create Dpu", "DPUSet", dpuSet)
				return ctrl.Result{}, errors.Wrap(err, "failed to create Dpu")
			}
		} else {
			delete(dpuMap, nodeName)
		}
	}

	// delete dpu if node does not exist
	for nodeName, dpu := range dpuMap {
		if err := r.Delete(ctx, &dpu); err != nil {
			logger.Error(err, "Failed to delete Dpu", "DPUSet", dpuSet)
			return ctrl.Result{}, errors.Wrap(err, "failed to delete Dpu")
		}
		logger.V(3).Info("Dpu is deleted", "nodename", nodeName)
	}

	// handle rolling update
	dpuMap, err = r.getDpusMap(ctx, dpuSet)
	if err != nil {
		logger.Error(err, "Failed to get Dpus", "DPUSet", dpuSet)
		return ctrl.Result{}, errors.Wrap(err, "failed to get Dpus")
	}

	switch dpuSet.Spec.Strategy.Type {
	case provisioningv1.RecreateStrategyType:
		if err := r.rolloutRecreate(ctx, dpuSet, dpuMap); err != nil {
			logger.Error(err, "Failed to rollout Dpu", "DPUSet", dpuSet)
			return ctrl.Result{}, errors.Wrap(err, "failed to rollout Dpu")
		}
	case provisioningv1.RollingUpdateStrategyType:
		if err := r.rolloutRolling(ctx, dpuSet, dpuMap, len(nodeMap)); err != nil {
			logger.Error(err, "Failed to rollout Dpu", "DPUSet", dpuSet)
			return ctrl.Result{}, errors.Wrap(err, "failed to rollout Dpu")
		}
	}

	if err := updateDPUSetStatus(ctx, dpuSet, dpuMap, r.Client); err != nil {
		logger.Error(err, "Failed to update DpuSet status", "DPUSet", dpuSet)
		return ctrl.Result{}, errors.Wrap(err, "failed to update DPUSet status")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DpuSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&provisioningv1.DpuSet{}).
		Owns(&provisioningv1.Dpu{}).
		Watches(&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.nodeToDpuSetReq),
			builder.WithPredicates(predicate.LabelChangedPredicate{})).
		Watches(&provisioningv1.DPUFlavor{},
			handler.EnqueueRequestsFromMapFunc(r.flavorToDpuSeqReq)).
		Complete(r)
}

func (r *DpuSetReconciler) nodeToDpuSetReq(ctx context.Context, resource client.Object) []reconcile.Request {
	requests := make([]reconcile.Request, 0)
	dpuSetList := &provisioningv1.DpuSetList{}
	if err := r.List(ctx, dpuSetList); err == nil {
		for _, item := range dpuSetList.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				}})
		}
	}
	return requests
}

func (r *DpuSetReconciler) flavorToDpuSeqReq(ctx context.Context, resource client.Object) []reconcile.Request {
	flavor := resource.(*provisioningv1.DPUFlavor)
	dpuSetList := &provisioningv1.DpuSetList{}
	if err := r.List(ctx, dpuSetList); err != nil {
		return nil
	}
	requests := []reconcile.Request{}
	for _, item := range dpuSetList.Items {
		if item.Spec.DpuTemplate.Spec.DPUFlavor != flavor.Name {
			continue
		}
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			},
		})
	}
	return requests
}

func (r *DpuSetReconciler) getNodeMap(ctx context.Context, nselector *metav1.LabelSelector, dselector map[string]string) (map[string]corev1.Node, error) {
	nodeMap := make(map[string]corev1.Node)
	nodeList := &corev1.NodeList{}
	nodeSelector, err := metav1.LabelSelectorAsSelector(nselector)
	for k, v := range dselector {
		if r, err := labels.NewRequirement(k, selection.Equals, []string{v}); err != nil {
			return nodeMap, err
		} else {
			nodeSelector.Add(*r)
		}
	}

	if err != nil {
		return nodeMap, err
	}
	listOptions := client.ListOptions{
		LabelSelector: nodeSelector,
	}

	if err := r.List(ctx, nodeList, &listOptions); err != nil {
		return nodeMap, err
	}

	for _, node := range nodeList.Items {
		nodeMap[node.Name] = node
	}
	return nodeMap, nil
}

func (r *DpuSetReconciler) getDpusMap(ctx context.Context, dpuSet *provisioningv1.DpuSet) (map[string]provisioningv1.Dpu, error) {
	dpuMap := make(map[string]provisioningv1.Dpu)
	dpuList := &provisioningv1.DpuList{}
	if err := r.List(ctx, dpuList, client.MatchingLabels{
		cutil.DpuSetNameLabel:      dpuSet.Name,
		cutil.DpuSetNamespaceLabel: dpuSet.Namespace,
	}); err != nil {
		return dpuMap, err
	}
	for _, dpu := range dpuList.Items {
		dpuMap[dpu.Spec.NodeName] = dpu
	}
	return dpuMap, nil
}

func (r *DpuSetReconciler) createDpu(ctx context.Context, dpuSet *provisioningv1.DpuSet,
	node corev1.Node) (*provisioningv1.Dpu, error) {
	logger := log.FromContext(ctx)

	labels := map[string]string{cutil.DpuSetNameLabel: dpuSet.Name, cutil.DpuSetNamespaceLabel: dpuSet.Namespace}
	for k, v := range dpuSet.Labels {
		labels[k] = v
	}

	for k, v := range node.Labels {
		if strings.HasSuffix(k, cutil.DpuPCIAddress) {
			if len(v) != 0 {
				labels[cutil.DpuPCIAddressLabel] = v
			} else {
				return nil, fmt.Errorf("the label of %s on node %s is empty", cutil.DpuPCIAddress, node.Name)
			}
		}
		if strings.HasSuffix(k, cutil.DpuPFName) {
			if len(v) != 0 {
				labels[cutil.DpuPFNameLabel] = v
			} else {
				return nil, fmt.Errorf("the label of %s on node %s is empty", cutil.DpuPFName, node.Name)
			}
		}
	}
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			labels[cutil.DpuHostIPLabel] = address.Address
			break
		}
	}

	dpuName := fmt.Sprintf("%s-%s", strings.ToLower(node.Name), labels[cutil.DpuPCIAddressLabel])
	owner := metav1.NewControllerRef(dpuSet,
		provisioningv1.GroupVersion.WithKind("DpuSet"))

	dpu := &provisioningv1.Dpu{
		ObjectMeta: metav1.ObjectMeta{
			Name:            dpuName,
			Namespace:       dpuSet.Namespace,
			Labels:          labels,
			Annotations:     make(map[string]string),
			OwnerReferences: []metav1.OwnerReference{*owner},
		},
		Spec: provisioningv1.DpuSpec{
			NodeName:            node.Name,
			BFB:                 dpuSet.Spec.DpuTemplate.Spec.Bfb.BFBName,
			NodeEffect:          dpuSet.Spec.DpuTemplate.Spec.NodeEffect,
			Cluster:             dpuSet.Spec.DpuTemplate.Spec.Cluster,
			DPUFlavor:           dpuSet.Spec.DpuTemplate.Spec.DPUFlavor,
			AutomaticNodeReboot: dpuSet.Spec.DpuTemplate.Spec.AutomaticNodeReboot,
		},
	}
	// do we really need this?
	for k, v := range dpuSet.Annotations {
		dpu.Annotations[k] = v
	}
	if v, ok := dpuSet.Spec.DpuTemplate.Annotations[reboot.HostPowerCycleRequireKey]; ok {
		dpu.Annotations[reboot.HostPowerCycleRequireKey] = v
	}
	if err := r.Create(ctx, dpu); err != nil {
		return nil, err
	}
	logger.V(2).Info("Dpu is created", "Dpu", dpu)
	return dpu, nil
}

func (r *DpuSetReconciler) rolloutRecreate(ctx context.Context, dpuSet *provisioningv1.DpuSet,
	dpuMap map[string]provisioningv1.Dpu) error {
	for _, dpu := range dpuMap {
		if update, err := r.needUpdate(ctx, *dpuSet, dpu); err != nil {
			return err
		} else if update {
			if err := r.Delete(ctx, &dpu); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *DpuSetReconciler) rolloutRolling(ctx context.Context, dpuSet *provisioningv1.DpuSet,
	dpuMap map[string]provisioningv1.Dpu, total int) error {
	scaledValue, err := intstr.GetScaledValueFromIntOrPercent(intstr.ValueOrDefault(
		dpuSet.Spec.Strategy.RollingUpdate.MaxUnavailable, intstr.FromInt(0)), total, true)
	if err != nil {
		return err
	}

	if scaledValue <= 0 {
		scaledValue = 1
	} else if scaledValue > total {
		scaledValue = total
	}

	// The DPUs that have deleted should be considered as unavailable DPUs
	unavaiable := total - len(dpuMap)
	for _, dpu := range dpuMap {
		if isUnavailable(&dpu) {
			// A DPU which is not ready should be considered an unavailable DPU. Skip this one
			unavaiable++
		}
	}

	for _, dpu := range dpuMap {
		if update, err := r.needUpdate(ctx, *dpuSet, dpu); err != nil {
			return err
		} else if !update {
			continue
		}
		if isUnavailable(&dpu) {
			if err := r.Delete(ctx, &dpu); err != nil {
				return err
			}
		} else if unavaiable < scaledValue {
			if err := r.Delete(ctx, &dpu); err != nil {
				return err
			}
			unavaiable++
		}
	}
	return nil
}

func isUnavailable(dpu *provisioningv1.Dpu) bool {
	_, cond := cutil.GetDPUCondition(&dpu.Status, provisioningv1.DPUCondReady.String())
	return cond == nil || cond.Status == metav1.ConditionFalse
}

// TODO: check more informations
func (r *DpuSetReconciler) needUpdate(ctx context.Context, dpuSet provisioningv1.DpuSet, dpu provisioningv1.Dpu) (bool, error) {
	logger := log.FromContext(ctx)
	// update dpu node label
	newLabel := dpuSet.Spec.DpuTemplate.Spec.Cluster.NodeLabels
	oldLabel := dpu.Spec.Cluster.NodeLabels
	if !reflect.DeepEqual(newLabel, oldLabel) {
		patcher := patch.NewSerialPatcher(&dpu, r.Client)
		dpu.Spec.Cluster.NodeLabels = newLabel
		if err := patcher.Patch(ctx, &dpu); err != nil {
			return false, err
		} else {
			logger.V(3).Info(fmt.Sprintf("dpu %s label update to %v", dpu.Name, newLabel))
		}
	}

	return dpu.Spec.BFB != dpuSet.Spec.DpuTemplate.Spec.Bfb.BFBName || dpu.Spec.DPUFlavor != dpuSet.Spec.DpuTemplate.Spec.DPUFlavor, nil
}

func updateDPUSetStatus(ctx context.Context, dpuSet *provisioningv1.DpuSet,
	dpuMap map[string]provisioningv1.Dpu, client client.Client) error {
	dpuStatistics := make(map[provisioningv1.DpuPhase]int)
	for _, dpu := range dpuMap {
		switch dpu.Status.Phase {
		case "":
			dpuStatistics[provisioningv1.DPUInitializing]++

		case provisioningv1.DPUInitializing:
			dpuStatistics[provisioningv1.DPUInitializing]++

		case provisioningv1.DPUPending:
			dpuStatistics[provisioningv1.DPUPending]++

		case provisioningv1.DPUDMSDeployment:
			dpuStatistics[provisioningv1.DPUDMSDeployment]++

		case provisioningv1.DPUHostNetworkConfiguration:
			dpuStatistics[provisioningv1.DPUHostNetworkConfiguration]++

		case provisioningv1.DPUOSInstalling:
			dpuStatistics[provisioningv1.DPUOSInstalling]++

		case provisioningv1.DPUClusterConfig:
			dpuStatistics[provisioningv1.DPUClusterConfig]++

		case provisioningv1.DPUReady:
			dpuStatistics[provisioningv1.DPUReady]++

		case provisioningv1.DPUError:
			dpuStatistics[provisioningv1.DPUError]++

		case provisioningv1.DPUDeleting:
			dpuStatistics[provisioningv1.DPUDeleting]++
		}
	}

	needUpdate := false
	if len(dpuStatistics) != len(dpuSet.Status.Dpustatistics) {
		needUpdate = true
	} else {
		for key, count1 := range dpuStatistics {
			count2, ok := dpuSet.Status.Dpustatistics[key]
			if !ok {
				needUpdate = true
				break
			} else if count1 != count2 {
				needUpdate = true
				break
			}
		}
	}
	if needUpdate {
		dpuSet.Status.Dpustatistics = dpuStatistics
		if err := client.Status().Update(ctx, dpuSet); err != nil {
			return err
		}
	}

	return nil
}

func (r *DpuSetReconciler) createDPUClusterKubeConfig(ctx context.Context, dpuSet *provisioningv1.DpuSet) error {
	kubeConfigFile := cutil.GenerateKubeConfigFileName(dpuSet.Name, dpuSet.Namespace)
	if _, err := os.Stat(kubeConfigFile); err == nil {
		return nil
	} else if os.IsNotExist(err) {
		nn := types.NamespacedName{
			Namespace: dpuSet.Spec.DpuTemplate.Spec.Cluster.NameSpace,
			Name:      fmt.Sprintf("%s-%s", dpuSet.Spec.DpuTemplate.Spec.Cluster.Name, "admin-kubeconfig"),
		}
		kubeConfigSecret := &corev1.Secret{}
		if err := r.Client.Get(ctx, nn, kubeConfigSecret); err != nil {
			return err
		}

		kubeconfig := kubeConfigSecret.Data["admin.conf"]
		if err := os.WriteFile(kubeConfigFile, kubeconfig, 0644); err != nil {
			return err
		}
	} else {
		return err
	}

	return nil
}

func (r *DpuSetReconciler) finalizeDPUSet(ctx context.Context, dpuSet *provisioningv1.DpuSet) error {
	kubeConfigFile := cutil.GenerateKubeConfigFileName(dpuSet.Name, dpuSet.Namespace)
	if _, err := os.Stat(kubeConfigFile); err == nil {
		err := os.Remove(kubeConfigFile)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	controllerutil.RemoveFinalizer(dpuSet, provisioningv1.DpuSetFinalizer)
	if err := r.Client.Update(ctx, dpuSet); err != nil {
		return err
	}
	return nil
}
