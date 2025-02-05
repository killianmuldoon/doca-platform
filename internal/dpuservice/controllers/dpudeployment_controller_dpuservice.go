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
	"encoding/json"
	"fmt"
	"strings"

	dpuservicev1 "github.com/nvidia/doca-platform/api/dpuservice/v1alpha1"
	"github.com/nvidia/doca-platform/internal/conditions"
	"github.com/nvidia/doca-platform/internal/digest"
	"github.com/nvidia/doca-platform/internal/dpuservice/utils"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// reconcileDPUServices reconciles the DPUServices created by the DPUDeployment
func reconcileDPUServices(
	ctx context.Context,
	c client.Client,
	dpuDeployment *dpuservicev1.DPUDeployment,
	dependencies *dpuDeploymentDependencies,
	dpuNodeLabels map[string]string,
) (ctrl.Result, error) {
	owner := metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)
	requeue := ctrl.Result{}

	existingDPUServices := &dpuservicev1.DPUServiceList{}
	if err := c.List(ctx,
		existingDPUServices,
		client.MatchingLabels{
			dpuservicev1.ParentDPUDeploymentNameLabel: getParentDPUDeploymentLabelValue(client.ObjectKeyFromObject(dpuDeployment)),
		},
		client.InNamespace(dpuDeployment.Namespace)); err != nil {
		return requeue, fmt.Errorf("failed to list existing DPUServices: %w", err)
	}

	existingDPUServicesMap := make(map[string]dpuservicev1.DPUService)
	for _, dpuService := range existingDPUServices.Items {
		existingDPUServicesMap[dpuService.Name] = dpuService
	}

	var errs []error
	// Create or update DPUServices to match what is defined in the DPUDeployment
	for dpuServiceName := range dpuDeployment.Spec.Services {
		serviceConfig := dependencies.DPUServiceConfigurations[dpuServiceName]
		serviceTemplate := dependencies.DPUServiceTemplates[dpuServiceName]
		interfaces := retrieveInterfacesFromDPUServiceConfiguration(dependencies.DPUServiceConfigurations[dpuServiceName], dpuServiceName, dpuDeployment.Name)
		versionDigest := calculateDPUServiceVersionDigest(serviceConfig, serviceTemplate, interfaces)

		newSvc, err := generateDPUService(client.ObjectKeyFromObject(dpuDeployment),
			owner,
			dpuServiceName,
			serviceConfig,
			serviceTemplate,
			interfaces,
			versionDigest,
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to generate DPUService %s: %w", dpuServiceName, err))
			continue
		}

		// filter existing services by name
		currSvc, oldSvcs := getCurrentAndStaleDPUServices(dpuServiceName, versionDigest, existingDPUServices)
		switch {
		case currSvc != nil:
			// we found the current revision based on the digest, there might be old revisions to handle
			req := reconcileCurrentDPUServiceRevision(ctx, c, currSvc, oldSvcs, dpuServiceName, existingDPUServicesMap, dpuNodeLabels, serviceConfig)

			if !req.IsZero() {
				requeue = req
			}
			continue
		case len(oldSvcs) > 0:
			// we have only previous revisions
			req, err := reconcileDPUServiceWithOldRevisions(ctx, c, newSvc, oldSvcs, dpuServiceName, versionDigest, dpuDeployment, serviceConfig, existingDPUServicesMap, dpuNodeLabels)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if !req.IsZero() {
				requeue = req
			}
		default:
			// no previous version, we are creating a new one
			reconcileNewDPUServiceRevision(newSvc, dpuServiceName, versionDigest, serviceConfig, dpuDeployment, dpuNodeLabels)
		}

		if err := c.Patch(ctx, newSvc, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName)); err != nil {
			errs = append(errs, fmt.Errorf("failed to patch DPUService %s: %w", client.ObjectKeyFromObject(newSvc), err))
		}
	}

	// Cleanup the remaining stale DPUServices
	for _, dpuService := range existingDPUServicesMap {
		if err := c.Delete(ctx, &dpuService); err != nil {
			return requeue, fmt.Errorf("failed to delete DPUService %s: %w", client.ObjectKeyFromObject(&dpuService), err)
		}
	}

	return requeue, kerrors.NewAggregate(errs)
}

