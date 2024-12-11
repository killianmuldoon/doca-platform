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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// VolumeAttachmentReconciler reconciles a VolumeAttachment object
type VolumeAttachmentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const dpuFinalizer = "storage.nvidia.com/dpu-attachment-protection"

//+kubebuilder:rbac:groups=storage.nvidia.com,resources=volumeattachments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storage.nvidia.com,resources=volumeattachments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storage.nvidia.com,resources=volumeattachments/finalizers,verbs=update

func (r *VolumeAttachmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("* Starting Reconciliation for VolumeAttachment CR", "volumeAttachment", req.NamespacedName)

	// Fetch the VolumeAttachment instance
	volumeAttachment := &snapstoragev1.VolumeAttachment{}
	if err := r.Get(ctx, req.NamespacedName, volumeAttachment); err != nil {
		if apierrors.IsNotFound(err) {
			// CR not found, could have been deleted after reconcile request
			logger.Info("VolumeAttachment resource not found. It may have been deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to retrieve VolumeAttachment")
		return ctrl.Result{}, err
	}

	// Log details about the fetched CR
	logger.Info("Fetched VolumeAttachment CR",
		"Namespace", volumeAttachment.Namespace,
		"Name", volumeAttachment.Name,
		"NodeName", volumeAttachment.Spec.NodeName,
		"VolumeRef", volumeAttachment.Spec.Source.VolumeRef,
		"Parameters", volumeAttachment.Spec.Parameters,
		"StorageAttached", volumeAttachment.Status.StorageAttached,
	)

	// Check if marked for deletion
	if volumeAttachment.ObjectMeta.DeletionTimestamp != nil {
		return r.handleDetachment(ctx, volumeAttachment)
	}

	// Wait until storageAttached is set to true before proceeding
	if !volumeAttachment.Status.StorageAttached {
		logger.Info("storageAttached is not true yet, requeue")
		// Requeue to check again later if status changes
		return ctrl.Result{}, nil
	}

	// Add finalizer if not already added
	if !controllerutil.ContainsFinalizer(volumeAttachment, dpuFinalizer) {
		logger.Info("Adding finalizer",
			"Finalizer", dpuFinalizer,
			"VolumeAttachment", volumeAttachment.Name)
		controllerutil.AddFinalizer(volumeAttachment, dpuFinalizer)
		if err := r.Update(ctx, volumeAttachment); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Handle attachment
	return r.handleAttachment(ctx, volumeAttachment)
}

// handleAttachment handles the attachment workflow
func (r *VolumeAttachmentReconciler) handleAttachment(ctx context.Context, volumeAttachment *snapstoragev1.VolumeAttachment) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling attachment for VolumeAttachment", "Name", volumeAttachment.Name)

	// Fetch the referenced Volume
	if volumeAttachment.Spec.Source.VolumeRef.Name == "" {
		logger.Info("No VolumeRef specified in VolumeAttachment, skipping Volume lookup")
		return ctrl.Result{}, nil
	}

	volumeKey := client.ObjectKey{
		Namespace: volumeAttachment.Spec.Source.VolumeRef.Namespace,
		Name:      volumeAttachment.Spec.Source.VolumeRef.Name,
	}

	volume := &snapstoragev1.Volume{}
	if err := r.Get(ctx, volumeKey, volume); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, "Referenced Volume not found", "Volume", volumeKey)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to retrieve Volume", "Volume", volumeKey)
		return ctrl.Result{}, err
	}

	logger.Info("Fetched referenced Volume CR",
		"Namespace", volume.Namespace,
		"Name", volume.Name,
		"StorageVendorName", volume.Spec.DPUVolume.StorageVendorName,
		"StorageVendorPluginName", volume.Spec.DPUVolume.StorageVendorPluginName,
		"Status", volume.Status,
	)

	// Retrieve necessary fields for CreateDevice call
	volumeID := volume.Spec.DPUVolume.CSIReference.PVCRef.Name
	accessModes := volume.Spec.DPUVolume.AccessModes           // from Volume CR
	volumeMode := volume.Spec.Request.VolumeMode               // or from CRD fields
	publishContext := volumeAttachment.Spec.Parameters         // from VolumeAttachment
	volumeAttributes := volume.Spec.DPUVolume.VolumeAttributes // from Volume CR
	storageParameters := mergeStorageParameters(volume.Spec.StorageParameters, volume.Spec.StoragePolicyParameters)

	deviceName, isBlockDevice, nsID, fsTag, err := r.callCreateDeviceAPI(
		volumeID, accessModes, volumeMode, publishContext, volumeAttributes, storageParameters,
	)
	if err != nil {
		logger.Error(err, "Failed to create device via gRPC API")
		return ctrl.Result{}, err
	}

	// Update VolumeAttachment status based on the device type
	volumeAttachment.Status.DPU.DeviceName = deviceName
	if isBlockDevice {
		// Set bdevAttrs and ensure fsdevAttrs is not set
		volumeAttachment.Status.DPU.BdevAttrs = snapstoragev1.BdevAttrs{NVMeNsID: nsID}
		logger.Info("VolumeAttachment DPU attributes updated",
			"DeviceName", deviceName,
			"BdevAttrs", nsID,
		)
	} else {
		// Set fsdevAttrs and ensure bdevAttrs is not set
		volumeAttachment.Status.DPU.FSdevAttrs = snapstoragev1.FSdevAttrs{FilesystemTag: fsTag}
		logger.Info("VolumeAttachment DPU attributes updated",
			"DeviceName", deviceName,
			"FSdevAttrs", fsTag,
		)
	}

	if err := r.Status().Update(ctx, volumeAttachment); err != nil {
		logger.Error(err, "Failed to update VolumeAttachment after CreateDevice")
		return ctrl.Result{}, err
	}

	// Retrieve SNAP provider
	snapProvider, err := r.getSnapProvider(volume)
	if err != nil {
		logger.Error(err, "Failed to get SNAP provider")
		return ctrl.Result{}, err
	}

	// Allocate a PCI function for the volume (placeholder)
	pciAddr, err := r.allocatePCIFunction(snapProvider, volumeAttachment)
	if err != nil {
		logger.Error(err, "Failed to allocate PCI function")
		return ctrl.Result{}, err
	}

	// Update VolumeAttachment with PCI address
	volumeAttachment.Status.DPU.PCIDeviceAddress = pciAddr
	if err := r.Status().Update(ctx, volumeAttachment); err != nil {
		logger.Error(err, "Failed to update VolumeAttachment with PCI device address")
		return ctrl.Result{}, err
	}

	// Use SNAP JSON-RPC APIs to expose device (placeholder)
	if err := r.exposeDeviceOnSNAP(snapProvider, deviceName, pciAddr); err != nil {
		logger.Error(err, "Failed to expose device on SNAP")
		return ctrl.Result{}, err
	}

	// Set status.dpu.attached = true
	volumeAttachment.Status.DPU.Attached = true
	if err := r.Status().Update(ctx, volumeAttachment); err != nil {
		logger.Error(err, "Failed to update VolumeAttachment DPU attached status")
		return ctrl.Result{}, err
	}

	logger.Info("Attachment completed successfully for VolumeAttachment",
		"Namespace", volumeAttachment.Namespace,
		"Name", volumeAttachment.Name)
	return ctrl.Result{}, nil
}

