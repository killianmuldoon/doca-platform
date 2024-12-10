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

package controllers

import (
	"context"

	snapstoragev1 "github.com/nvidia/doca-platform/api/storage/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	NVVolumeFinalizer      = "storage.nvidia.com/volume-protection"
	nvVolumeControllerName = "nvVolumeController"
)

// NVVolume reconciles a Volume object
type NVVolume struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=storage.dpu.nvidia.com,resources=volumes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=storage.dpu.nvidia.com,resources=volumes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=storage.dpu.nvidia.com,resources=volumes/finalizers,verbs=update

func (r *NVVolume) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrllog.FromContext(ctx)
	log.Info("Reconciling")

	nvVolume := &snapstoragev1.Volume{}
	if err := r.Get(ctx, req.NamespacedName, nvVolume); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	/*
		patcher := patch.NewSerialPatcher(nvVolume, r.Client)
		// Defer a patch call to always patch the object when Reconcile exits.
		defer func() {
			log.Info("Patching")

			if err := r.updateSummary(ctx, nvVolume); err != nil {
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}

			if err := patcher.Patch(ctx, nvVolume,
				patch.WithFieldOwner(nvVolumeControllerName),
				patch.WithStatusObservedGeneration{},
			); err != nil {
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}()

		// Handle deletion reconciliation loop.
		if !nvVolume.DeletionTimestamp.IsZero() {
			return r.reconcileDelete(ctx, nvVolume)
		}
		// Add finalizer if not set.
		if !controllerutil.ContainsFinalizer(nvVolume, NVVolumeFinalizer) {
			log.Info("Adding finalizer")
			controllerutil.AddFinalizer(nvVolume, NVVolumeFinalizer)
			return ctrl.Result{}, nil
		}
	*/
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NVVolume) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&snapstoragev1.Volume{}).
		Complete(r)
}

/*
func (r *NVVolume) reconcileDelete(_ context.Context, volume *snapstoragev1.Volume) (ctrl.Result, error) {
	controllerutil.RemoveFinalizer(volume, NVVolumeFinalizer)
	return ctrl.Result{}, nil
}

// updateSummary updates the fields of the Volume
func (r *NVVolume) updateSummary(_ context.Context, _ *snapstoragev1.Volume) error {
	return nil
}*/
