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
	"context"
	"fmt"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"
	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application/v1alpha1"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane/metadata"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/inventory"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// The DPFOperatorConfig reconciler is responsible for ensuring that the entire DPF system is uninstalled on deletion.
// This includes resources which may have been installed by other controllers.
//
//nolint:unparam
func (r *DPFOperatorConfigReconciler) reconcileDelete(ctx context.Context, dpfOperatorConfig *operatorv1.DPFOperatorConfig) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling delete")
	var errs []error
	vars := getVariablesFromConfig(dpfOperatorConfig)
	// also need to ensure argoCD components are deleted.

	log.Info("Ensuring DPF system DPUServices are deleted")
	// Delete DPUServices for system components deployed to the DPU cluster.
	if err := r.deleteObjects(ctx, r.Inventory.ServiceFunctionChainSet, vars); err != nil {
		errs = append(errs, err)
	}
	if err := r.deleteObjects(ctx, r.Inventory.SRIOVDevicePlugin, vars); err != nil {
		errs = append(errs, err)
	}
	if err := r.deleteObjects(ctx, r.Inventory.Multus, vars); err != nil {
		errs = append(errs, err)
	}
	if err := r.deleteObjects(ctx, r.Inventory.Flannel, vars); err != nil {
		errs = append(errs, err)
	}
	if err := r.deleteObjects(ctx, r.Inventory.NvIPAM, vars); err != nil {
		errs = append(errs, err)
	}
	if err := r.deleteObjects(ctx, r.Inventory.OvsCni, vars); err != nil {
		errs = append(errs, err)
	}
	if err := r.deleteObjects(ctx, r.Inventory.SfcController, vars); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		log.Error(kerrors.NewAggregate(errs), "DPF System DPUServices not yet deleted: Requeueing.")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	log.Info("Ensuring DPF Custom Resources are deleted")
	// Delete all instances of every resource that the operator is responsible for.
	if err := r.deleteCustomResources(ctx); err != nil {
		log.Error(err, "Custom resources not yet deleted: Requeueing.")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	log.Info("Ensuring DPF system components are deleted")
	// Delete objects for components deployed to the management cluster.
	if err := r.deleteObjects(ctx, r.Inventory.DPUService, vars); err != nil {
		errs = append(errs, err)
	}
	if err := r.deleteObjects(ctx, r.Inventory.DPFProvisioning, vars); err != nil {
		errs = append(errs, err)
	}
	if err := r.deleteObjects(ctx, r.Inventory.ArgoCD, vars); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		log.Error(kerrors.NewAggregate(errs), "DPF system components not yet deleted: Requeueing.")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	if err := r.deleteArgoObjects(ctx); err != nil {
		log.Error(err, "ArgoCD objects not yet deleted: Requeueing.")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Delete the objects deployed by the controller.
	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(dpfOperatorConfig, operatorv1.DPFOperatorConfigFinalizer)
	// We should have an ownerReference chain in order to delete subordinate objects.
	return ctrl.Result{}, nil
}

// deleteArgoObjects delete additional objects created for ArgoCD in the DPUService controller.
// TODO: This will be solved through ownerReferences when ArgoCD is deployed by the operator.
func (r *DPFOperatorConfigReconciler) deleteArgoObjects(ctx context.Context) error {
	// Delete ArgoCD secrets created for DPF.
	secrets := &corev1.SecretList{}
	err := r.List(ctx, secrets, client.HasLabels{controlplanemeta.DPFClusterLabelKey})
	if err != nil {
		return err
	}
	for _, secret := range secrets.Items {
		if err := r.Delete(ctx, &secret); err != nil {
			return err
		}
	}
	if len(secrets.Items) > 0 {
		return fmt.Errorf("%d ArgoCD secrets still deleting", len(secrets.Items))
	}
	// Delete ArgoCD AppProjects created for DPF.
	projects := &argov1.AppProjectList{}
	err = r.List(ctx, projects, client.HasLabels{operatorv1.DPFComponentLabelKey})
	if err != nil {
		return err
	}
	for _, project := range projects.Items {
		if err := r.Delete(ctx, &project); err != nil {
			return err
		}
	}

	if len(projects.Items) > 0 {
		return fmt.Errorf("%d ArgoCD AppProjects still deleting", len(secrets.Items))
	}

	return nil
}
func (r *DPFOperatorConfigReconciler) deleteObjects(ctx context.Context, manifests inventory.Component, vars inventory.Variables) error {
	objs, err := manifests.GenerateManifests(vars)
	if err != nil {
		return fmt.Errorf("error while generating manifests for flannel, err: %v", err)
	}
	var errs []error
	for _, obj := range objs {
		err := r.Client.Delete(ctx, obj)
		if client.IgnoreNotFound(err) != nil {
			errs = append(errs, fmt.Errorf("error deleting %v %v: %w",
				obj.GetObjectKind().GroupVersionKind().Kind,
				klog.KObj(obj),
				err))
		}
		uns := &unstructured.Unstructured{}
		uns.SetKind(obj.GetObjectKind().GroupVersionKind().Kind)
		uns.SetAPIVersion(obj.GetObjectKind().GroupVersionKind().GroupVersion().String())
		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(obj), uns); !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("object %v/%v still exists", obj.GetObjectKind().GroupVersionKind().Kind, klog.KObj(obj)))
		}
	}
	return kerrors.NewAggregate(errs)
}

// operatorControlledResources tracks the resources which need to be deleted by the DPF Operator.
// deleting these resources should result in the deletion of resources they own on the DPU clusters.
var operatorControlledResources = []schema.GroupVersionKind{
	dpuservicev1.DPUServiceGroupVersionKind,
	sfcv1.GroupVersion.WithKind("DPUServiceChain"),
	sfcv1.GroupVersion.WithKind("DPUServiceInterface"),
}

func (r *DPFOperatorConfigReconciler) deleteCustomResources(ctx context.Context) error {
	var errs []error

	// For each kind of resource controlled by the DPF Operator.
	for _, resource := range operatorControlledResources {
		// Check if any objects of that type remain in the cluster.
		objListKind := fmt.Sprintf("%vList", resource.Kind)
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(resource.GroupVersion().WithKind(objListKind))
		if err := r.Client.List(ctx, list); err != nil {
			errs = append(errs, err)
		}

		// If no items exist all are deleted.
		if len(list.Items) == 0 {
			continue
		}
		for _, obj := range list.Items {
			errs = append(errs, fmt.Errorf("%d instances of resource Kind %v still exist. ", len(list.Items), resource))
			if err := r.Client.Delete(ctx, &obj); client.IgnoreNotFound(err) != nil {
				errs = append(errs, err)
			}
		}
	}
	return kerrors.NewAggregate(errs)
}