// reconcileNewDPUServiceRevision reconciles a new revision of the DPUService
// It accepts a new DPUService object and the versionDigest of the DPUService
// It sets the nodeSelector for the DPUService and updates the DPUNodeLabels
func reconcileNewDPUServiceRevision(newDPUService client.Object, name, versionDigest string,
	serviceConfig *dpuservicev1.DPUServiceConfiguration,
	dpuDeployment *dpuservicev1.DPUDeployment,
	dpuNodeLabels map[string]string,
) {
	newSvc := newDPUService.(*dpuservicev1.DPUService)
	dpuServiceNodeSelector := newObjectNodeSelectorWithOwner(getDPUServiceVersionLabelKey(name), versionDigest, client.ObjectKeyFromObject(dpuDeployment))
	if !serviceConfig.Spec.ServiceConfiguration.ShouldDeployInCluster() {
		newSvc.SetServiceDeamonSetNodeSelector(dpuServiceNodeSelector)
	}

	// This is needed in all cases, because we don't know if a dpuSet will be updated or created
	setDPUServiceNodeLabelValue(serviceConfig, name, versionDigest, dpuNodeLabels)

	// TODO: By default when creating a new non-disruptive DPUService
	// we should not cause nodeEffect because of labels
}

// reconcileDPUServiceWithOldRevisions reconciles a new revision of the DPUService and the old revisions
// It accepts a new DPUService object, the old revisions and the versionDigest of the DPUService
// It sets the nodeSelector for the DPUService and updates the DPUNodeLabels. It also pauses the old revisions
func reconcileDPUServiceWithOldRevisions(ctx context.Context, c client.Client, newDPUService client.Object, oldRevisions []client.Object,
	name, versionDigest string,
	dpuDeployment *dpuservicev1.DPUDeployment,
	serviceConfig *dpuservicev1.DPUServiceConfiguration,
	existingDPUServicesMap map[string]dpuservicev1.DPUService,
	dpuNodeLabels map[string]string) (ctrl.Result, error) {
	newSvc := newDPUService.(*dpuservicev1.DPUService)
	dpuServiceNodeSelector := newObjectNodeSelectorWithOwner(getDPUServiceVersionLabelKey(name), versionDigest, client.ObjectKeyFromObject(dpuDeployment))
	nodeLabelValue := versionDigest
	var toRetain []client.Object
	if serviceConfig.Spec.NodeEffect != nil && *serviceConfig.Spec.NodeEffect {
		// the logic regarding the previous revisions is as follows:
		// - pause all previous revisions before creating the new one
		//   - The number of previous are controlled by the revisionHistoryLimit. This limit can be hit when several updates are done in a short period of time
		//     while the previous revisions are still not ready. The previous revisions are always sorted by creation timestamp. If the limit is hit, the oldest
		//     revisions are deleted in order to comply with the limit.
		// - once the youngest revision is ready, we can resume and delete the previous revisions.
		//
		// Hence we never delete old revisions while the current one is not ready, this is to avoid downtime
		toRetain = getRevisionHistoryLimitList(oldRevisions, *dpuDeployment.Spec.RevisionHistoryLimit-1)
		if err := pauseStaleDPUServices(ctx, c, clientObjectToDPUServiceList(toRetain)); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to pause stale %s %s: %w", newSvc.GetObjectKind().GroupVersionKind().String(), newSvc.GetName(), err)
		}

		if !serviceConfig.Spec.ServiceConfiguration.ShouldDeployInCluster() {
			newSvc.SetServiceDeamonSetNodeSelector(dpuServiceNodeSelector)
		}
	} else {
		// just patch the youngest existing service
		toRetain = getRevisionHistoryLimitList(oldRevisions, *dpuDeployment.Spec.RevisionHistoryLimit)
		oldSvc := toRetain[0].(*dpuservicev1.DPUService)
		newSvc.Name = oldSvc.Name
		if !serviceConfig.Spec.ServiceConfiguration.ShouldDeployInCluster() {
			// we are not creating a new service, we are updating the existing one
			// so we need to keep the same `version` and `owned-by-dpudeployment` selector to avoid downtime
			newSvc.SetServiceDeamonSetNodeSelector(&corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{getNodeSelectorTermForDPUServiceVersion(oldSvc.Spec.ServiceDaemonSet.NodeSelector, name)},
			})
			nodeLabelValue = getNodeSelectorDPUServiceVersionValue(oldSvc.Spec.ServiceDaemonSet.NodeSelector, name)
		}
	}

	for _, svc := range toRetain {
		delete(existingDPUServicesMap, svc.GetName())
	}

	// This is needed in all cases, because we don't know if a dpuSet will be updated or created
	setDPUServiceNodeLabelValue(serviceConfig, name, nodeLabelValue, dpuNodeLabels)

	return ctrl.Result{RequeueAfter: reconcileRequeueDuration}, nil
}

