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
	"net"
	"slices"
	"strings"
	"time"

	ovnkubernetesoperatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/ovnkubernetesoperator/v1alpha1"
	dpucniprovisionerconfig "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/dpu/config"
	hostcniprovisionerconfig "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/host/config"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/operator/utils"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/ovnkubernetesoperator/consts"

	"github.com/google/go-cmp/cmp"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

const (
	DeploymentKind  = "Deployment"
	DaemonSetKind   = "DaemonSet"
	StatefulSetKind = "StatefulSet"
	ConfigMapKind   = "ConfigMap"
	NodeKind        = "Node"
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
	// ovnKubernetesEntrypointConfigMapScriptKey is the key in the OVN Kubernetes Entrypoint ConfigMap that is populated
	// by the OpenShift Cluster Network Operator. This is a field the DPF Operator makes changes.
	ovnKubernetesEntrypointConfigMapScriptKey = "ovnkube-lib.sh"
	// ovnKubernetesNodeChassisIDAnnotation is an OVN Kubernetes Annotation that we need to cleanup
	// https://github.com/openshift/ovn-kubernetes/blob/release-4.14/go-controller/pkg/util/node_annotations.go#L65-L66
	ovnKubernetesNodeChassisIDAnnotation = "k8s.ovn.org/node-chassis-id"
	// dpuOVSRemote is the OVS remote that the host side can use to configure the OVS on the DPU side.
	// The IP below is the static IP that uses the *Host VF<->DPU SF* communication channel
	// The Port is statically configured by the DPU CNI Provisioner
	// TODO: Consider having common constants across components where needed
	dpuOVSRemote = "tcp:169.254.55.1:8500"

	// customOVNKubernetesResourceNameSuffix is the suffix used in the custom OVN Kubernetes resources this controller is
	// creating.
	customOVNKubernetesResourceNameSuffix = "dpf"
	// networkOperatorNamespace is the Namespace where the cluster-network-operator is deployed in OpenShift
	networkOperatorNamespace = "openshift-network-operator"
	// networkOperatorDeploymentName is the Deployment name of the cluster-network-operator that is deployed in
	// OpenShift
	networkOperatorDeploymentName = "network-operator"
	// clusterVersionCRName is the name of the ClusterVersion CR that is deployed in OpenShift
	clusterVersionCRName = "version"
	// nodeIdentityWebhookConfigurationName is the name of the ValidatingWebhookConfiguration deployed in OpenShift that
	// controls which entities can update the Node object.
	nodeIdentityWebhookConfigurationName = "network-node-identity.openshift.io"
	// clusterConfigConfigMapName is the name of the ConfigMap that contains the cluster configuration in
	// OpenShift.
	clusterConfigConfigMapName = "cluster-config-v1"
	// clusterConfigNamespace is the Namespace where the OpenShift cluster configuration ConfigMap exists.
	clusterConfigNamespace = "kube-system"

	// dpuPCIAddressLabelKey is the node label that indicates the PCI address of the DPU. This information is useful
	// so that we can construct the dpu node name in the tenant cluster. This label is added via the a script that the
	// user preconfigures on the cluster using MachineConfig.
	dpuPCIAddressLabelKey = "feature.node.kubernetes.io/dpu.features-dpu-pciAddress"
	// dpuPFNameLabelKey is the node label that indicates the PF name of the DPU. This label is added via the a script
	// that the user preconfigures on the cluster using MachineConfig.
	dpuPFNameLabelKey = "feature.node.kubernetes.io/dpu.features-dpu-pf-name"
	// provisioningDoneNodeLabelKey is the node label that indicates that a node has its DPU provisioned. This label
	// is the one that the provisioning workflow takes action on.
	provisioningDoneNodeLabelKey = "ovn.dpf.nvidia.com/provisioning-done"
	// networkSetupReadyNodeLabel is the label used to determine when a node is ready to run the custom OVN Kubernetes
	// Pod.
	networkPreconfigurationReadyNodeLabel = "ovn.dpf.nvidia.com/network-preconfig-ready"
)

//go:embed manifests/hostcniprovisioner.yaml
var hostCNIProvisionerManifestContent []byte

//go:embed manifests/dpucniprovisioner.yaml
var dpuCNIProvisionerManifestContent []byte

// TODO: Revisit this blanket RBAC.
// +kubebuilder:rbac:groups=*,resources=*,verbs=*

// DPFOVNKubernetesOperatorConfigReconciler reconciles a DPFOVNKubernetesOperatorConfig object
type DPFOVNKubernetesOperatorConfigReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Settings *DPFOVNKubernetesOperatorConfigReconcilerSettings
}

// DPFOVNKubernetesOperatorConfigReconcilerSettings contains settings related to the DPFOVNKubernetesOperatorConfig.
type DPFOVNKubernetesOperatorConfigReconcilerSettings struct {
	// CustomOVNKubernetesDPUImage the OVN Kubernetes image deployed by the operator to the DPU enabled nodes (workers).
	CustomOVNKubernetesDPUImage string

	// CustomOVNKubernetesNonDPUImage the OVN Kubernetes image deployed by the operator to the non DPU enabled nodes
	// (control plane)
	CustomOVNKubernetesNonDPUImage string

	// WebhookEnabled indicates whether the webhook is enabled
	WebhookEnabled bool
}

const (
	dpfOVNKubernetesOperatorConfigControllerName = "dpfovnkubernetesoperatorconfig-controller"
)

func (r *DPFOVNKubernetesOperatorConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")

	operatorConfig := &ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, operatorConfig); err != nil {
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
		operatorConfig.SetGroupVersionKind(ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfigGroupVersionKind)
		// Do not include manged fields in the patch call. This does not remove existing fields.
		operatorConfig.ObjectMeta.ManagedFields = nil
		err := r.Client.Patch(ctx, operatorConfig, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName))
		reterr = kerrors.NewAggregate([]error{reterr, err})
	}()

	// Handle deletion reconciliation loop.
	if !operatorConfig.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, operatorConfig)
	}

	if err := r.reconcile(ctx, operatorConfig); err != nil {
		log.Error(err, "error while reconciling")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	return ctrl.Result{RequeueAfter: 3 * time.Minute}, nil
}

// reconcile runs the main reconciliation loop
func (r *DPFOVNKubernetesOperatorConfigReconciler) reconcile(ctx context.Context, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig) error {
	if r.Settings.WebhookEnabled {
		if err := r.reconcileNetworkInjectorPrerequisites(ctx, operatorConfig); err != nil {
			return fmt.Errorf("error while reconciling the Network Injector prerequisites: %w", err)
		}
	}

	if err := r.reconcileCustomOVNKubernetesDeployment(ctx, operatorConfig); err != nil {
		return fmt.Errorf("error while reconciling the custom OVN Kubernetes deployment: %w", err)
	}

	return nil
}

func (r *DPFOVNKubernetesOperatorConfigReconciler) reconcileDelete(ctx context.Context, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling delete")
	// We explicitly don't specify a finalizer as this might create issues with the way we deploy the operator via Helm.
	// In particular, the object deployed by Helm must be deleted before the operator is deleted.
	// Today we don't plan to support the deletion flow, so that's fine.
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DPFOVNKubernetesOperatorConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	dpu := &metav1.PartialObjectMetadata{}
	dpu.SetGroupVersionKind(schema.FromAPIVersionAndKind("provisioning.dpf.nvidia.com/v1alpha1", "DPU"))
	return ctrl.NewControllerManagedBy(mgr).
		For(&ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig{}).
		WatchesMetadata(dpu, handler.EnqueueRequestsFromMapFunc(r.generateRequestForConfig)).
		Complete(r)
}

