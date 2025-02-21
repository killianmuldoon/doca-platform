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
	"maps"
	"slices"
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

// reconcileDPUServices reconciles the DPUServices created by the DPUDeployment.
func reconcileDPUServices(
	ctx context.Context,
	c client.Client,
	dpuDeployment *dpuservicev1.DPUDeployment,
	dependencies *dpuDeploymentDependencies,
	interfaceNameByServiceName map[string]interfaceNameToDPUServiceInterfaceName,
	dpuNodeLabels map[string]string,
) (ctrl.Result, error) {
	owner := metav1.NewControllerRef(dpuDeployment, dpuservicev1.DPUDeploymentGroupVersionKind)
	requeue := ctrl.Result{}

	// Get all existing DPUServices
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
	// Create or update DPUServices to match what is defined in the DPUDeployment.
	for dpuServiceName := range dpuDeployment.Spec.Services {
		serviceConfig := dependencies.DPUServiceConfigurations[dpuServiceName]
		serviceTemplate := dependencies.DPUServiceTemplates[dpuServiceName]
		versionDigest := calculateDPUServiceVersionDigest(serviceConfig, serviceTemplate)
		newRevision, err := generateDPUService(client.ObjectKeyFromObject(dpuDeployment),
			owner,
			dpuServiceName,
			serviceConfig,
			serviceTemplate,
			interfaceNameByServiceName[dpuServiceName],
			versionDigest,
		)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to generate DPUService %s: %w", dpuServiceName, err))
			continue
		}

		// filter existing services by name
		currentRevision, oldRevisions := getCurrentAndStaleDPUServices(dpuServiceName, versionDigest, existingDPUServices)
		switch {
		case currentRevision != nil:
			// we found the current revision based on the digest, there might be old revisions to handle
			req := reconcileCurrentDPUServiceRevision(ctx, c, newRevision, currentRevision, oldRevisions, dpuServiceName, serviceConfig, dpuNodeLabels, existingDPUServicesMap)

			if !req.IsZero() {
				requeue = req
			}
		case len(oldRevisions) > 0:
			// we have only previous revisions
			req, err := reconcileDPUServiceWithOldRevisions(ctx, c, newRevision, oldRevisions, dpuServiceName, dpuDeployment, serviceConfig, dpuNodeLabels, existingDPUServicesMap)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if !req.IsZero() {
				requeue = req
			}
		default:
			// no previous version, we are creating a new one
			reconcileNewDPUServiceRevision(newRevision, dpuServiceName, dpuDeployment, serviceConfig, dpuNodeLabels)
		}

		newRevision.SetManagedFields(nil)
		newRevision.SetGroupVersionKind(dpuservicev1.DPUServiceGroupVersionKind)
		if err := c.Patch(ctx, newRevision, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName)); err != nil {
			errs = append(errs, fmt.Errorf("failed to patch DPUService %s: %w", client.ObjectKeyFromObject(newRevision), err))
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

// reconcileNewDPUServiceRevision reconciles a new revision of the DPUService.
// It accepts a new DPUService object and the DPUService name as defined in the DPUDeployment.
// It sets the nodeSelector for the DPUService and updates the DPUNodeLabels.
func reconcileNewDPUServiceRevision(newRevision *dpuservicev1.DPUService, dpuServiceName string,
	dpuDeployment *dpuservicev1.DPUDeployment,
	serviceConfig *dpuservicev1.DPUServiceConfiguration,
	dpuNodeLabels map[string]string,
) {
	dpuServiceNodeSelector := newObjectNodeSelectorWithOwner(getDPUServiceVersionLabelKey(dpuServiceName), newRevision.Name, client.ObjectKeyFromObject(dpuDeployment))
	if !serviceConfig.Spec.ServiceConfiguration.ShouldDeployInCluster() {
		newRevision.SetServiceDeamonSetNodeSelector(dpuServiceNodeSelector)
	}

	// This is needed in all cases, because we don't know if a dpuSet will be updated or created
	// This node label will be used to patch the dpuSets in the same reconcile loop
	setDPUServiceNodeLabelValue(dpuServiceName, newRevision.Name, serviceConfig, dpuNodeLabels)

	// TODO: By default when creating a new non-disruptive DPUService
	// we should not cause nodeEffect because of labels
}

// reconcileDPUServiceWithOldRevisions reconciles a new revision of the DPUService and the old revisions.
// It accepts a new DPUService object,the DPUService name as defined in the DPUDeployment and the old revisions.
// It sets the nodeSelector for the DPUService and updates the DPUNodeLabels. It also pauses the old revisions.
func reconcileDPUServiceWithOldRevisions(ctx context.Context, c client.Client, newRevision *dpuservicev1.DPUService, oldRevisions []client.Object,
	dpuServiceName string,
	dpuDeployment *dpuservicev1.DPUDeployment,
	serviceConfig *dpuservicev1.DPUServiceConfiguration,
	dpuNodeLabels map[string]string,
	existingDPUServicesMap map[string]dpuservicev1.DPUService) (ctrl.Result, error) {
	dpuServiceNodeSelector := newObjectNodeSelectorWithOwner(getDPUServiceVersionLabelKey(dpuServiceName), newRevision.Name, client.ObjectKeyFromObject(dpuDeployment))
	nodeLabelValue := newRevision.Name
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
			return ctrl.Result{}, fmt.Errorf("failed to pause stale %s %s: %w", newRevision.GetObjectKind().GroupVersionKind().String(), newRevision.GetName(), err)
		}

		if !serviceConfig.Spec.ServiceConfiguration.ShouldDeployInCluster() {
			newRevision.SetServiceDeamonSetNodeSelector(dpuServiceNodeSelector)
		}
	} else {
		// just patch the youngest existing service
		toRetain = getRevisionHistoryLimitList(oldRevisions, *dpuDeployment.Spec.RevisionHistoryLimit)
		oldSvc := toRetain[0].(*dpuservicev1.DPUService)
		newRevision.Name = oldSvc.Name
		if !serviceConfig.Spec.ServiceConfiguration.ShouldDeployInCluster() {
			// we are not creating a new service, we are updating the existing one
			// so we need to keep the same `version` and `owned-by-dpudeployment` selector to avoid downtime
			newRevision.SetServiceDeamonSetNodeSelector(&corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{getNodeSelectorTermForDPUServiceVersion(oldSvc.Spec.ServiceDaemonSet.NodeSelector, dpuServiceName)},
			})
			nodeLabelValue = getNodeSelectorDPUServiceVersionValue(oldSvc.Spec.ServiceDaemonSet.NodeSelector, dpuServiceName)
		}
	}

	for _, svc := range toRetain {
		delete(existingDPUServicesMap, svc.GetName())
	}

	// This is needed in all cases, because we don't know if a dpuSet will be updated or created
	// This node label will be used to patch the dpuSets in the same reconcile loop
	setDPUServiceNodeLabelValue(dpuServiceName, nodeLabelValue, serviceConfig, dpuNodeLabels)

	return ctrl.Result{RequeueAfter: reconcileRequeueDuration}, nil
}

