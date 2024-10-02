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
	"errors"

	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/util/reboot"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var dpusetlog = logf.Log.WithName("dpuset-resource")

func (r *DPUSet) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-provisioning-dpu-nvidia-com-v1alpha1-dpuset,mutating=true,failurePolicy=fail,sideEffects=None,groups=provisioning.dpu.nvidia.com,resources=dpusets,verbs=create;update,versions=v1alpha1,name=mdpuset.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &DPUSet{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *DPUSet) Default() {
	dpusetlog.V(4).Info("default", "name", r.Name)

	if r.Spec.Strategy == nil {
		r.Spec.Strategy = &DPUSetStrategy{
			Type: RecreateStrategyType,
		}
	} else if r.Spec.Strategy.Type == RollingUpdateStrategyType {
		if r.Spec.Strategy.RollingUpdate == nil {
			defaultValue := intstr.IntOrString{Type: intstr.Int, IntVal: 1}
			r.Spec.Strategy.RollingUpdate = &RollingUpdateDPU{
				MaxUnavailable: &defaultValue,
			}
		}
	}

	if r.Spec.DPUTemplate.Spec.NodeEffect == nil {
		r.Spec.DPUTemplate.Spec.NodeEffect = &NodeEffect{
			NoEffect: true,
		}
	}
}

//+kubebuilder:webhook:path=/validate-provisioning-dpu-nvidia-com-v1alpha1-dpuset,mutating=false,failurePolicy=fail,sideEffects=None,groups=provisioning.dpu.nvidia.com,resources=dpusets,verbs=create;update,versions=v1alpha1,name=vdpuset.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &DPUSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *DPUSet) ValidateCreate() (admission.Warnings, error) {
	dpusetlog.V(4).Info("validate create", "name", r.Name)
	errs := field.ErrorList{}
	newPath := field.NewPath("spec")
	if err := validateStrategy(*r.Spec.Strategy); err != nil {
		errs = append(errs, field.Invalid(newPath.Child("strategy"), r.Spec.Strategy, err.Error()))

	}

	if err := ValidateNodeEffect(*r.Spec.DPUTemplate.Spec.NodeEffect); err != nil {
		errs = append(errs, field.Invalid(newPath.Child("dpu_template.spec.node_effect"), r.Spec.DPUTemplate.Spec.NodeEffect, err.Error()))
	}
	if err := reboot.ValidateHostPowerCycleRequire(r.Spec.DPUTemplate.Annotations); err != nil {
		errs = append(errs, field.Invalid(newPath.Child("dpu_template.annotations", reboot.HostPowerCycleRequireKey), r.Annotations[reboot.HostPowerCycleRequireKey], err.Error()))
	}
	if len(errs) != 0 {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: "provisioning.dpu.nvidia.com", Kind: "DPUSet"},
			r.Name,
			errs)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *DPUSet) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	dpusetlog.V(4).Info("validate update", "name", r.Name)
	errs := field.ErrorList{}
	newPath := field.NewPath("spec")
	if err := validateStrategy(*r.Spec.Strategy); err != nil {
		errs = append(errs, field.Invalid(newPath.Child("strategy"), r.Spec.Strategy, err.Error()))

	}

	if err := ValidateNodeEffect(*r.Spec.DPUTemplate.Spec.NodeEffect); err != nil {
		errs = append(errs, field.Invalid(newPath.Child("dpu_template.spec.node_effect"), r.Spec.Strategy, err.Error()))
	}

	if len(errs) != 0 {
		return nil, apierrors.NewInvalid(schema.GroupKind{Group: "provisioning.dpu.nvidia.com", Kind: "DPUSet"},
			r.Name,
			errs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *DPUSet) ValidateDelete() (admission.Warnings, error) {
	dpusetlog.V(4).Info("validate delete", "name", r.Name)

	return nil, nil
}

func validateStrategy(strategy DPUSetStrategy) error {
	if strategy.Type == RollingUpdateStrategyType {
		switch strategy.RollingUpdate.MaxUnavailable.Type {
		case intstr.String:
			if scaledValue, err := intstr.GetScaledValueFromIntOrPercent(strategy.RollingUpdate.MaxUnavailable, 100, false); err != nil {
				return err
			} else {
				if scaledValue <= 0 || scaledValue > 100 {
					return errors.New("the value range of maxUnavailable must be greater than 0% and less than or equal to 100%")
				}
			}

		case intstr.Int:
			if strategy.RollingUpdate.MaxUnavailable.IntVal <= 0 {
				return errors.New("the value range of maxUnavailable must be greater 0")
			}

		}
	}

	return nil
}

func ValidateNodeEffect(nodeEffect NodeEffect) error {
	count := 0
	if nodeEffect.Taint != nil {
		count++
	}
	if nodeEffect.NoEffect {
		count++
	}
	if len(nodeEffect.CustomLabel) != 0 {
		count++
	}
	if nodeEffect.Drain != nil {
		count++
	}
	if count > 1 {
		return errors.New("nodeEffect can only be one of \"taint\", \"no_effect\" , \"drain\" and \"custom_label\"")
	}
	return nil
}
