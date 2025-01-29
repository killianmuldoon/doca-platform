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
	"fmt"

	dpuservicev1 "github.com/nvidia/doca-platform/api/dpuservice/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// reconcileDPUServiceChain reconciles the DPUServiceChain object created by the DPUDeployment
func reconcileDPUServiceChain(ctx context.Context, c client.Client, dpuDeployment *dpuservicev1.DPUDeployment, dpuNodeLabels map[string]string) error {
	owner := metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)

	dpuServiceChain := generateDPUServiceChain(client.ObjectKeyFromObject(dpuDeployment), owner, dpuDeployment.Spec.ServiceChains)

	// we save the version in the annotations, as this will permit us to retrieve
	// a dpuServiceChain by its version
	dpuServiceChain.Annotations = map[string]string{
		"svc.dpu.nvidia.com/dpuservicechain-version": dpuServiceObjectVersionPlaceholder,
	}

	// Add the node selector to the DPUService only if it is deployed on a dpu cluster
	nodeLabelKey := dpuServiceChainVersionLabelKey
	nodeLabelValue := dpuServiceObjectVersionPlaceholder
	dpuServiceNodeSelector := newDPUServiceObjectLabelSelectorWithOwner(nodeLabelKey, nodeLabelValue, client.ObjectKeyFromObject(dpuDeployment))
	dpuServiceChain.Spec.Template.Spec.NodeSelector = dpuServiceNodeSelector
	dpuNodeLabels[nodeLabelKey] = nodeLabelValue

	if err := c.Patch(ctx, dpuServiceChain, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName)); err != nil {
		return fmt.Errorf("error while patching %s %s: %w", dpuServiceChain.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(dpuServiceChain), err)
	}

	return nil
}

// generateDPUServiceChain generates a DPUServiceChain according to the DPUDeployment
func generateDPUServiceChain(dpuDeploymentNamespacedName types.NamespacedName, owner *metav1.OwnerReference, switches []dpuservicev1.DPUDeploymentSwitch) *dpuservicev1.DPUServiceChain {
	sw := make([]dpuservicev1.Switch, 0, len(switches))

	for _, s := range switches {
		sw = append(sw, convertToSFCSwitch(dpuDeploymentNamespacedName, s))
	}

	dpuServiceChain := &dpuservicev1.DPUServiceChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dpuDeploymentNamespacedName.Name,
			Namespace: dpuDeploymentNamespacedName.Namespace,
			Labels: map[string]string{
				dpuservicev1.ParentDPUDeploymentNameLabel: getParentDPUDeploymentLabelValue(dpuDeploymentNamespacedName),
			},
		},
		Spec: dpuservicev1.DPUServiceChainSpec{
			// TODO: Derive and add cluster selector
			Template: dpuservicev1.ServiceChainSetSpecTemplate{
				Spec: dpuservicev1.ServiceChainSetSpec{
					// TODO: Figure out what to do with NodeSelector
					Template: dpuservicev1.ServiceChainSpecTemplate{
						Spec: dpuservicev1.ServiceChainSpec{
							Switches: sw,
						},
					},
				},
			},
		},
	}
	dpuServiceChain.SetOwnerReferences([]metav1.OwnerReference{*owner})
	dpuServiceChain.ObjectMeta.ManagedFields = nil
	dpuServiceChain.SetGroupVersionKind(dpuservicev1.DPUServiceChainGroupVersionKind)

	return dpuServiceChain
}

// convertToSFCSwitch converts a dpuservicev1.DPUDeploymentSwitch to a dpuservicev1.DPUDeploymentSwitch
func convertToSFCSwitch(dpuDeploymentNamespacedName types.NamespacedName, sw dpuservicev1.DPUDeploymentSwitch) dpuservicev1.Switch {
	o := dpuservicev1.Switch{
		Ports: make([]dpuservicev1.Port, 0, len(sw.Ports)),
	}

	for _, inPort := range sw.Ports {
		outPort := dpuservicev1.Port{}

		if inPort.Service != nil {
			// construct ServiceIfc that references serviceInterface for the service
			if inPort.Service.IPAM != nil {
				outPort.ServiceInterface.IPAM = inPort.Service.IPAM.DeepCopy()
			}
			outPort.ServiceInterface.MatchLabels = map[string]string{
				dpuservicev1.DPFServiceIDLabelKey:  getServiceID(dpuDeploymentNamespacedName, inPort.Service.Name),
				ServiceInterfaceInterfaceNameLabel: inPort.Service.InterfaceName,
			}
		}

		if inPort.ServiceInterface != nil {
			outPort.ServiceInterface = *inPort.ServiceInterface.DeepCopy()
		}

		o.Ports = append(o.Ports, outPort)

	}
	return o
}
