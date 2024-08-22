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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/dpuservice/v1alpha1"
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/operator/v1alpha1"
	provisioningv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane/metadata"
	nvipamv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/nvipam/api/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/test/utils/collector"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// pullSecretName must match the name given to the secret in `create-artefact-secrets.sh`
	pullSecretName = "dpf-pull-secret"
)

var (
	testObjectsPath            = "../objects/"
	numClusters                = 1
	dpfOperatorSystemNamespace = "dpf-operator-system"
	argoCDInstanceLabel        = "argocd.argoproj.io/instance"

	// Labels and resources targeted for cleanup before running our e2e tests.
	// This cleanup is typically handled by cleanupObjs, but if an e2e test fails, the standard cleanup may not be executed.
	cleanupLabels     = map[string]string{"dpf-operator-e2e-test-cleanup": "true"}
	labelSelector     = labels.SelectorFromSet(cleanupLabels)
	resourcesToDelete = []client.ObjectList{
		&provisioningv1.DpuList{},
		&provisioningv1.DpuSetList{},
		&provisioningv1.BfbList{},
		&sfcv1.DPUServiceIPAMList{},
		&sfcv1.DPUServiceChainList{},
		&sfcv1.DPUServiceInterfaceList{},
		&dpuservicev1.DPUServiceList{},
		&operatorv1.DPFOperatorConfigList{},
		&appsv1.DeploymentList{},
		&corev1.PersistentVolumeClaimList{},
		&corev1.NamespaceList{},
	}
)