// reconcileCurrentDPUServiceRevision reconciles the current revision of the DPUService and the old revisions
// It accepts the current DPUService object, the old revisions and the versionDigest of the DPUService
// It cleans the old revisions and updates the Node Label value
func reconcileCurrentDPUServiceRevision(ctx context.Context, c client.Client, current client.Object, oldRevisions []client.Object,
	name string, existingDPUServicesMap map[string]dpuservicev1.DPUService,
	dpuNodeLabels map[string]string,
	serviceConfig *dpuservicev1.DPUServiceConfiguration,
) ctrl.Result {
	log := ctrllog.FromContext(ctx)
	requeue := ctrl.Result{RequeueAfter: reconcileRequeueDuration}
	currSvc, oldSvcs := current.(*dpuservicev1.DPUService), clientObjectToDPUServiceList(oldRevisions)

	delete(existingDPUServicesMap, currSvc.GetName())

	// if the current revision is still not ready, keep the eventual old revisions and requeue
	// otherwise, clean old revisions
	if conditions.IsTrue(currSvc, conditions.TypeReady) {
		err := cleanStaleDPUServices(ctx, c, oldSvcs)
		if err != nil {
			log.Error(err, "failed to resume stale DPUService")
		}
		// don't requeue if the current revision is ready
		requeue = ctrl.Result{}
	}

	for _, svc := range oldSvcs {
		delete(existingDPUServicesMap, svc.Name)
	}

	// This is needed because we don't know if a dpuSet will be updated or created
	setDPUServiceNodeLabelValue(serviceConfig, name, getNodeSelectorDPUServiceVersionValue(currSvc.Spec.ServiceDaemonSet.NodeSelector, name), dpuNodeLabels)

	// Never patch the current service, it could have stale nodeSelector terms that we don't want to remove
	// The user should not change the dpuservice manually, because the controller will overwrite the changes
	// at some point as part of the upgrade process.
	return requeue
}

// setDPUServiceNodeLabelValue sets the value of the node label for the DPUService
func setDPUServiceNodeLabelValue(serviceConfig *dpuservicev1.DPUServiceConfiguration, name, value string, nodeLabels map[string]string) {
	if !serviceConfig.Spec.ServiceConfiguration.ShouldDeployInCluster() {
		nodeLabels[getDPUServiceVersionLabelKey(name)] = value
	}
}

// getCurrentAndStaleDPUServices returns the current and stale DPUService objects
func getCurrentAndStaleDPUServices(name, currentVersionDigest string, existingDPUServices *dpuservicev1.DPUServiceList) (client.Object, []client.Object) {
	match := []client.Object{}
	var current client.Object
	for _, dpuService := range existingDPUServices.Items {
		if strings.HasPrefix(dpuService.Name, name) || strings.HasSuffix(dpuService.Name, name) {
			if dpuService.Annotations[dpuServiceVersionAnnotationKey] == currentVersionDigest {
				current = &dpuService
			} else {
				match = append(match, &dpuService)
			}
		}
	}
	return current, match
}

// CalculateDPUServiceVersionDigest calculates the digest of the DPUService
func calculateDPUServiceVersionDigest(configuration *dpuservicev1.DPUServiceConfiguration, template *dpuservicev1.DPUServiceTemplate, Interfaces []string) string {
	// The nodeEffect change should not affect the digest
	config := configuration.DeepCopy()
	config.Spec.NodeEffect = nil
	return digest.Short(digest.FromObjects(config.Spec, template.Spec, Interfaces), 10)
}

func getDPUServiceVersionLabelKey(name string) string {
	return fmt.Sprintf("svc.dpu.nvidia.com/dpuservice-%s-version", name)
}