// generateRequestForConfig returns a single request for the DPFOperatorConfig that exists in the cluster if it exists
func (r *DPFOVNKubernetesOperatorConfigReconciler) generateRequestForConfig(ctx context.Context, o client.Object) []ctrl.Request {
	log := ctrllog.FromContext(ctx)
	reqs := []ctrl.Request{}

	operatorConfigList := &ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfigList{}
	if err := r.Client.List(ctx, operatorConfigList); err != nil {
		return nil
	}

	if len(operatorConfigList.Items) != 1 {
		log.Info("expected a single DPFOVNKubernetesOperatorConfig in the cluster")
		return reqs
	}

	reqs = append(reqs, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&operatorConfigList.Items[0])})
	return reqs
}

// reconcileNetworkInjectorPrerequisites reconciles the prerequisites needed by the Network Injector
func (r *DPFOVNKubernetesOperatorConfigReconciler) reconcileNetworkInjectorPrerequisites(ctx context.Context, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig) error {
	if err := validateConfigForNetworkInjector(operatorConfig); err != nil {
		return fmt.Errorf("error while validating the config for Network Injector: %w", err)
	}

	if err := createNetworkInjectorNetAttachDef(ctx, r.Client, operatorConfig); err != nil {
		return fmt.Errorf("error while creating NetworkAttachmentDefinition: %w", err)
	}

	return nil
}

// validateConfigForNetworkInjector validates the DPFOVNKubernetesOperatorConfig for user provided settings required
// by the Network Injector
func validateConfigForNetworkInjector(operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig) error {
	if operatorConfig.Spec.VFResourceName == "" {
		return fmt.Errorf("vfResourceName should have a non empty value")
	}
	return nil
}

// createNetworkInjectorNetAttachDef creates the NetworkAttachmentDefinition needed by the Network Injector webhook
func createNetworkInjectorNetAttachDef(ctx context.Context, c client.Client, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig) error {
	netAttachDef, err := generateNetworkInjectorNetAttachDef(operatorConfig)
	if err != nil {
		return fmt.Errorf("error while generating the object: %w", err)
	}

	if err := reconcileUnstructuredObjects(ctx, c, []*unstructured.Unstructured{netAttachDef}); err != nil {
		return fmt.Errorf("error while reconciling the object: %w", err)
	}
	return nil
}

// generateNetworkInjectorNetAttachDef creates the NetworkAttachmentDefinition needed by the Network Injector webhook
func generateNetworkInjectorNetAttachDef(operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig) (*unstructured.Unstructured, error) {
	netAttachDef := &unstructured.Unstructured{}
	netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	})
	netAttachDef.SetName(consts.NetworkInjectorNetAttachDefName)
	netAttachDef.SetNamespace(operatorConfig.Namespace)
	netAttachDef.SetAnnotations(map[string]string{consts.NetAttachDefResourceNameAnnotation: operatorConfig.Spec.VFResourceName})
	if err := unstructured.SetNestedStringMap(netAttachDef.Object,
		// This value is derived out of the default OVN kubernetes CNI config
		map[string]string{"config": `{"cniVersion":"0.3.1","name":"ovn-kubernetes","type":"ovn-k8s-cni-overlay","ipam":{},"dns":{}}`},
		"spec"); err != nil {
		return nil, fmt.Errorf("error while setting spec: %w", err)
	}

	return netAttachDef, nil
}

// reconcileCustomOVNKubernetesDeployment ensures that custom OVN Kubernetes is deployed
// TODO: Prefetch all the necessary objects and pass them in functions instead of fetching them multiple times as part
// of the reconcile loop
func (r *DPFOVNKubernetesOperatorConfigReconciler) reconcileCustomOVNKubernetesDeployment(ctx context.Context, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig) error {
	if err := markNodesAsInstallable(ctx, r.Client); err != nil {
		return fmt.Errorf("error while marking nodes as installable: %w", err)
	}

	// - ensure cluster version operator doesn't reconcile cluster network operator
	// - ensure network operator is scaled down
	// - ensure node identity webhook is removed
	// - ensure OVN Kubernetes daemonset has different nodeSelector (i.e. point to control plane only)
	if err := scaleDownOVNKubernetesComponents(ctx, r.Client); err != nil {
		return fmt.Errorf("error while scaling down OVN Kubernetes components: %w", err)
	}

	// - ensure no original OVN Kubernetes pods runs on worker
	if err := validateOVNKubernetesComponentsDown(ctx, r.Client); err != nil {
		return fmt.Errorf("error while scaling down OVN Kubernetes components: %w", err)
	}

	// - remove node annotation k8s.ovn.org/node-chassis-id (avoid removing again on next reconciliation loop, needs status)
	if err := cleanupCluster(ctx, r.Client); err != nil {
		return fmt.Errorf("error while cleaning cluster CNI provisioners: %w", err)
	}

	// - ensure DPU CNI Provisioner is deployed
	// - ensure Host CNI provisioner is deployed
	// - ensure both provisioners are ready and have more than 1 pods
	if err := deployCNIProvisioners(ctx, r.Client, operatorConfig); err != nil {
		return fmt.Errorf("error while deploying CNI provisioners: %w", err)
	}

	// - mark nodes with network preconfiguration ready label
	if err := markNetworkPreconfigurationReady(ctx, r.Client); err != nil {
		return fmt.Errorf("error while marking network preconfiguration as ready: %w", err)
	}

	// - deploy custom OVN Kubernetes
	if err := deployCustomOVNKubernetes(ctx, r.Client, operatorConfig, r.Settings.CustomOVNKubernetesDPUImage, r.Settings.CustomOVNKubernetesNonDPUImage); err != nil {
		return fmt.Errorf("error deploying custom OVN Kubernetes: %w", err)
	}

	return nil
}

// markNodesAsInstallable marks the relevant nodes as installable so that the rest of the flow can be executed only for
// those nodes.
func markNodesAsInstallable(ctx context.Context, c client.Client) error {
	var errs []error

	dpus := &unstructured.UnstructuredList{}
	dpus.SetGroupVersionKind(schema.FromAPIVersionAndKind("provisioning.dpf.nvidia.com/v1alpha1", "DPU"))
	if err := c.List(ctx, dpus); err != nil {
		return fmt.Errorf("error while listing %s as unstructured: %w", dpus.GetObjectKind().GroupVersionKind().String(), err)
	}

	nodesWithProvisionedDPU := make(map[string]struct{})
	for _, dpu := range dpus.Items {
		phase, exists, err := unstructured.NestedString(dpu.Object, "status", "phase")
		if err != nil {
			return fmt.Errorf("error while extracting phase from DPU: %w", err)
		}
		if !exists {
			continue
		}
		if phase != "Ready" {
			continue
		}

		nodeName, exists, err := unstructured.NestedString(dpu.Object, "spec", "nodeName")
		if err != nil {
			return fmt.Errorf("error while extracting nodeName from DPU: %w", err)
		}
		if !exists {
			continue
		}

		nodesWithProvisionedDPU[nodeName] = struct{}{}
	}

	nodes := &corev1.NodeList{}
	err := c.List(ctx, nodes)
	if err != nil {
		return fmt.Errorf("error while listing nodes: %w", err)
	}

	for i, node := range nodes.Items {
		if _, ok := nodesWithProvisionedDPU[node.Name]; !ok {
			continue
		}

		if nodes.Items[i].Labels == nil {
			nodes.Items[i].Labels = make(map[string]string)
		}
		nodes.Items[i].Labels[provisioningDoneNodeLabelKey] = ""
		nodes.Items[i].SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(NodeKind))
		nodes.Items[i].ObjectMeta.ManagedFields = nil
		if err := c.Patch(ctx, &nodes.Items[i], client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
			errs = append(errs, fmt.Errorf("error while patching %s %s: %w", nodes.Items[i].GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&nodes.Items[i]).String(), err))
		}
	}
	return kerrors.NewAggregate(errs)
}

