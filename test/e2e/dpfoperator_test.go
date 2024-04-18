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

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane/metadata"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	artifactsPath = "../objects/"
	numClusters   = 2
)

var _ = Describe("Testing DPF Operator controller", Ordered, func() {
	dpuserviceName := "dpu-01"
	dpuServiceNamespace := "default"
	Context("deploying a DPUService", func() {
		var cleanupObjs []client.Object
		AfterAll(func() {
			By("cleaning up objects created during the test", func() {
				for _, object := range cleanupObjs {
					if err := testClient.Delete(ctx, object); err != nil && !apierrors.IsNotFound(err) {
						Expect(err).ToNot(HaveOccurred())
					}
				}
			})
		})

		It("ensure the DPF Operator is running and ready", func() {
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{
					Namespace: "dpf-operator-system",
					Name:      "dpf-operator-controller-manager"},
					deployment)).To(Succeed())
				g.Expect(deployment.Status.ReadyReplicas).To(Equal(*deployment.Spec.Replicas))
			}).WithTimeout(60 * time.Second).Should(Succeed())
		})

		It("create underlying DPU clusters for test", func() {
			for i := 0; i < numClusters; i++ {
				clusterNamespace := fmt.Sprintf("dpu-%d-", i)
				clusterName := fmt.Sprintf("dpu-cluster-%d", i)
				// Create the namespace.
				// Note: we use a randomized namespace here for test isolation.
				ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: clusterNamespace}}
				if err := testClient.Create(ctx, ns); err != nil {
					// Fail if this returns any error other than alreadyExists.
					if !apierrors.IsAlreadyExists(err) {
						Expect(err).NotTo(HaveOccurred())
					}
				}
				cleanupObjs = append(cleanupObjs, ns)
				// Read the TenantControlPlane from file and create it.
				// Note: This process doesn't cover all the places in the file the name should be set.
				data, err := os.ReadFile(filepath.Join(artifactsPath, "infrastructure/dpu-control-plane.yaml"))
				Expect(err).ToNot(HaveOccurred())
				controlPlane := &unstructured.Unstructured{}
				Expect(yaml.Unmarshal(data, controlPlane)).To(Succeed())
				controlPlane.SetName(clusterName)
				controlPlane.SetNamespace(ns.Name)
				if err := testClient.Create(ctx, controlPlane); err != nil {
					// Fail if this returns any error other than alreadyExists.
					if !apierrors.IsAlreadyExists(err) {
						Expect(err).NotTo(HaveOccurred())
					}
				}
				cleanupObjs = append(cleanupObjs, controlPlane)
			}
			Eventually(func(g Gomega) {
				clusters := &unstructured.UnstructuredList{}
				clusters.SetGroupVersionKind(controlplanemeta.TenantControlPlaneGVK)
				g.Expect(testClient.List(ctx, clusters)).To(Succeed())
				g.Expect(clusters.Items).To(HaveLen(2))
				for _, cluster := range clusters.Items {
					// Note: ControlPlane readiness here is picked using a well-known path.
					// TODO: Make an equivalent check part of the control plane package.
					status, found, err := unstructured.NestedString(cluster.UnstructuredContent(), "status", "kubernetesResources", "version", "status")
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(found).To(BeTrue())
					g.Expect(status).To(Equal("Ready"))
				}
			}).WithTimeout(300 * time.Second).Should(Succeed())
		})

		// TODO: Create hollow nodes to join the cluster and host the applications.

		It("create the DPFOperatorConfig for the system", func() {
			config := &operatorv1.DPFOperatorConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dpfoperatorconfig",
					Namespace: "dpf-operator-system",
				},
				Spec: operatorv1.DPFOperatorConfigSpec{
					HostNetworkConfiguration: operatorv1.HostNetworkConfiguration{
						DPUConfiguration: make(map[string]operatorv1.DPUConfiguration),
						HostIPs:          make(map[string]string),
					},
				},
			}
			Expect(testClient.Create(ctx, config)).To(Succeed())
		})

		It("ensure the DPUService controller is running and ready", func() {
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{
					Namespace: "dpf-operator-system",
					Name:      "dpuservice-controller-manager"},
					deployment)).To(Succeed())
				g.Expect(deployment.Status.ReadyReplicas).To(Equal(*deployment.Spec.Replicas))
			}).WithTimeout(60 * time.Second).Should(Succeed())
		})

		It("create a DPUService and check that it is mirrored to each cluster", func() {
			// Read the DPUService from file and create it.
			data, err := os.ReadFile(filepath.Join(artifactsPath, "application/dpuservice.yaml"))
			Expect(err).ToNot(HaveOccurred())
			dpuService := &unstructured.Unstructured{}
			Expect(yaml.Unmarshal(data, dpuService)).To(Succeed())
			dpuService.SetName(dpuserviceName)
			dpuService.SetNamespace(dpuServiceNamespace)
			if err := testClient.Create(ctx, dpuService); err != nil {
				// Fail if this returns any error other than alreadyExists.
				if !apierrors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}
			}
			cleanupObjs = append(cleanupObjs, dpuService)
			Eventually(func(g Gomega) {
				// Get the control plane secrets.
				controlPlaneSecrets := corev1.SecretList{}
				g.Expect(testClient.List(ctx, &controlPlaneSecrets, client.MatchingLabels(controlplanemeta.DPFClusterSecretLabels))).To(Succeed())
				g.Expect(controlPlaneSecrets.Items).To(HaveLen(numClusters))

				for i := range controlPlaneSecrets.Items {
					dpuClient, err := clientForDPUCluster(&controlPlaneSecrets.Items[i])
					g.Expect(err).ToNot(HaveOccurred())
					deploymentList := appsv1.DeploymentList{}
					g.Expect(dpuClient.List(ctx, &deploymentList, client.HasLabels{"app", "release"})).To(Succeed())
					g.Expect(deploymentList.Items).To(HaveLen(1))
					g.Expect(deploymentList.Items[0].Name).To(ContainSubstring("helm-guestbook"))
				}
			}).WithTimeout(600 * time.Second).Should(Succeed())
		})

		It("delete the DPUService and check that the applications are cleaned up", func() {
			svc := &dpuservicev1.DPUService{}
			Expect(testClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: dpuserviceName}, svc)).To(Succeed())
			Expect(testClient.Delete(ctx, svc)).To(Succeed())
			// Get the control plane secrets.
			controlPlaneSecrets := corev1.SecretList{}
			Expect(testClient.List(ctx, &controlPlaneSecrets, client.MatchingLabels(controlplanemeta.DPFClusterSecretLabels))).To(Succeed())
			Expect(controlPlaneSecrets.Items).To(HaveLen(numClusters))
			Eventually(func(g Gomega) {
				for i := range controlPlaneSecrets.Items {
					dpuClient, err := clientForDPUCluster(&controlPlaneSecrets.Items[i])
					g.Expect(err).ToNot(HaveOccurred())
					deploymentList := appsv1.DeploymentList{}
					g.Expect(dpuClient.List(ctx, &deploymentList, client.HasLabels{"app", "release"})).To(Succeed())
					g.Expect(deploymentList.Items).To(BeEmpty())
				}
			}).WithTimeout(120 * time.Second).Should(Succeed())
		})
	})
})

func clientForDPUCluster(secret *corev1.Secret) (client.Client, error) {
	adminSecret, ok := secret.Data["admin.conf"]
	if !ok {
		return nil, fmt.Errorf("secret %s malformed", secret.GetName())
	}
	restCfg, err := clientcmd.RESTConfigFromKubeConfig(adminSecret)
	if err != nil {
		return nil, err
	}
	dpuClient, err := client.New(restCfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}
	return dpuClient, nil
}