// generateDPUService generates a DPUService according to the DPUDeployment
func generateDPUService(dpuDeploymentNamespacedName types.NamespacedName,
	owner *metav1.OwnerReference,
	name string,
	ServiceConfig *dpuservicev1.DPUServiceConfiguration,
	ServiceTemplate *dpuservicev1.DPUServiceTemplate,
	Interfaces []string,
	versionDigest string,
) (*dpuservicev1.DPUService, error) {

	var ServiceConfigValues, ServiceTemplateValues map[string]interface{}
	if ServiceConfig.Spec.ServiceConfiguration.HelmChart.Values != nil {
		if err := json.Unmarshal(ServiceConfig.Spec.ServiceConfiguration.HelmChart.Values.Raw, &ServiceConfigValues); err != nil {
			return nil, fmt.Errorf("error while unmarshaling ServiceConfig values: %w", err)
		}
	}
	if ServiceTemplate.Spec.HelmChart.Values != nil {
		if err := json.Unmarshal(ServiceTemplate.Spec.HelmChart.Values.Raw, &ServiceTemplateValues); err != nil {
			return nil, fmt.Errorf("error while unmarshaling ServiceTemplate values: %w", err)
		}
	}

	mergedValues := utils.MergeMaps(ServiceConfigValues, ServiceTemplateValues)
	var mergedValuesRawExtension *runtime.RawExtension
	if mergedValues != nil {
		mergedValuesRaw, err := json.Marshal(mergedValues)
		if err != nil {
			return nil, fmt.Errorf("error while marshaling merged ServiceTemplate and ServiceConfig values: %w", err)
		}
		mergedValuesRawExtension = &runtime.RawExtension{Raw: mergedValuesRaw}
	}

	dpuService := &dpuservicev1.DPUService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", name, utilrand.String(randomLength)),
			Namespace: dpuDeploymentNamespacedName.Namespace,
			// we save the version in the annotations, as this will permit us to retrieve
			// a dpuService by its version
			Annotations: map[string]string{
				dpuServiceVersionAnnotationKey: versionDigest,
			},
			Labels: map[string]string{
				dpuservicev1.ParentDPUDeploymentNameLabel: getParentDPUDeploymentLabelValue(dpuDeploymentNamespacedName),
			},
		},
		Spec: dpuservicev1.DPUServiceSpec{
			HelmChart: dpuservicev1.HelmChart{
				Source: ServiceTemplate.Spec.HelmChart.Source,
				Values: mergedValuesRawExtension,
			},
			ServiceID:       ptr.To[string](getServiceID(dpuDeploymentNamespacedName, name)),
			DeployInCluster: ServiceConfig.Spec.ServiceConfiguration.DeployInCluster,
			Interfaces:      Interfaces,
		},
	}

	if ServiceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.Labels != nil ||
		ServiceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations != nil {
		dpuService.Spec.ServiceDaemonSet = &dpuservicev1.ServiceDaemonSetValues{
			Labels:      ServiceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.Labels,
			Annotations: ServiceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations,
		}
	}

	dpuService.SetOwnerReferences([]metav1.OwnerReference{*owner})
	dpuService.ObjectMeta.ManagedFields = nil
	dpuService.SetGroupVersionKind(dpuservicev1.DPUServiceGroupVersionKind)

	return dpuService, nil
}

// cleanStaleDPUServices deletes all the stale DPUService objects that are not needed anymore
// it resumes the reconciliation as well in order for the deletion to take effect
func cleanStaleDPUServices(ctx context.Context, c client.Client, oldSvcs []dpuservicev1.DPUService) error {
	// deletion happens before resume here because we want deletion to take effect immediately.
	// This is the case if the object are marked while the controller is ignoring them
	for _, dpuService := range oldSvcs {
		if err := c.Delete(ctx, &dpuService); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("error while deleting %s %s: %w", dpuService.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&dpuService), err)
			}
		}
	}
	for _, dpuService := range oldSvcs {
		dpuService.Spec.Paused = ptr.To(false)
		if err := c.Patch(ctx, &dpuService, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName)); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("error while patching %s %s: %w", dpuService.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&dpuService), err)
			}
		}
	}
	return nil
}

// getNodeSelectorDPUServiceVersionValue returns the NodeSelectorTerm for the given DPUService version label key
func getNodeSelectorDPUServiceVersionValue(nodeSelector *corev1.NodeSelector, dpuserviceName string) string {
	var version string
	for _, term := range nodeSelector.NodeSelectorTerms {
		for _, req := range term.MatchExpressions {
			if req.Key == getDPUServiceVersionLabelKey(dpuserviceName) {
				version = req.Values[0]
				break
			}
		}
	}
	return version
}