// scaleDownOVNKubernetesComponents scales down pre-existing OVN Kubernetes related components to prepare the cluster
// for subsequent configuration.
func scaleDownOVNKubernetesComponents(ctx context.Context, c client.Client) error {
	if err := adjustCVO(ctx, c); err != nil {
		return fmt.Errorf("error while adjusting Cluster Version Operator: %w", err)
	}

	if err := scaleDownNetworkOperator(ctx, c); err != nil {
		return fmt.Errorf("error while scaling down Network Operator: %w", err)
	}

	if err := removeNodeIdentityValidatingWebhookConfiguration(ctx, c); err != nil {
		return fmt.Errorf("error while removing Node Identify Validating Webhook Configuration: %w", err)
	}

	if err := adjustOVNKubernetesDaemonSetNodeSelector(ctx, c); err != nil {
		return fmt.Errorf("error while adjusting OVN Kubernetes DaemonSet Node Selector: %w", err)
	}

	return nil
}

// adjustCVO adjusts the OpenShift Cluster Version Operator to not reconcile the objects we are modifying as part of
// the DPF installation.
func adjustCVO(ctx context.Context, c client.Client) error {
	clusterVersionCR := &unstructured.Unstructured{}
	clusterVersionCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "config.openshift.io",
		Version: "v1",
		Kind:    "ClusterVersion",
	})
	key := client.ObjectKey{Name: clusterVersionCRName}
	err := c.Get(ctx, key, clusterVersionCR)
	if err != nil {
		return fmt.Errorf("error while getting unstructured %s %s: %w", clusterVersionCR.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}

	overrides, found, err := unstructured.NestedSlice(clusterVersionCR.Object, "spec", "overrides")
	if err != nil {
		return fmt.Errorf("error while parsing overrides from unstructured: %w", err)
	}
	if !found {
		overrides = make([]interface{}, 0, 1)
	}

	networkOperatorOverride := map[string]interface{}{
		"kind":      DeploymentKind,
		"group":     "apps",
		"name":      networkOperatorDeploymentName,
		"namespace": networkOperatorNamespace,
		"unmanaged": true,
	}

	for _, override := range overrides {
		if cmp.Equal(override, networkOperatorOverride) {
			return nil
		}
	}

	overrides = append(overrides, networkOperatorOverride)
	err = unstructured.SetNestedSlice(clusterVersionCR.Object, overrides, "spec", "overrides")
	if err != nil {
		return fmt.Errorf("error while setting overrides to unstructured: %w", err)
	}
	clusterVersionCR.SetManagedFields(nil)
	if err := c.Patch(ctx, clusterVersionCR, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", clusterVersionCR.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	return nil
}

// scaleDownNetworkOperator scales down the network operator
func scaleDownNetworkOperator(ctx context.Context, c client.Client) error {
	networkOperatorDeployment := &appsv1.Deployment{}
	key := client.ObjectKey{Namespace: networkOperatorNamespace, Name: networkOperatorDeploymentName}
	err := c.Get(ctx, key, networkOperatorDeployment)
	if err != nil {
		return fmt.Errorf("error while getting %s %s: %w", networkOperatorDeployment.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	networkOperatorDeployment.Spec.Replicas = ptr.To[int32](0)
	networkOperatorDeployment.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind(DeploymentKind))
	networkOperatorDeployment.ObjectMeta.ManagedFields = nil
	if err := c.Patch(ctx, networkOperatorDeployment, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", networkOperatorDeployment.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	return nil
}

// removeNodeIdentityValidatingWebhookConfiguration removes the node identity validating webhook configuration
func removeNodeIdentityValidatingWebhookConfiguration(ctx context.Context, c client.Client) error {
	nodeIdentityWebhookConfiguration := &admissionregistrationv1.ValidatingWebhookConfiguration{}
	key := client.ObjectKey{Name: nodeIdentityWebhookConfigurationName}
	err := c.Get(ctx, key, nodeIdentityWebhookConfiguration)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error while getting %s %s: %w", nodeIdentityWebhookConfiguration.GetObjectKind().GroupVersionKind().String(), key.String(), err)
		}
	} else {
		if err = c.Delete(ctx, nodeIdentityWebhookConfiguration); err != nil {
			return fmt.Errorf("error while deleting %s %s: %w", nodeIdentityWebhookConfiguration.GetObjectKind().GroupVersionKind().String(), key.String(), err)
		}
	}

	return nil
}

// adjustOVNKubernetesDaemonSetNodeSelector adjusts the OVN Kubernetes DaemonSet Node Selector
func adjustOVNKubernetesDaemonSetNodeSelector(ctx context.Context, c client.Client) error {
	ovnKubernetesDaemonset := &appsv1.DaemonSet{}
	key := client.ObjectKey{Namespace: ovnKubernetesNamespace, Name: ovnKubernetesDaemonsetName}
	err := c.Get(ctx, key, ovnKubernetesDaemonset)
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
								Key:      provisioningDoneNodeLabelKey,
								Operator: corev1.NodeSelectorOpDoesNotExist,
							},
						},
					},
				},
			},
		},
	}
	ovnKubernetesDaemonset.Spec.Template.Spec.Affinity = affinity
	ovnKubernetesDaemonset.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind(DaemonSetKind))
	ovnKubernetesDaemonset.ObjectMeta.ManagedFields = nil
	if err := c.Patch(ctx, ovnKubernetesDaemonset, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", ovnKubernetesDaemonset.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	return nil
}

// validateOVNKubernetesComponentsDown validates that specific OVN Kubernetes Components are down.
func validateOVNKubernetesComponentsDown(ctx context.Context, c client.Client) error {
	if err := ensureNoOriginalOVNKubernetesPodsRunning(ctx, c); err != nil {
		return fmt.Errorf("error while ensuring original OVN Kubernetes pods are not running on nodes they shouldn't: %w", err)
	}
	return nil
}

