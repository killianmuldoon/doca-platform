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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"
	nvipamv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/nvipam/api/v1alpha1"

	multusclient "gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/k8sclient"
	multustypes "gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PodIpamReconciler reconciles a Pod object
type PodIpamReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type poolConfig struct {
	PoolName string
	PoolType string
	AssignGW bool
}

const (
	bridgeSfcNAD           = "mybrsfc"
	ipamPoolNamesParam     = "poolNames"
	podIpamControllerName  = "podIpamcontroller"
	networkAttachmentAnnot = "k8s.v1.cni.cncf.io/networks"
)

//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch;update
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sfc.dpf.nvidia.com,resources=servicechains,verbs=get;list;watch
//+kubebuilder:rbac:groups=nv-ipam.nvidia.com,resources=ippools,verbs=get;list;watch
//+kubebuilder:rbac:groups=nv-ipam.nvidia.com,resources=cidrpools,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch

func (r *PodIpamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling")
	pod := &corev1.Pod{}
	if err := r.Client.Get(ctx, req.NamespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if !pod.ObjectMeta.DeletionTimestamp.IsZero() {
		// Return early, the object is deleting.
		return ctrl.Result{}, nil
	}
	if pod.Spec.NodeName == "" {
		// Pod not scheduled on a node yet
		return ctrl.Result{}, nil
	}
	if pod.Status.Phase != corev1.PodPending {
		// Pod is not pending
		return ctrl.Result{}, nil
	}
	sfcNets, err := getBrSFCNetworks(ctx, pod)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(sfcNets) == 0 {
		log.Info("Pod without br-sfc network")
		return ctrl.Result{}, nil
	}
	ifcToSvc := make(map[string]*sfcv1.Service)
	for _, net := range sfcNets {
		// No Interface requested
		if net.InterfaceRequest == "" {
			log.Info("Pod with br-sfc network, but without interface requested")
			return ctrl.Result{}, nil
		}
		ifcToSvc[net.InterfaceRequest] = nil
	}
	err = r.getServicesFromChain(ctx, ifcToSvc, pod.Spec.NodeName)
	if err != nil {
		log.Error(err, "Fail to get Service from Chain")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	for ifc, svc := range ifcToSvc {
		if svc == nil {
			log.Info("No Service definition for requested interface found. Requeing", "interface", ifc)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		if svc.IPAM == nil {
			log.Info("No IPAM requested for interface", "interface", ifc)
			continue
		}
		if svc.IPAM.Reference == nil && len(svc.IPAM.MatchLabels) < 1 {
			return ctrl.Result{}, fmt.Errorf("Service IPAM should have Reference or MatchLabels. Interface:%s", ifc)
		}
	}

	ifcToPoolCfg, err := r.getPoolsConfig(ctx, ifcToSvc)
	if err != nil {
		// No IpPool/CidrPool found requeuing
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	return r.updatePodAnnotation(ctx, pod, ifcToPoolCfg)
}

func (r *PodIpamReconciler) updatePodAnnotation(ctx context.Context, pod *corev1.Pod,
	ifcToPoolCfg map[string]*poolConfig) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	networks, err := multusclient.GetPodNetwork(pod)
	if err != nil {
		log.Error(err, "failed to get networks from pod")
		return ctrl.Result{}, err
	}
	changed := false
	for _, net := range networks {
		if net.Name == bridgeSfcNAD {
			pollCfg, ok := ifcToPoolCfg[net.InterfaceRequest]
			if ok {
				cniArgs := make(map[string]interface{})
				cniArgs["allocateDefaultGateway"] = pollCfg.AssignGW
				cniArgs["poolNames"] = []string{pollCfg.PoolName}
				cniArgs["poolType"] = pollCfg.PoolType
				net.CNIArgs = &cniArgs
				changed = true
			}
		}
	}
	if !changed {
		return ctrl.Result{}, nil
	}
	j, err := json.Marshal(networks)
	if err != nil {
		log.Error(err, "failed to marshal networks to json")
		return ctrl.Result{}, err
	}
	pod.Annotations[networkAttachmentAnnot] = string(j)
	pod.ObjectMeta.ManagedFields = nil
	pod.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
	return ctrl.Result{}, r.Client.Patch(ctx, pod, client.Apply, client.ForceOwnership, client.FieldOwner(podIpamControllerName))
}

func (r *PodIpamReconciler) getServicesFromChain(ctx context.Context, ifcToSvc map[string]*sfcv1.Service, node string) error {
	scList := &sfcv1.ServiceChainList{}
	if err := r.Client.List(ctx, scList); err != nil {
		return err
	}
	if len(scList.Items) == 0 {
		return fmt.Errorf("No Service Chain for Node found. Requeing")
	}
	for _, serviceChain := range scList.Items {
		if serviceChain.Spec.Node == node {
			for _, sw := range serviceChain.Spec.Switches {
				for _, port := range sw.Ports {
					if port.Service != nil {
						if _, ok := ifcToSvc[port.Service.InterfaceName]; ok {
							ifcToSvc[port.Service.InterfaceName] = port.Service
						}
					}
				}
			}
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodIpamReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}

func getBrSFCNetworks(ctx context.Context, pod *corev1.Pod) ([]*multustypes.NetworkSelectionElement, error) {
	log := log.FromContext(ctx)
	var sfcNets []*multustypes.NetworkSelectionElement
	networks, err := multusclient.GetPodNetwork(pod)
	if err != nil {
		if _, ok := err.(*multusclient.NoK8sNetworkError); ok {
			log.Info("No networks found in pod annotation")
			return nil, nil
		}
		log.Error(err, "failed to get networks from pod")
		return nil, err
	}
	for _, net := range networks {
		net := net
		if net.Name == bridgeSfcNAD {
			sfcNets = append(sfcNets, net)
		}
	}
	return sfcNets, nil
}

func (r *PodIpamReconciler) getPoolsConfig(ctx context.Context, ifcToSvc map[string]*sfcv1.Service) (map[string]*poolConfig, error) {
	ifcToPoolCfg := make(map[string]*poolConfig)
	for ifc, svc := range ifcToSvc {
		if svc.IPAM != nil {
			poolCfg, err := r.getPoolConfig(ctx, svc.IPAM)
			if err != nil {
				return nil, err
			}
			ifcToPoolCfg[ifc] = poolCfg
		}
	}
	return ifcToPoolCfg, nil
}

func (r *PodIpamReconciler) getPoolConfig(ctx context.Context, ipam *sfcv1.IPAM) (*poolConfig, error) {
	poolCfg := &poolConfig{
		AssignGW: ipam.DefaultGateway,
	}
	var pool, poolType string
	var err error
	if ipam.Reference != nil {
		pool, poolType, err = r.getPoolByRef(ctx, ipam)
		if err != nil {
			return nil, err
		}
	} else {
		pool, poolType, err = r.getPoolByMatchLabel(ctx, ipam)
		if err != nil {
			return nil, err
		}
	}
	poolCfg.PoolType = poolType
	poolCfg.PoolName = pool
	return poolCfg, nil
}

func (r *PodIpamReconciler) getPoolByRef(ctx context.Context, ipam *sfcv1.IPAM) (string, string, error) {
	ipPool := &nvipamv1.IPPool{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: ipam.Reference.Namespace,
		Name: ipam.Reference.Name}, ipPool)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", "", err
		}
		// Continue to check if CIDRPool exists
	} else {
		// IPPool found
		return ipam.Reference.Name, strings.ToLower(nvipamv1.IPPoolKind), nil
	}
	//IPPool not found, check CIDRPool
	cidrPool := &nvipamv1.CIDRPool{}
	err = r.Client.Get(ctx, client.ObjectKey{Namespace: ipam.Reference.Namespace,
		Name: ipam.Reference.Name}, cidrPool)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", "", err
		}
		// No IpPool/CidrPool found requeuing
		return "", "", fmt.Errorf("No IPPool or CidrPool found for Ref %s/%s", ipam.Reference.Namespace, ipam.Reference.Name)
	} else {
		// cidrPool found
		return ipam.Reference.Name, strings.ToLower(nvipamv1.CIDRPoolKind), nil
	}
}

