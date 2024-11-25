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

package webhooks

import (
	"context"
	"fmt"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-provisioning-dpu-nvidia-com-v1alpha1-dpudevice,mutating=false,failurePolicy=fail,sideEffects=None,groups=provisioning.dpu.nvidia.com,resources=dpudevices,verbs=create;update;delete,versions=v1alpha1,name=vdpdevices.kb.io,admissionReviewVersions=v1

// DPUDevice implements a webhook for the DPUDevice object.
type DPUDevice struct{}

var _ webhook.CustomValidator = &DPUDevice{}

// log is for logging in this package.
var (
	dpudevicelog = logf.Log.WithName("dpudevice-resource")
)

// SetupWebhookWithManager sets up the manager to manage the webhooks.
func (r *DPUDevice) SetupWebhookWithManager(mgr ctrl.Manager) error {
	manager = mgr
	return ctrl.NewWebhookManagedBy(mgr).
		For(&provisioningv1.DPUDevice{}).
		WithValidator(r).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *DPUDevice) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	dpuDevice, ok := obj.(*provisioningv1.DPUDevice)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("invalid object type: expected DPUDevice but got %s", obj.GetObjectKind().GroupVersionKind().String()))
	}

	dpudevicelog.V(4).Info("validate create", "name", dpuDevice.Name)

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *DPUDevice) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	dpuDevice, ok := newObj.(*provisioningv1.DPUDevice)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("invalid object type: expected DPUDevice but got %s", newObj.GetObjectKind().GroupVersionKind().String()))
	}

	dpudevicelog.V(4).Info("validate update", "name", dpuDevice.Name)

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *DPUDevice) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	dpuDevice, ok := obj.(*provisioningv1.DPUDevice)
	if !ok {
		return admission.Warnings{}, apierrors.NewBadRequest(fmt.Sprintf("invalid object type: expected DPUDevice but got %s", obj.GetObjectKind().GroupVersionKind().String()))
	}

	dpudevicelog.V(4).Info("validate delete", "name", dpuDevice.Name)

	// Example: Ensure no dependencies exist before deletion.
	// if isInUse(dpuDevice) {
	// 	return admission.Warnings{}, apierrors.NewForbidden(dpuDevice.GroupVersionKind().GroupResource(), dpuDevice.Name, fmt.Errorf("DPUDevice is in use and cannot be deleted"))
	// }

	return nil, nil
}