// ensureNoOriginalOVNKubernetesPodsRunning checks whether original OVN Kubernetes pods are still running on nodes they
// shouldn't. Returns an error if that's not the case.
func ensureNoOriginalOVNKubernetesPodsRunning(ctx context.Context, c client.Client) error {
	ovnKubernetesDaemonset := &appsv1.DaemonSet{}
	key := client.ObjectKey{Namespace: ovnKubernetesNamespace, Name: ovnKubernetesDaemonsetName}
	err := c.Get(ctx, key, ovnKubernetesDaemonset)
	if err != nil {
		return fmt.Errorf("error while getting %s %s: %w", ovnKubernetesDaemonset.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	if ovnKubernetesDaemonset.Status.NumberMisscheduled != 0 {
		return errors.New("original OVN Kubernetes pods are still running on nodes they shouldn't.")
	}
	return nil
}

// cleanupCluster cleans up the cluster from original OVN Kubernetes leftovers
func cleanupCluster(ctx context.Context, c client.Client) error {
	if err := removeNodeChassisAnnotations(ctx, c); err != nil {
		return fmt.Errorf("error while removing the node chassis annotation from the nodes: %w", err)
	}
	return nil
}

// removeNodeChassisAnnotations removes the node chassis annotations from the relevant nodes
func removeNodeChassisAnnotations(ctx context.Context, c client.Client) error {
	var errs []error

	nodes := &corev1.NodeList{}
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(provisioningDoneNodeLabelKey, selection.Exists, nil)
	if err != nil {
		return fmt.Errorf("error while creating label selector requirement for label %s: %w", provisioningDoneNodeLabelKey, err)
	}
	labelSelector = labelSelector.Add(*req)
	req, err = labels.NewRequirement(networkPreconfigurationReadyNodeLabel, selection.DoesNotExist, nil)
	if err != nil {
		return fmt.Errorf("error while creating label selector requirement for label %s: %w", networkPreconfigurationReadyNodeLabel, err)
	}
	labelSelector = labelSelector.Add(*req)
	err = c.List(ctx, nodes, &client.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("error while listing nodes with selector %s: %w", labelSelector.String(), err)
	}

	for i := range nodes.Items {
		if _, ok := nodes.Items[i].Annotations[ovnKubernetesNodeChassisIDAnnotation]; ok {
			// First we patch to take ownership of the annotation
			nodes.Items[i].Annotations[ovnKubernetesNodeChassisIDAnnotation] = "taking-annotation-ownership"
			nodes.Items[i].SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(NodeKind))
			nodes.Items[i].ObjectMeta.ManagedFields = nil
			if err := c.Patch(ctx, &nodes.Items[i], client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
				errs = append(errs, fmt.Errorf("error while patching %s %s: %w", nodes.Items[i].GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&nodes.Items[i]).String(), err))
			}
			// Then we patch to remove the annotation
			delete(nodes.Items[i].Annotations, ovnKubernetesNodeChassisIDAnnotation)
			nodes.Items[i].SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(NodeKind))
			nodes.Items[i].ObjectMeta.ManagedFields = nil
			if err := c.Patch(ctx, &nodes.Items[i], client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
				errs = append(errs, fmt.Errorf("error while patching %s %s: %w", nodes.Items[i].GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&nodes.Items[i]).String(), err))
			}
		}
	}

	return kerrors.NewAggregate(errs)

}

// deployCNIProvisioners deploys CNI provisioners that configure the hosts and the DPUs and
// returns no error when the system has been configured.
func deployCNIProvisioners(ctx context.Context, c client.Client, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig) error {
	dpuClusterClients, err := getDPUClusterClients(ctx, c)
	if err != nil {
		return fmt.Errorf("error getting clients for the DPU clusters: %w", err)
	}

	dpuCNIProvisionerObjects, err := deployDPUCNIProvisioner(ctx, c, dpuClusterClients, operatorConfig)
	if err != nil {
		return fmt.Errorf("error while deploying DPU CNI Provisioner: %w", err)
	}

	hostCNIProvisionerObjects, err := deployHostCNIProvisioner(ctx, c, operatorConfig)
	if err != nil {
		return fmt.Errorf("error while deploying Host CNI Provisioner: %w", err)
	}

	// TODO: Move these validations in markNetworkPreconfigurationReady when inventory is implemented and is high level
	// object.
	if err := ensureDPUCNIProvisionerReady(ctx, dpuClusterClients, dpuCNIProvisionerObjects); err != nil {
		return fmt.Errorf("error while checking whether the DPU CNI Provisioner is ready : %w", err)
	}

	if err := ensureHostCNIProvisionerReady(ctx, c, hostCNIProvisionerObjects); err != nil {
		return fmt.Errorf("error while checking whether the Host CNI Provisioner is ready : %w", err)
	}

	return nil
}

