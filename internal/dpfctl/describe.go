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

package dpfctl

import (
	"context"
	"fmt"
	"time"

	operatorv1 "github.com/nvidia/doca-platform/api/operator/v1alpha1"
	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type objectScope struct {
	client client.Client
	tree   ObjectTree
	opts   ObjectTreeOptions
}

// TreeDiscovery returns a tree of objects representing the DPF status.
func TreeDiscovery(ctx context.Context, c client.Client, opts ObjectTreeOptions) (*ObjectTree, error) {
	dpfOperatorConfig, err := getDPFOperatorConfig(ctx, c)
	if err != nil {
		return nil, err
	}

	t := NewObjectTree(dpfOperatorConfig, opts)

	scope := objectScope{
		client: c,
		tree:   *t,
		opts:   opts,
	}

	// TODO: add KamajiControlPlane
	if err = addDPUClusters(ctx, scope, dpfOperatorConfig); err != nil {
		return nil, err
	}

	if err = addDPUSets(ctx, scope, dpfOperatorConfig, nil); err != nil {
		return nil, err
	}

	if err = addDPUs(ctx, scope, dpfOperatorConfig, nil); err != nil {
		return nil, err
	}

	return t, nil
}

func getDPFOperatorConfig(ctx context.Context, c client.Client) (*operatorv1.DPFOperatorConfig, error) {
	dpfOperatorConfigList := &operatorv1.DPFOperatorConfigList{}
	if err := c.List(ctx, dpfOperatorConfigList); err != nil {
		return nil, err
	}
	if len(dpfOperatorConfigList.Items) == 0 {
		return nil, fmt.Errorf("DPFOperatorConfig not found")
	}
	if len(dpfOperatorConfigList.Items) > 1 {
		return nil, fmt.Errorf("more than one DPFOperatorConfigs found")
	}
	dpfOperatorConfig := dpfOperatorConfigList.Items[0].DeepCopy()
	dpfOperatorConfig.TypeMeta = metav1.TypeMeta{
		Kind:       operatorv1.DPFOperatorConfigKind,
		APIVersion: operatorv1.GroupVersion.String(),
	}
	return dpfOperatorConfig, nil
}

func addDPUClusters(ctx context.Context, o objectScope, root client.Object) error {
	if !showResource(o.opts.ShowResources, provisioningv1.DPUClusterKind) {
		return nil
	}

	dpuClusterList := &provisioningv1.DPUClusterList{}
	if err := o.client.List(ctx, dpuClusterList); err != nil {
		return err
	}

	addToTree := []client.Object{}
	for _, dpuCluster := range dpuClusterList.Items {
		dpuCluster.TypeMeta = metav1.TypeMeta{
			Kind:       provisioningv1.DPUClusterKind,
			APIVersion: provisioningv1.GroupVersion.String(),
		}
		addToTree = append(addToTree, &dpuCluster)
	}

	o.tree.AddMultipleWithHeader(root, addToTree, "DPUClusters")
	return nil
}

func addDPUSets(ctx context.Context, o objectScope, root client.Object, matchLabels client.MatchingLabels) error {
	if !showResource(o.opts.ShowResources, provisioningv1.DPUSetKind) {
		return nil
	}

	// Override the ShowResources option to show all the resources recursively.
	o.opts.ShowResources = ""

	dpuSetList := &provisioningv1.DPUSetList{}
	if err := o.client.List(ctx, dpuSetList, matchLabels); err != nil {
		return err
	}

	addToTree := []client.Object{}
	for _, dpuSet := range dpuSetList.Items {
		dpuSet.TypeMeta = metav1.TypeMeta{
			Kind:       provisioningv1.DPUSetKind,
			APIVersion: provisioningv1.GroupVersion.String(),
		}
		addToTree = append(addToTree, &dpuSet)

		if err := addDPUs(ctx, o, dpuSet.DeepCopy(), client.MatchingLabels{
			util.DPUSetNameLabel:      dpuSet.Name,
			util.DPUSetNamespaceLabel: dpuSet.Namespace,
		}); err != nil {
			return err
		}
	}

	o.tree.AddMultipleWithHeader(root, addToTree, "DPUSets")
	return nil
}

func addDPUs(ctx context.Context, o objectScope, root client.Object, matchLabels client.MatchingLabels) error {
	if !showResource(o.opts.ShowResources, provisioningv1.DPUKind) {
		return nil
	}

	dpuList := &provisioningv1.DPUList{}
	if err := o.client.List(ctx, dpuList, matchLabels); err != nil {
		return err
	}

	addToTree := []client.Object{}
	for _, dpu := range dpuList.Items {
		if matchLabels == nil && dpu.GetLabels()[util.DPUSetNameLabel] != "" {
			continue
		}
		dpu.TypeMeta = metav1.TypeMeta{
			Kind:       provisioningv1.DPUKind,
			APIVersion: provisioningv1.GroupVersion.String(),
		}

		conds := dpu.GetConditions()

		// TODO: remove this workaround as soon as all conditions gets initialized.
		// get ready condition
		_, readyCondition := util.GetDPUCondition(&dpu.Status, string(provisioningv1.DPUReady))
		if readyCondition != nil {
			addToTree = append(addToTree, &dpu)
			continue
		}
		// Set a fake Ready condition based on the status.Phase.
		dpuStatus := metav1.ConditionFalse
		if dpu.Status.Phase == provisioningv1.DPUReady {
			dpuStatus = metav1.ConditionTrue
		}

		// find newest lastTransitionTime in conditions
		newestLastTransitionTime := metav1.NewTime(time.Time{})
		for _, c := range conds {
			if c.LastTransitionTime.After(newestLastTransitionTime.Time) {
				newestLastTransitionTime = c.LastTransitionTime
			}
		}
		if !dpu.DeletionTimestamp.IsZero() && dpu.DeletionTimestamp.Time.After(newestLastTransitionTime.Time) {
			newestLastTransitionTime = *dpu.DeletionTimestamp
		}
		conds = append(conds, metav1.Condition{
			Type:               "Ready",
			Status:             dpuStatus,
			LastTransitionTime: newestLastTransitionTime,
			Reason:             string(dpu.Status.Phase),
		})
		dpu.SetConditions(conds)
		addToTree = append(addToTree, &dpu)
	}

	// If matchLabels is nil, it means that the DPUs are not part of a DPUSet.
	if matchLabels == nil {
		o.tree.AddMultipleWithHeader(root, addToTree, "DPUs")
		return nil
	}

	for _, dpu := range addToTree {
		o.tree.Add(root, dpu)
	}

	return nil
}