// handleDetachment handles the detachment workflow once deletionTimestamp is set
func (r *VolumeAttachmentReconciler) handleDetachment(ctx context.Context, volumeAttachment *snapstoragev1.VolumeAttachment) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling detachment for VolumeAttachment", "Name", volumeAttachment.Name)

	// Fetch Volume (same as in handleAttachment)
	if volumeAttachment.Spec.Source.VolumeRef.Name != "" {
		volumeKey := client.ObjectKey{
			Namespace: volumeAttachment.Spec.Source.VolumeRef.Namespace,
			Name:      volumeAttachment.Spec.Source.VolumeRef.Name,
		}

		volume := &snapstoragev1.Volume{}
		if err := r.Get(ctx, volumeKey, volume); err != nil && !apierrors.IsNotFound(err) {
			logger.Error(err, "Failed to retrieve Volume during detachment", "Volume", volumeKey)
			return ctrl.Result{}, err
		}

		// If found, perform detach steps:
		// 1. Use SNAP provider to detach volume
		snapProvider, err := r.getSnapProvider(volume)
		if err == nil && snapProvider != "" {
			if err := r.detachFromSNAP(snapProvider, volumeAttachment.Status.DPU.PCIDeviceAddress); err != nil {
				logger.Error(err, "Failed to detach from SNAP")
				return ctrl.Result{}, err
			}
		}

		// 2. Deallocate the PCI emulated function
		if err := r.deallocatePCIFunction(snapProvider, volumeAttachment.Status.DPU.PCIDeviceAddress); err != nil {
			logger.Error(err, "Failed to deallocate PCI function")
			return ctrl.Result{}, err
		}

		// 3. Call DeleteDevice gRPC API
		if err := r.callDeleteDeviceAPI(volumeAttachment.Status.DPU.DeviceName); err != nil {
			logger.Error(err, "Failed to delete device via gRPC API")
			return ctrl.Result{}, err
		}

		// Update status.dpu.attached = false
		volumeAttachment.Status.DPU.Attached = false
		if err := r.Status().Update(ctx, volumeAttachment); err != nil {
			logger.Error(err, "Failed to update VolumeAttachment status during detachment")
			return ctrl.Result{}, err
		}
	}

	// Finally, remove the finalizer
	logger.Info("Removing finalizer",
		"Finalizer", dpuFinalizer,
		"VolumeAttachment", volumeAttachment.Name)
	if controllerutil.ContainsFinalizer(volumeAttachment, dpuFinalizer) {
		controllerutil.RemoveFinalizer(volumeAttachment, dpuFinalizer)
		if err := r.Update(ctx, volumeAttachment); err != nil {
			logger.Error(err, "Failed to remove finalizer")
			return ctrl.Result{}, err
		}
	}

	logger.Info("Detachment completed successfully for volumeAttachment",
		"Name", volumeAttachment.Name)
	return ctrl.Result{}, nil
}