// reconcileCurrentDPUServiceRevision reconciles the current revision of the DPUService and the old revisions.
// It accepts the current DPUService object and the old revisions of the DPUService.
// It cleans the old revisions and updates the Node Label value.
func reconcileCurrentDPUServiceRevision(ctx context.Context, c client.Client,
	newRevision *dpuservicev1.DPUService,
	currentRevision client.Object,
	oldRevisions []client.Object,
	dpuServiceName string,
	serviceConfig *dpuservicev1.DPUServiceConfiguration,
	dpuNodeLabels map[string]string,
	existingDPUServicesMap map[string]dpuservicev1.DPUService,
) ctrl.Result {
	log := ctrllog.FromContext(ctx)
	requeue := ctrl.Result{RequeueAfter: reconcileRequeueDuration}
	currentRev, oldRevs := currentRevision.(*dpuservicev1.DPUService), clientObjectToDPUServiceList(oldRevisions)

	// We update the name of the newRevision to the currentRevision so that we can patch the existing object
	newRevision.Name = currentRevision.GetName()

	// We update the newRevision nodeSelector to match the currentRevision nodeSelector since it relies on the revision name
	if !serviceConfig.Spec.ServiceConfiguration.ShouldDeployInCluster() {
		newRevision.SetServiceDeamonSetNodeSelector(&corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{getNodeSelectorTermForDPUServiceVersion(currentRev.Spec.ServiceDaemonSet.NodeSelector, dpuServiceName)},
		})
	}

	// We delete the current revision so that it doesn't get cleaned up
	delete(existingDPUServicesMap, newRevision.GetName())

	// This is needed because we don't know if a dpuSet will be updated or created
	setDPUServiceNodeLabelValue(dpuServiceName, getNodeSelectorDPUServiceVersionValue(currentRev.Spec.ServiceDaemonSet.NodeSelector, dpuServiceName), serviceConfig, dpuNodeLabels)

	// If there are no old revisions, we don't need to enqueue as there are no old revisions that need cleanup.
	if len(oldRevs) == 0 {
		return ctrl.Result{}
	}

	// if the current revision is still not ready, keep the eventual old revisions and requeue
	// otherwise, clean old revisions
	if conditions.IsTrue(currentRev, conditions.TypeReady) {
		err := cleanStaleDPUServices(ctx, c, oldRevs)
		if err != nil {
			log.Error(err, "failed to resume stale DPUService")
		}
		// don't requeue if the current revision is ready
		requeue = ctrl.Result{}
	}

	for _, svc := range oldRevs {
		delete(existingDPUServicesMap, svc.Name)
	}

	return requeue
}

