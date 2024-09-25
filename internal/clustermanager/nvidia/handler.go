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

package nvidia

import (
	"context"
	"fmt"

	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/clustermanager/controller"
	kamaji "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/kamaji/api/v1alpha1"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

func init() {
	controller.RegisterWatch(&kamaji.TenantControlPlane{}, handler.EnqueueRequestsFromMapFunc(tcpToDPUCluster))
	controller.RegisterWatch(&appsv1.DaemonSet{}, handler.EnqueueRequestsFromMapFunc(daemonsetToDPUCluster))
}

type clusterHandler struct {
	client.Client
	Scheme *runtime.Scheme
}

func NewHandler(client client.Client, scheme *runtime.Scheme) controller.ClusterHandler {
	return &clusterHandler{Client: client, Scheme: scheme}
}

func (cm *clusterHandler) ReconcileCluster(ctx context.Context, dc *provisioningv1.DPUCluster) (string, []metav1.Condition, error) {
	var conds []metav1.Condition
	cond := cm.reconcileKeepalived(ctx, dc)
	if cond != nil {
		conds = append(conds, *cond)
	}

	kubeconfig, cond, err := cm.reconcileKamaji(ctx, dc)
	if cond != nil {
		conds = append(conds, *cond)
	}
	return kubeconfig, conds, err
}

func (cm *clusterHandler) CleanUpCluster(ctx context.Context, dc *provisioningv1.DPUCluster) (bool, error) {
	// cleanup is done via owner reference
	return true, nil
}

func (cm clusterHandler) Type() string {
	return "nvidia"
}

func (cm *clusterHandler) reconcileKeepalived(_ context.Context, dc *provisioningv1.DPUCluster) *metav1.Condition {
	if dc.Spec.ClusterEndpoint == nil || dc.Spec.ClusterEndpoint.Keepalived == nil {
		return nil
	}
	// todo: deploy keepalived
	return cutil.NewCondition("KeepalivedReady", nil, "Deployed", "")
}

func (cm *clusterHandler) reconcileKamaji(ctx context.Context, dc *provisioningv1.DPUCluster) (string, *metav1.Condition, error) {
	nn := kamajiTCPName(dc)
	svc := &corev1.Service{}
	svcCreated, tcpCreated := true, true
	if err := cm.Client.Get(ctx, nn, svc); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", nil, fmt.Errorf("failed to get service, err: %v", err)
		}
		svcCreated = false
	}
	tcp := &kamaji.TenantControlPlane{}
	if err := cm.Client.Get(ctx, nn, tcp); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", nil, fmt.Errorf("failed to get TCP, err: %v", err)
		}
		tcpCreated = false
	}
	if !svcCreated && !tcpCreated {
		if err := cm.Client.Create(ctx, expectedService(dc)); err != nil {
			return "", nil, fmt.Errorf("failed to create service, err: %v", err)
		}
	}
	if !tcpCreated {
		var nodePort int32
		for _, p := range svc.Spec.Ports {
			if p.Name != "kube-apiserver" || p.NodePort == 0 {
				continue
			}
			nodePort = p.NodePort
			break
		}
		if nodePort == 0 {
			return "", nil, fmt.Errorf("missing NodePort for kube-apiserver")
		}
		var err error
		tcp, err = expectedTCP(dc, cm.Scheme, nodePort)
		if err != nil {
			return "", nil, fmt.Errorf("failed to generate expected TCP, err: %v", err)
		}
		if err := cm.Client.Create(ctx, tcp); err != nil {
			return "", nil, fmt.Errorf("failed to create cluster request, err: %v", err)
		}
	}
	tcpStatus := tcp.Status.Kubernetes.Version.Status
	if tcpStatus == nil || *tcpStatus != kamaji.VersionReady {
		return "", cutil.NewCondition(string(provisioningv1.ConditionCreated), fmt.Errorf("creating cluster"), "ClusterNotReady", ""), nil
	}
	return adminKubeconfigName(dc), cutil.NewCondition(string(provisioningv1.ConditionCreated), nil, "Created", ""), nil
}

