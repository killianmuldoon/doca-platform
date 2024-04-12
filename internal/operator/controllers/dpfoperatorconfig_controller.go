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

package controller

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"
	dpucniprovisionerconfig "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/dpu/config"
	hostcniprovisionerconfig "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/host/config"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/utils"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// ovnKubernetesNamespace is the Namespace where OVN Kubernetes is deployed in OpenShift
	ovnKubernetesNamespace = "openshift-ovn-kubernetes"
	// ovnKubernetesDaemonsetName is the name of the OVN Kubernetes DaemonSet in OpenShift
	ovnKubernetesDaemonsetName = "ovnkube-node"
	// ovnKubernetesKubeControllerContainerName is the name of the OVN kube controller container that is part of the OVN
	// Kubernetes DaemonSet in OpenShift
	ovnKubernetesKubeControllerContainerName = "ovnkube-controller"
	// ovnKubernetesOVNControllerContainerName is the name of the OVN controller container that is part of the OVN
	// Kubernetes DaemonSet in OpenShift
	ovnKubernetesOVNControllerContainerName = "ovn-controller"
	// ovnKubernetesConfigMapName is the name of the OVN Kubernetes ConfigMap used to configure OVN Kubernetes in
	// OpenShift
	ovnKubernetesConfigMapName = "ovnkube-config"
	// ovnKubernetesConfigMapDataKey is the key in the OVN Kubernetes ConfigMap where this controller makes changes
	ovnKubernetesConfigMapDataKey = "ovnkube.conf"
	// ovnKubernetesEntrypointConfigMapName is the name of the OVN Kubernetes ConfigMap which contains the init script
	// used in the `ovnkube-node` DaemonSet in OpenShift.
	ovnKubernetesEntrypointConfigMapName = "ovnkube-script-lib"
	// ovnKubernetesEntrypointConfigMapNameDataKey is the key in the OVN Kubernetes Entrypoint ConfigMap where this
	// controller makes changes
	ovnKubernetesEntrypointConfigMapNameDataKey = "ovnkube-lib.sh"
	// ovnKubernetesNodeChassisIDAnnotation is an OVN Kubernetes Annotation that we need to cleanup
	// https://github.com/openshift/ovn-kubernetes/blob/release-4.14/go-controller/pkg/util/node_annotations.go#L65-L66
	ovnKubernetesNodeChassisIDAnnotation = "k8s.ovn.org/node-chassis-id"
	// ovnManagementVFName is the name of the VF that should be used by the OVN Kubernetes
	// TODO (decision needed): Either of the following options:
	// * Replace value with VF name coming from the DPFOperatorConfig
	// * Keep as is and have the user configure that name for the vf0 of p0 on the host.
	ovnManagementVFName = "enp23s0f0v0"
	// pfRepresentor is the name of the PF representor on the host
	// TODO (decision needed): Either of the following options
	// * Replace with PF name coming from the DPFOperatorConfig
	// * Keep as is and have the user configure that name for the vf0 of p0 on the host.
	// TODO: Consider having common constants across components where needed
	pfRepresentor = "ens2f0np0"
	// dpuOVSRemote is the OVS remote that the host side can use to configure the OVS on the DPU side.
	// The IP below is the static IP that uses the *Host VF<->DPU SF* communication channel
	// The Port is statically configured by the DPU CNI Provisioner
	// TODO: Consider having common constants across components where needed
	dpuOVSRemote = "tcp:10.100.1.1:8500"

	// customOVNKubernetesResourceNameSuffix is the suffix used in the custom OVN Kubernetes resources this controller is
	// creating.
	customOVNKubernetesResourceNameSuffix = "dpf"
	// networkOperatorNamespace is the Namespace where the cluster-network-operator is deployed in OpenShift
	networkOperatorNamespace = "openshift-network-operator"
	// networkOperatorDeploymentName is the Deployment name of the cluster-network-operator that is deployed in
	// OpenShift
	networkOperatorDeploymentName = "network-operator"
	// clusterVersionOperatorNamespace is the Namespace where the cluster-version-operator is deployed in OpenShift
	clusterVersionOperatorNamespace = "openshift-cluster-version"
	// clusterVersionOperatorDeploymentName is the Deployment name of the cluster-version-operator that is deployed in
	// OpenShift
	clusterVersionOperatorDeploymentName = "cluster-version-operator"
	// nodeIdentityWebhookConfigurationName is the name of the ValidatingWebhookConfiguration deployed in OpenShift that
	// controls which entities can update the Node object.
	nodeIdentityWebhookConfigurationName = "network-node-identity.openshift.io"

	// controlPlaneNodeLabel is the well known Kubernetes label:
	// https://kubernetes.io/docs/reference/labels-annotations-taints/#node-role-kubernetes-io-control-plane
	controlPlaneNodeLabel = "node-role.kubernetes.io/control-plane"
	// workerNodeLabel is the label that is added on worker nodes in OpenShift
	workerNodeLabel = "node-role.kubernetes.io/worker"
	// networkSetupReadyNodeLabel is the label used to determine when a node is ready to run the custom OVN Kubernetes
	// Pod.
	networkPreconfigurationReadyNodeLabel = "dpf.nvidia.com/network-preconfig-ready"
)