// setDPUServiceNodeLabelValue sets the value of the node label for the DPUService.
func setDPUServiceNodeLabelValue(dpuServiceName, value string, serviceConfig *dpuservicev1.DPUServiceConfiguration, nodeLabels map[string]string) {
	if !serviceConfig.Spec.ServiceConfiguration.ShouldDeployInCluster() {
		nodeLabels[getDPUServiceVersionLabelKey(dpuServiceName)] = value
	}
}

// getCurrentAndStaleDPUServices returns the current and stale DPUService objects.
func getCurrentAndStaleDPUServices(dpuServiceName, currentVersionDigest string, existingDPUServices *dpuservicev1.DPUServiceList) (client.Object, []client.Object) {
	match := []client.Object{}
	var current client.Object
	for _, dpuService := range existingDPUServices.Items {
		if strings.HasPrefix(dpuService.Name, dpuServiceName) || strings.HasSuffix(dpuService.Name, dpuServiceName) {
			if dpuService.Annotations[dpuServiceVersionAnnotationKey] == currentVersionDigest {
				current = &dpuService
			} else {
				match = append(match, &dpuService)
			}
		}
	}
	return current, match
}

// CalculateDPUServiceVersionDigest calculates the digest of the DPUService.
func calculateDPUServiceVersionDigest(configuration *dpuservicev1.DPUServiceConfiguration, template *dpuservicev1.DPUServiceTemplate) string {
	// The nodeEffect change should not affect the digest
	config := configuration.DeepCopy()
	config.Spec.NodeEffect = nil
	return digest.Short(digest.FromObjects(config.Spec, template.Spec), 10)
}

func getDPUServiceVersionLabelKey(name string) string {
	return fmt.Sprintf("svc.dpu.nvidia.com/dpuservice-%s-version", name)
}