// deployDPUCNIProvisioner deploys the DPU CNI provisioner to the DPU clusters
// OpenShift exposes a configmap that is used for the whole installation of the cluster. This configmap contains the
// Host CIDR. Openshift Cluster Network Operator is also using that ConfigMap to extract information it needs. See:
// * https://github.com/openshift/cluster-network-operator/blob/release-4.14/pkg/controller/proxyconfig/controller.go#L188-L195
// * https://github.com/openshift/cluster-network-operator/blob/release-4.14/pkg/util/proxyconfig/no_proxy.go#L70-L78
func deployDPUCNIProvisioner(ctx context.Context, c client.Client, dpuClustersClients map[controlplane.DPFCluster]client.Client, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig) ([]*unstructured.Unstructured, error) {
	var errs []error
	openshiftClusterConfig := &corev1.ConfigMap{}
	key := client.ObjectKey{Namespace: clusterConfigNamespace, Name: clusterConfigConfigMapName}
	if err := c.Get(ctx, key, openshiftClusterConfig); err != nil {
		return nil, fmt.Errorf("error while getting %s %s: %w", openshiftClusterConfig.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}

	hostNodeList := &corev1.NodeList{}
	if err := c.List(ctx, hostNodeList, client.MatchingLabels{provisioningDoneNodeLabelKey: ""}); err != nil {
		return nil, fmt.Errorf("error while listing dpu enabled nodes: %w", err)
	}

	dpuCNIProvisionerObjects, err := generateDPUCNIProvisionerObjects(operatorConfig, openshiftClusterConfig, hostNodeList.Items)
	if err != nil {
		return nil, fmt.Errorf("error while generating DPU CNI provisioner objects: %w", err)
	}

	for _, cl := range dpuClustersClients {
		if err := reconcileUnstructuredObjects(ctx, cl, dpuCNIProvisionerObjects); err != nil {
			errs = append(errs, fmt.Errorf("error while reconciling DPU CNI provisioner manifests: %w", err))
		}
	}
	return dpuCNIProvisionerObjects, kerrors.NewAggregate(errs)
}

// getDPUClusterClients returns a map containing clients to all the DPU clusters
func getDPUClusterClients(ctx context.Context, c client.Client) (map[controlplane.DPFCluster]client.Client, error) {
	var errs []error
	clusters, err := controlplane.GetDPFClusters(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("error while getting the DPF clusters: %w", err)
	}
	clients := make(map[controlplane.DPFCluster]client.Client)
	for _, cluster := range clusters {
		cl, err := cluster.NewClient(ctx, c)
		if err != nil {
			errs = append(errs, fmt.Errorf("error while getting client for cluster %s: %w", cluster.String(), err))
			continue
		}
		clients[cluster] = cl
	}
	return clients, kerrors.NewAggregate(errs)
}

// deployHostCNIProvisioner deploys the Host CNI Provisioner to the Host (OCP) cluster
func deployHostCNIProvisioner(ctx context.Context, c client.Client, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig) ([]*unstructured.Unstructured, error) {
	hostNodeList := &corev1.NodeList{}
	if err := c.List(ctx, hostNodeList, client.MatchingLabels{provisioningDoneNodeLabelKey: ""}); err != nil {
		return nil, fmt.Errorf("error while listing dpu enabled nodes: %w", err)
	}
	hostCNIProvisionerObjects, err := generateHostCNIProvisionerObjects(operatorConfig, hostNodeList.Items)
	if err != nil {
		return nil, fmt.Errorf("error while generating Host CNI provisioner objects: %w", err)
	}
	if err := reconcileUnstructuredObjects(ctx, c, hostCNIProvisionerObjects); err != nil {
		return nil, fmt.Errorf("error while reconciling Host CNI provisioner manifests: %w", err)
	}
	return hostCNIProvisionerObjects, nil
}

// ensureDPUCNIProvisionerReady ensures that the DPU CNI Provisioner is ready. Returns an error otherwise.
func ensureDPUCNIProvisionerReady(ctx context.Context, dpuClustersClients map[controlplane.DPFCluster]client.Client, dpuCNIProvisionerObjects []*unstructured.Unstructured) error {
	var errs []error
	dpuCNIProvisionerDaemonSet := &appsv1.DaemonSet{}
	var key client.ObjectKey
	for _, obj := range dpuCNIProvisionerObjects {
		if obj.GetKind() == DaemonSetKind {
			key = client.ObjectKeyFromObject(obj)
		}
	}
	if key == (client.ObjectKey{}) {
		return fmt.Errorf("skipping cleaning up the cluster and deploying CNI provisioners since there is no DaemonSet in DPU CNI Provisioner objects")
	}
	for _, cl := range dpuClustersClients {
		err := cl.Get(ctx, key, dpuCNIProvisionerDaemonSet)
		if err != nil {
			errs = append(errs, fmt.Errorf("error while getting %s %s: %w", dpuCNIProvisionerDaemonSet.GetObjectKind().GroupVersionKind().String(), key.String(), err))
			continue
		}
		if !isCNIProvisionerReady(dpuCNIProvisionerDaemonSet) {
			errs = append(errs, errors.New("DPU CNI Provisioner is not yet ready"))
		}
	}
	return kerrors.NewAggregate(errs)
}

// ensureHostCNIProvisionerReady ensures that the Host CNI Provisioner is ready. Returns an error otherwise.
func ensureHostCNIProvisionerReady(ctx context.Context, c client.Client, hostCNIProvisionerObjects []*unstructured.Unstructured) error {
	hostCNIProvisionerDaemonSet := &appsv1.DaemonSet{}
	var key client.ObjectKey
	for _, obj := range hostCNIProvisionerObjects {
		if obj.GetKind() == DaemonSetKind {
			key = client.ObjectKeyFromObject(obj)
		}
	}
	if key == (client.ObjectKey{}) {
		return fmt.Errorf("skipping cleaning up the cluster and deploying CNI provisioners since there is no DaemonSet in Host CNI Provisioner objects")
	}
	err := c.Get(ctx, key, hostCNIProvisionerDaemonSet)
	if err != nil {
		return fmt.Errorf("error while getting %s %s: %w", hostCNIProvisionerDaemonSet.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	if !isCNIProvisionerReady(hostCNIProvisionerDaemonSet) {
		return errors.New("Host CNI Provisioner is not yet ready")
	}
	return nil
}

// markNetworkPreconfigurationReady marks the cluster as ready to receive the custom OVN Kubernetes
// TODO: Refactor this function to do that per node readiness vs daemonset readiness.
func markNetworkPreconfigurationReady(ctx context.Context, c client.Client) error {
	var errs []error
	nodes := &corev1.NodeList{}
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(provisioningDoneNodeLabelKey, selection.Exists, nil)
	if err != nil {
		return fmt.Errorf("error while creating label selector requirement for label %s: %w", provisioningDoneNodeLabelKey, err)
	}
	labelSelector = labelSelector.Add(*req)
	err = c.List(ctx, nodes, &client.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return fmt.Errorf("error while listing nodes with selector %s: %w", labelSelector.String(), err)
	}
	for i := range nodes.Items {
		nodes.Items[i].Labels[networkPreconfigurationReadyNodeLabel] = ""
		nodes.Items[i].SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(NodeKind))
		nodes.Items[i].ObjectMeta.ManagedFields = nil
		if err := c.Patch(ctx, &nodes.Items[i], client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
			errs = append(errs, fmt.Errorf("error while patching %s %s: %w", nodes.Items[i].GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&nodes.Items[i]).String(), err))
		}
	}
	return kerrors.NewAggregate(errs)
}

// deployCustomOVNKubernetes reads the relevant OVN Kubernetes objects from the cluster, creates copies of them, adjusts
// them according to https://docs.google.com/document/d/1dvFvG9NR4biWuGnTcee9t6DPAbFKKKJxGi30QuQDjqI/edit#heading=h.6kp1qrhfqf61
// and applies them in the cluster.
func deployCustomOVNKubernetes(ctx context.Context,
	c client.Client,
	operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig,
	customOVNKubernetesDPUImage string,
	customOVNKubernetesNonDPUImage string) error {

	if err := replicateImagePullSecrets(ctx, c, operatorConfig); err != nil {
		return fmt.Errorf("error while replicating image pull secrets: %w", err)
	}

	if err := deployCustomOVNKubernetesForWorkers(ctx, c, operatorConfig, customOVNKubernetesDPUImage); err != nil {
		return fmt.Errorf("error while deploying the custom OVN Kubernetes for worker nodes: %w", err)
	}

	if err := deployCustomOVNKubernetesForControlPlane(ctx, c, operatorConfig, customOVNKubernetesNonDPUImage); err != nil {
		return fmt.Errorf("error while deploying the custom OVN Kubernetes for control plane nodes: %w", err)
	}

	return nil
}

// replicateImagePullSecrets replicates image pull secrets required by OVN Kubernetes
func replicateImagePullSecrets(ctx context.Context, c client.Client, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig) error {
	for _, secret := range operatorConfig.Spec.ImagePullSecrets {
		currentSecret := &corev1.Secret{}
		key := client.ObjectKey{Namespace: operatorConfig.Namespace, Name: secret.Name}
		if err := c.Get(ctx, key, currentSecret); err != nil {
			return fmt.Errorf("error while getting %s %s: %w", currentSecret.GetObjectKind().GroupVersionKind().String(), key.String(), err)
		}

		replicatedSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: ovnKubernetesNamespace, Name: secret.Name}}
		replicatedSecret.Type = currentSecret.Type
		replicatedSecret.Data = currentSecret.Data
		replicatedSecret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))
		replicatedSecret.SetManagedFields(nil)
		if err := c.Patch(ctx, replicatedSecret, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
			return fmt.Errorf("error while patching %s %s: %w", replicatedSecret.GetObjectKind().GroupVersionKind().String(), key.String(), err)
		}
	}

	return nil
}

// deployCustomOVNKubernetesForWorkers generates and deploys the custom OVN Kubernetes components for the worker nodes.
// TODO: Sort out owner references. Currently the DPFOperatorConfig is namespaced, and cross namespace ownership is not
// allowed by design.
func deployCustomOVNKubernetesForWorkers(ctx context.Context, c client.Client, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig, customOVNKubernetesDPUImage string) error {
	if err := deployCustomOVNKubernetesConfigMap(ctx, c); err != nil {
		return fmt.Errorf("error while deploying custom OVN Kubernetes ConfigMap: %w", err)
	}

	if err := deployCustomOVNKubernetesEntrypointConfigMap(ctx, c, operatorConfig); err != nil {
		return fmt.Errorf("error while deploying custom OVN Kubernetes Entrypoint ConfigMap: %w", err)
	}

	if err := deployCustomOVNKubernetesDaemonSet(ctx, c, operatorConfig, customOVNKubernetesDPUImage); err != nil {
		return fmt.Errorf("error while deploying custom OVN Kubernetes ConfigMap: %w", err)
	}
	return nil
}

