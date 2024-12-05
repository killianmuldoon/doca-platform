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

package snap

import (
	"context"

	storagev1 "github.com/nvidia/doca-platform/api/storage/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	NVVolumeAttachmentFinalizer = "storage.nvidia.com/attachment-protection"
)

// NVVolumeAttachment reconciles a VolumeAttachment object
type NVVolumeAttachment struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=storage.dpu.nvidia.com,resources=volumeattachments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=storage.dpu.nvidia.com,resources=volumeattachments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=storage.dpu.nvidia.com,resources=volumeattachments/finalizers,verbs=update

func (r *NVVolumeAttachment) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling")

	/*nvVolumeAttachment := &storagev1.VolumeAttachment{}
	if err := r.Get(ctx, req.NamespacedName, nvVolumeAttachment); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if nvVolumeAttachment.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(nvVolumeAttachment, NVVolumeAttachmentFinalizer) {
			controllerutil.AddFinalizer(nvVolumeAttachment, NVVolumeAttachmentFinalizer)
			if err := r.Update(ctx, nvVolumeAttachment); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		return ctrl.Result{}, r.reconcileDelete(ctx, nvVolumeAttachment)
	}*/

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NVVolumeAttachment) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1.VolumeAttachment{}).
		Complete(r)
}

/*
func (r *NVVolumeAttachment) reconcileDelete(ctx context.Context, volumeAttachment *storagev1.VolumeAttachment) error {
	controllerutil.RemoveFinalizer(volumeAttachment, NVVolumeAttachmentFinalizer)
	if err := r.Update(ctx, volumeAttachment); err != nil {
		return err
	}
	return nil
}*/
