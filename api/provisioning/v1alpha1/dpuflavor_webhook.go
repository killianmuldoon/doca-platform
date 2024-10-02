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
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var (
	dpuflavorlog = logf.Log.WithName("dpuflavor-resource")
	manager      ctrl.Manager
)

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *DPUFlavor) SetupWebhookWithManager(mgr ctrl.Manager) error {
	manager = mgr
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-provisioning-dpu-nvidia-com-v1alpha1-dpuflavor,mutating=false,failurePolicy=fail,sideEffects=None,groups=provisioning.dpu.nvidia.com,resources=dpuflavors,verbs=create;update;delete,versions=v1alpha1,name=vdpuflavor.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &DPUFlavor{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *DPUFlavor) ValidateCreate() (admission.Warnings, error) {
	dpuflavorlog.Info("validate create", "name", r.Name)
	return nil, validateNVConfig(r)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *DPUFlavor) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	dpuflavorlog.Info("validate update", "name", r.Name)
	// This is a no-op as this type is immutable. The immutability validation is done inside the CRD definition.
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *DPUFlavor) ValidateDelete() (admission.Warnings, error) {
	dpuflavorlog.Info("validate delete", "name", r.Name)
	dpuSetList := &DPUSetList{}
	if err := manager.GetClient().List(context.TODO(), dpuSetList, &client.ListOptions{Namespace: r.Namespace}); err != nil {
		return nil, fmt.Errorf("list DPUSets failed, err: %v", err)
	}
	var ref []string
	for _, ds := range dpuSetList.Items {
		if ds.Spec.DPUTemplate.Spec.DPUFlavor == r.Name {
			ref = append(ref, ds.Name)
		}
	}
	if len(ref) > 0 {
		return nil, fmt.Errorf("DPUFlavor is being referred to by DPUSet(s) %s, you must delete the DPUSet(s) first", ref)
	}

	dpuList := &DPUList{}
	if err := manager.GetClient().List(context.TODO(), dpuList, &client.ListOptions{Namespace: r.Namespace}); err != nil {
		return nil, fmt.Errorf("list DPUs failed, err: %v", err)
	}
	for _, dpu := range dpuList.Items {
		if dpu.Spec.DPUFlavor == r.Name {
			ref = append(ref, dpu.Name)
		}
	}
	if len(ref) > 0 {
		return nil, fmt.Errorf("DPUFlavor is being refferred to by DPU(s) %s, you must delete the DPU(s) first", ref)
	}
	return nil, nil
}

func validateNVConfig(flavor *DPUFlavor) error {
	if len(flavor.Spec.NVConfig) == 0 {
		return nil
	} else if len(flavor.Spec.NVConfig) > 1 {
		return fmt.Errorf("there must be at most one element in nvconfig, and the element applies to all devices on a host")
	}
	nvcfg := flavor.Spec.NVConfig[0]
	// TODO: support regex device?
	if nvcfg.Device != nil && strings.TrimSpace(*nvcfg.Device) != "*" {
		return fmt.Errorf("nvconfig[*].device must be \"*\"")
	}
	return nil
}
