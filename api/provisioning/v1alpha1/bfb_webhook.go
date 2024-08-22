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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	BFBFileNameExtension = ".bfb"
)

var (
	// log is for logging in this package.
	bfblog = logf.Log.WithName("bfb-resource")
	bfbMgr ctrl.Manager
)

func (r *Bfb) SetupWebhookWithManager(mgr ctrl.Manager) error {
	bfbMgr = mgr
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-provisioning-dpf-nvidia-com-v1alpha1-bfb,mutating=true,failurePolicy=fail,sideEffects=None,groups=provisioning.dpf.nvidia.com,resources=bfbs,verbs=create;update,versions=v1alpha1,name=mbfb.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Bfb{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Bfb) Default() {
	bfblog.V(4).Info("default", "name", r.Name)
	if r.Spec.FileName == "" {
		r.Spec.FileName = fmt.Sprintf("%s-%s%s", r.Namespace, r.Name, BFBFileNameExtension)
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-provisioning-dpf-nvidia-com-v1alpha1-bfb,mutating=false,failurePolicy=fail,sideEffects=None,groups=provisioning.dpf.nvidia.com,resources=bfbs,verbs=create;update;delete,versions=v1alpha1,name=vbfb.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Bfb{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Bfb) ValidateCreate() (admission.Warnings, error) {
	bfblog.V(4).Info("validate create", "name", r.Name)

	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Bfb) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	bfblog.V(4).Info("validate update", "name", r.Name)

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Bfb) ValidateDelete() (admission.Warnings, error) {
	bfblog.V(4).Info("validate delete", "name", r.Name)

	dpusetList := &DpuSetList{}
	if err := bfbMgr.GetClient().List(context.TODO(), dpusetList); err != nil {
		return nil, fmt.Errorf("list DPUSets failed, err: %v", err)
	}
	var ref []string
	for _, ds := range dpusetList.Items {
		if ds.Spec.DpuTemplate.Spec.Bfb.BFBName == r.Name {
			ref = append(ref, ds.Name)
		}
	}
	if len(ref) > 0 {
		return nil, fmt.Errorf("being referred to by DPUSet(s) %s, you must delete the DPUSet(s) first", ref)
	}

	dpuList := &DpuList{}
	if err := bfbMgr.GetClient().List(context.TODO(), dpuList); err != nil {
		return nil, fmt.Errorf("list DPUs failed, err: %v", err)
	}
	for _, dpu := range dpuList.Items {
		if dpu.Spec.BFB == r.Name {
			ref = append(ref, dpu.Name)
		}
	}
	if len(ref) > 0 {
		return nil, fmt.Errorf("being refferred to by DPU(s) %s, you must delete the DPU(s) first", ref)
	}
	return nil, nil
}
