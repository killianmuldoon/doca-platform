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

package serversideapply

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	autoscalingv1 "k8s.io/client-go/applyconfigurations/autoscaling/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Server Side Apply", func() {
	var (
		assertPod *corev1.Pod
		pod       *corev1.Pod
	)

	BeforeEach(func() {
		autoscalingv1.CrossVersionObjectReference()
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-" + fmt.Sprintf("%d", GinkgoRandomSeed()),
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:1.7.9",
					},
				},
			},
		}
		assertPod = &corev1.Pod{}
	})
	Context("should be able to patch", func() {
		It("finalizers and remove it", func() {
			Expect(Patch(ctx, testClient, "test-owner", pod)).To(Succeed())

			// Add finalizer and expect it to be on the object
			pod.Finalizers = []string{"n.com/false"}
			Expect(Patch(ctx, testClient, "test-owner", pod)).To(Succeed())
			Expect(testClient.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, assertPod)).To(Succeed())
			Expect(assertPod.Finalizers).To(ConsistOf("n.com/false"))

			// Remove finalizer and expect it to be removed from the object
			controllerutil.RemoveFinalizer(pod, "n.com/false")
			Expect(Patch(ctx, testClient, "test-owner", pod)).To(Succeed())
			Expect(testClient.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, assertPod)).To(Succeed())
			Expect(assertPod.Finalizers).To(BeEmpty())
		})
		It("annoations and remove it", func() {
			Expect(Patch(ctx, testClient, "test-owner", pod)).To(Succeed())

			pod.Annotations = map[string]string{"foo": "n.com/false"}
			Expect(Patch(ctx, testClient, "test-owner", pod)).To(Succeed())
			Expect(testClient.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, assertPod)).To(Succeed())
			Expect(assertPod.Annotations).To(HaveKeyWithValue("foo", "n.com/false"))

			pod.Annotations = map[string]string{}
			Expect(Patch(ctx, testClient, "test-owner", pod)).To(Succeed())
			Expect(testClient.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, assertPod)).To(Succeed())
			Expect(assertPod.Annotations).To(BeEmpty())
		})
		It("status and remove it", func() {
			Expect(Patch(ctx, testClient, "test-owner", pod)).To(Succeed())

			expectedConditionType := corev1.PodConditionType("foobar")
			expectedConditionStatus := corev1.ConditionStatus("True")
			pod.Status = corev1.PodStatus{
				Conditions: []corev1.PodCondition{{
					Type:   expectedConditionType,
					Status: expectedConditionStatus,
				}},
			}
			Expect(Patch(ctx, testClient, "test-owner", pod)).To(Succeed())
			Expect(testClient.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, assertPod)).To(Succeed())

			Expect(assertPod.Status.Conditions).To(ContainElement(
				And(
					HaveField("Type", expectedConditionType),
					HaveField("Status", expectedConditionStatus)),
			))

			pod.Status.Conditions = []corev1.PodCondition{}
			Expect(Patch(ctx, testClient, "test-owner", pod)).To(Succeed())
			Expect(testClient.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, assertPod)).To(Succeed())
			Expect(assertPod.Status.Conditions).To(BeEmpty())
		})
	})
})
