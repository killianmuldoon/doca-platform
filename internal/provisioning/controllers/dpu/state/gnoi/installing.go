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

package gnoi

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/bfcfg"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util/dms"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util/future"

	"github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/gnoigo"
	gos "github.com/openconfig/gnoigo/os"
	gsystem "github.com/openconfig/gnoigo/system"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	CloudInitDefaultTimeout = 90
	// HostNameDPULabelKey is the label added to the DPU Kubernetes Node that indicates the hostname of the host that
	// this DPU belongs to.
	HostNameDPULabelKey = "provisioning.dpu.nvidia.com/host"
)

func Installing(ctx context.Context, dpu *provisioningv1.DPU, ctrlCtx *dutil.ControllerContext) (provisioningv1.DPUStatus, error) {
	logger := log.FromContext(ctx)
	dmsTaskName := generateDMSTaskName(dpu)
	state := dpu.Status.DeepCopy()
	if !dpu.DeletionTimestamp.IsZero() {
		logger.V(3).Info(fmt.Sprintf("DPU %v is deleting, but waiting for bfb-install to complete", dpu.Name))
	}

	// check whether bfb exist
	nn := types.NamespacedName{
		Namespace: dpu.Namespace,
		Name:      dpu.Spec.BFB,
	}
	bfb := &provisioningv1.BFB{}
	if err := ctrlCtx.Get(ctx, nn, bfb); err != nil {
		if apierrors.IsNotFound(err) {
			// BFB does not exist, dpu keeps in installing state.
			logger.V(3).Info(fmt.Sprintf("BFB %v is not ready for DMS %v", dpu.Spec.BFB, dmsTaskName))
			return *state, nil
		}
		return *state, err
	}

	if task, ok := dutil.OsInstallTaskMap.Load(dmsTaskName); ok {
		taskWithRetry := task.(dutil.TaskWithRetry)
		retryCount := taskWithRetry.RetryCount
		dmsTask := taskWithRetry.Task
		if dmsTask.GetState() == future.Ready {
			dutil.OsInstallTaskMap.Delete(dmsTaskName)
			if _, err := dmsTask.GetResult(); err == nil {
				logger.V(3).Info(fmt.Sprintf("DMS task %v is finished", dmsTaskName))
				return updateState(state, provisioningv1.DPURebooting, "DPU is rebooting"), nil
			} else {
				if retryCount >= dutil.MaxRetryCount {
					logger.V(3).Info(fmt.Sprintf("DMS task %v is failed with err: %v", dmsTaskName, err))
					return updateState(state, provisioningv1.DPUError, err.Error()), err
				} else {
					msg := fmt.Sprintf("DMS task %v retried %d times, error: %v", dmsTaskName, retryCount, err)
					logger.Info(msg)
					// Retry the os install process
					dmsHandler(ctx, ctrlCtx.Client, dpu, bfb, retryCount+1, ctrlCtx)
					cond := cutil.DPUCondition(provisioningv1.DPUCondOSInstalled, "", msg)
					cond.Status = metav1.ConditionFalse
					cutil.SetDPUCondition(state, cond)
					return *state, nil
				}
			}
		} else {
			logger.V(3).Info(fmt.Sprintf("DMS task %v is being processed", dmsTaskName))
		}
	} else {
		dmsHandler(ctx, ctrlCtx.Client, dpu, bfb, 0, ctrlCtx)
	}

	return *state, nil
}

func updateState(state *provisioningv1.DPUStatus, phase provisioningv1.DPUPhase, message string) provisioningv1.DPUStatus {
	state.Phase = phase
	cond := cutil.DPUCondition(provisioningv1.DPUCondOSInstalled, "", message)
	if phase == provisioningv1.DPUError {
		cond.Status = metav1.ConditionFalse
		cond.Reason = "InstallationFailed"
	}
	cutil.SetDPUCondition(state, cond)
	return *state
}

