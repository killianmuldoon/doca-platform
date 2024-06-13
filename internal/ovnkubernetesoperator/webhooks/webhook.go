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

package webhooks

import (
	"context"
	"fmt"

	ovnkubernetesoperatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/ovnkubernetesoperator/v1alpha1"
	consts "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/ovnkubernetesoperator/consts"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

type NetworkInjector struct {
	Client client.Reader
}

var (
	// annotationKeyToBeInjected is the multus annotation we inject to the pods so that multus can inject the VFs
	annotationKeyToBeInjected = "v1.multus-cni.io/default-network"

	controlPlaneNodeLabels = []string{
		"node-role.kubernetes.io/master",
		"node-role.kubernetes.io/control-plane",
	}
)

var _ webhook.CustomDefaulter = &NetworkInjector{}

// +kubebuilder:webhook:path=/mutate--v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create,versions=v1,name=network-injector.dpf.nvidia.com,admissionReviewVersions=v1

func (webhook *NetworkInjector) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Pod{}).
		WithDefaulter(webhook).
		Complete()
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (webhook *NetworkInjector) Default(ctx context.Context, obj runtime.Object) error {
	log := ctrl.LoggerFrom(ctx)

	configs := &ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfigList{}
	if err := webhook.Client.List(ctx, configs); err != nil {
		return fmt.Errorf("error while listing DPFOVNKubernetesOperatorConfigs: %w", err)
	}

	config, err := getConfig(configs)
	if err != nil {
		log.Error(err, "bad configuration, skipping mutation")
		return nil
	}

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a Pod but got a %T", obj))
	}

	ctrl.LoggerInto(ctx, log.WithValues("Pod", types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}))
	// If the pod is on the host network no-op.
	if pod.Spec.HostNetwork {
		return nil
	}

	// If the pod explicitly selects a control plane node no-op.
	controlPlanePod, err := webhook.isScheduledToControlPlane(ctx, pod)
	if err != nil {
		return err
	}
	if controlPlanePod {
		return nil
	}

	vfResourceName, err := getVFResourceName(ctx, webhook.Client, config.Namespace)
	if err != nil {
		return fmt.Errorf("error while getting VF resource name: %w", err)
	}

	return injectNetworkResources(ctx, pod, config.Namespace, vfResourceName)
}

// getVFResourceName gets the resource name that relates to the VFs that should be injected.
func getVFResourceName(ctx context.Context, c client.Reader, netAttachDefNamespace string) (corev1.ResourceName, error) {
	netAttachDef := &unstructured.Unstructured{}
	netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	})
	key := client.ObjectKey{Namespace: netAttachDefNamespace, Name: consts.NetworkInjectorNetAttachDefName}
	if err := c.Get(ctx, key, netAttachDef); err != nil {
		return "", fmt.Errorf("error while getting %s %s: %w", netAttachDef.GetObjectKind().GroupVersionKind().String(), key.String(), err)
	}

	if v, ok := netAttachDef.GetAnnotations()[consts.NetAttachDefResourceNameAnnotation]; ok {
		return corev1.ResourceName(v), nil
	}

	return "", fmt.Errorf("resource can't be found in network attachment definition because annotation %s doesn't exist", consts.NetAttachDefResourceNameAnnotation)
}

// If the pod has nodeAffinity set to a specific name check if the node it's scheduled to is a control plane node.
// This is the case for pods created by DaemonSets which set node affinity matching to a node name on creation.
func (webhook *NetworkInjector) isScheduledToControlPlane(ctx context.Context, pod *corev1.Pod) (bool, error) {
	// If the pod has a control plane node selector no-op.
	for _, label := range controlPlaneNodeLabels {
		if _, ok := pod.Spec.NodeSelector[label]; ok {
			return true, nil
		}
	}

	var nodeName string
	if pod.Spec.Affinity != nil &&
		pod.Spec.Affinity.NodeAffinity != nil &&
		pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil &&
		pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms != nil {
		terms := pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
		for _, term := range terms {
			if term.MatchFields != nil {
				for _, field := range term.MatchFields {
					if field.Key == "metadata.name" {
						nodeName = field.Values[0]
					}
				}
			}
		}
	}
	if nodeName == "" {
		return false, nil
	}
	node := &corev1.Node{}
	if err := webhook.Client.Get(ctx, client.ObjectKey{Namespace: "", Name: nodeName}, node); err != nil {
		return false, fmt.Errorf("failed to get node pod is scheduled to %q: %w", pod.Spec.NodeName, err)
	}
	if node.Labels == nil {
		return false, nil
	}

	for _, label := range controlPlaneNodeLabels {
		if _, ok := node.Labels[label]; ok {
			return true, nil
		}
	}
	return false, nil
}

// getConfig returns the correct DPFOVNKubernetesOperatorConfig from the given list. Returns error if bad configuration
// is found.
func getConfig(configs *ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfigList) (*ovnkubernetesoperatorv1.DPFOVNKubernetesOperatorConfig, error) {
	// If no config exists the webhook is disabled.
	if len(configs.Items) == 0 {
		return nil, fmt.Errorf("no DPFOVNKubernetesOperatorConfig found")
	}

	// If more than one config exists the webhook is disabled as we are in an invalid state.
	if len(configs.Items) > 1 {
		return nil, fmt.Errorf("multiple DPFOVNKubernetesOperatorConfigs found")
	}

	return &configs.Items[0], nil
}

func injectNetworkResources(ctx context.Context, pod *corev1.Pod, netAttachDefNamespace string, vfResourceName corev1.ResourceName) error {
	log := ctrl.LoggerFrom(ctx)
	// Inject device requests. One additional VF.
	if pod.Spec.Containers[0].Resources.Requests == nil {
		pod.Spec.Containers[0].Resources.Requests = corev1.ResourceList{}
	}
	if pod.Spec.Containers[0].Resources.Limits == nil {
		pod.Spec.Containers[0].Resources.Limits = corev1.ResourceList{}

	}
	if _, ok := pod.Spec.Containers[0].Resources.Requests[vfResourceName]; ok {
		res := pod.Spec.Containers[0].Resources.Requests[vfResourceName]
		res.Add(resource.MustParse("1"))
		pod.Spec.Containers[0].Resources.Requests[vfResourceName] = res
	} else {
		pod.Spec.Containers[0].Resources.Requests[vfResourceName] = resource.MustParse("1")
	}

	if _, ok := pod.Spec.Containers[0].Resources.Limits[vfResourceName]; ok {
		res := pod.Spec.Containers[0].Resources.Limits[vfResourceName]
		res.Add(resource.MustParse("1"))
		pod.Spec.Containers[0].Resources.Limits[vfResourceName] = res
	} else {
		pod.Spec.Containers[0].Resources.Limits[vfResourceName] = resource.MustParse("1")
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[annotationKeyToBeInjected] = fmt.Sprintf("%s/%s", netAttachDefNamespace, consts.NetworkInjectorNetAttachDefName)
	log.Info(fmt.Sprintf("injected resource %v into pod", vfResourceName))
	return nil
}