//nolint:dupl
var _ = Describe("Testing DPF Operator controller", Ordered, func() {
	// TODO: Consolidate all the DPUService* objects in one namespace to illustrate user behavior
	dpuServiceName := "dpu-01"
	hostDPUServiceName := "host-dpu-service"
	dpuServiceNamespace := "default"
	dpuServiceInterfaceName := "pf0-vf2"
	dpuServiceInterfaceNamespace := "test"
	dpuServiceChainName := "svc-chain-test"
	dpuServiceChainNamespace := "test-2"
	dpuServiceIPAMWithIPPoolName := "switched-application"
	dpuServiceIPAMWithCIDRPoolName := "routed-application"
	dpuServiceIPAMNamespace := "test-3"
	dpfProvisioningControllerPVCName := "bfb-pvc"

	imagePullSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pullSecretName,
			Namespace: "dpf-operator-system",
			Labels:    cleanupLabels,
		},
	}
	// The DPFOperatorConfig for the test.
	config := &operatorv1.DPFOperatorConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dpfoperatorconfig",
			Namespace: dpfOperatorSystemNamespace,
			Labels:    cleanupLabels,
		},
		Spec: operatorv1.DPFOperatorConfigSpec{
			ProvisioningConfiguration: operatorv1.ProvisioningConfiguration{
				BFBPersistentVolumeClaimName: dpfProvisioningControllerPVCName,
				// Note: This is hardcoded to work with the implementation in DPF Standalone.
				DHCPServerAddress: "10.33.33.254",
			},
			ImagePullSecrets: []string{
				imagePullSecret.Name,
			},
		},
	}

	Context("DPF Operator initialization", func() {
		BeforeAll(func() {
			By("cleaning up objects created during recent tests", func() {
				Expect(utils.CleanupWithLabelAndWait(ctx, testClient, labelSelector, resourcesToDelete...)).To(Succeed())
			})
		})

		AfterAll(func() {
			By("collecting resources and logs for the clusters")
			err := collectResourcesAndLogs(ctx)
			if err != nil {
				// Don't fail the test if the log collector fails - just print the errors.
				GinkgoLogr.Error(err, "failed to collect resources and logs for the clusters")
			}
			By("cleaning up objects created during the test", func() {
				Expect(utils.CleanupWithLabelAndWait(ctx, testClient, labelSelector, resourcesToDelete...)).To(Succeed())
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

		It("create the PersistentVolumeClaim for the DPF Provisioning controller", func() {
			data, err := os.ReadFile(filepath.Join(testObjectsPath, "infrastructure/dpf-provisioning-pvc.yaml"))
			Expect(err).ToNot(HaveOccurred())
			pvc := &corev1.PersistentVolumeClaim{}
			Expect(yaml.Unmarshal(data, pvc)).To(Succeed())
			pvc.SetName(dpfProvisioningControllerPVCName)
			pvc.SetNamespace(dpfOperatorSystemNamespace)
			pvc.SetLabels(cleanupLabels)
			// If there are real nodes we need to allocate storage for the volume.
			if numNodes > 0 {
				pvc.Spec.VolumeMode = ptr.To(corev1.PersistentVolumeFilesystem)
				pvc.Spec.Resources.Requests = corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				}
				pvc.Spec.StorageClassName = ptr.To("local-path")
			}
			Expect(testClient.Create(ctx, pvc)).To(Succeed())
		})

		It("create the imagePullSecret for the DPF OperatorConfig", func() {
			err := testClient.Create(ctx, imagePullSecret)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				Expect(err).ToNot(HaveOccurred())
			}
		})

		if deployKamajiControlPlane {
			It("create underlying DPU clusters for test", func() {
				for i := 0; i < numClusters; i++ {
					clusterName := fmt.Sprintf("dpu-cplane-tenant%d", i+1)
					clusterNamespace := fmt.Sprintf("dpu-cplane-tenant%d", i+1)
					// Create the namespace.
					ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: clusterNamespace}}
					ns.SetLabels(cleanupLabels)
					Expect(testClient.Create(ctx, ns)).To(Succeed())
					// Read the TenantControlPlane from file and create it.
					// Note: This process doesn't cover all the places in the file the name should be set.
					data, err := os.ReadFile(filepath.Join(testObjectsPath, "infrastructure/dpu-control-plane.yaml"))
					Expect(err).ToNot(HaveOccurred())
					controlPlane := &unstructured.Unstructured{}
					Expect(yaml.Unmarshal(data, controlPlane)).To(Succeed())
					controlPlane.SetName(clusterName)
					controlPlane.SetNamespace(ns.Name)
					controlPlane.SetLabels(cleanupLabels)

					By(fmt.Sprintf("Creating DPU Cluster %s/%s", controlPlane.GetNamespace(), controlPlane.GetName()))
					Expect(testClient.Create(ctx, controlPlane)).To(Succeed())
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
		}

		It("create the DPFOperatorConfig for the system", func() {
			Expect(testClient.Create(ctx, config)).To(Succeed())
		})

		It("ensure the DPF controllers are running and ready", func() {
			Eventually(func(g Gomega) {
				// Check the DPUService controller manager is up and ready.
				dpuServiceDeployment := &appsv1.Deployment{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{
					Namespace: dpfOperatorSystemNamespace,
					Name:      "dpuservice-controller-manager"},
					dpuServiceDeployment)).To(Succeed())
				g.Expect(dpuServiceDeployment.Status.ReadyReplicas).To(Equal(*dpuServiceDeployment.Spec.Replicas))

				// Check the DPF provisioning controller manager is up and ready.
				dpfProvisioningDeployment := &appsv1.Deployment{}
				g.Expect(testClient.Get(ctx, client.ObjectKey{
					Namespace: dpfOperatorSystemNamespace,
					Name:      "dpf-provisioning-controller-manager"},
					dpfProvisioningDeployment)).To(Succeed())
				g.Expect(dpfProvisioningDeployment.Status.ReadyReplicas).To(Equal(*dpfProvisioningDeployment.Spec.Replicas))
			}).WithTimeout(300 * time.Second).Should(Succeed())
		})

		It("create the provisioning objects", func() {
			Eventually(func(g Gomega) {
				bfb := &provisioningv1.Bfb{}
				// Read the BFB object and create it.
				By("creating the BFB")
				data, err := os.ReadFile(filepath.Join(testObjectsPath, "infrastructure/bfb.yaml"))
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(yaml.Unmarshal(data, bfb)).To(Succeed())
				bfb.SetLabels(cleanupLabels)
				g.Expect(testClient.Create(ctx, bfb)).To(Succeed())
			}).WithTimeout(10 * time.Second).Should(Succeed())

			dpuClusters, err := controlplane.GetDPFClusters(ctx, testClient)
			Expect(err).ToNot(HaveOccurred())

			Eventually(func(g Gomega) {
				for _, cluster := range dpuClusters {
					data, err := os.ReadFile(filepath.Join(testObjectsPath, "infrastructure/dpuset.yaml"))
					g.Expect(err).ToNot(HaveOccurred())
					dpuset := &provisioningv1.DpuSet{}
					Expect(yaml.Unmarshal(data, dpuset)).To(Succeed())
					By(fmt.Sprintf("Creating the DPUSet for cluster %s/%s", cluster.Name, cluster.Namespace))
					dpuset.Spec.DpuTemplate.Spec.Cluster.Name = cluster.Name
					dpuset.Spec.DpuTemplate.Spec.Cluster.NameSpace = cluster.Namespace
					dpuset.SetLabels(cleanupLabels)
					g.Expect(testClient.Create(ctx, dpuset)).To(Succeed())
				}
			}).WithTimeout(60 * time.Second).Should(Succeed())

			By(fmt.Sprintf("checking that the number of nodes is equal to %d", numNodes))
			Eventually(func(g Gomega) {
				// If we're not expecting any nodes in the cluster return with success.
				if numNodes == 0 {
					return
				}
				for i := range dpuClusters {
					nodes := &corev1.NodeList{}
					dpuClient, err := dpuClusters[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(dpuClient.List(ctx, nodes)).To(Succeed())
					By(fmt.Sprintf("Expected number of nodes %d to equal %d", len(nodes.Items), numNodes))
					g.Expect(nodes.Items).To(HaveLen(numNodes))
				}
			}).WithTimeout(30 * time.Minute).WithPolling(120 * time.Second).Should(Succeed())
		})

		It("ensure the system DPUServices are created", func() {
			Eventually(func(g Gomega) {
				dpuServices := &dpuservicev1.DPUServiceList{}
				g.Expect(testClient.List(ctx, dpuServices)).To(Succeed())
				g.Expect(dpuServices.Items).To(HaveLen(7))
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
				g.Expect(found).To(HaveKey("ovs-cni"))
				g.Expect(found).To(HaveKey("sfc-controller"))
			}).WithTimeout(60 * time.Second).Should(Succeed())

			By("Checking that DPUService objects have been mirrored to the DPUClusters")
			dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func(g Gomega) {
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())
					deployments := appsv1.DeploymentList{}
					g.Expect(dpuClient.List(ctx, &deployments, client.HasLabels{argoCDInstanceLabel})).To(Succeed())
					found := map[string]bool{}
					for i := range deployments.Items {
						g.Expect(deployments.Items[i].GetLabels()).To(HaveKey(argoCDInstanceLabel))
						g.Expect(deployments.Items[i].GetLabels()[argoCDInstanceLabel]).NotTo(Equal(""))
						found[deployments.Items[i].GetLabels()[argoCDInstanceLabel]] = true
					}
					daemonsets := appsv1.DaemonSetList{}
					g.Expect(dpuClient.List(ctx, &daemonsets, client.HasLabels{argoCDInstanceLabel})).To(Succeed())
					for i := range daemonsets.Items {
						g.Expect(daemonsets.Items[i].GetLabels()).To(HaveKey(argoCDInstanceLabel))
						g.Expect(daemonsets.Items[i].GetLabels()[argoCDInstanceLabel]).NotTo(Equal(""))
						found[daemonsets.Items[i].GetLabels()[argoCDInstanceLabel]] = true
					}

					// Expect each of the following to have been created by the operator.
					// These are labels on the appv1 type - e.g. DaemonSet or Deployment on the DPU cluster.
					g.Expect(found).To(HaveLen(7))
					g.Expect(found).To(HaveKey(ContainSubstring("multus")))
					g.Expect(found).To(HaveKey(ContainSubstring("sriov-device-plugin")))
					g.Expect(found).To(HaveKey(ContainSubstring("flannel")))
					g.Expect(found).To(HaveKey(ContainSubstring("servicefunctionchainset-controller")))
					// Note: The NVIPAM DPUService contains both a Daemonset and a Deployment - but this is overwritten in the map.
					g.Expect(found).To(HaveKey(ContainSubstring("nvidia-k8s-ipam")))
					g.Expect(found).To(HaveKey(ContainSubstring("ovs-cni")))
					g.Expect(found).To(HaveKey(ContainSubstring("sfc-controller")))
				}
			}).WithTimeout(600 * time.Second).Should(Succeed())
		})

		It("create DPUServiceInterface and check that it is mirrored to each cluster", func() {
			By("create test namespace")
			testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceInterfaceNamespace}}
			testNS.SetLabels(cleanupLabels)
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			By("create DPUServiceInterface")
			data, err := os.ReadFile(filepath.Join(testObjectsPath, "application/dpuserviceinterface.yaml"))
			Expect(err).ToNot(HaveOccurred())
			dpuServiceInterface := &unstructured.Unstructured{}
			Expect(yaml.Unmarshal(data, dpuServiceInterface)).To(Succeed())
			dpuServiceInterface.SetName(dpuServiceInterfaceName)
			dpuServiceInterface.SetNamespace(dpuServiceInterfaceNamespace)
			dpuServiceInterface.SetLabels(cleanupLabels)
			Expect(testClient.Create(ctx, dpuServiceInterface)).To(Succeed())
			By("verify ServiceInterfaceSet is created in DPF clusters")
			Eventually(func(g Gomega) {
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())
					scs := &sfcv1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceInterfaceName, Namespace: dpuServiceInterfaceNamespace}}
					g.Expect(dpuClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
				}
			}, time.Second*300, time.Millisecond*250).Should(Succeed())
		})

		It("create DPUServiceChain and check that it is mirrored to each cluster", func() {
			By("create test namespace")
			testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceChainNamespace}}
			testNS.SetLabels(cleanupLabels)
			Expect(testClient.Create(ctx, testNS)).To(Succeed())
			By("create DPUServiceChain")
			data, err := os.ReadFile(filepath.Join(testObjectsPath, "application/dpuservicechain.yaml"))
			Expect(err).ToNot(HaveOccurred())
			dpuServiceChain := &unstructured.Unstructured{}
			Expect(yaml.Unmarshal(data, dpuServiceChain)).To(Succeed())
			dpuServiceChain.SetName(dpuServiceChainName)
			dpuServiceChain.SetNamespace(dpuServiceChainNamespace)
			dpuServiceChain.SetLabels(cleanupLabels)
			Expect(testClient.Create(ctx, dpuServiceChain)).To(Succeed())
			By("verify ServiceChainSet is created in DPF clusters")
			Eventually(func(g Gomega) {
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())
					scs := &sfcv1.ServiceChainSet{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceChainName, Namespace: dpuServiceChainNamespace}}
					g.Expect(dpuClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
				}
			}, time.Second*300, time.Millisecond*250).Should(Succeed())
		})

		It("delete the DPUServiceChain & DPUServiceInterface and check that the Sets are cleaned up", func() {
			dsi := &sfcv1.DPUServiceInterface{}
			Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceInterfaceNamespace, Name: dpuServiceInterfaceName}, dsi)).To(Succeed())
			Expect(testClient.Delete(ctx, dsi)).To(Succeed())
			dsc := &sfcv1.DPUServiceChain{}
			Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceChainNamespace, Name: dpuServiceChainName}, dsc)).To(Succeed())
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

		It("create a DPUService and check Objects and ImagePullSecrets are mirrored correctly", func() {
			By("create a DPUService to be deployed on the DPUCluster")
			// Create DPUCluster DPUService and check it's correctly reconciled.
			dpuService := getDPUService(dpuServiceNamespace, dpuServiceName, false)
			dpuService.SetLabels(cleanupLabels)
			Expect(testClient.Create(ctx, dpuService)).To(Succeed())

			By("create a DPUService to be deployed on the host cluster")
			// Create a host DPUService and check it's correctly reconciled
			// Read the DPUService from file and create it.
			hostDPUService := getDPUService(dpuServiceNamespace, hostDPUServiceName, true)
			hostDPUService.SetLabels(cleanupLabels)
			Expect(testClient.Create(ctx, hostDPUService)).To(Succeed())

			By("verify DPUServices and deployments are created")
			Eventually(func(g Gomega) {
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())

					// Check the deployment from the DPUService can be found on the destination cluster.
					deploymentList := appsv1.DeploymentList{}
					g.Expect(dpuClient.List(ctx, &deploymentList, client.HasLabels{"app", "release"})).To(Succeed())
					g.Expect(deploymentList.Items).To(HaveLen(1))
					g.Expect(deploymentList.Items[0].Name).To(ContainSubstring("helm-guestbook"))

					// Check an imagePullSecret was created in the same namespace in the destination cluster.
					g.Expect(dpuClient.Get(ctx, client.ObjectKey{
						Namespace: dpuService.GetNamespace(),
						Name:      config.Spec.ImagePullSecrets[0]}, &corev1.Secret{})).To(Succeed())
				}
			}).WithTimeout(600 * time.Second).Should(Succeed())

			By("verify DPUService is created in the host cluster")
			Eventually(func(g Gomega) {
				// Check the deployment from the DPUService can be found on the host cluster.
				deploymentList := appsv1.DeploymentList{}
				g.Expect(testClient.List(ctx, &deploymentList, client.HasLabels{"app", "release"})).To(Succeed())
				g.Expect(deploymentList.Items).To(HaveLen(1))
				g.Expect(deploymentList.Items[0].Name).To(ContainSubstring("helm-guestbook"))
			}).WithTimeout(600 * time.Second).Should(Succeed())
		})

		It("delete the DPUServices and check that the applications are cleaned up", func() {
			By("delete the DPUServices")
			svc := &dpuservicev1.DPUService{}
			// Delete the DPUCluster DPUService.
			Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceNamespace, Name: dpuServiceName}, svc)).To(Succeed())
			Expect(testClient.Delete(ctx, svc)).To(Succeed())

			// Delete the host cluster DPUService.
			Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceNamespace, Name: hostDPUServiceName}, svc)).To(Succeed())
			Expect(testClient.Delete(ctx, svc)).To(Succeed())

			// Check the DPUCluster DPUService is correctly deleted.
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

			// Ensure the hostDPUService deployment is deleted from the host cluster.
			Eventually(func(g Gomega) {
				deploymentList := appsv1.DeploymentList{}
				g.Expect(testClient.List(ctx, &deploymentList, client.HasLabels{"app", "release"})).To(Succeed())
				g.Expect(deploymentList.Items).To(BeEmpty())
			}).WithTimeout(300 * time.Second).Should(Succeed())
		})

		It("create an invalid DPUServiceIPAM and ensure that the webhook rejects the request", func() {
			By("creating the DPUServiceIPAM Namespace")
			testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceIPAMNamespace}}
			testNS.SetLabels(cleanupLabels)
			Expect(testClient.Create(ctx, testNS)).To(Succeed())

			By("creating the invalid DPUServiceIPAM CR")
			dpuServiceIPAM := &sfcv1.DPUServiceIPAM{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-name",
					Namespace: dpuServiceIPAMNamespace,
				},
			}
			dpuServiceIPAM.SetGroupVersionKind(sfcv1.DPUServiceIPAMGroupVersionKind)
			dpuServiceIPAM.SetLabels(cleanupLabels)
			err := testClient.Create(ctx, dpuServiceIPAM)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("either ipv4Subnet or ipv4Network must be specified"))
		})

		It("create a DPUServiceIPAM with subnet split per node configuration and check NVIPAM IPPool is created to each cluster", func() {
			By("creating the DPUServiceIPAM CR")
			data, err := os.ReadFile(filepath.Join(testObjectsPath, "application/dpuserviceipam_subnet.yaml"))
			Expect(err).ToNot(HaveOccurred())
			dpuServiceIPAM := &unstructured.Unstructured{}
			Expect(yaml.Unmarshal(data, dpuServiceIPAM)).To(Succeed())
			dpuServiceIPAM.SetName(dpuServiceIPAMWithIPPoolName)
			dpuServiceIPAM.SetNamespace(dpuServiceIPAMNamespace)
			dpuServiceIPAM.SetLabels(cleanupLabels)
			Expect(testClient.Create(ctx, dpuServiceIPAM)).To(Succeed())

			By("checking that NVIPAM IPPool CR is created in the DPU clusters")
			Eventually(func(g Gomega) {
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())

					ipPools := &nvipamv1.IPPoolList{}
					g.Expect(dpuClient.List(ctx, ipPools, client.MatchingLabels{
						"dpf.nvidia.com/dpuserviceipam-name":      dpuServiceIPAM.GetName(),
						"dpf.nvidia.com/dpuserviceipam-namespace": dpuServiceIPAM.GetNamespace(),
					})).To(Succeed())
					g.Expect(ipPools.Items).To(HaveLen(1))

					// TODO: Check that NVIPAM has reconciled the resources and status reflects that.
				}
			}).WithTimeout(180 * time.Second).Should(Succeed())
		})

		It("delete the DPUServiceIPAM with subnet split per node configuration and check NVIPAM IPPool is deleted in each cluster", func() {
			By("deleting the DPUServiceIPAM")
			dpuServiceIPAM := &sfcv1.DPUServiceIPAM{}
			Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceIPAMNamespace, Name: dpuServiceIPAMWithIPPoolName}, dpuServiceIPAM)).To(Succeed())
			Expect(testClient.Delete(ctx, dpuServiceIPAM)).To(Succeed())

			By("checking that NVIPAM IPPool CR is deleted in each DPU cluster")
			Eventually(func(g Gomega) {
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())

					ipPools := &nvipamv1.IPPoolList{}
					g.Expect(dpuClient.List(ctx, ipPools, client.MatchingLabels{
						"dpf.nvidia.com/dpuserviceipam-name":      dpuServiceIPAM.GetName(),
						"dpf.nvidia.com/dpuserviceipam-namespace": dpuServiceIPAM.GetNamespace(),
					})).To(Succeed())
					g.Expect(ipPools.Items).To(BeEmpty())
				}
			}).WithTimeout(180 * time.Second).Should(Succeed())
		})

		It("create a DPUServiceIPAM with cidr split in subnet per node configuration and check NVIPAM CIDRPool is created to each cluster", func() {
			By("creating the DPUServiceIPAM CR")
			data, err := os.ReadFile(filepath.Join(testObjectsPath, "application/dpuserviceipam_cidr.yaml"))
			Expect(err).ToNot(HaveOccurred())
			dpuServiceIPAM := &unstructured.Unstructured{}
			Expect(yaml.Unmarshal(data, dpuServiceIPAM)).To(Succeed())
			dpuServiceIPAM.SetName(dpuServiceIPAMWithCIDRPoolName)
			dpuServiceIPAM.SetNamespace(dpuServiceIPAMNamespace)
			dpuServiceIPAM.SetLabels(cleanupLabels)
			Expect(testClient.Create(ctx, dpuServiceIPAM)).To(Succeed())

			By("checking that NVIPAM CIDRPool CR is created in the DPU clusters")
			Eventually(func(g Gomega) {
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())

					cidrPools := &nvipamv1.CIDRPoolList{}
					g.Expect(dpuClient.List(ctx, cidrPools, client.MatchingLabels{
						"dpf.nvidia.com/dpuserviceipam-name":      dpuServiceIPAM.GetName(),
						"dpf.nvidia.com/dpuserviceipam-namespace": dpuServiceIPAM.GetNamespace(),
					})).To(Succeed())
					g.Expect(cidrPools.Items).To(HaveLen(1))

					// TODO: Check that NVIPAM has reconciled the resources and status reflects that.
				}
			}).WithTimeout(180 * time.Second).Should(Succeed())
		})

		It("delete the DPUServiceIPAM with cidr split in subnet per node configuration and check NVIPAM CIDRPool is deleted in each cluster", func() {
			By("deleting the DPUServiceIPAM")
			dpuServiceIPAM := &sfcv1.DPUServiceIPAM{}
			Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceIPAMNamespace, Name: dpuServiceIPAMWithCIDRPoolName}, dpuServiceIPAM)).To(Succeed())
			Expect(testClient.Delete(ctx, dpuServiceIPAM)).To(Succeed())

			By("checking that NVIPAM CIDRPool CR is deleted in each DPU cluster")
			Eventually(func(g Gomega) {
				dpuControlPlanes, err := controlplane.GetDPFClusters(ctx, testClient)
				g.Expect(err).ToNot(HaveOccurred())
				for i := range dpuControlPlanes {
					dpuClient, err := dpuControlPlanes[i].NewClient(ctx, testClient)
					g.Expect(err).ToNot(HaveOccurred())

					cidrPools := &nvipamv1.CIDRPoolList{}
					g.Expect(dpuClient.List(ctx, cidrPools, client.MatchingLabels{
						"dpf.nvidia.com/dpuserviceipam-name":      dpuServiceIPAM.GetName(),
						"dpf.nvidia.com/dpuserviceipam-namespace": dpuServiceIPAM.GetNamespace(),
					})).To(Succeed())
					g.Expect(cidrPools.Items).To(BeEmpty())
				}
			}).WithTimeout(180 * time.Second).Should(Succeed())
		})

		It("delete DPUs, DPUSets and BFBs and ensure they are deleted", func() {
			dpuset := &provisioningv1.DpuSet{}
			data, err := os.ReadFile(filepath.Join(testObjectsPath, "infrastructure/dpuset.yaml"))
			Expect(err).ToNot(HaveOccurred())
			Expect(yaml.Unmarshal(data, dpuset)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(client.IgnoreNotFound(testClient.Delete(ctx, dpuset))).To(Succeed())
				g.Expect(apierrors.IsNotFound(testClient.Get(ctx, client.ObjectKeyFromObject(dpuset), dpuset))).To(BeTrue())
			}).WithTimeout(10 * time.Second).Should(Succeed())

			bfb := &provisioningv1.Bfb{}
			data, err = os.ReadFile(filepath.Join(testObjectsPath, "infrastructure/bfb.yaml"))
			Expect(err).ToNot(HaveOccurred())
			Expect(yaml.Unmarshal(data, bfb)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(client.IgnoreNotFound(testClient.Delete(ctx, bfb))).To(Succeed())
				g.Expect(apierrors.IsNotFound(testClient.Get(ctx, client.ObjectKeyFromObject(bfb), bfb))).To(BeTrue())
			}).WithTimeout(10 * time.Second).Should(Succeed())
		})

		It("delete the DPFOperatorConfig and ensure it is deleted", func() {
			// Check that all deployments and DPUServices are deleted.
			Eventually(func(g Gomega) {
				g.Expect(client.IgnoreNotFound(testClient.Delete(ctx, config))).To(Succeed())
				g.Expect(apierrors.IsNotFound(testClient.Get(ctx, client.ObjectKeyFromObject(config), config))).To(BeTrue())
			}).WithTimeout(600 * time.Second).Should(Succeed())
		})
	})
})