func dmsHandler(ctx context.Context, k8sClient client.Client, dpu *provisioningv1.DPU, bfb *provisioningv1.BFB, retry int, ctrlContext *dutil.ControllerContext) {
	dmsTaskName := generateDMSTaskName(dpu)
	dmsTask := future.New(func() (any, error) {
		logger := log.FromContext(ctx)
		logger.V(3).Info(fmt.Sprintf("DMS %s start os installation", dmsTaskName))

		conn, err := createGRPCConnection(ctx, k8sClient, dpu)
		if err != nil {
			logger.Error(err, fmt.Sprintf("Error creating gRPC connection: %v", err))
			return nil, err
		}
		defer conn.Close() //nolint: errcheck
		gnoiClient := gnoigo.NewClients(conn)

		logger.V(3).Info(fmt.Sprintf("TLS Connection established between DPU controller to DMS %s", dmsTaskName))

		osInstall := gos.NewInstallOperation()
		osInstall.Version(bfb.Status.FileName)
		// Open the file at the specified path
		bfbfile, err := os.Open(dpu.Status.BFBFile)
		if err != nil {
			logger.Error(err, "failed to open file", "name", bfb.Status.FileName)
			return nil, err
		}
		defer bfbfile.Close() //nolint: errcheck
		osInstall.Reader(bfbfile)
		if response, err := gnoigo.Execute(ctx, gnoiClient, osInstall); err != nil {
			return nil, err
		} else {
			logger.V(3).Info(fmt.Sprintf("DMS %s Performed InstallOperation %v", dmsTaskName, response))
		}

		flavor := &provisioningv1.DPUFlavor{}
		if err := k8sClient.Get(ctx, types.NamespacedName{
			Namespace: dpu.Namespace,
			Name:      dpu.Spec.DPUFlavor,
		}, flavor); err != nil {
			return nil, err
		}
		dc := &provisioningv1.DPUCluster{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: dpu.Spec.Cluster.Namespace, Name: dpu.Spec.Cluster.Name}, dc); err != nil {
			return nil, fmt.Errorf("failed to get DPUCluster, err: %v", err)
		}
		node := &corev1.Node{}
		if err := k8sClient.Get(ctx, types.NamespacedName{
			Namespace: "",
			Name:      dpu.Spec.NodeName,
		}, node); err != nil {
			return nil, err
		}

		// Generate the KubeadmJoin command.
		joinCommand, err := ctrlContext.JoinCommandGenerator.GenerateJoinCommand(dc)
		if err != nil {
			return nil, err
		}
		data, err := bfcfg.GenerateBFConfig(ctx, ctrlContext.Options.BFCFGTemplateFile, dpu, node, flavor, joinCommand)
		if err != nil || data == nil {
			logger.Error(err, fmt.Sprintf("failed bf.cfg creation for %s/%s", dpu.Namespace, dpu.Name))
			return nil, err
		}
		cfgInstall := gos.NewInstallOperation()
		// DMS has a default rule: bf cfg file must end with .cfg
		// cfgVersion is the BF CFG file name used in DMS
		cfgVersion := cutil.GenerateBFCFGFileName(dpu.Name)

		// Make sure there is no old bf cfg file in the shared volume
		cfgFile := cutil.GenerateBFBCFGFilePath(cfgVersion)
		if err := os.Remove(cfgFile); err != nil && !os.IsNotExist(err) {
			msg := fmt.Sprintf("Delete old BFB CFG file %s failed", cfgFile)
			logger.Error(err, msg)
			return nil, err
		}

		cfgInstall.Version(cfgVersion)
		cfgInstall.Reader(bytes.NewReader(data))
		if response, err := gnoigo.Execute(ctx, gnoiClient, cfgInstall); err != nil {
			return nil, err
		} else {
			logger.V(3).Info(fmt.Sprintf("DMS %s Performed BF configuration %v", dmsTaskName, response))
		}

		bfbPathInDms := dms.DMSImageFolder + string(filepath.Separator) + bfb.Status.FileName
		if md5, _, err := computeBFBMD5InDms(dpu.Namespace, cutil.GenerateDMSPodName(dpu), "", bfbPathInDms); err == nil {
			logger.V(3).Info(fmt.Sprintf("md5sum of %s in DMS is %s", bfb.Status.FileName, md5))
		} else {
			logger.Error(err, "Failed to get md5sum from dms", "md5", dpu)
		}

		activateOp := gos.NewActivateOperation()
		noReboot := false
		activateOp.NoReboot(noReboot)
		activateOp.Version(fmt.Sprintf("%s;%s", bfb.Status.FileName, cfgVersion))

		logger.V(3).Info("Starting execute activate operation")
		// set 30 minutes timeout for OS installation
		activateCtx, cancel := context.WithTimeout(context.Background(), 30*60*time.Second)
		defer cancel()
		if response, err := gnoigo.Execute(activateCtx, gnoiClient, activateOp); err != nil {
			logger.Error(err, fmt.Sprintf("DMS %s failed to Execute activateOp", dmsTaskName))
			return nil, err
		} else {
			logger.V(3).Info("Performed activateOp", "response", response)
		}

		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-activateCtx.Done():
				logger.Error(activateCtx.Err(), fmt.Sprintf("DMS %s RebootStatusOperation timed out or was canceled", dmsTaskName))
				return nil, activateCtx.Err()
			case <-ticker.C:
				rebootStatusOp := gsystem.NewRebootStatusOperation().Subcomponents([]*tpb.Path{{Elem: []*tpb.PathElem{{Name: "CPU"}}}})
				response, err := gnoigo.Execute(activateCtx, gnoiClient, rebootStatusOp)
				if err != nil {
					logger.Error(err, fmt.Sprintf("DMS %s failed to Execute rebootStatusOp", dmsTaskName))
					return nil, err
				}
				if !response.Active {
					// Once the reboot is no longer active, check the status
					switch response.Status.Status {
					case system.RebootStatus_STATUS_SUCCESS:

						// TODO: wait for 90 sec for cloud-init
						cloudInitTimeout := CloudInitDefaultTimeout
						if value, exists := os.LookupEnv("CLOUD_INIT_TIMEOUT"); exists {
							if cloudInitTimeout, err = strconv.Atoi(value); err != nil {
								cloudInitTimeout = CloudInitDefaultTimeout
							}
						}

						time.Sleep(time.Duration(cloudInitTimeout) * time.Second)
						logger.V(3).Info(fmt.Sprintf("DMS %s install DPU successfully", dmsTaskName))

						/*verifyOp := gos.NewVerifyOperation()
						if response, err := gnoigo.Execute(ctx, clients, verifyOp); err != nil {
							logger.Error(err, fmt.Sprintf("DMS %s failed to Execute verifyOp", dmsTaskName))
							return future.Ready, err
						} else {
							logger.V(3).Info("Performed verifyOp", "response", response)
							if response.Version != fmt.Sprintf("%s;%s", bfb.Spec.FileName, cfgVersion) {
								return future.Ready, fmt.Errorf("verifyOp failure: expected: %s, received: %s", fmt.Sprintf("%s;%s", bfb.Spec.FileName, cfgVersion), response.Version)
							}
						}
						*/

						if dpu.Spec.Cluster.NodeLabels == nil {
							dpu.Spec.Cluster.NodeLabels = make(map[string]string)
						}

						// Add DOCA_BFB version to DPU NodeLabels as it is found in DPU ARM under /etc/mlnx-release. e.g.,
						// cat /etc/mlnx-release
						// bf-bundle-2.7.0-33_24.04_ubuntu-22.04_prod
						patch := client.MergeFrom(dpu.DeepCopy())
						dpu.Spec.Cluster.NodeLabels["provisioning.dpu.nvidia.com/DOCA-BFB-version"] = cutil.GenerateBFBVersionFromURL(bfb.Spec.URL)
						dpu.Spec.Cluster.NodeLabels[HostNameDPULabelKey] = dpu.Spec.NodeName
						if err := k8sClient.Patch(ctx, dpu, patch); err != nil {
							return nil, err
						}
						return nil, nil
					case system.RebootStatus_STATUS_RETRIABLE_FAILURE:
						logger.Error(fmt.Errorf("DMS %s reboot encountered a retriable failure: %s", dmsTaskName, response.Reason), fmt.Sprintf("DMS %s reboot failed", dmsTaskName))
						return nil, fmt.Errorf("retriable failure: %s", response.Reason)
					case system.RebootStatus_STATUS_FAILURE:
						logger.Error(fmt.Errorf("reboot failure: %s", response.Reason), fmt.Sprintf("DMS %s reboot failed", dmsTaskName))
						return nil, fmt.Errorf("reboot failure: %s", response.Reason)
					default:
						logger.Error(fmt.Errorf("unknown reboot status: %s", response.Status), fmt.Sprintf("DMS %s encountered an unknown reboot status", dmsTaskName))
						return nil, fmt.Errorf("unknown reboot status: %s", response.Status)
					}
				}
			}
		}
	})
	taskWithRetryCount := dutil.TaskWithRetry{
		Task:       dmsTask,
		RetryCount: retry,
	}
	dutil.OsInstallTaskMap.Store(dmsTaskName, taskWithRetryCount)
}

func computeBFBMD5InDms(ns, name, container, filepath string) (string, string, error) {
	md5CMD := fmt.Sprintf("md5sum %s", filepath)
	return cutil.RemoteExec(ns, name, container, md5CMD)
}
