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

package state

import (
	"context"
	"fmt"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util/future"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/util/reboot"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Rebooting(ctx context.Context, dpu *provisioningv1.DPU, ctrlCtx *dutil.ControllerContext) (provisioningv1.DPUStatus, error) {
	logger := log.FromContext(ctx)

	rebootTaskName := generateRebootTaskName(dpu)
	state := dpu.Status.DeepCopy()
	if !dpu.DeletionTimestamp.IsZero() {
		dutil.RebootTaskMap.Delete(rebootTaskName)
		state.Phase = provisioningv1.DPUDeleting
		return *state, nil
	}

	nn := types.NamespacedName{
		Namespace: "",
		Name:      dpu.Spec.NodeName,
	}
	node := &corev1.Node{}
	if err := ctrlCtx.Get(ctx, nn, node); err != nil {
		return *state, err
	}

	_, cond := cutil.GetDPUCondition(state, provisioningv1.DPUCondDMSRunning.String())
	if cond == nil || cond.Status != metav1.ConditionTrue {
		err := fmt.Errorf("trying to reboot the host before %s", provisioningv1.DPUCondOSInstalled.String())
		c := cutil.DPUCondition(provisioningv1.DPUCondRebooted, "InvalidState", err.Error())
		c.Status = metav1.ConditionFalse
		cutil.SetDPUCondition(state, c)
		return *state, err
	}

	duration := int(metav1.Now().Sub(cond.LastTransitionTime.Time).Seconds())
	// If we can not get uptime, the host should be rebooting
	uptime, err := ctrlCtx.HostUptimeChecker.HostUptime(dpu.Namespace, cutil.GenerateDMSPodName(dpu), "")
	if err != nil {
		return *state, err
	}

	logger.V(3).Info(fmt.Sprintf("Rebooting duration is %d, host uptime is %d", duration, uptime))

	// If the pod is available after rebooting, move to next phase
	if duration > uptime {
		state.Phase = provisioningv1.DPUHostNetworkConfiguration
		cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondRebooted, "", ""))
		return *state, nil
	}

	if (dpu.Spec.NodeEffect != nil && dpu.Spec.NodeEffect.IsDrain()) ||
		dpu.Spec.AutomaticNodeReboot {
		cmd, rebootType, err := reboot.GenerateCmd(node.Annotations, dpu.Annotations)
		if err != nil {
			logger.Error(err, "failed to generate ipmitool command")
			return *state, err
		}

		// Return early and set node to ready if we should skip the powercycle/reboot command.
		// Note: skipping the powercycle/reboot may cause issues with the firmware installation and configuration.
		if cmd == reboot.Skip {
			logger.Info("Warning not rebooting: this may cause issues with DPU firmware installation and configuration")
			state.Phase = provisioningv1.DPUHostNetworkConfiguration
			cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondRebooted, "", ""))
			return *state, nil
		} else if rebootType == reboot.PowerCycle {
			logger.Info(fmt.Sprintf("powercycle with command %q", cmd))
			if _, _, err := cutil.RemoteExec(dpu.Namespace, cutil.GenerateDMSPodName(dpu), "", cmd); err != nil {
				// TODO: broadcast an event
				return *state, err
			}
		} else if rebootType == reboot.WarmReboot {
			if pciAddress, err := cutil.GetPCIAddrFromLabel(dpu.Labels, true); err != nil {
				logger.Error(err, "Failed to get pci address from node label", "dms", err)
				return *state, err
			} else {
				if task, ok := dutil.RebootTaskMap.Load(rebootTaskName); ok {
					rebootTaskWithRetry := task.(dutil.TaskWithRetry)
					retryCount := rebootTaskWithRetry.RetryCount
					rebootTask := rebootTaskWithRetry.Task
					if rebootTask.GetState() == future.Ready {
						dutil.RebootTaskMap.Delete(rebootTaskName)
						if _, err := rebootTask.GetResult(); err == nil {
							cutil.SetDPUCondition(state, cutil.DPUCondition(provisioningv1.DPUCondRebooted, "", ""))
							return *state, nil
						} else {
							if retryCount >= dutil.MaxRetryCount {
								logger.V(3).Info(fmt.Sprintf("Reboot task %v failed with err: %v", rebootTaskName, err))
								state.Phase = provisioningv1.DPUError
								cutil.SetDPUCondition(state, cutil.NewCondition(string(provisioningv1.DPUCondRebooted), err, "RebootFailed", ""))
								return *state, err
							} else {
								msg := fmt.Sprintf("DMS task %v retried %d times, error: %v", rebootTaskName, retryCount, err)
								logger.Info(msg)
								// Retry the reboot process
								rebootHandler(ctx, dpu, pciAddress, cmd, retryCount+1)
								cond := cutil.DPUCondition(provisioningv1.DPUCondOSInstalled, "", msg)
								cond.Status = metav1.ConditionFalse
								cutil.SetDPUCondition(state, cond)
								return *state, nil
							}
						}
					} else {
						logger.V(3).Info(fmt.Sprintf("Reboot task %v is being processed", rebootTaskName))
					}
				} else {
					rebootHandler(ctx, dpu, pciAddress, cmd, 0)
				}
			}
		}
	} else {
		logger.Info("waiting for manual power cycle or reboot")
		c := cutil.DPUCondition(provisioningv1.DPUCondRebooted, "WaitingForManualPowerCycleOrReboot", "")
		c.Status = metav1.ConditionFalse
		cutil.SetDPUCondition(state, c)
		return *state, nil
	}

	return *state, nil
}