// pauseStaleDPUServices disable reconciliation on previous versions of a DPUService up to the revisionHistoryLimit
// all older versions will be deleted.
// Any used Interfaces will also be released so that they can be used by new revisions.
func pauseStaleDPUServices(ctx context.Context, c client.Client, dpuServices []dpuservicev1.DPUService) error {
	Interfaces := map[types.NamespacedName]struct{}{}
	dpuServiceNames := make(map[string]struct{})
	for _, dpuService := range dpuServices {
		dpuService.Spec.Paused = ptr.To(true)
		dpuService.SetManagedFields(nil)
		if err := c.Patch(ctx, &dpuService, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName)); err != nil {
			return fmt.Errorf("failed to patch DPUService %s: %w", client.ObjectKeyFromObject(&dpuService), err)
		}
		dpuServiceNames[dpuService.Name] = struct{}{}
		// collect Interfaces to release
		for _, intf := range dpuService.Spec.Interfaces {
			Interfaces[types.NamespacedName{Namespace: dpuService.Namespace, Name: intf}] = struct{}{}
		}
	}

	// attempt to release the Interfaces so that they can be used by new revisions
	for intf := range Interfaces {
		dpuServiceInterface := &dpuservicev1.DPUServiceInterface{}
		if err := c.Get(ctx, intf, dpuServiceInterface); err != nil {
			// if the interface is not found, we can ignore it
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to get DPUServiceInterface %s: %w", intf.String(), err)
			}
			continue
		}
		annotations := dpuServiceInterface.GetAnnotations()
		// remove the DPUServiceInterface annotation to release it from the DPUService
		if annotations != nil {
			if _, ok := dpuServiceNames[annotations[dpuservicev1.DPUServiceInterfaceAnnotationKey]]; ok {
				delete(annotations, dpuservicev1.DPUServiceInterfaceAnnotationKey)
				dpuServiceInterface.SetManagedFields(nil)
				dpuServiceInterface.SetGroupVersionKind(dpuservicev1.DPUServiceInterfaceGroupVersionKind)
				if err := c.Update(ctx, dpuServiceInterface); err != nil {
					return fmt.Errorf("failed to update DPUServiceInterface %s: %w", client.ObjectKeyFromObject(dpuServiceInterface), err)
				}
			}
		}
	}
	return nil
}

// getNodeSelectorTermForDPUServiceVersion returns the NodeSelectorTerm for the given DPUService version label key
func getNodeSelectorTermForDPUServiceVersion(nodeSelector *corev1.NodeSelector, dpuserviceName string) corev1.NodeSelectorTerm {
	var target corev1.NodeSelectorTerm
	for _, term := range nodeSelector.NodeSelectorTerms {
		for _, req := range term.MatchExpressions {
			if req.Key == getDPUServiceVersionLabelKey(dpuserviceName) {
				target = term
				break
			}
		}
	}
	return target
}

// generateServiceID generates the serviceID for the child resources of a DPUDeployment
func getServiceID(dpuDeploymentNamespacedName types.NamespacedName, serviceName string) string {
	return fmt.Sprintf("dpudeployment_%s_%s", dpuDeploymentNamespacedName.Name, serviceName)
}

func clientObjectToDPUServiceList(oldRevisions []client.Object) []dpuservicev1.DPUService {
	oldSvcs := make([]dpuservicev1.DPUService, 0, len(oldRevisions))
	for _, svc := range oldRevisions {
		oldSvcs = append(oldSvcs, *svc.(*dpuservicev1.DPUService))
	}
	return oldSvcs
}

func retrieveInterfacesFromDPUServiceConfiguration(dpuServiceConfiguration *dpuservicev1.DPUServiceConfiguration, dpuServiceName, dpuDeploymentName string) []string {
	if len(dpuServiceConfiguration.Spec.Interfaces) == 0 {
		return nil
	}

	interfaces := make([]string, 0, len(dpuServiceConfiguration.Spec.Interfaces))
	for _, serviceInterface := range dpuServiceConfiguration.Spec.Interfaces {
		interfaces = append(interfaces, generateDPUServiceInterfaceName(dpuDeploymentName, dpuServiceName, serviceInterface.Name))
	}

	return interfaces
}