// generateDPUService generates a DPUService according to the DPUDeployment.
func generateDPUService(dpuDeploymentNamespacedName types.NamespacedName,
	owner *metav1.OwnerReference,
	name string,
	serviceConfig *dpuservicev1.DPUServiceConfiguration,
	serviceTemplate *dpuservicev1.DPUServiceTemplate,
	interfaces map[string]string,
	versionDigest string,
) (*dpuservicev1.DPUService, error) {

	var serviceConfigValues, serviceTemplateValues map[string]interface{}
	if serviceConfig.Spec.ServiceConfiguration.HelmChart.Values != nil {
		if err := json.Unmarshal(serviceConfig.Spec.ServiceConfiguration.HelmChart.Values.Raw, &serviceConfigValues); err != nil {
			return nil, fmt.Errorf("error while unmarshaling serviceConfig values: %w", err)
		}
	}
	if serviceTemplate.Spec.HelmChart.Values != nil {
		if err := json.Unmarshal(serviceTemplate.Spec.HelmChart.Values.Raw, &serviceTemplateValues); err != nil {
			return nil, fmt.Errorf("error while unmarshaling serviceTemplate values: %w", err)
		}
	}

	mergedValues := utils.MergeMaps(serviceConfigValues, serviceTemplateValues)
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
				Source: serviceTemplate.Spec.HelmChart.Source,
				Values: mergedValuesRawExtension,
			},
			ServiceID:       ptr.To[string](getServiceID(dpuDeploymentNamespacedName, name)),
			DeployInCluster: serviceConfig.Spec.ServiceConfiguration.DeployInCluster,
			Interfaces:      slices.Sorted(maps.Values(interfaces)),
		},
	}

	if serviceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.Labels != nil ||
		serviceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations != nil {
		dpuService.Spec.ServiceDaemonSet = &dpuservicev1.ServiceDaemonSetValues{
			Labels:      serviceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.Labels,
			Annotations: serviceConfig.Spec.ServiceConfiguration.ServiceDaemonSet.Annotations,
		}
	}

	dpuService.SetOwnerReferences([]metav1.OwnerReference{*owner})

	return dpuService, nil
}

// cleanStaleDPUServices deletes all the stale DPUService objects that are not needed anymore.
// It resumes the reconciliation as well in order for the deletion to take effect.
func cleanStaleDPUServices(ctx context.Context, c client.Client, oldRevisions []dpuservicev1.DPUService) error {
	// deletion happens before resume here because we want deletion to take effect immediately.
	// This is the case if the object are marked while the controller is ignoring them
	for _, dpuService := range oldRevisions {
		if err := c.Delete(ctx, &dpuService); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("error while deleting %s %s: %w", dpuService.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&dpuService), err)
			}
		}
	}
	for _, dpuService := range oldRevisions {
		dpuService.Spec.Paused = ptr.To(false)
		dpuService.SetManagedFields(nil)
		dpuService.SetGroupVersionKind(dpuservicev1.DPUServiceGroupVersionKind)
		if err := c.Patch(ctx, &dpuService, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName)); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("error while patching %s %s: %w", dpuService.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(&dpuService), err)
			}
		}
	}
	return nil
}

// getNodeSelectorDPUServiceVersionValue returns the NodeSelectorTerm for the given DPUService version label key.
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

// pauseStaleDPUServices disable reconciliation on previous versions of a DPUService up to the revisionHistoryLimit.
// All older versions will be deleted.
func pauseStaleDPUServices(ctx context.Context, c client.Client, dpuServices []dpuservicev1.DPUService) error {
	for _, dpuService := range dpuServices {
		dpuService.Spec.Paused = ptr.To(true)
		dpuService.SetManagedFields(nil)
		if err := c.Patch(ctx, &dpuService, client.Apply, client.ForceOwnership, client.FieldOwner(dpuDeploymentControllerName)); err != nil {
			return fmt.Errorf("failed to patch DPUService %s: %w", client.ObjectKeyFromObject(&dpuService), err)
		}
	}

	return nil
}

// getNodeSelectorTermForDPUServiceVersion returns the NodeSelectorTerm for the given DPUService version label key.
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

// generateServiceID generates the serviceID for the child resources of a DPUDeployment.
func getServiceID(dpuDeploymentNamespacedName types.NamespacedName, serviceName string) string {
	return fmt.Sprintf("dpudeployment_%s_%s", dpuDeploymentNamespacedName.Name, serviceName)
}

func clientObjectToDPUServiceList(oldRevisions []client.Object) []dpuservicev1.DPUService {
	oldRevs := make([]dpuservicev1.DPUService, 0, len(oldRevisions))
	for _, svc := range oldRevisions {
		oldRevs = append(oldRevs, *svc.(*dpuservicev1.DPUService))
	}
	return oldRevs
}
