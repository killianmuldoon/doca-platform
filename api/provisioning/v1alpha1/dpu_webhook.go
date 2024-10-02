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

package v1alpha1

import (
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util/reboot"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var dpulog = logf.Log.WithName("dpu-resource")

func (r *DPU) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-provisioning-dpu-nvidia-com-v1alpha1-dpu,mutating=true,failurePolicy=fail,sideEffects=None,groups=provisioning.dpu.nvidia.com,resources=dpus,verbs=create;update,versions=v1alpha1,name=mdpu.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &DPU{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *DPU) Default() {
	dpulog.V(4).Info("default", "name", r.Name)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-provisioning-dpu-nvidia-com-v1alpha1-dpu,mutating=false,failurePolicy=fail,sideEffects=None,groups=provisioning.dpu.nvidia.com,resources=dpus,verbs=create;update,versions=v1alpha1,name=vdpu.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &DPU{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *DPU) ValidateCreate() (admission.Warnings, error) {
	dpulog.V(4).Info("validate create", "name", r.Name)

	errs := field.ErrorList{}
	newPath := field.NewPath("spec")

	if err := ValidateNodeEffect(*r.Spec.NodeEffect); err != nil {
		errs = append(errs, field.Invalid(newPath.Child(".node_effect"), r.Spec.NodeEffect, err.Error()))
	}
	if err := reboot.ValidateHostPowerCycleRequire(r.Annotations); err != nil {
		errs = append(errs, field.Invalid(newPath.Child(".annotations", reboot.HostPowerCycleRequireKey), r.Annotations[reboot.HostPowerCycleRequireKey], err.Error()))
	}

	if len(errs) != 0 {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: "provisioning.dpu.nvidia.com", Kind: "DPUSet"},
			r.Name,
			errs)
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *DPU) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	dpulog.V(4).Info("validate update", "name", r.Name)

	errs := field.ErrorList{}
	newPath := field.NewPath("spec")

	if err := ValidateNodeEffect(*r.Spec.NodeEffect); err != nil {
		errs = append(errs, field.Invalid(newPath.Child(".node_effect"), r.Spec.NodeEffect, err.Error()))
	}
	if len(errs) != 0 {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: "provisioning.dpu.nvidia.com", Kind: "DPUSet"},
			r.Name,
			errs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *DPU) ValidateDelete() (admission.Warnings, error) {
	dpulog.V(4).Info("validate delete", "name", r.Name)

	return nil, nil
}