func kamajiTCPName(dc *provisioningv1.DPUCluster) types.NamespacedName {
	return cutil.GetNamespacedName(dc)
}

func adminKubeconfigName(dc *provisioningv1.DPUCluster) string {
	return fmt.Sprintf("%s-admin-kubeconfig", kamajiTCPName(dc).Name)
}

func expectedService(dc *provisioningv1.DPUCluster) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dc.Name,
			Namespace: dc.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Name:     "kube-apiserver",
					Protocol: corev1.ProtocolTCP,
					// Port could be any value, because it will be updated by Kamaji once the TCP is created
					Port: 6443,
				},
			},
			Selector: map[string]string{
				"kamaji.clastix.io/name": dc.Name,
			},
		},
	}
	return svc
}

func expectedTCP(dc *provisioningv1.DPUCluster, scheme *runtime.Scheme, nodePort int32) (*kamaji.TenantControlPlane, error) {
	nn := kamajiTCPName(dc)
	tcp := &kamaji.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
			Labels: map[string]string{
				"tenant.clastix.io": nn.Name,
			},
		},
		Spec: kamaji.TenantControlPlaneSpec{
			DataStore: "default",
			ControlPlane: kamaji.ControlPlane{
				Deployment: kamaji.DeploymentSpec{
					// TODO: this should be a nodeAffinity and we have to add tolerations.
					// See test/objects/infrastructure/dpu-control-plane.yaml for reference.
					NodeSelector: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
					Replicas: int32Ptr(3),
					AdditionalMetadata: kamaji.AdditionalMetadata{
						Labels: map[string]string{
							"tenant.clastix.io": nn.Name,
						},
					},
					Resources: &kamaji.ControlPlaneComponentsResources{
						APIServer: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
						ControllerManager: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("125m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
						Scheduler: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("125m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
					},
				},
				Service: kamaji.ServiceSpec{
					AdditionalMetadata: kamaji.AdditionalMetadata{
						Labels: map[string]string{
							"tenant.clastix.io": nn.Name,
						},
					},
					ServiceType: kamaji.ServiceTypeNodePort,
				},
			},
			Kubernetes: kamaji.KubernetesSpec{
				Version: dc.Spec.Version,
				Kubelet: kamaji.KubeletSpec{
					CGroupFS: "systemd",
				},
				AdmissionControllers: kamaji.AdmissionControllers{
					"ResourceQuota",
					"LimitRanger",
				},
			},
		},
	}
	if dc.Spec.ClusterEndpoint != nil && dc.Spec.ClusterEndpoint.Keepalived != nil {
		tcp.Spec.NetworkProfile = kamaji.NetworkProfileSpec{
			Address: dc.Spec.ClusterEndpoint.Keepalived.VIP,
			Port:    nodePort,
		}
	}
	if err := controllerutil.SetOwnerReference(dc, tcp, scheme); err != nil {
		return nil, fmt.Errorf("failed to set owner reference, err: %v", err)
	}
	return tcp, nil
}

func int32Ptr(i int32) *int32 {
	return &i
}

func tcpToDPUCluster(ctx context.Context, o client.Object) []reconcile.Request {
	return []reconcile.Request{
		{
			NamespacedName: cutil.GetNamespacedName(o),
		},
	}
}

func daemonsetToDPUCluster(ctx context.Context, o client.Object) []reconcile.Request {
	refs := o.GetOwnerReferences()
	for _, ref := range refs {
		if ref.APIVersion != "cluster-manager.dpf.nvidia.com" || ref.Kind != "DPUCluster" {
			continue
		}
		nn := types.NamespacedName{
			Name:      ref.Name,
			Namespace: o.GetNamespace(),
		}
		return []reconcile.Request{
			{
				NamespacedName: nn,
			},
		}
	}
	return nil
}