//go:embed manifests/hostcniprovisioner.yaml
var hostCNIProvisionerManifestContent []byte

//go:embed manifests/dpucniprovisioner.yaml
var dpuCNIProvisionerManifestContent []byte

// DPFOperatorConfigReconciler reconciles a DPFOperatorConfig object
// TODO: Consider creating a constructor
type DPFOperatorConfigReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Settings *DPFOperatorConfigReconcilerSettings
}

// DPFOperatorConfigReconcilerSettings contains settings related to the DPFOperatorConfig.
type DPFOperatorConfigReconcilerSettings struct {
	// CustomOVNKubernetesImage the OVN Kubernetes image deployed by the operator
	CustomOVNKubernetesImage string
}

//+kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.dpf.nvidia.com,resources=dpfoperatorconfigs/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=nodes;secrets,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=core,resources=serviceaccounts;configmaps,verbs=get;list;watch;create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments;daemonsets,verbs=get;list;watch;create;patch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;patch
//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;delete

const (
	dpfOperatorConfigControllerName = "dpfoperatorconfig-controller"
)

// Reconcile reconciles changes in a DPFOperatorConfig.
func (r *DPFOperatorConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")
	dpfOperatorConfig := &operatorv1.DPFOperatorConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, dpfOperatorConfig); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if the object is not found.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	//original := dpfOperatorConfig.DeepCopy()
	// Defer a patch call to always patch the object when Reconcile exits.
	defer func() {
		log.Info("Calling defer")
		// TODO: Make this a generic patcher.
		// TODO: There is an issue patching status here with SSA - the finalizer managed field becomes broken and the finalizer can not be removed. Investigate.
		// Set the GVK explicitly for the patch.
		dpfOperatorConfig.SetGroupVersionKind(operatorv1.DPFOperatorConfigGroupVersionKind)
		// Do not include manged fields in the patch call. This does not remove existing fields.
		dpfOperatorConfig.ObjectMeta.ManagedFields = nil
		err := r.Client.Patch(ctx, dpfOperatorConfig, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName))
		reterr = kerrors.NewAggregate([]error{reterr, err})
	}()

	// Handle deletion reconciliation loop.
	if !dpfOperatorConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("Removing")
		return r.reconcileDelete(ctx, dpfOperatorConfig)
	}

	// Add finalizer if not set.
	if !controllerutil.ContainsFinalizer(dpfOperatorConfig, operatorv1.DPFOperatorConfigFinalizer) {
		log.Info("Adding finalizer")
		controllerutil.AddFinalizer(dpfOperatorConfig, operatorv1.DPFOperatorConfigFinalizer)
		return ctrl.Result{}, nil
	}
	return r.reconcile(ctx, dpfOperatorConfig)
}

//nolint:unparam
func (r *DPFOperatorConfigReconciler) reconcileDelete(ctx context.Context, dpfOperatorConfig *operatorv1.DPFOperatorConfig) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpfOperatorConfig, operatorv1.DPFOperatorConfigFinalizer)
	// We should have an ownerReference chain in order to delete subordinate objects.
	return ctrl.Result{}, nil
}

