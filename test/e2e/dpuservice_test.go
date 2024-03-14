/*
Copyright 2024.

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

package e2e

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("controller", Ordered, func() {
	Context("When deploying a DPUservice applications should be created on every existing DPUCluster cluster", func() {
		It("should ensure the DPUService controller is running and ready", func() {
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{
					Namespace: "dpuservice-controller-system",
					Name:      "dpuservice-controller-manager"},
					deployment)).To(Succeed())
				g.Expect(deployment.Status.ReadyReplicas).To(Equal(*deployment.Spec.Replicas))
			}).WithTimeout(60 * time.Second).Should(Succeed())
		})
		It("should create two DPU clusters", func() {})
		It("should create hollow-nodes to join to the DPU clusters", func() {})
		It("should create a DPUService application and check that it is mirrored to each cluster", func() {})
	})
})