//nolint:unparam
func (r *VolumeAttachmentReconciler) callCreateDeviceAPI(
	volumeID string,
	accessModes []corev1.PersistentVolumeAccessMode,
	volumeMode *corev1.PersistentVolumeMode,
	publishContext, volumeAttributes, storageParameters map[string]string,
) (deviceName string, isBlockDevice bool, NVMeNsID int64, filesystemTag string, err error) {

	// call CreateDevice(ctx, req)

	if volumeMode != nil && *volumeMode == corev1.PersistentVolumeBlock {
		// Simulate a block device creation
		return "nvme0", true, 1, "", nil
	} else {
		// Simulate a filesystem device creation
		return "fsdev", false, 0, "fsTagExample", nil
	}
}

func (r *VolumeAttachmentReconciler) callDeleteDeviceAPI(deviceName string) error {
	// Implement the gRPC call to delete the device
	return nil
}

func (r *VolumeAttachmentReconciler) getSnapProvider(volume *snapstoragev1.Volume) (string, error) {
	// Implement logic to retrieve SNAP provider based on the volume
	return "nvidia", nil
}

func (r *VolumeAttachmentReconciler) allocatePCIFunction(snapProvider string, volumeAttachment *snapstoragev1.VolumeAttachment) (string, error) {
	// Implement logic to allocate PCI function
	return "0000:00:1f.0", nil
}

func (r *VolumeAttachmentReconciler) deallocatePCIFunction(snapProvider, pciAddr string) error {
	// Implement logic to deallocate PCI function
	return nil
}

func (r *VolumeAttachmentReconciler) exposeDeviceOnSNAP(snapProvider, deviceName, pciAddr string) error {
	// Implement SNAP JSON-RPC calls to expose the device
	return nil
}

func (r *VolumeAttachmentReconciler) detachFromSNAP(snapProvider, pciAddr string) error {
	// Implement logic to detach device from SNAP
	return nil
}

func mergeStorageParameters(storageParams, policyParams map[string]string) map[string]string {
	// Implement logic to merge storagePolicyParameters into storageParameters
	// with policyParams overriding as needed.
	merged := map[string]string{}
	for k, v := range storageParams {
		merged[k] = v
	}
	for k, v := range policyParams {
		merged[k] = v
	}
	return merged
}

// SetupWithManager sets up the controller with the Manager.
func (r *VolumeAttachmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&snapstoragev1.VolumeAttachment{}).
		Complete(r)
}
