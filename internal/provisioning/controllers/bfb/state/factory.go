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

	provisioningdpfv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/provisioning/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type State interface {
	Handle(ctx context.Context, client client.Client) (provisioningdpfv1alpha1.BfbStatus, error)
}

func GetBfbState(bfb *provisioningdpfv1alpha1.Bfb) State {
	switch bfb.Status.Phase {
	case provisioningdpfv1alpha1.BfbInitializing:
		return &bfbInitializingState{
			bfb,
		}
	case provisioningdpfv1alpha1.BfbDownloading:
		return &bfbDownloadingState{
			bfb,
		}
	case provisioningdpfv1alpha1.BfbReady:
		return &bfbReadyState{
			bfb,
		}
	case provisioningdpfv1alpha1.BfbDeleting:
		return &bfbDeletingState{
			bfb,
		}
	case provisioningdpfv1alpha1.BfbError:
		return &bfbErrorState{
			bfb,
		}
	}

	return &bfbInitializingState{
		bfb,
	}
}

func isDeleting(bfb *provisioningdpfv1alpha1.Bfb) bool {
	return !bfb.DeletionTimestamp.IsZero()
}