func getDPUService(namespace, name string, host bool) *unstructured.Unstructured {
	// Create a host DPUService and check it's correctly reconciled
	// Read the DPUService from file and create it.
	data, err := os.ReadFile(filepath.Join(testObjectsPath, "application/dpuservice.yaml"))
	Expect(err).ToNot(HaveOccurred())
	svc := &unstructured.Unstructured{}
	// Create DPUCluster DPUService and check it's correctly reconciled.
	Expect(yaml.Unmarshal(data, svc)).To(Succeed())
	svc.SetName(name)
	svc.SetNamespace(namespace)

	// This annotation is what defines a host DPUService.
	if host {
		dpuService := &dpuservicev1.DPUService{}
		Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(svc.UnstructuredContent(), dpuService)).ToNot(HaveOccurred())
		dpuService.Spec.DeployInCluster = &host
		obj, err := machineryruntime.DefaultUnstructuredConverter.ToUnstructured(dpuService)
		Expect(err).ToNot(HaveOccurred())
		svc = &unstructured.Unstructured{
			Object: obj,
		}
	}
	return svc
}

func collectResourcesAndLogs(ctx context.Context) error {
	// Get the path to place artifacts in
	_, basePath, _, _ := runtime.Caller(0)
	artifactsPath := filepath.Join(filepath.Dir(basePath), "../../artifacts")
	inventoryManifestsPath := filepath.Join(filepath.Dir(basePath), "../../internal/operator/inventory/manifests")

	// Create a resourceCollector to dump logs and resources for test debugging.
	clusters, err := collector.GetClusterCollectors(ctx, testClient, artifactsPath, inventoryManifestsPath, restConfig)
	Expect(err).NotTo(HaveOccurred())
	return collector.New(clusters).Run(ctx)
}