// deployCustomOVNKubernetesConfigMap deploys the custom OVN Kubernetes ConfigMap needed by the workers
func deployCustomOVNKubernetesConfigMap(ctx context.Context, c client.Client) error {
	ovnKubernetesConfigMap := &corev1.ConfigMap{}
	key := client.ObjectKey{Namespace: ovnKubernetesNamespace, Name: ovnKubernetesConfigMapName}
	if err := c.Get(ctx, key, ovnKubernetesConfigMap); err != nil {
		return fmt.Errorf("error while getting %s %s: %w", ovnKubernetesConfigMap.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	customOVNKubernetesConfigMap, err := generateCustomOVNKubernetesConfigMap(ovnKubernetesConfigMap)
	if err != nil {
		return fmt.Errorf("error while generating custom OVN Kubernetes ConfigMap: %w", err)
	}
	customOVNKubernetesConfigMap.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(ConfigMapKind))
	customOVNKubernetesConfigMap.ObjectMeta.ManagedFields = nil
	if err := c.Patch(ctx, customOVNKubernetesConfigMap, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", customOVNKubernetesConfigMap.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	return nil
}

// deployCustomOVNKubernetesEntrypointConfigMap deploys the custom OVN Kubernetes EntryPoint ConfigMap needed by the workers
func deployCustomOVNKubernetesEntrypointConfigMap(ctx context.Context, c client.Client, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig) error {
	ovnKubernetesEntrypointConfigMap := &corev1.ConfigMap{}
	key := client.ObjectKey{Namespace: ovnKubernetesNamespace, Name: ovnKubernetesEntrypointConfigMapName}
	if err := c.Get(ctx, key, ovnKubernetesEntrypointConfigMap); err != nil {
		return fmt.Errorf("error while getting %s %s: %w", ovnKubernetesEntrypointConfigMap.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}

	hostNodeList := &corev1.NodeList{}
	if err := c.List(ctx, hostNodeList, client.MatchingLabels{provisioningDoneNodeLabelKey: ""}); err != nil {
		return fmt.Errorf("error while listing dpu enabled nodes: %w", err)
	}

	customOVNKubernetesEntrypointConfigMap, err := generateCustomOVNKubernetesEntrypointConfigMap(ovnKubernetesEntrypointConfigMap, operatorConfig, hostNodeList.Items)
	if err != nil {
		return fmt.Errorf("error while generating custom OVN Kubernetes Entrypoint ConfigMap: %w", err)
	}
	customOVNKubernetesEntrypointConfigMap.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(ConfigMapKind))
	customOVNKubernetesEntrypointConfigMap.ObjectMeta.ManagedFields = nil
	if err := c.Patch(ctx, customOVNKubernetesEntrypointConfigMap, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", customOVNKubernetesEntrypointConfigMap.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	return nil
}

// deployCustomOVNKubernetesDaemonSet deploys the custom OVN Kubernetes Daemonset for the workers
func deployCustomOVNKubernetesDaemonSet(ctx context.Context, c client.Client, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig, customOVNKubernetesDPUImage string) error {
	ovnKubernetesDaemonset := &appsv1.DaemonSet{}
	key := client.ObjectKey{Namespace: ovnKubernetesNamespace, Name: ovnKubernetesDaemonsetName}
	if err := c.Get(ctx, key, ovnKubernetesDaemonset); err != nil {
		return fmt.Errorf("error while getting %s %s: %w", ovnKubernetesDaemonset.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	customOVNKubernetesDaemonset, err := generateCustomOVNKubernetesDaemonSet(ovnKubernetesDaemonset, operatorConfig, customOVNKubernetesDPUImage)
	if err != nil {
		return fmt.Errorf("error while generating custom OVN Kubernetes DaemonSet: %w", err)
	}
	customOVNKubernetesDaemonset.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind(DaemonSetKind))
	customOVNKubernetesDaemonset.ObjectMeta.ManagedFields = nil
	if err := c.Patch(ctx, customOVNKubernetesDaemonset, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", customOVNKubernetesDaemonset.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	return nil
}

// deployCustomOVNKubernetesForControlPlane deploys the custom OVN Kubernetes components for the control plane nodes.
// TODO: Check if it makes sense to pack those changes in the place where we adjust the original DaemonSet to run only
// on control plane nodes
func deployCustomOVNKubernetesForControlPlane(ctx context.Context, c client.Client, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig, customOVNKubernetesNonDPUImage string) error {
	ovnKubernetesDaemonset := &appsv1.DaemonSet{}
	key := client.ObjectKey{Namespace: ovnKubernetesNamespace, Name: ovnKubernetesDaemonsetName}
	if err := c.Get(ctx, key, ovnKubernetesDaemonset); err != nil {
		return fmt.Errorf("error while getting %s %s: %w", ovnKubernetesDaemonset.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	if err := adjustDefaultOVNKubernetesDaemonSet(ovnKubernetesDaemonset, operatorConfig, customOVNKubernetesNonDPUImage); err != nil {
		return fmt.Errorf("error adjusting the default OVN Kubernetes component: %w", err)
	}
	ovnKubernetesDaemonset.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind(DaemonSetKind))
	ovnKubernetesDaemonset.ObjectMeta.ManagedFields = nil
	if err := c.Patch(ctx, ovnKubernetesDaemonset, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", ovnKubernetesDaemonset.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}
	return nil
}

// adjustDefaultOVNKubernetesDaemonSet adjusts the default OVN Kubernetes daemonset to ensure it's working as expected
// with the modified OVN Kubernetes running on the worker nodes.
func adjustDefaultOVNKubernetesDaemonSet(ovnKubernetesDaemonset *appsv1.DaemonSet, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig, customOVNKubernetesNonDPUImage string) error {
	var configuredKubeControllerImage bool
	for i, container := range ovnKubernetesDaemonset.Spec.Template.Spec.Containers {
		if container.Name == ovnKubernetesKubeControllerContainerName {
			ovnKubernetesDaemonset.Spec.Template.Spec.Containers[i].Image = customOVNKubernetesNonDPUImage
			configuredKubeControllerImage = true
		}
	}

	if !configuredKubeControllerImage {
		return fmt.Errorf("error while adjusting image for container %s in %s Daemonset: container not found", ovnKubernetesKubeControllerContainerName, ovnKubernetesDaemonsetName)
	}

	// Update image pull secrets
	setDaemonSetImagePullSecrets(ovnKubernetesDaemonset, operatorConfig.Spec.ImagePullSecrets)

	return nil
}

// reconcileUnstructuredObjects reconciles unstructured objects using the given client.
func reconcileUnstructuredObjects(ctx context.Context, c client.Client, objects []*unstructured.Unstructured) error {
	var errs []error
	for _, obj := range objects {
		key := client.ObjectKeyFromObject(obj)
		obj.SetManagedFields(nil)
		if err := c.Patch(ctx, obj, client.Apply, client.ForceOwnership, client.FieldOwner(dpfOVNKubernetesOperatorConfigControllerName)); err != nil {
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
func generateCustomOVNKubernetesDaemonSet(base *appsv1.DaemonSet, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig, customOVNKubernetesImage string) (*appsv1.DaemonSet, error) {
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

		configured = true
		break
	}

	if !configured {
		errs = append(errs, fmt.Errorf("error while configuring volume for container %s in %s Daemonset: container not found", ovnKubernetesKubeControllerContainerName, ovnKubernetesDaemonsetName))
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

	// Update image pull secrets
	setDaemonSetImagePullSecrets(out, operatorConfig.Spec.ImagePullSecrets)

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
	//   1. https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/commit/a8173f89d60949df9b8bd49697ad383db5c47353
	//   2. https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/commit/4ccaa91d8a242386bab7e13f240054257a904f60
	var customConfig strings.Builder
	var foundGatewaySection bool
	var foundDefaultSection bool
	scanner := bufio.NewScanner(strings.NewReader(value))
	for scanner.Scan() {
		line := scanner.Text()
		customConfig.WriteString(line + "\n")
		if strings.Contains(line, "[gateway]") {
			foundGatewaySection = true
			_, err := customConfig.WriteString("disable-pkt-mtu-check=true\n")
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
func generateCustomOVNKubernetesEntrypointConfigMap(base *corev1.ConfigMap, operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig, hostNodes []corev1.Node) (*corev1.ConfigMap, error) {
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

	type dpfInventoryEntry struct {
		HostClusterNodeName string `json:"hostClusterNodeName"`
		Gateway             string `json:"gateway"`
		HostPF0VF0          string `json:"hostPF0VF0"`
		HostPF0             string `json:"hostPF0"`
	}

	dpfInventory := []dpfInventoryEntry{}

	pf0vf0PerHost := getPF0VF0PerHost(hostNodes)
	pf0PerHost := getPF0PerHost(hostNodes)

	for _, h := range operatorConfig.Spec.Hosts {
		o := dpfInventoryEntry{
			HostClusterNodeName: h.HostClusterNodeName,
			Gateway:             h.Gateway,
		}

		pf0vf0, ok := pf0vf0PerHost[h.HostClusterNodeName]
		if !ok {
			continue
		}

		o.HostPF0VF0 = pf0vf0

		pf0, ok := pf0PerHost[h.HostClusterNodeName]
		if !ok {
			continue
		}

		o.HostPF0 = pf0

		dpfInventory = append(dpfInventory, o)
	}

	// Create new field with partial content of the DPFOperatorConfig
	configMapInventoryField := "dpf-inventory.json"
	hostNetConfigBytes, err := json.Marshal(dpfInventory)
	if err != nil {
		return nil, fmt.Errorf("error while converting the internal dpfInventory DTO into bytes: %w", err)
	}

	out.Data[configMapInventoryField] = string(hostNetConfigBytes)

	// Modify the script coming from the OpenShift Network Cluster Operator
	value, ok := out.Data[ovnKubernetesEntrypointConfigMapScriptKey]
	if !ok {
		return nil, fmt.Errorf("error while trying to get key %s in %s ConfigMap: key doesn't exist", ovnKubernetesEntrypointConfigMapScriptKey, ovnKubernetesEntrypointConfigMapName)
	}

	// Apply the following patches:
	//   1. (configmap - ovnkube-script-lib) https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/commit/2cafd30005ca855acacb9d87220052768f133094
	//   2. (configmap - ovnkube-script-lib) https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/commit/25cc213122e0348ff1f1a275f07066c17f339f81
	value = strings.ReplaceAll(value, "vswitch_dbsock=\"/var/run/openvswitch/db.sock\"", fmt.Sprintf("vswitch_remote=\"%s\"", dpuOVSRemote))
	value = strings.ReplaceAll(value, "unix:${vswitch_dbsock}", "${vswitch_remote}")
	// Gateway needs to be specific per node if we have different subnets for each node (routed HBN use case). To achieve
	// that we query the DPFOperatorConfig from the OVN Kubernetes init script (requires RBAC) we anyway overwrite and
	// find the correct gateway based on the host the pod is running on (env variable provided by downward API).
	//
	// This patch, replaces the original patch on the configmap because the configmap is shared with all the OVN Kubernetes
	// instances and can't be parameterized.
	// https://gitlab-master.nvidia.com/vremmas/dpf-dpu-ovs-for-host/-/commit/0e5a2e5d76b1472a853e081693a9c28ae8a16b5e
	//
	// Note: the OVN Kubernetes DaemonSet mounts the ovnkube-script-lib configmap under /ovnkube-lib.
	value = strings.ReplaceAll(value,
		"gateway_mode_flags=\"--gateway-mode shared --gateway-interface br-ex\"",
		fmt.Sprintf("gateway_mode_flags=\"--gateway-mode shared --gateway-interface $(cat /ovnkube-lib/%s | jq -r \".[] | select(.hostClusterNodeName==\\\"${K8S_NODE}\\\").hostPF0\") --gateway-nexthop $(cat /ovnkube-lib/%s | jq -r \".[] | select(.hostClusterNodeName==\\\"${K8S_NODE}\\\").gateway\")\"",
			configMapInventoryField,
			configMapInventoryField))

	// To support different pf0vf0 name per host and also avoid requiring from the user to specify the pf0vf0 of the
	// hosts via the config CR, we have to figure to configure that pf0vf0 per pod using the same mechanism as above.
	value = strings.Replace(value,
		"node_mgmt_port_netdev_flags=",
		fmt.Sprintf("node_mgmt_port_netdev_flags=\"--ovnkube-node-mgmt-port-netdev $(cat /ovnkube-lib/%s | jq -r \".[] | select(.hostClusterNodeName==\\\"${K8S_NODE}\\\").hostPF0VF0\")\"",
			configMapInventoryField), 1)

	out.Data[ovnKubernetesEntrypointConfigMapScriptKey] = value

	return out, kerrors.NewAggregate(errs)
}

// generateDPUCNIProvisionerObjects generates the DPU CNI Provisioner objects
func generateDPUCNIProvisionerObjects(operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig, openshiftClusterConfig *corev1.ConfigMap, hostNodes []corev1.Node) ([]*unstructured.Unstructured, error) {
	dpuCNIProvisionerObjects, err := utils.BytesToUnstructured(dpuCNIProvisionerManifestContent)
	if err != nil {
		return nil, fmt.Errorf("error while converting manifests to objects: %w", err)
	}

	hostCIDR, err := getHostCIDRFromOpenShiftClusterConfig(openshiftClusterConfig)
	if err != nil {
		return nil, fmt.Errorf("error while getting Host CIDR from OpenShift cluster config: %w", err)
	}

	hostToDPUNodeName := getHostToDPUNodeName(hostNodes)
	pf0PerHost := getPF0PerHost(hostNodes)

	config := dpucniprovisionerconfig.DPUCNIProvisionerConfig{
		PerNodeConfig: make(map[string]dpucniprovisionerconfig.PerNodeConfig),
		VTEPCIDR:      operatorConfig.Spec.CIDR,
		HostCIDR:      hostCIDR.String(),
	}
	for _, host := range operatorConfig.Spec.Hosts {
		dpuName, ok := hostToDPUNodeName[host.HostClusterNodeName]
		if !ok {
			continue
		}
		pf0, ok := pf0PerHost[host.HostClusterNodeName]
		if !ok {
			continue
		}

		config.PerNodeConfig[dpuName] = dpucniprovisionerconfig.PerNodeConfig{
			VTEPIP:  host.DPUIP,
			Gateway: host.Gateway,
			HostPF0: pf0,
		}
	}

	err = populateCNIProvisionerConfigMap(dpuCNIProvisionerObjects, "dpu-cni-provisioner", config)
	if err != nil {
		return nil, fmt.Errorf("error while populating configmap: %w", err)
	}

	err = setImagePullSecrets(dpuCNIProvisionerObjects, operatorConfig.Spec.ImagePullSecrets)
	if err != nil {
		return nil, fmt.Errorf("error setting image pull secrets: %w", err)
	}

	return dpuCNIProvisionerObjects, err
}

// getHostToDPUNodeName returns the mapping between the host node name and the dpu node name in the tenant cluster.
func getHostToDPUNodeName(nodes []corev1.Node) map[string]string {
	m := make(map[string]string)
	for _, n := range nodes {
		if pciAddress, ok := n.Labels[dpuPCIAddressLabelKey]; ok {
			// This is a contract with the provisioning team
			m[n.Name] = fmt.Sprintf("%s-%s", n.Name, pciAddress)
		}
	}
	return m
}

// getPF0VF0PerHost returns the mapping between the host node name and the name of the pf0vf0 VF.
func getPF0VF0PerHost(nodes []corev1.Node) map[string]string {
	m := make(map[string]string)
	for _, n := range nodes {
		if pfName, ok := n.Labels[dpuPFNameLabelKey]; ok {
			// This is an assumption. It's not guaranteed that the VF will always look like that and it heavily depends
			// on udev rules and other configuration on the system.
			nameWithoutPF := pfName[:len(pfName)-3]
			vf0Name := nameWithoutPF + "v0"
			m[n.Name] = vf0Name
		}
	}
	return m
}

// getPF0PerHost returns the mapping between the host node name and the name of the pf0 PF.
func getPF0PerHost(nodes []corev1.Node) map[string]string {
	m := make(map[string]string)
	for _, n := range nodes {
		if pfName, ok := n.Labels[dpuPFNameLabelKey]; ok {
			m[n.Name] = pfName
		}
	}
	return m
}

// getHostCIDRFromOpenShiftClusterConfig extracts the Host CIDR from the given OpenShift Cluster Configuration.
func getHostCIDRFromOpenShiftClusterConfig(openshiftClusterConfig *corev1.ConfigMap) (net.IPNet, error) {
	// Unfortunately I couldn't find good documentation for what fields are available to add a link here. The best I could
	// find is this IBM specific (?) documentation:
	// https://docs.openshift.com/container-platform/4.14/installing/installing_ibm_cloud/install-ibm-cloud-installation-workflow.html#additional-install-config-parameters_install-ibm-cloud-installation-workflow
	type machineNetworkEntry struct {
		CIDR string `yaml:"cidr"`
	}
	type installConfig struct {
		Networking struct {
			MachineNetwork []machineNetworkEntry `yaml:"machineNetwork"`
		} `yaml:"networking"`
	}

	var config installConfig
	data, ok := openshiftClusterConfig.Data["install-config"]
	if !ok {
		return net.IPNet{}, errors.New("install-config key is not found in ConfigMap data")
	}

	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
		return net.IPNet{}, fmt.Errorf("error while unmarshalling data into struct: %w", err)
	}

	if len(config.Networking.MachineNetwork) == 0 {
		return net.IPNet{}, errors.New("host CIDR not found in cluster config")
	}

	// We use the first CIDR that we find. If there are clusters with multiple CIDRs defined here, we need to adjust the
	// logic and see how each CIDR is correlated with the primary IP of the node. Ultimately, we want to CIDR that contains
	// the primary IP of the node or else, the encap IP that is set by the OVN Kubernetes for the node.
	cidrRaw := config.Networking.MachineNetwork[0].CIDR
	_, cidr, err := net.ParseCIDR(cidrRaw)
	if err != nil {
		return net.IPNet{}, fmt.Errorf("error while parsing CIDR from %s: %w", cidrRaw, err)
	}

	return *cidr, nil
}

// generateHostCNIProvisionerObjects generates the Host CNI Provisioner objects
func generateHostCNIProvisionerObjects(operatorConfig *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig, hostNodes []corev1.Node) ([]*unstructured.Unstructured, error) {
	hostCNIProvisionerObjects, err := utils.BytesToUnstructured(hostCNIProvisionerManifestContent)
	if err != nil {
		return nil, fmt.Errorf("error while converting manifests to objects: %w", err)
	}

	pf0PerHost := getPF0PerHost(hostNodes)

	config := hostcniprovisionerconfig.HostCNIProvisionerConfig{
		PerNodeConfig: make(map[string]hostcniprovisionerconfig.PerNodeConfig),
	}
	for _, host := range operatorConfig.Spec.Hosts {
		pf0, ok := pf0PerHost[host.HostClusterNodeName]
		if !ok {
			continue
		}

		config.PerNodeConfig[host.HostClusterNodeName] = hostcniprovisionerconfig.PerNodeConfig{
			PFIP:    host.HostIP,
			HostPF0: pf0,
		}
	}

	err = populateCNIProvisionerConfigMap(hostCNIProvisionerObjects, "host-cni-provisioner", config)
	if err != nil {
		return nil, fmt.Errorf("error while populating configmap: %w", err)
	}

	err = setImagePullSecrets(hostCNIProvisionerObjects, operatorConfig.Spec.ImagePullSecrets)
	if err != nil {
		return nil, fmt.Errorf("error setting image pull secrets: %w", err)
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
		if !(o.GetKind() == ConfigMapKind && o.GetName() == configMapName) {
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

// setImagePullSecrets sets the imagePullSecrets field in any relevant unstructured object
func setImagePullSecrets(objects []*unstructured.Unstructured, imagePullSecrets []corev1.LocalObjectReference) error {
	toBeAdded := make([]interface{}, 0, len(imagePullSecrets))
	for _, imagePullSecret := range imagePullSecrets {
		out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&imagePullSecret)
		if err != nil {
			return fmt.Errorf("error converting imagePullSecret to unstructured: %w", err)
		}
		toBeAdded = append(toBeAdded, out)
	}

	for _, obj := range objects {
		switch obj.GetKind() {
		case DeploymentKind, DaemonSetKind, StatefulSetKind:
			currentImagePullSecrets, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "imagePullSecrets")
			if err != nil {
				return fmt.Errorf("error while parsing imagePullSecrets from unstructured %s %s: %w", obj.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(obj), err)
			}
			if !found {
				currentImagePullSecrets = make([]interface{}, 0, len(toBeAdded))
			}
			currentImagePullSecrets = slices.Concat(currentImagePullSecrets, toBeAdded)
			currentImagePullSecrets = slices.CompactFunc(currentImagePullSecrets, func(first, second interface{}) bool {
				one := first.(map[string]interface{})
				two := second.(map[string]interface{})
				return one["Name"] != two["Name"]
			})

			err = unstructured.SetNestedSlice(obj.Object, currentImagePullSecrets, "spec", "template", "spec", "imagePullSecrets")
			if err != nil {
				return fmt.Errorf("error while setting imagePullSecrets to unstructured %s %s: %w", obj.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(obj), err)
			}
		default:
		}
	}
	return nil
}

// setDaemonSetImagePullSecrets sets the imagePullSecrets to the given daemonset
func setDaemonSetImagePullSecrets(daemonset *appsv1.DaemonSet, imagePullSecrets []corev1.LocalObjectReference) {
	daemonset.Spec.Template.Spec.ImagePullSecrets = append(daemonset.Spec.Template.Spec.ImagePullSecrets, imagePullSecrets...)
	daemonset.Spec.Template.Spec.ImagePullSecrets = slices.Compact(daemonset.Spec.Template.Spec.ImagePullSecrets)
}
