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

	provisioningdpfv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type State interface {
	Handle(ctx context.Context, client client.Client, option util.DPUOptions) (provisioningdpfv1alpha1.DpuStatus, error)
}

func GetDPUState(dpu *provisioningdpfv1alpha1.Dpu) State {
	switch dpu.Status.Phase {
	case provisioningdpfv1alpha1.DPUInitializing:
		return &dpuInitializingState{
			dpu,
		}
	case provisioningdpfv1alpha1.DPUNodeEffect:
		return &dpuNodeEffectState{
			dpu,
		}
	case provisioningdpfv1alpha1.DPUPending:
		return &dpuPendingState{
			dpu,
		}
	case provisioningdpfv1alpha1.DPUDMSDeployment:
		return &dmsDeploymentState{
			dpu,
		}
	case provisioningdpfv1alpha1.DPUOSInstalling:
		return &dpuOSInstallingState{
			dpu,
		}
	case provisioningdpfv1alpha1.DPUReady:
		return &dpuReadyState{
			dpu,
		}
	case provisioningdpfv1alpha1.DPUError:
		return &dpuErrorState{
			dpu,
		}
	case provisioningdpfv1alpha1.DPUDeleting:
		return &dpuDeletingState{
			dpu,
		}
	case provisioningdpfv1alpha1.DPURebooting:
		return &dpuRebootingState{
			dpu,
		}
	case provisioningdpfv1alpha1.DPUClusterConfig:
		return &dpuClusterConfig{
			dpu,
		}
	}

	return &dpuInitializingState{
		dpu,
	}
}

func isDeleting(dpu *provisioningdpfv1alpha1.Dpu) bool {
	return !dpu.DeletionTimestamp.IsZero()
}
