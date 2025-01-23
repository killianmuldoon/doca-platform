/*
Copyright 2025 NVIDIA

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
	"testing"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDMSServerReconciler(t *testing.T) {
	g := NewWithT(t)

	bfbName := "bfb-name"
	nodeName := "node-1"
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				// DPUs require a node with this label to advance past the "Initializing" phase
				cutil.NodeFeatureDiscoveryLabelPrefix + cutil.DPUOOBBridgeConfiguredLabel: "true",
			},
		},
	}

	bfb := &provisioningv1.BFB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bfbName,
			Namespace: "default",
		},
		Spec: provisioningv1.BFBSpec{
			URL: "http://BlueField/BFBs/bf-bundle-dummy-8KB.bfb",
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "bfb",
					Image: "nvidia/cuda/bundle:1.0",
				},
			},
		},
	}
	tests := []struct {
		name  string
		input *provisioningv1.DPU
	}{
		{
			name: "Can provision DPU past initializing",
			input: &provisioningv1.DPU{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dpu-1",
					Namespace: "default",
				},
				Spec: provisioningv1.DPUSpec{
					NodeName:   nodeName,
					BFB:        bfbName,
					PCIAddress: "08",
					NodeEffect: &provisioningv1.NodeEffect{
						// TODO: NoEffect should be changed here to test more of the code.
						NoEffect: ptr.To(true),
					},
					Cluster: provisioningv1.K8sCluster{
						Namespace: "default",
						Name:      "cluster-1",
					},
					DPUFlavor:           "",
					AutomaticNodeReboot: false,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Set up the prerequisite objects for the DPU.
			g.Expect(testClient.Create(ctx, node)).To(Succeed())
			g.Expect(testClient.Create(ctx, bfb)).To(Succeed())
			g.Expect(testClient.Create(ctx, pod)).To(Succeed())
			bfb.Status.Phase = provisioningv1.BFBReady
			g.Expect(testClient.Status().Update(ctx, bfb)).To(Succeed())
			pod.Status.Phase = corev1.PodRunning
			g.Expect(testClient.Status().Update(ctx, pod)).To(Succeed())

			// Create the DPU and check its provisioning process.
			g.Expect(testClient.Create(ctx, tt.input)).To(Succeed())
			dpu := &provisioningv1.DPU{}
			g.Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(tt.input), dpu)).To(Succeed())
				g.Expect(dpu.Annotations).To(HaveKey(cutil.OverrideDMSPodNameAnnotationKey))
				g.Expect(dpu.Annotations).To(HaveKey(cutil.OverrideDMSPortAnnotationKey))

				g.Expect(dpu.Status.Phase).To(Equal(provisioningv1.DPUInitializeInterface))
			}).WithTimeout(100 * time.Second).Should(Succeed())
		})
	}
}