func (r *PodIpamReconciler) getPoolByMatchLabel(ctx context.Context, ipam *sfcv1.IPAM) (string, string, error) {
	log := log.FromContext(ctx)
	listOptions := client.MatchingLabels(ipam.MatchLabels)
	ipPoolList := &nvipamv1.IPPoolList{}
	if err := r.List(ctx, ipPoolList, &listOptions); err != nil {
		return "", "", err
	}
	if len(ipPoolList.Items) > 0 {
		if len(ipPoolList.Items) > 1 {
			log.Info("Service IPAM MatchLabels matched more than one IPPool", "labels", ipam.MatchLabels)
		}
		return ipPoolList.Items[0].Name, strings.ToLower(nvipamv1.IPPoolKind), nil
	}
	cidrPoolList := &nvipamv1.CIDRPoolList{}
	if err := r.List(ctx, cidrPoolList, &listOptions); err != nil {
		return "", "", err
	}
	if len(cidrPoolList.Items) > 0 {
		if len(ipPoolList.Items) > 1 {
			log.Info("Service IPAM MatchLabels matched more than one CIDRPool", "labels", ipam.MatchLabels)
		}
		return cidrPoolList.Items[0].Name, strings.ToLower(nvipamv1.CIDRPoolKind), nil
	}
	// No IpPool/CidrPool found requeuing
	return "", "", fmt.Errorf("No IPPool or CidrPool found for Labels %v", ipam.MatchLabels)
}