//nolint:unparam
func (r *DPFOperatorConfigReconciler) reconcile(ctx context.Context, dpfOperatorConfig *operatorv1.DPFOperatorConfig) (ctrl.Result, error) {
	// Ensure Custom OVN Kubernetes Deployment is done
	if err := r.reconcileCustomOVNKubernetesDeployment(ctx, dpfOperatorConfig); err != nil {
		// TODO: In future we should tolerate this error, but only when we have status reporting.
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileCustomOVNKubernetesDeployment ensures that custom OVN Kubernetes is deployed
func (r *DPFOperatorConfigReconciler) reconcileCustomOVNKubernetesDeployment(ctx context.Context, dpfOperatorConfig *operatorv1.DPFOperatorConfig) error {
	// Phase 1
	// - ensure cluster version operator is scaled down
	// - ensure network operator is scaled down
	// - ensure webhook is removed
	// - ensure OVN Kubernetes daemonset has different nodeSelector (i.e. point to control plane only)
	if err := r.scaleDownOVNKubernetesComponents(ctx); err != nil {
		return fmt.Errorf("error while scaling down OVN Kubernetes components: %w", err)
	}
	// Phase 2
	// - ensure no original OVN Kubernetes pods runs on worker
	// - remove node annotation k8s.ovn.org/node-chassis-id (avoid removing again on next reconciliation loop, needs status)
	// - ensure DPU CNI Provisioner is deployed
	// - ensure Host CNI provisioner is deployed
	// - ensure both provisioners are ready and have more than 1 pods
	if err := r.cleanupClusterAndDeployCNIProvisioners(ctx, dpfOperatorConfig); err != nil {
		return fmt.Errorf("error while cleaning cluster and deploying CNI provisioners: %w", err)
	}

	// - ensure both provisioners are ready and have more than 1 pods
	// - mark nodes with network preconfiguration ready label
	if err := r.markNetworkPreconfigurationReady(ctx); err != nil {
		return fmt.Errorf("error while marking network preconfiguration as ready: %w", err)
	}
	// Phase 3
	// - deploy custom OVN Kubernetes
	if err := r.deployCustomOVNKubernetes(ctx); err != nil {
		return fmt.Errorf("error deploying custom OVN Kubernetes: %w", err)
	}
	return nil
}

// scaleDownOVNKubernetesComponents scales down pre-existing OVN Kubernetes related components to prepare the cluster
// for subsequent configuration.
func (r *DPFOperatorConfigReconciler) scaleDownOVNKubernetesComponents(ctx context.Context) error {
	// Ensure cluster version operator is scaled down
	clusterVersionOperatorDeployment := &appsv1.Deployment{}
	key := client.ObjectKey{Namespace: clusterVersionOperatorNamespace, Name: clusterVersionOperatorDeploymentName}
	err := r.Client.Get(ctx, key, clusterVersionOperatorDeployment)
	if err != nil {
		return fmt.Errorf("error while getting %s %s: %w", clusterVersionOperatorDeployment.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	clusterVersionOperatorDeployment.Spec.Replicas = ptr.To[int32](0)
	clusterVersionOperatorDeployment.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
	clusterVersionOperatorDeployment.ObjectMeta.ManagedFields = nil
	if err := r.Client.Patch(ctx, clusterVersionOperatorDeployment, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", clusterVersionOperatorDeployment.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}

	// Ensure network operator is scaled down
	networkOperatorDeployment := &appsv1.Deployment{}
	key = client.ObjectKey{Namespace: networkOperatorNamespace, Name: networkOperatorDeploymentName}
	err = r.Client.Get(ctx, key, networkOperatorDeployment)
	if err != nil {
		return fmt.Errorf("error while getting %s %s: %w", networkOperatorDeployment.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	networkOperatorDeployment.Spec.Replicas = ptr.To[int32](0)
	networkOperatorDeployment.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
	networkOperatorDeployment.ObjectMeta.ManagedFields = nil
	if err := r.Client.Patch(ctx, networkOperatorDeployment, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", networkOperatorDeployment.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}

	// Ensure node identity webhook is removed
	nodeIdentityWebhookConfiguration := &admissionregistrationv1.ValidatingWebhookConfiguration{}
	key = client.ObjectKey{Name: nodeIdentityWebhookConfigurationName}
	err = r.Client.Get(ctx, key, nodeIdentityWebhookConfiguration)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error while getting %s %s: %w", nodeIdentityWebhookConfiguration.GetObjectKind().GroupVersionKind().String(), key.String(), err)
		}
	} else {
		if err := r.Client.Delete(ctx, nodeIdentityWebhookConfiguration); err != nil {
			return fmt.Errorf("error while deleting %s %s: %w", nodeIdentityWebhookConfiguration.GetObjectKind().GroupVersionKind().String(), key.String(), err)
		}
	}

	// Ensure OVN Kubernetes daemonset has different nodeSelector (i.e. point to control plane only)
	ovnKubernetesDaemonset := &appsv1.DaemonSet{}
	key = client.ObjectKey{Namespace: ovnKubernetesNamespace, Name: ovnKubernetesDaemonsetName}
	err = r.Client.Get(ctx, key, ovnKubernetesDaemonset)
	if err != nil {
		return fmt.Errorf("error while getting %s %s: %w", ovnKubernetesDaemonset.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	affinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      controlPlaneNodeLabel,
								Operator: corev1.NodeSelectorOpExists,
							},
						},
					},
				},
			},
		},
	}
	ovnKubernetesDaemonset.Spec.Template.Spec.Affinity = affinity
	ovnKubernetesDaemonset.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DaemonSet"))
	ovnKubernetesDaemonset.ObjectMeta.ManagedFields = nil
	if err := r.Client.Patch(ctx, ovnKubernetesDaemonset, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", ovnKubernetesDaemonset.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}

	return nil
}

// cleanupClusterAndDeployCNIProvisioners cleans up cluster level OVN Kubernetes leftovers, deploys CNI provisioners
// that configure the hosts and the DPUs and returns no error when the system has been configured.
func (r *DPFOperatorConfigReconciler) cleanupClusterAndDeployCNIProvisioners(ctx context.Context, dpfOperatorConfig *operatorv1.DPFOperatorConfig) error {
	var errs []error
	// Ensure no original OVN Kubernetes pods runs on worker
	ovnKubernetesDaemonset := &appsv1.DaemonSet{}
	key := client.ObjectKey{Namespace: ovnKubernetesNamespace, Name: ovnKubernetesDaemonsetName}
	err := r.Client.Get(ctx, key, ovnKubernetesDaemonset)
	if err != nil {
		return fmt.Errorf("error while getting %s %s: %w", ovnKubernetesDaemonset.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	if ovnKubernetesDaemonset.Status.NumberMisscheduled != 0 {
		return fmt.Errorf("skipping cleaning up the cluster and deploying CNI provisioners since there are still OVN Kubernetes Pods running on nodes they shouldn't.")
	}

	// Remove node annotation k8s.ovn.org/node-chassis-id
	err = r.cleanupCluster(ctx)
	if err != nil {
		return fmt.Errorf("error while cleaning up cluster: %w", err)
	}

	// Ensure DPU CNI Provisioner is deployed
	dpuCNIProvisionerObjects, err := generateDPUCNIProvisionerObjects(dpfOperatorConfig)
	if err != nil {
		return fmt.Errorf("error while generating DPU CNI provisioner objects: %w", err)
	}
	clusters, err := controlplane.GetDPFClusters(ctx, r.Client)
	if err != nil {
		return fmt.Errorf("error while getting the DPF clusters: %w", err)
	}
	clients := make(map[controlplane.DPFCluster]client.Client)
	for _, c := range clusters {
		cl, err := c.NewClient(ctx, r.Client)
		if err != nil {
			errs = append(errs, fmt.Errorf("error while getting client for cluster %s: %w", c.String(), err))
			continue
		}
		clients[c] = cl
		err = reconcileUnstructuredObjects(ctx, cl, dpuCNIProvisionerObjects)
		if err != nil {
			errs = append(errs, fmt.Errorf("error while reconciling DPU CNI provisioner manifests: %w", err))
		}
	}
	if len(errs) > 0 {
		return kerrors.NewAggregate(errs)
	}

	// Ensure Host CNI provisioner is deployed
	hostCNIProvisionerObjects, err := generateHostCNIProvisionerObjects(dpfOperatorConfig)
	if err != nil {
		return fmt.Errorf("error while generating Host CNI provisioner objects: %w", err)
	}
	err = reconcileUnstructuredObjects(ctx, r.Client, hostCNIProvisionerObjects)
	if err != nil {
		return fmt.Errorf("error while reconciling Host CNI provisioner manifests: %w", err)
	}

	// Ensure both provisioners are ready
	dpuCNIProvisionerDaemonSet := &appsv1.DaemonSet{}
	for _, obj := range dpuCNIProvisionerObjects {
		if obj.GetKind() == "DaemonSet" {
			key = client.ObjectKeyFromObject(obj)
		}
	}
	if key == (client.ObjectKey{}) {
		return fmt.Errorf("skipping cleaning up the cluster and deploying CNI provisioners since there is no DaemonSet in DPU CNI Provisioner objects")
	}
	for _, client := range clients {
		err = client.Get(ctx, key, dpuCNIProvisionerDaemonSet)
		if err != nil {
			errs = append(errs, fmt.Errorf("error while getting %s %s: %w", dpuCNIProvisionerDaemonSet.GetObjectKind().GroupVersionKind().String(), key.String(), err))
			continue
		}
		if !isCNIProvisionerReady(dpuCNIProvisionerDaemonSet) {
			errs = append(errs, errors.New("DPU CNI Provisioner is not yet ready"))
		}
	}
	if len(errs) > 0 {
		return kerrors.NewAggregate(errs)
	}

	hostCNIProvisionerDaemonSet := &appsv1.DaemonSet{}
	for _, obj := range hostCNIProvisionerObjects {
		if obj.GetKind() == "DaemonSet" {
			key = client.ObjectKeyFromObject(obj)
		}
	}
	if key == (client.ObjectKey{}) {
		return fmt.Errorf("skipping cleaning up the cluster and deploying CNI provisioners since there is no DaemonSet in Host CNI Provisioner objects")
	}
	err = r.Client.Get(ctx, key, hostCNIProvisionerDaemonSet)
	if err != nil {
		return fmt.Errorf("error while getting %s %s: %w", hostCNIProvisionerDaemonSet.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	if !isCNIProvisionerReady(hostCNIProvisionerDaemonSet) {
		return errors.New("Host CNI Provisioner is not yet ready")
	}

	return kerrors.NewAggregate(errs)
}

// cleanupCluster cleans up the cluster from original OVN Kubernetes leftovers
func (r *DPFOperatorConfigReconciler) cleanupCluster(ctx context.Context) error {
	var errs []error

	nodes := &corev1.NodeList{}
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(workerNodeLabel, selection.Exists, nil)
	if err != nil {
		return fmt.Errorf("error while creating label selector requirement for label %s: %w", workerNodeLabel, err)
	}
	labelSelector = labelSelector.Add(*req)
	req, err = labels.NewRequirement(networkPreconfigurationReadyNodeLabel, selection.DoesNotExist, nil)
	if err != nil {
		return fmt.Errorf("error while creating label selector requirement for label %s: %w", networkPreconfigurationReadyNodeLabel, err)
	}
	labelSelector = labelSelector.Add(*req)
	err = r.Client.List(ctx, nodes, &client.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("error while listting nodes with selector %s: %w", labelSelector.String(), err)
	}

	for i := range nodes.Items {
		if _, ok := nodes.Items[i].Annotations[ovnKubernetesNodeChassisIDAnnotation]; ok {
			// First we patch to take ownership of the annotation
			nodes.Items[i].Annotations[ovnKubernetesNodeChassisIDAnnotation] = "taking-annotation-ownership"
			nodes.Items[i].SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
			nodes.Items[i].ObjectMeta.ManagedFields = nil
			if err := r.Client.Patch(ctx, &nodes.Items[i], client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
				errs = append(errs, fmt.Errorf("error while patching %s %s: %w", nodes.Items[i].GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&nodes.Items[i]).String(), err))
			}
			// Then we patch to remove the annotation
			delete(nodes.Items[i].Annotations, ovnKubernetesNodeChassisIDAnnotation)
			nodes.Items[i].SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
			nodes.Items[i].ObjectMeta.ManagedFields = nil
			if err := r.Client.Patch(ctx, &nodes.Items[i], client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
				errs = append(errs, fmt.Errorf("error while patching %s %s: %w", nodes.Items[i].GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&nodes.Items[i]).String(), err))
			}
		}
	}

	return kerrors.NewAggregate(errs)
}

// markNetworkPreconfigurationReady marks the cluster as ready to receive the custom OVN Kubernetes
// TODO: Refactor this function to do that per node readiness vs daemonset readiness.
func (r *DPFOperatorConfigReconciler) markNetworkPreconfigurationReady(ctx context.Context) error {
	var errs []error
	nodes := &corev1.NodeList{}
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(workerNodeLabel, selection.Exists, nil)
	if err != nil {
		return fmt.Errorf("error while creating label selector requirement for label %s: %w", workerNodeLabel, err)
	}
	labelSelector = labelSelector.Add(*req)
	err = r.Client.List(ctx, nodes, &client.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("error while listting nodes with selector %s: %w", labelSelector.String(), err)
	}
	for i := range nodes.Items {
		nodes.Items[i].Labels[networkPreconfigurationReadyNodeLabel] = ""
		nodes.Items[i].SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Node"))
		nodes.Items[i].ObjectMeta.ManagedFields = nil
		if err := r.Client.Patch(ctx, &nodes.Items[i], client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
			errs = append(errs, fmt.Errorf("error while patching %s %s: %w", nodes.Items[i].GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&nodes.Items[i]).String(), err))
		}
	}
	return kerrors.NewAggregate(errs)
}

// deployCustomOVNKubernetes reads the relevant OVN Kubernetes objects from the cluster, creates copies of them, adjusts
// them according to https://docs.google.com/document/d/1dvFvG9NR4biWuGnTcee9t6DPAbFKKKJxGi30QuQDjqI/edit#heading=h.6kp1qrhfqf61
// and applies them in the cluster.
// TODO: Sort out owner references. Currently the DPFOperatorConfig is namespaced, and cross namespace ownership is not
// allowed by design.
func (r *DPFOperatorConfigReconciler) deployCustomOVNKubernetes(ctx context.Context) error {
	// Create custom OVN Kubernetes ConfigMap
	ovnKubernetesConfigMap := &corev1.ConfigMap{}
	key := client.ObjectKey{Namespace: ovnKubernetesNamespace, Name: ovnKubernetesConfigMapName}
	if err := r.Client.Get(ctx, key, ovnKubernetesConfigMap); err != nil {
		return fmt.Errorf("error while getting %s %s: %w", ovnKubernetesConfigMap.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	customOVNKubernetesConfigMap, err := generateCustomOVNKubernetesConfigMap(ovnKubernetesConfigMap)
	if err != nil {
		return fmt.Errorf("error while generating custom OVN Kubernetes ConfigMap: %w", err)
	}
	customOVNKubernetesConfigMap.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	customOVNKubernetesConfigMap.ObjectMeta.ManagedFields = nil
	if err := r.Client.Patch(ctx, customOVNKubernetesConfigMap, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", customOVNKubernetesConfigMap.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}

	// Create custom OVN Kubernetes Entrypoint ConfigMap
	ovnKubernetesEntrypointConfigMap := &corev1.ConfigMap{}
	key = client.ObjectKey{Namespace: ovnKubernetesNamespace, Name: ovnKubernetesEntrypointConfigMapName}
	if err := r.Client.Get(ctx, key, ovnKubernetesEntrypointConfigMap); err != nil {
		return fmt.Errorf("error while getting %s %s: %w", ovnKubernetesEntrypointConfigMap.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	customOVNKubernetesEntrypointConfigMap, err := generateCustomOVNKubernetesEntrypointConfigMap(ovnKubernetesEntrypointConfigMap)
	if err != nil {
		return fmt.Errorf("error while generating custom OVN Kubernetes Entrypoint ConfigMap: %w", err)
	}
	customOVNKubernetesEntrypointConfigMap.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	customOVNKubernetesEntrypointConfigMap.ObjectMeta.ManagedFields = nil
	if err := r.Client.Patch(ctx, customOVNKubernetesEntrypointConfigMap, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", customOVNKubernetesEntrypointConfigMap.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}

	// Create custom OVN Kubernetes DaemonSet
	ovnKubernetesDaemonset := &appsv1.DaemonSet{}
	key = client.ObjectKey{Namespace: ovnKubernetesNamespace, Name: ovnKubernetesDaemonsetName}
	if err := r.Client.Get(ctx, key, ovnKubernetesDaemonset); err != nil {
		return fmt.Errorf("error while getting %s %s: %w", ovnKubernetesDaemonset.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	customOVNKubernetesDaemonset, err := generateCustomOVNKubernetesDaemonSet(ovnKubernetesDaemonset, r.Settings.CustomOVNKubernetesImage)
	if err != nil {
		return fmt.Errorf("error while generating custom OVN Kubernetes DaemonSet: %w", err)
	}
	customOVNKubernetesDaemonset.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("DaemonSet"))
	customOVNKubernetesDaemonset.ObjectMeta.ManagedFields = nil
	if err := r.Client.Patch(ctx, customOVNKubernetesDaemonset, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", customOVNKubernetesDaemonset.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DPFOperatorConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1.DPFOperatorConfig{}).
		Complete(r)
}

// reconcileUnstructuredObjects reconciles unstructured objects using the given client.
func reconcileUnstructuredObjects(ctx context.Context, c client.Client, objects []*unstructured.Unstructured) error {
	var errs []error
	for _, obj := range objects {
		key := client.ObjectKeyFromObject(obj)
		obj.SetManagedFields(nil)
		if err := c.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOperatorConfigControllerName)); err != nil {
			errs = append(errs, fmt.Errorf("error while patching %s %s: %w", obj.GetObjectKind().GroupVersionKind().String(), key.String(), err))
		}
	}
	return kerrors.NewAggregate(errs)
}

// checkCNIProvisionerReady checks if the given CNI provisioner is ready
func isCNIProvisionerReady(d *appsv1.DaemonSet) bool {
	return d.Status.DesiredNumberScheduled == d.Status.NumberReady && d.Status.NumberReady > 0
}

// generateCustomOVNKubernetesDaemonSet returns a custom OVN Kubernetes DaemonSet based on the given DaemonSet. Returns
// error if any of configuration is not reflected on the returned object.
// TODO: Set custom image when image is available.
func generateCustomOVNKubernetesDaemonSet(base *appsv1.DaemonSet, customOVNKubernetesImage string) (*appsv1.DaemonSet, error) {
	if base == nil {
		return nil, fmt.Errorf("input is nil")
	}

	dirtyOriginal := base.DeepCopy()
	var errs []error

	// Rename manifest with custom prefix
	out := &appsv1.DaemonSet{}
	out.ObjectMeta = metav1.ObjectMeta{
		Name:      fmt.Sprintf("%s-%s", ovnKubernetesDaemonsetName, customOVNKubernetesResourceNameSuffix),
		Namespace: ovnKubernetesNamespace,
	}
	out.Spec = dirtyOriginal.Spec

	// Adjust labels
	if out.Spec.Selector != nil {
		if v, ok := out.Spec.Selector.MatchLabels["app"]; !ok {
			errs = append(errs, fmt.Errorf("error while settings label selector on the %s DaemonSet: label key `app` doesn't exist", ovnKubernetesDaemonsetName))
		} else {
			out.Spec.Selector.MatchLabels["app"] = fmt.Sprintf("%s-%s", v, customOVNKubernetesResourceNameSuffix)
		}
	}

	if v, ok := out.Spec.Template.Labels["app"]; !ok {
		errs = append(errs, fmt.Errorf("error while settings label in the pod template of the %s DaemonSet: label key `app` doesn't exist", ovnKubernetesDaemonsetName))
	} else {
		out.Spec.Template.Labels["app"] = fmt.Sprintf("%s-%s", v, customOVNKubernetesResourceNameSuffix)

	}

	// Adjust affinity to run only on nodes that have the network preconfiguration done
	affinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      networkPreconfigurationReadyNodeLabel,
								Operator: corev1.NodeSelectorOpExists,
							},
						},
					},
				},
			},
		},
	}
	out.Spec.Template.Spec.Affinity = affinity

	// Configure fake sys volume
	// https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/commit/581bfbd5302195a7b44fd232995568c78d1ae92d
	volumeName := "fake-sys"
	hostPath := "/var/dpf/sys"
	out.Spec.Template.Spec.Volumes = append(out.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: hostPath,
				Type: ptr.To[corev1.HostPathType](corev1.HostPathDirectory),
			},
		},
	})

	var configured bool
	for i, container := range out.Spec.Template.Spec.Containers {
		if container.Name != ovnKubernetesKubeControllerContainerName {
			continue
		}

		out.Spec.Template.Spec.Containers[i].VolumeMounts = append(out.Spec.Template.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: hostPath,
		})

		// https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/commit/57e552e831fef852ae25c24c8e02698130dd19e7
		out.Spec.Template.Spec.Containers[i].Env = append(out.Spec.Template.Spec.Containers[i].Env, corev1.EnvVar{
			Name:  "OVNKUBE_NODE_MGMT_PORT_NETDEV",
			Value: ovnManagementVFName,
		})
		configured = true
		break
	}

	if !configured {
		errs = append(errs, fmt.Errorf("error while configuring volume and env variables for container %s in %s Daemonset: container not found", ovnKubernetesKubeControllerContainerName, ovnKubernetesDaemonsetName))
	}

	// Use custom config maps
	var configuredEntrypointConfigMap bool
	var configuredConfigMap bool
	for i, volume := range out.Spec.Template.Spec.Volumes {
		if volume.Name == ovnKubernetesEntrypointConfigMapName {
			out.Spec.Template.Spec.Volumes[i].ConfigMap.Name = fmt.Sprintf("%s-%s", ovnKubernetesEntrypointConfigMapName, customOVNKubernetesResourceNameSuffix)
			configuredEntrypointConfigMap = true
		}
		if volume.Name == ovnKubernetesConfigMapName {
			out.Spec.Template.Spec.Volumes[i].ConfigMap.Name = fmt.Sprintf("%s-%s", ovnKubernetesConfigMapName, customOVNKubernetesResourceNameSuffix)
			configuredConfigMap = true
		}
	}

	if !configuredEntrypointConfigMap {
		errs = append(errs, fmt.Errorf("error while adjusting volume related to %s ConfigMap in %s Daemonset: volume not found", ovnKubernetesEntrypointConfigMapName, ovnKubernetesDaemonsetName))
	}

	if !configuredConfigMap {
		errs = append(errs, fmt.Errorf("error while adjusting volume related to %s ConfigMap in %s Daemonset: volume not found", ovnKubernetesConfigMapName, ovnKubernetesDaemonsetName))
	}

	// Use custom images
	var configuredKubeControllerImage bool
	var configuredOVNControllerImage bool
	for i, container := range out.Spec.Template.Spec.Containers {
		if container.Name == ovnKubernetesKubeControllerContainerName {
			out.Spec.Template.Spec.Containers[i].Image = customOVNKubernetesImage
			configuredKubeControllerImage = true
		}
		if container.Name == ovnKubernetesOVNControllerContainerName {
			out.Spec.Template.Spec.Containers[i].Image = customOVNKubernetesImage
			configuredOVNControllerImage = true
		}
	}

	if !configuredKubeControllerImage {
		errs = append(errs, fmt.Errorf("error while adjusting image for container %s in %s Daemonset: container not found", ovnKubernetesKubeControllerContainerName, ovnKubernetesDaemonsetName))
	}

	if !configuredOVNControllerImage {
		errs = append(errs, fmt.Errorf("error while adjusting image for container %s in %s Daemonset: container not found", ovnKubernetesOVNControllerContainerName, ovnKubernetesDaemonsetName))
	}

	return out, kerrors.NewAggregate(errs)
}