func rebootHandler(ctx context.Context, dpu *provisioningv1.DPU, pciAddress string, cmd string, retry int) {
	logger := log.FromContext(ctx)
	rebootTaskName := generateRebootTaskName(dpu)
	logger.V(3).Info(fmt.Sprintf("BF-SLR for %s", rebootTaskName))
	bfSLRShutdownARM := fmt.Sprintf("bf-slr.sh %s %s %s", pciAddress, cmd, "arm")
	bfSLRSRebootHost := fmt.Sprintf("bf-slr.sh %s %s %s", pciAddress, cmd, "host")

	rebootTask := future.New(func() (any, error) {
		// Shutdown ARM
		logger.V(3).Info(fmt.Sprintf("Bluefield System-Level-Reset ARM shutdown command: %s for dpu: %s", bfSLRShutdownARM, dpu.Name))
		if out, errMsg, err := cutil.RemoteExec(dpu.Namespace, cutil.GenerateDMSPodName(dpu), "", bfSLRShutdownARM); err != nil {
			logger.Error(err, fmt.Sprintf("DPU %s failed to shutdown ARM: %v, output: %s", dpu.Name, err, out))
			return future.Ready, fmt.Errorf("DPU %s reboot failed: %v, output: %s", dpu.Name, err, errMsg)
		} else {
			logger.V(3).Info(fmt.Sprintf("DPU %s Bluefield System-Level-Reset result: %s", dpu.Name, out))
		}

		// Reboot Host
		logger.V(3).Info(fmt.Sprintf("Bluefield System-Level-Reset reboot host command: %s for dpu: %s", bfSLRSRebootHost, dpu.Name))
		if out, _, err := cutil.RemoteExec(dpu.Namespace, cutil.GenerateDMSPodName(dpu), "", bfSLRSRebootHost); err != nil {
			logger.Error(err, fmt.Sprintf("Failed to reboot host: %v, output: %s", err, out))
			return future.Ready, err
		}

		return nil, nil
	})
	rebootTaskWithRetry := dutil.TaskWithRetry{
		Task:       rebootTask,
		RetryCount: retry,
	}
	dutil.RebootTaskMap.Store(rebootTaskName, rebootTaskWithRetry)
}

func generateRebootTaskName(dpu *provisioningv1.DPU) string {
	return fmt.Sprintf("%s/%s", dpu.Namespace, dpu.Name)
}
