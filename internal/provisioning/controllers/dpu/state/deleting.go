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
	"os"

	provisioningdpfv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/provisioning/v1alpha1"
	dutil "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/provisioning/controllers/dpu/util"
	cutil "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/provisioning/controllers/util"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type dpuDeletingState struct {
	dpu *provisioningdpfv1alpha1.Dpu
}

func (st *dpuDeletingState) Handle(ctx context.Context, client client.Client, _ dutil.DPUOptions) (provisioningdpfv1alpha1.DpuStatus, error) {
	logger := log.FromContext(ctx)
	state := st.dpu.Status.DeepCopy()

	cfgFile := cutil.GenerateBFConfigPath(st.dpu.Name)
	err := os.Remove(cfgFile)
	if err != nil && !os.IsNotExist(err) {
		return *state, err
	}

	if err := RemoveNodeEffect(ctx, client, *st.dpu.Spec.NodeEffect, st.dpu.Spec.NodeName, st.dpu.Namespace); err != nil {
		return *state, err
	}

	deleteObjects := []crclient.Object{
		&certmanagerv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cutil.GenerateDMSClientCertName(st.dpu.Namespace),
				Namespace: st.dpu.Namespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cutil.GenerateDMSClientSecretName(st.dpu.Namespace),
				Namespace: st.dpu.Namespace,
			},
		},
		&certmanagerv1.Issuer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cutil.GenerateDMSClientIssuerName(st.dpu.Namespace),
				Namespace: st.dpu.Namespace,
			},
		},
		&certmanagerv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cutil.GenerateDMSServerCertName(st.dpu.Name),
				Namespace: st.dpu.Namespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cutil.GenerateDMSServerSecretName(st.dpu.Name),
				Namespace: st.dpu.Namespace,
			},
		},
		&certmanagerv1.Issuer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cutil.GenerateDMSServerIssuerName(st.dpu.Name),
				Namespace: st.dpu.Namespace,
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cutil.GenerateDMSPodName(st.dpu.Name),
				Namespace: st.dpu.Namespace,
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cutil.GenerateHostnetworkPodName(st.dpu.Name),
				Namespace: st.dpu.Namespace,
			},
		},
	}

	if objects, err := cutil.GetObjects(client, deleteObjects); err != nil {
		return *state, err
	} else {
		for _, object := range objects {
			logger.V(3).Info(fmt.Sprintf("delete object %s/%s", object.GetNamespace(), object.GetName()))
			if err := cutil.DeleteObject(client, object); err != nil {
				return *state, err
			}
		}
		if len(objects) == 0 {
			st.dpu.Finalizers = nil
			if err := client.Update(ctx, st.dpu); err != nil {
				return *state, err
			}
		}
	}

	return *state, nil
}
