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
	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane/metadata"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	testObjectsPath            = "../objects/"
	numClusters                = 1
	dpfOperatorSystemNamespace = "dpf-operator-system"
)

//nolint:dupl
var _ = Describe("Testing DPF Operator controller", Ordered, func() {
	dpuserviceName := "dpu-01"
	dpuServiceNamespace := "default"
	dpuServiceInterfaceNamespace := "test"
	dpuserviceinterfaceName := "pf0-vf2"
	dpuServiceChainNamespace := "test-2"
	dpuservicechainName := "svc-chain-test"
	dpfProvisioningControllerPVCName := "dpf-provisioning-volume"

	// The DPFOperatorConfig for the test.
	config := &operatorv1.DPFOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dpfoperatorconfig",
			Namespace: dpfOperatorSystemNamespace,
		},
		Spec: operatorv1.DPFOperatorConfigSpec{
			HostNetworkConfiguration: operatorv1.HostNetworkConfiguration{
				Hosts: []operatorv1.Host{},
			},
			ProvisioningConfiguration: operatorv1.ProvisioningConfiguration{
				BFBPersistentVolumeClaimName: dpfProvisioningControllerPVCName,
				ImagePullSecret:              "some-secret",
			},
		},
	}

	Context("deploying a DPUService", func() {
		var cleanupObjs []client.Object
		AfterAll(func() {
			By("collecting resources and logs for the clusters")
			Expect(resourceCollector.Run(ctx)).To(Succeed())
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
					Namespace: dpfOperatorSystemNamespace,
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
				data, err := os.ReadFile(filepath.Join(testObjectsPath, "infrastructure/dpu-control-plane.yaml"))
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
				g.Expect(clusters.Items).To(HaveLen(numClusters))
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

		It("create the PersistentVolumeClaim for the DPF Provisioning controller", func() {
			data, err := os.ReadFile(filepath.Join(testObjectsPath, "infrastructure/dpf-provisioning-pvc.yaml"))
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			Expect(yaml.Unmarshal(data, pvc)).To(Succeed())
			pvc.SetName(dpfProvisioningControllerPVCName)
			pvc.SetNamespace(dpfOperatorSystemNamespace)
			if err := testClient.Create(ctx, pvc); err != nil {
				// Fail if this returns any error other than alreadyExists.
				if !apierrors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}
			}
		})

		It("create the DPFOperatorConfig for the system", func() {
			Expect(testClient.Create(ctx, config)).To(Succeed())
		})

		It("ensure the DPUService controller is running and ready", func() {
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{
					Namespace: dpfOperatorSystemNamespace,
					Name:      "dpuservice-controller-manager"},
					deployment)).To(Succeed())
				g.Expect(deployment.Status.ReadyReplicas).To(Equal(*deployment.Spec.Replicas))
			}).WithTimeout(60 * time.Second).Should(Succeed())
		})

		It("ensure the DPF Provisioning controller is running and ready", func() {
			Eventually(func(g Gomega) {
				deployment := &appsv1.Deployment{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{
					Namespace: dpfOperatorSystemNamespace,
					Name:      "dpf-provisioning-controller-manager"},
					deployment)).To(Succeed())
				g.Expect(deployment.Status.ReadyReplicas).To(Equal(*deployment.Spec.Replicas))
			}).WithTimeout(60 * time.Second).Should(Succeed())
		})

		It("ensure the system DPUServices are created and mirrored to the tenant clusters", func() {
			Eventually(func(g Gomega) {
				dpuServices := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, dpuServices)).To(Succeed())
				g.Expect(dpuServices.Items).To(HaveLen(5))
				found := map[string]bool{}
				for i := range dpuServices.Items {
					found[dpuServices.Items[i].Name] = true
				}

				// Expect each of the following to have been created by the operator.
				g.Expect(found).To(HaveKey("multus"))
				g.Expect(found).To(HaveKey("sriov-device-plugin"))
				g.Expect(found).To(HaveKey("flannel"))
				g.Expect(found).To(HaveKey("servicefunctionchainset-controller"))
				g.Expect(found).To(HaveKey("nvidia-k8s-ipam"))

			}).WithTimeout(60 * time.Second).Should(Succeed())

			Eventually(func(g Gomega) {
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					found := map[string]bool{}
					g.Expect(err).ToNot(HaveOccurred())
					deployments := appsv1.DeploymentList{}
					g.Expect(dpuClient.List(ctx, &deployments)).To(Succeed())
					for i := range deployments.Items {
						found[deployments.Items[i].GetLabels()["app.kubernetes.io/instance"]] = true
					}
					daemonsets := appsv1.DaemonSetList{}
					g.Expect(dpuClient.List(ctx, &daemonsets)).To(Succeed())
					for i := range daemonsets.Items {
						found[daemonsets.Items[i].GetLabels()["app.kubernetes.io/instance"]] = true
					}

					// Expect each of the following to have been created by the operator.
					// These are labels of the appv1 type - e.g. DaemonSet or Deployment on the DPU cluster.
					g.Expect(found).To(HaveKey(ContainSubstring("multus")))
					g.Expect(found).To(HaveKey(ContainSubstring("sriov-device-plugin")))
					g.Expect(found).To(HaveKey(ContainSubstring("flannel")))
					g.Expect(found).To(HaveKey(ContainSubstring("servicefunctionchainset-controller")))
					g.Expect(found).To(HaveKey(ContainSubstring("nvidia-k8s-ipam")))
				}
			}).WithTimeout(180 * time.Second).Should(Succeed())
		})

		It("create DPUServiceInterface and check that it is mirrored to each cluster", func() {
			By("create test namespace")
			testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceInterfaceNamespace}}
			if err := testClient.Create(ctx, testNS); err != nil {
				// Fail if this returns any error other than alreadyExists.
				if !apierrors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}
			}
			cleanupObjs = append(cleanupObjs, testNS)
			By("create DPUServiceInterface")
			data, err := os.ReadFile(filepath.Join(testObjectsPath, "application/dpuserviceinterface.yaml"))
			Expect(err).ToNot(HaveOccurred())
			dpuServiceInterface := &unstructured.Unstructured{}
			Expect(yaml.Unmarshal(data, dpuServiceInterface)).To(Succeed())
			dpuServiceInterface.SetName(dpuserviceinterfaceName)
			dpuServiceInterface.SetNamespace(dpuServiceInterfaceNamespace)
			if err := testClient.Create(ctx, dpuServiceInterface); err != nil {
				// Fail if this returns any error other than alreadyExists.
				if !apierrors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}
			}
			cleanupObjs = append(cleanupObjs, dpuServiceInterface)
			By("verify ServiceInterfaceSet is created in DPF clusters")
			Eventually(func(g Gomega) {
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())
					scs := &sfcv1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: dpuserviceinterfaceName, Namespace: dpuServiceInterfaceNamespace}}
					g.Expect(dpuClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
				}
			}, time.Second*300, time.Millisecond*250).Should(Succeed())
		})

		It("create DPUServiceChain and check that it is mirrored to each cluster", func() {
			By("create test namespace")
			testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceChainNamespace}}
			if err := testClient.Create(ctx, testNS); err != nil {
				// Fail if this returns any error other than alreadyExists.
				if !apierrors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}
			}
			cleanupObjs = append(cleanupObjs, testNS)
			By("create DPUServiceChain")
			data, err := os.ReadFile(filepath.Join(testObjectsPath, "application/dpuservicechain.yaml"))
			Expect(err).ToNot(HaveOccurred())
			dpuServiceChain := &unstructured.Unstructured{}
			Expect(yaml.Unmarshal(data, dpuServiceChain)).To(Succeed())
			dpuServiceChain.SetName(dpuservicechainName)
			dpuServiceChain.SetNamespace(dpuServiceChainNamespace)
			if err := testClient.Create(ctx, dpuServiceChain); err != nil {
				// Fail if this returns any error other than alreadyExists.
				if !apierrors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}
			}
			cleanupObjs = append(cleanupObjs, dpuServiceChain)
			By("verify ServiceChainSet is created in DPF clusters")
			Eventually(func(g Gomega) {
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())
					scs := &sfcv1.ServiceChainSet{ObjectMeta: metav1.ObjectMeta{Name: dpuservicechainName, Namespace: dpuServiceChainNamespace}}
					g.Expect(dpuClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
				}
			}, time.Second*300, time.Millisecond*250).Should(Succeed())
		})

		It("delete the DPUServiceChain & DPUServiceInterface and check that the Sets are cleaned up", func() {
			dsi := &sfcv1.DPUServiceInterface{}
			Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceInterfaceNamespace, Name: dpuserviceinterfaceName}, dsi)).To(Succeed())
			Expect(testClient.Delete(ctx, dsi)).To(Succeed())
			dsc := &sfcv1.DPUServiceChain{}
			Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceChainNamespace, Name: dpuservicechainName}, dsc)).To(Succeed())
			Expect(testClient.Delete(ctx, dsc)).To(Succeed())
			// Get the control plane secrets.
			Eventually(func(g Gomega) {
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())
					serviceChainSetList := sfcv1.ServiceChainSetList{}
					g.Expect(dpuClient.List(ctx, &serviceChainSetList,
						&client.ListOptions{Namespace: dpuServiceChainNamespace})).To(Succeed())
					g.Expect(serviceChainSetList.Items).To(BeEmpty())
					serviceInterfaceSetList := sfcv1.ServiceInterfaceSetList{}
					g.Expect(dpuClient.List(ctx, &serviceInterfaceSetList,
						&client.ListOptions{Namespace: dpuServiceInterfaceNamespace})).To(Succeed())
					g.Expect(serviceInterfaceSetList.Items).To(BeEmpty())
				}
			}).WithTimeout(300 * time.Second).Should(Succeed())
		})

		It("create a DPUService and check that it is mirrored to each cluster", func() {
			// Read the DPUService from file and create it.
			data, err := os.ReadFile(filepath.Join(testObjectsPath, "application/dpuservice.yaml"))
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
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())
					deploymentList := appsv1.DeploymentList{}
					g.Expect(dpuClient.List(ctx, &deploymentList, client.HasLabels{"app", "release"})).To(Succeed())
					g.Expect(deploymentList.Items).To(HaveLen(1))
					g.Expect(deploymentList.Items[0].Name).To(ContainSubstring("helm-guestbook"))
				}
			}).WithTimeout(180 * time.Second).Should(Succeed())
		})

		It("delete the DPUService and check that the applications are cleaned up", func() {
			svc := &dpuservicev1.DPUService{}
			Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceNamespace, Name: dpuserviceName}, svc)).To(Succeed())
			Expect(testClient.Delete(ctx, svc)).To(Succeed())
			// Get the control plane secrets.
			Eventually(func(g Gomega) {
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())
					deploymentList := appsv1.DeploymentList{}
					g.Expect(dpuClient.List(ctx, &deploymentList, client.HasLabels{"app", "release"})).To(Succeed())
					g.Expect(deploymentList.Items).To(BeEmpty())
				}
			}).WithTimeout(300 * time.Second).Should(Succeed())
		})

		It("delete the DPFOperatorConfig and ensure it is deleted", func() {
			// Check that all deployments and DPUServices are deleted.
			Expect(testClient.Delete(ctx, config)).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(apierrors.IsNotFound(testClient.Get(ctx, client.ObjectKeyFromObject(config), config))).To(BeTrue())
			}).WithTimeout(300 * time.Second).Should(Succeed())
		})
	})
})