// generateCustomOVNKubernetesConfigMap returns a custom OVN Kubernetes ConfigMap based on the given ConfigMap. Returns
// error if any of configuration is not reflected on the returned object.
func generateCustomOVNKubernetesConfigMap(base *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	if base == nil {
		return nil, fmt.Errorf("input is nil")
	}

	dirtyOriginal := base.DeepCopy()
	var errs []error

	// Rename manifest with custom prefix
	out := &corev1.ConfigMap{}
	out.ObjectMeta = metav1.ObjectMeta{
		Name:      fmt.Sprintf("%s-%s", ovnKubernetesConfigMapName, customOVNKubernetesResourceNameSuffix),
		Namespace: ovnKubernetesNamespace,
	}
	out.Data = dirtyOriginal.Data

	value, ok := out.Data[ovnKubernetesConfigMapDataKey]
	if !ok {
		return nil, fmt.Errorf("error while trying to get key %s in %s ConfigMap: key doesn't exist", ovnKubernetesConfigMapDataKey, ovnKubernetesConfigMapName)
	}

	// OVN Kubernetes is using gcfg. Unfortunately there is neither support to write back the struct into bytes nor to
	// read the content in a generic type like map[string]interface{}. Therefore, we have to do string manipulation instead
	// which is not as safe.
	// The following patches are applied:
	//   1. https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/commit/0e5a2e5d76b1472a853e081693a9c28ae8a16b5e
	//   2. https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/commit/a8173f89d60949df9b8bd49697ad383db5c47353
	//   3. https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/commit/4ccaa91d8a242386bab7e13f240054257a904f60
	var customConfig strings.Builder
	var foundGatewaySection bool
	var foundDefaultSection bool
	scanner := bufio.NewScanner(strings.NewReader(value))
	for scanner.Scan() {
		line := scanner.Text()
		customConfig.WriteString(line + "\n")
		if strings.Contains(line, "[gateway]") {
			foundGatewaySection = true
			_, err := customConfig.WriteString("next-hop=\"10.237.0.1\"\n")
			if err != nil {
				errs = append(errs, fmt.Errorf("error while writing next-hope setting to gateway section: %w", err))
			}
			_, err = customConfig.WriteString("disable-pkt-mtu-check=true\n")
			if err != nil {
				errs = append(errs, fmt.Errorf("error while writing disable-pkt-mtu-check setting to gateway section: %w", err))
			}
		}
		if strings.Contains(line, "[default]") {
			foundDefaultSection = true
			_, err := customConfig.WriteString("control-ovn-encap-ip-external-id=false\n")
			if err != nil {
				errs = append(errs, fmt.Errorf("error while writing control-ovn-encap-ip-external-id setting to default section: %w", err))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		errs = append(errs, fmt.Errorf("error while reading the content of key %s in ConfigMap %s: %w", ovnKubernetesConfigMapDataKey, ovnKubernetesConfigMapName, err))
	}

	if !foundDefaultSection {
		errs = append(errs, fmt.Errorf("couldn't find default section in %s in ConfigMap %s", ovnKubernetesConfigMapDataKey, ovnKubernetesConfigMapName))
	}

	if !foundGatewaySection {
		errs = append(errs, fmt.Errorf("couldn't find gateway section in %s in ConfigMap %s", ovnKubernetesConfigMapDataKey, ovnKubernetesConfigMapName))
	}

	out.Data[ovnKubernetesConfigMapDataKey] = strings.TrimSuffix(customConfig.String(), "\n")

	return out, kerrors.NewAggregate(errs)
}

// generateCustomOVNKubernetesEntrypointConfigMap returns a custom OVN Kubernetes Entrypoint ConfigMap based on the
// given ConfigMap. Returns error if any of configuration is not reflected on the returned object.
func generateCustomOVNKubernetesEntrypointConfigMap(base *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	if base == nil {
		return nil, fmt.Errorf("input is nil")
	}

	dirtyOriginal := base.DeepCopy()
	var errs []error

	// Rename manifest with custom prefix
	out := &corev1.ConfigMap{}
	out.ObjectMeta = metav1.ObjectMeta{
		Name:      fmt.Sprintf("%s-%s", ovnKubernetesEntrypointConfigMapName, customOVNKubernetesResourceNameSuffix),
		Namespace: ovnKubernetesNamespace,
	}
	out.Data = dirtyOriginal.Data

	value, ok := out.Data[ovnKubernetesEntrypointConfigMapNameDataKey]
	if !ok {
		return nil, fmt.Errorf("error while trying to get key %s in %s ConfigMap: key doesn't exist", ovnKubernetesEntrypointConfigMapNameDataKey, ovnKubernetesEntrypointConfigMapName)
	}

	// Apply the following patches:
	//   1. (configmap - ovnkube-script-lib) https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/commit/2cafd30005ca855acacb9d87220052768f133094
	//   2. (configmap - ovnkube-script-lib) https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/commit/25cc213122e0348ff1f1a275f07066c17f339f81
	value = strings.ReplaceAll(value, "vswitch_dbsock=\"/var/run/openvswitch/db.sock\"", fmt.Sprintf("vswitch_remote=\"%s\"", dpuOVSRemote))
	value = strings.ReplaceAll(value, "unix:${vswitch_dbsock}", "${vswitch_remote}")
	value = strings.ReplaceAll(value, "gateway_mode_flags=\"--gateway-mode shared --gateway-interface br-ex\"", fmt.Sprintf("gateway_mode_flags=\"--gateway-mode shared --gateway-interface %s\"", pfRepresentor))

	out.Data[ovnKubernetesEntrypointConfigMapNameDataKey] = value

	return out, kerrors.NewAggregate(errs)
}

// generateDPUCNIProvisionerObjects generates the DPU CNI Provisioner objects
func generateDPUCNIProvisionerObjects(dpfOperatorConfig *operatorv1.DPFOperatorConfig) ([]*unstructured.Unstructured, error) {
	dpuCNIProvisionerObjects, err := utils.BytesToUnstructured(dpuCNIProvisionerManifestContent)
	if err != nil {
		return nil, fmt.Errorf("error while converting manifests to objects: %w", err)
	}
	config := dpucniprovisionerconfig.DPUCNIProvisionerConfig{
		VTEPIPs: dpfOperatorConfig.Spec.HostNetworkConfiguration.DPUIPs,
	}

	err = populateCNIProvisionerConfigMap(dpuCNIProvisionerObjects, "dpu-cni-provisioner", config)
	if err != nil {
		return nil, fmt.Errorf("error while populating configmap: %w", err)
	}

	return dpuCNIProvisionerObjects, err
}

// generateHostCNIProvisionerObjects generates the Host CNI Provisioner objects
func generateHostCNIProvisionerObjects(dpfOperatorConfig *operatorv1.DPFOperatorConfig) ([]*unstructured.Unstructured, error) {
	hostCNIProvisionerObjects, err := utils.BytesToUnstructured(hostCNIProvisionerManifestContent)
	if err != nil {
		return nil, fmt.Errorf("error while converting manifests to objects: %w", err)
	}

	config := hostcniprovisionerconfig.HostCNIProvisionerConfig{
		PFIPs: dpfOperatorConfig.Spec.HostNetworkConfiguration.HostIPs,
	}

	err = populateCNIProvisionerConfigMap(hostCNIProvisionerObjects, "host-cni-provisioner", config)
	if err != nil {
		return nil, fmt.Errorf("error while populating configmap: %w", err)
	}

	return hostCNIProvisionerObjects, err
}

// populateCNIProvisionerConfigMap populates a ConfigMap object with the provided name that is found in the provided objects
// with the config provided. It mutates the input objects in place.
//
//nolint:goconst
func populateCNIProvisionerConfigMap(objects []*unstructured.Unstructured, configMapName string, config interface{}) error {
	var adjustedConfigMap bool
	for i, o := range objects {
		if !(o.GetKind() == "ConfigMap" && o.GetName() == configMapName) {
			continue
		}

		var configMap corev1.ConfigMap
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, &configMap)
		if err != nil {
			return err
		}

		data, err := json.Marshal(config)
		if err != nil {
			return err
		}

		configMap.Data["config.yaml"] = string(data)
		objects[i].Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&configMap)
		if err != nil {
			return err
		}

		adjustedConfigMap = true
	}

	if !adjustedConfigMap {
		return fmt.Errorf("couldn't find %s configmap in objects", configMapName)
	}

	return nil
}
