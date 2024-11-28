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

package controller

import (
	"context"
	"time"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DPUSet", func() {
	var testNS *corev1.Namespace

	BeforeEach(func() {
		By("creating the namespace")
		testNS = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "provisioning"}}
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, testNS))).To(Succeed())
	})

	Context("obj test context", func() {
		ctx := context.Background()

		It("create  and delete DPUSet", func() {
			dpuset := baseDPUSet(testNS.Name)
			Expect(k8sClient.Create(ctx, dpuset)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, dpuset)

			objFetched := &provisioningv1.DPUSet{}

			By("checking the finalizer")
			Eventually(func(g Gomega) []string {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(dpuset), objFetched)).To(Succeed())
				return objFetched.Finalizers
			}).WithTimeout(10 * time.Second).Should(ConsistOf([]string{provisioningv1.DPUSetFinalizer}))
		})
		It("overwrite default DPUSet automaticNodeReboot", func() {
			dpuset := baseDPUSet(testNS.Name)
			dpuset.Spec.DPUTemplate.Spec.AutomaticNodeReboot = ptr.To(false)
			Expect(k8sClient.Create(ctx, dpuset)).To(Succeed())
			DeferCleanup(k8sClient.Delete, ctx, dpuset)

			got := &provisioningv1.DPUSet{}

			By("checking the field is correctly set")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(dpuset), got)).To(Succeed())
				g.Expect(*got.Spec.DPUTemplate.Spec.AutomaticNodeReboot).To(BeFalse())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})
	})
})

func baseDPUSet(ns string) *provisioningv1.DPUSet {
	return &provisioningv1.DPUSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dpuset",
			Namespace: ns,
		},
		Spec: provisioningv1.DPUSetSpec{
			NodeSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"feature.node.kubernetes.io/dpu-enabled": "true"}},
			DPUSelector: map[string]string{
				"feature.node.kubernetes.io/dpu-0-psid":        "MT_0000000375",
				"feature.node.kubernetes.io/dpu-0-pci-address": "0000-04-00",
			},
			Strategy: &provisioningv1.DPUSetStrategy{
				Type: provisioningv1.RollingUpdateStrategyType,
				RollingUpdate: &provisioningv1.RollingUpdateDPU{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				},
			},
			DPUTemplate: provisioningv1.DPUTemplate{
				Annotations: map[string]string{
					"nvidia.com/dpuOperator-override-powercycle-command": "cycle",
				},
				Spec: provisioningv1.DPUTemplateSpec{
					AutomaticNodeReboot: ptr.To(true),
					DPUFlavor:           "hbn",
					NodeEffect: &provisioningv1.NodeEffect{
						Drain: &provisioningv1.Drain{
							AutomaticNodeReboot: false,
						},
					},
					Cluster: &provisioningv1.ClusterSpec{
						NodeLabels: map[string]string{
							"dpf.node.dpu/role": "worker",
						},
					},
				},
			},
		},
	}
}
