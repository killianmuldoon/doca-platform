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
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"time"

	dpuservicev1 "github.com/nvidia/doca-platform/api/dpuservice/v1alpha1"
	operatorv1 "github.com/nvidia/doca-platform/api/operator/v1alpha1"
	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/conditions"
	"github.com/nvidia/doca-platform/internal/dpfctl"
	dpucluster "github.com/nvidia/doca-platform/internal/dpucluster"
	"github.com/nvidia/doca-platform/test/utils"
	"github.com/nvidia/doca-platform/test/utils/collector"
	"github.com/nvidia/doca-platform/test/utils/metrics"
	nvipamv1 "github.com/nvidia/doca-platform/third_party/api/nvipam/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DPFSystemTest provisions a cluster with a DPF system and runs the inputted tests on it.
// This function is designed to be run inside a ginkgo `Describe` test spec.
func DPFSystemTest(input systemTestInput, tests []dpfTest) {
	DeployDPFSystemComponents(ctx, DeployDPFSystemComponentsInput{
		systemNamespace:           input.namespace,
		operatorConfig:            input.config,
		ImagePullSecrets:          input.pullSecretNames,
		ProvisioningControllerPVC: input.pvc,
	})

	ProvisionDPUClusters(ctx, ProvisionDPUClustersInput{
		numberOfNodesPerCluster: input.numberOfDPUNodes,
		dpuClusterPrerequisites: input.additionalProvisioningObjects,
		// This annotation prevents nodes from being restarted during the e2e provisioning test flow which speeds up the test.
		nodeAnnotations: map[string]string{"provisioning.dpu.nvidia.com/reboot-command": "skip"},
		dpuCluster:      input.dpuCluster,
		dpuSet:          input.dpuSet,
		bfb:             input.bfb,
		// This server override enables running the e2e tests using Docker Desktop on MacOS. The port must match the port contained
		// in the nodeport defined in the nodePortService in the dpuClusterPrerequisites.
		dpuClusterClientOptions: []dpucluster.ClientOption{dpucluster.OverrideClientConfigHost{Server: "https://127.0.0.1:32443"}},
	})

	// Run each additional test passed to the spec
	// TODO: Consider using a map here to ensure the tests are independent.
	for _, test := range tests {
		test(ctx, input)
	}
}

type dpfTest func(context.Context, systemTestInput)

type systemTestInput struct {
	namespace                     string
	config                        *operatorv1.DPFOperatorConfig
	pvc                           *corev1.PersistentVolumeClaim
	additionalProvisioningObjects []client.Object
	dpuCluster                    *provisioningv1.DPUCluster
	dpuService                    *dpuservicev1.DPUService
	dpuServiceInterface           *dpuservicev1.DPUServiceInterface
	dpuServiceChain               *dpuservicev1.DPUServiceChain
	bfb                           *provisioningv1.BFB
	dpuSet                        *provisioningv1.DPUSet
	dpuDeployment                 *dpuservicev1.DPUDeployment
	dpuServiceConfiguration       *dpuservicev1.DPUServiceConfiguration
	dpuServiceTemplate            *dpuservicev1.DPUServiceTemplate
	cidrDPUServiceIPAM            *dpuservicev1.DPUServiceIPAM
	ipPoolDPUServiceIPAM          *dpuservicev1.DPUServiceIPAM
	dpuServiceCredentialRequest   *dpuservicev1.DPUServiceCredentialRequest
	numberOfDPUNodes              int
	pullSecretNames               []string
}

func (t *systemTestInput) applyConfig(conf config) {
	bfb := &provisioningv1.BFB{}
	bfbUnstructured := unstructuredFromFile(conf.BFBPath)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(bfbUnstructured.Object, bfb)).To(Succeed())
	t.bfb = bfb

	dpuSet := &provisioningv1.DPUSet{}
	dpuSetUnstructured := unstructuredFromFile(conf.DPUSetPath)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(dpuSetUnstructured.Object, dpuSet)).To(Succeed())
	t.dpuSet = dpuSet

	pvc := &corev1.PersistentVolumeClaim{}
	pvcUnstructured := unstructuredFromFile(conf.ProvisioningControllerPVCPath)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(pvcUnstructured.Object, pvc)).To(Succeed())
	t.pvc = pvc

	dpuCluster := &provisioningv1.DPUCluster{}
	dpuClusterUnstructured := unstructuredFromFile(conf.DPUClusterPath)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(dpuClusterUnstructured.Object, dpuCluster)).To(Succeed())
	t.dpuCluster = dpuCluster

	dpuServiceInterface := &dpuservicev1.DPUServiceInterface{}
	dsi := unstructuredFromFile(conf.DPUServiceInterfacePath)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(dsi.Object, dpuServiceInterface)).To(Succeed())
	t.dpuServiceInterface = dpuServiceInterface

	dpuService := &dpuservicev1.DPUService{}
	svc := unstructuredFromFile(conf.DPUServicePath)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(svc.Object, dpuService)).To(Succeed())
	t.dpuService = dpuService

	dpuClusterPrerequisiteObjects := []client.Object{}
	for _, path := range conf.DPUClusterPrerequisiteObjectPaths {
		dpuClusterPrerequisiteObjects = append(dpuClusterPrerequisiteObjects, unstructuredFromFile(path))
	}
	t.additionalProvisioningObjects = dpuClusterPrerequisiteObjects

	dpuServiceTemplate := &dpuservicev1.DPUServiceTemplate{}
	tmp := unstructuredFromFile(conf.DPUServiceTemplatePath)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(tmp.Object, dpuServiceTemplate)).To(Succeed())
	t.dpuServiceTemplate = dpuServiceTemplate

	dpuServiceConfiguration := &dpuservicev1.DPUServiceConfiguration{}
	svcConfig := unstructuredFromFile(conf.DPUServiceConfiguration)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(svcConfig.Object, dpuServiceConfiguration)).To(Succeed())
	t.dpuServiceConfiguration = dpuServiceConfiguration

	dpuDeployment := &dpuservicev1.DPUDeployment{}
	deployment := unstructuredFromFile(conf.DPUDeploymentPath)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(deployment.Object, dpuDeployment)).To(Succeed())
	t.dpuDeployment = dpuDeployment

	ipPoolDPUServiceIPAM := &dpuservicev1.DPUServiceIPAM{}
	subnetIPAM := unstructuredFromFile(conf.IPPoolDPUServiceIPAMPath)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(subnetIPAM.Object, ipPoolDPUServiceIPAM)).To(Succeed())
	t.ipPoolDPUServiceIPAM = ipPoolDPUServiceIPAM

	cidrDPUServiceIPAM := &dpuservicev1.DPUServiceIPAM{}
	cidrIPAM := unstructuredFromFile(conf.CIDRPoolDPUServiceIPAMPath)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(cidrIPAM.Object, cidrDPUServiceIPAM)).To(Succeed())
	t.cidrDPUServiceIPAM = cidrDPUServiceIPAM

	dpuServiceChain := &dpuservicev1.DPUServiceChain{}
	chain := unstructuredFromFile(conf.DPUServiceChainPath)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(chain.Object, dpuServiceChain)).To(Succeed())
	t.dpuServiceChain = dpuServiceChain

	dpuServiceCredentialRequest := &dpuservicev1.DPUServiceCredentialRequest{}
	request := unstructuredFromFile(conf.DPUServiceCredentialRequestPath)
	Expect(machineryruntime.DefaultUnstructuredConverter.FromUnstructured(request.Object, dpuServiceCredentialRequest)).To(Succeed())
	t.dpuServiceCredentialRequest = dpuServiceCredentialRequest

	t.numberOfDPUNodes = conf.NumberOfDPUNodes
}

type DeployDPFSystemComponentsInput struct {
	operatorConfig            *operatorv1.DPFOperatorConfig
	systemNamespace           string
	ProvisioningControllerPVC *corev1.PersistentVolumeClaim
	ImagePullSecrets          []string
}

// DeployDPFSystemComponents creates the operatorConfig and some dependencies and checks that the system components
// are deployed from the operator.
// 1) Ensures the DPF Operator is running and ready
// 2) Creates a PersistentVolumeClaim for the Provisioning controller
// 3) Creates ImagePullSecrets which are tested as part of the e2e flow (note these are fake and could possibly be replaced by real ones)
// 4) Creates the operatorConfig for the test
// 5) Ensures the DPF System components - including DPUServices - have been deployed.
func DeployDPFSystemComponents(ctx context.Context, input DeployDPFSystemComponentsInput) {
	It("ensure the DPF Operator is running and ready", func() {
		Eventually(func(g Gomega) {
			deployment := &appsv1.Deployment{}
			g.Expect(testClient.Get(ctx, client.ObjectKey{
				Namespace: input.systemNamespace,
				Name:      "dpf-operator-controller-manager"},
				deployment)).To(Succeed())
			g.Expect(deployment.Status.ReadyReplicas).To(Equal(*deployment.Spec.Replicas))
		}).WithTimeout(60 * time.Second).Should(Succeed())
	})

	It("create the PersistentVolumeClaim for the DPF Provisioning controller", func() {
		pvc := input.ProvisioningControllerPVC.DeepCopy()
		pvc.SetName(input.operatorConfig.Spec.ProvisioningController.BFBPersistentVolumeClaimName)
		pvc.SetNamespace(input.systemNamespace)
		pvc.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, pvc)).To(Succeed())
	})

	It("creates the imagePullSecrets for the DPFOperatorConfig", func() {
		for _, secretName := range input.ImagePullSecrets {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: input.systemNamespace,
					Labels:    cleanupLabels,
				},
			}
			Expect(client.IgnoreAlreadyExists(testClient.Create(ctx, secret))).ToNot(HaveOccurred())
		}
	})

	It("create the DPFOperatorConfig for the system", func() {
		Expect(testClient.Create(ctx, input.operatorConfig)).To(Succeed())
	})

	It("ensure the DPF controllers are running and ready", func() {
		Eventually(func(g Gomega) {
			// Check the DPUService controller manager is up and ready.
			dpuServiceDeployment := &appsv1.Deployment{}
			g.Expect(testClient.Get(ctx, client.ObjectKey{
				Namespace: input.systemNamespace,
				Name:      "dpuservice-controller-manager"},
				dpuServiceDeployment)).To(Succeed())
			g.Expect(dpuServiceDeployment.Status.ReadyReplicas).To(Equal(*dpuServiceDeployment.Spec.Replicas))

			// Check the DPF provisioning controller manager is up and ready.
			dpfProvisioningDeployment := &appsv1.Deployment{}
			g.Expect(testClient.Get(ctx, client.ObjectKey{
				Namespace: input.systemNamespace,
				Name:      "dpf-provisioning-controller-manager"},
				dpfProvisioningDeployment)).To(Succeed())
			g.Expect(dpfProvisioningDeployment.Status.ReadyReplicas).To(Equal(*dpfProvisioningDeployment.Spec.Replicas))
		}).WithTimeout(300 * time.Second).Should(Succeed())
	})

	It("ensure the system DPUServices are created", func() {
		Eventually(func(g Gomega) {
			dpuServices := &dpuservicev1.DPUServiceList{}
			g.Expect(testClient.List(ctx, dpuServices)).To(Succeed())
			g.Expect(dpuServices.Items).To(HaveLen(9))
			found := map[string]bool{}
			for i := range dpuServices.Items {
				found[dpuServices.Items[i].Name] = true
			}

			// Expect each of the following to have been created by the operator.
			g.Expect(found).To(HaveKey(operatorv1.MultusName))
			g.Expect(found).To(HaveKey(operatorv1.SRIOVDevicePluginName))
			g.Expect(found).To(HaveKey(operatorv1.ServiceSetControllerName))
			g.Expect(found).To(HaveKey(operatorv1.FlannelName))
			g.Expect(found).To(HaveKey(operatorv1.NVIPAMName))
			g.Expect(found).To(HaveKey(operatorv1.OVSCNIName))
			g.Expect(found).To(HaveKey(operatorv1.SFCControllerName))
			g.Expect(found).To(HaveKey(operatorv1.OVSHelperName))
		}).WithTimeout(60 * time.Second).Should(Succeed())
	})
}

type ProvisionDPUClustersInput struct {
	numberOfNodesPerCluster int
	dpuClusterPrerequisites []client.Object
	nodeAnnotations         map[string]string
	dpuClusterClientOptions []dpucluster.ClientOption
	dpuCluster              *provisioningv1.DPUCluster
	bfb                     *provisioningv1.BFB
	dpuSet                  *provisioningv1.DPUSet
}

func ProvisionDPUClusters(ctx context.Context, input ProvisionDPUClustersInput) {
	// TODO: Pass this in as config instead of as a global.
	if bfbImageURL != "" {
		It("override BFB URL with the value from the env BFB_IMAGE_URL", func() {
			input.bfb.Spec.URL = bfbImageURL
		})
	}
	// Add additional annotations to the Nodes.
	if len(input.nodeAnnotations) > 0 {
		It("annotate nodes from cluster", func() {
			Eventually(func(g Gomega) {
				nodes := &corev1.NodeList{}
				g.Expect(testClient.List(ctx, nodes)).To(Succeed())
				for _, node := range nodes.Items {
					original := node.DeepCopy()
					annotations := node.GetAnnotations()
					if annotations == nil {
						annotations = map[string]string{}
					}
					for k, v := range input.nodeAnnotations {
						annotations[k] = v
					}
					node.SetAnnotations(annotations)
					g.Expect(testClient.Patch(ctx, &node, client.MergeFrom(original))).To(Succeed())
				}
			}).Should(Succeed())
		})
	}

	It("create prerequisites objects for DPUClusters", func() {
		for _, obj := range input.dpuClusterPrerequisites {
			obj.SetLabels(cleanupLabels)
			By(fmt.Sprintf("Creating prerequisite object %s %s/%s", obj.GetObjectKind().GroupVersionKind().String(), obj.GetNamespace(), obj.GetName()))
			Expect(testClient.Create(ctx, obj)).To(Succeed())
		}
	})

	It("create DPUCluster", func() {
		input.dpuCluster.SetLabels(cleanupLabels)
		By(fmt.Sprintf("Creating DPU Cluster %s/%s", input.dpuCluster.GetNamespace(), input.dpuCluster.GetName()))
		Expect(testClient.Create(ctx, input.dpuCluster)).To(Succeed())

		Eventually(func(g Gomega) {
			clusters := &provisioningv1.DPUClusterList{}
			g.Expect(testClient.List(ctx, clusters)).To(Succeed())
			g.Expect(clusters.Items).To(HaveLen(1))
			for _, cluster := range clusters.Items {
				g.Expect(cluster.Status.Phase).Should(Equal(provisioningv1.PhaseReady))
			}
		}).WithTimeout(300 * time.Second).Should(Succeed())
	})

	It("create the BFB and DPUSet", func() {
		Eventually(func(g Gomega) {
			By("creating the BFB")
			bfb := input.bfb.DeepCopy()
			bfb.SetLabels(cleanupLabels)
			g.Expect(testClient.Create(ctx, bfb)).To(Succeed())
		}).WithTimeout(10 * time.Second).Should(Succeed())

		Eventually(func(g Gomega) {
			By("Creating the DPUSet")
			dpuset := input.dpuSet.DeepCopy()
			// TODO: Test the cleanup of the node related to the DPU.
			dpuset.SetLabels(cleanupLabels)
			g.Expect(testClient.Create(ctx, dpuset)).To(Succeed())
		}).WithTimeout(60 * time.Second).Should(Succeed())

		By("creating a client for the DPUCluster")
		Eventually(func(g Gomega) {
			var err error
			dpuClusters, err := dpucluster.GetConfigs(ctx, testClient)
			Expect(err).ToNot(HaveOccurred())
			Expect(dpuClusters).To(HaveLen(1))
			dpuClusterClient, err = dpuClusters[0].Client(ctx, input.dpuClusterClientOptions...)
			g.Expect(err).ToNot(HaveOccurred())
		}).WithTimeout(10 * time.Second).Should(Succeed())

		By(fmt.Sprintf("checking that the number of nodes is equal to %d", input.numberOfNodesPerCluster))
		Eventually(func(g Gomega) {
			nodes := &corev1.NodeList{}
			g.Expect(dpuClusterClient.List(ctx, nodes)).ToNot(HaveOccurred())
			By(fmt.Sprintf("Expected number of nodes %d to equal %d", len(nodes.Items), input.numberOfNodesPerCluster))
			g.Expect(nodes.Items).To(HaveLen(input.numberOfNodesPerCluster))
		}).WithTimeout(30 * time.Minute).WithPolling(30 * time.Second).Should(Succeed())
	})

	It("ensure the system DPUServices are created correctly", func() {
		By("Checking the DPUServices have been mirrored to the target cluster")
		Eventually(func(g Gomega) {
			serviceSetDeployment := &appsv1.Deployment{}
			g.Expect(testClient.Get(ctx, client.ObjectKey{
				Namespace: dpfOperatorSystemNamespace,
				Name:      "servicechainset-controller-manager"},
				serviceSetDeployment)).To(Succeed())
			g.Expect(serviceSetDeployment.Status.ReadyReplicas).To(Equal(*serviceSetDeployment.Spec.Replicas))
		}).WithTimeout(600 * time.Second).Should(Succeed())

		By("Checking that DPUService objects have been mirrored to the DPUClusters")
		Eventually(func(g Gomega) {
			deployments := &appsv1.DeploymentList{}
			g.Expect(dpuClusterClient.List(ctx, deployments, client.HasLabels{argoCDInstanceLabel})).To(Succeed())
			found := map[string]bool{}
			for i := range deployments.Items {
				g.Expect(deployments.Items[i].GetLabels()).To(HaveKey(argoCDInstanceLabel))
				g.Expect(deployments.Items[i].GetLabels()[argoCDInstanceLabel]).NotTo(Equal(""))
				found[deployments.Items[i].GetLabels()[argoCDInstanceLabel]] = true
			}
			daemonsets := appsv1.DaemonSetList{}
			g.Expect(dpuClusterClient.List(ctx, &daemonsets, client.HasLabels{argoCDInstanceLabel})).To(Succeed())
			for i := range daemonsets.Items {
				g.Expect(daemonsets.Items[i].GetLabels()).To(HaveKey(argoCDInstanceLabel))
				g.Expect(daemonsets.Items[i].GetLabels()[argoCDInstanceLabel]).NotTo(Equal(""))
				found[daemonsets.Items[i].GetLabels()[argoCDInstanceLabel]] = true
			}

			// Expect each of the following to have been created by the operator.
			// These are labels on the appv1 type - e.g. DaemonSet or Deployment on the DPU cluster.
			g.Expect(found).To(HaveLen(7))
			g.Expect(found).To(HaveKey(ContainSubstring(operatorv1.MultusName)))
			g.Expect(found).To(HaveKey(ContainSubstring(operatorv1.FlannelName)))
			g.Expect(found).To(HaveKey(ContainSubstring(operatorv1.SRIOVDevicePluginName)))
			// Note: The NVIPAM DPUService contains both a Daemonset and a Deployment - but this is overwritten in the map.
			g.Expect(found).To(HaveKey(ContainSubstring(operatorv1.NVIPAMName)))
			g.Expect(found).To(HaveKey(ContainSubstring(operatorv1.OVSCNIName)))
			g.Expect(found).To(HaveKey(ContainSubstring(operatorv1.SFCControllerName)))
			g.Expect(found).To(HaveKey(ContainSubstring(operatorv1.OVSHelperName)))
		}).WithTimeout(600 * time.Second).Should(Succeed())
	})
}

// VerifyDPFOperatorConfiguration verifies that DPFOperatorConfiguration options work.
// It changes the images for all system components to arbitrary values, checks that the changes have propagated
// and then changes them back to their default versions.
// This function tests DPUService image setting as it is complex and requires e2e testing.
func VerifyDPFOperatorConfiguration(ctx context.Context, input systemTestInput) {
	It("verify ImageConfiguration from DPUServices", func() {
		modifiedConfig := &operatorv1.DPFOperatorConfig{}
		Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpfOperatorSystemNamespace, Name: configName}, modifiedConfig)).To(Succeed())
		originalConfig := modifiedConfig.DeepCopy()

		dummyRegistryName := "dummy-registry.com"
		imageTemplate := "%s/%s:v1.0"
		// Update the config with a new image and tag.

		// For objects which are deployed as DPUServices set the helm chart field in configuration.
		// Excluding flannel which DPF Operator does not allow setting an image for.
		modifiedConfig.Spec.ServiceSetController = &operatorv1.ServiceSetControllerConfiguration{
			Image: ptr.To(fmt.Sprintf(imageTemplate, dummyRegistryName, operatorv1.ServiceSetControllerName)),
		}
		modifiedConfig.Spec.Multus = &operatorv1.MultusConfiguration{
			Image: ptr.To(fmt.Sprintf(imageTemplate, dummyRegistryName, operatorv1.MultusName)),
		}
		modifiedConfig.Spec.SRIOVDevicePlugin = &operatorv1.SRIOVDevicePluginConfiguration{
			Image: ptr.To(fmt.Sprintf(imageTemplate, dummyRegistryName, operatorv1.SRIOVDevicePluginName)),
		}
		modifiedConfig.Spec.OVSCNI = &operatorv1.OVSCNIConfiguration{
			Image: ptr.To(fmt.Sprintf(imageTemplate, dummyRegistryName, operatorv1.OVSCNIName)),
		}
		modifiedConfig.Spec.NVIPAM = &operatorv1.NVIPAMConfiguration{
			Image: ptr.To(fmt.Sprintf(imageTemplate, dummyRegistryName, operatorv1.NVIPAMName)),
		}
		modifiedConfig.Spec.SFCController = &operatorv1.SFCControllerConfiguration{
			Image: ptr.To(fmt.Sprintf(imageTemplate, dummyRegistryName, operatorv1.SFCControllerName)),
		}
		modifiedConfig.Spec.OVSHelper = &operatorv1.OVSHelperConfiguration{
			Image: ptr.To(fmt.Sprintf(imageTemplate, dummyRegistryName, operatorv1.OVSHelperName)),
		}
		Expect(testClient.Patch(ctx, modifiedConfig, client.MergeFrom(originalConfig))).To(Succeed())

		// Assert the images are set for the system components.
		Eventually(func(g Gomega) {
			inClusterDeploymentDPUServices := map[string]bool{
				operatorv1.ServiceSetControllerName: true,
			}
			deploymentDPUservices := map[string]bool{
				operatorv1.NVIPAMName: true,
			}
			daemonSetDPUServices := map[string]bool{
				operatorv1.SRIOVDevicePluginName: true,
				operatorv1.SFCControllerName:     true,
				operatorv1.OVSCNIName:            true,
				operatorv1.NVIPAMName:            true,
				operatorv1.MultusName:            true,
				operatorv1.OVSHelperName:         true,
			}

			// Verify images for inCluster DPUServices
			for name := range inClusterDeploymentDPUServices {
				deployments := appsv1.DeploymentList{}
				nameForCluster := fmt.Sprintf("%s-%s", "in-cluster", name)
				g.Expect(testClient.List(ctx, &deployments,
					client.MatchingLabels{argoCDInstanceLabel: nameForCluster})).To(Succeed())
				g.Expect(deployments.Items).To(HaveLen(1))
				deployment := deployments.Items[0]
				g.Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
				g.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(ContainSubstring(dummyRegistryName))
			}
			// Verify images in the DPUClusters
			for name := range deploymentDPUservices {
				deployments := appsv1.DeploymentList{}
				nameForCluster := fmt.Sprintf("%s-%s", input.dpuCluster.Name, name)
				g.Expect(dpuClusterClient.List(ctx, &deployments,
					client.MatchingLabels{argoCDInstanceLabel: nameForCluster})).To(Succeed())
				g.Expect(deployments.Items).To(HaveLen(1))
				deployment := deployments.Items[0]
				g.Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
				g.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(ContainSubstring(dummyRegistryName))
			}
			for name := range daemonSetDPUServices {
				daemonSets := appsv1.DaemonSetList{}
				nameForCluster := fmt.Sprintf("%s-%s", input.dpuCluster.Name, name)
				g.Expect(dpuClusterClient.List(ctx, &daemonSets,
					client.MatchingLabels{argoCDInstanceLabel: nameForCluster})).To(Succeed())
				g.Expect(daemonSets.Items).To(HaveLen(1))
				daemonSet := daemonSets.Items[0]
				g.Expect(daemonSet.Spec.Template.Spec.Containers[0].Image).To(ContainSubstring(dummyRegistryName))
			}

		}).WithTimeout(120 * time.Second).Should(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(modifiedConfig), modifiedConfig)).To(Succeed())
			resetConfig := modifiedConfig.DeepCopy()
			resetConfig.Spec = originalConfig.Spec
			// Revert the image versions to their previous values.
			g.Expect(testClient.Patch(ctx, resetConfig, client.MergeFrom(modifiedConfig))).To(Succeed())
			// Ensure the changes are reverted before continuing.
		}).Should(Succeed())
	})

	It("verify that the current MTU in the DPU clusters flannel configmap is 1500", func() {
		By("verify flannel configmap for cluster " + input.dpuCluster.Name)
		flannelConfigMap := &corev1.ConfigMap{}
		Expect(dpuClusterClient.Get(ctx, client.ObjectKey{Namespace: dpfOperatorSystemNamespace, Name: "kube-flannel-cfg"}, flannelConfigMap)).To(Succeed())
		Expect(flannelConfigMap.Data["net-conf.json"]).To(ContainSubstring("MTU\": 1500,"))
	})

	It("change the MTUs in the operatorConfig and verify that DPU Clusters are updated", func() {
		By("get the operatorConfig")
		config := &operatorv1.DPFOperatorConfig{}
		Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpfOperatorSystemNamespace, Name: configName}, config)).To(Succeed())
		By("update the MTU in the operatorConfig")
		originalConfig := config.DeepCopy()
		if config.Spec.Networking == nil {
			config.Spec.Networking = &operatorv1.Networking{}
		}
		config.Spec.Networking.ControlPlaneMTU = ptr.To(1200)
		config.Spec.Networking.HighSpeedMTU = ptr.To(9000)
		Eventually(func(g Gomega) {
			g.Expect(testClient.Patch(ctx, config, client.MergeFrom(originalConfig))).To(Succeed())
		}).Should(Succeed())

		By("verify flannel and multus for cluster " + input.dpuCluster.Name)
		Eventually(func(g Gomega) {
			flannelConfigMap := &corev1.ConfigMap{}
			g.Expect(dpuClusterClient.Get(ctx, client.ObjectKey{Namespace: dpfOperatorSystemNamespace, Name: "kube-flannel-cfg"}, flannelConfigMap)).To(Succeed())
			g.Expect(flannelConfigMap.Data["net-conf.json"]).To(ContainSubstring("MTU\": 1200,"))

			netAttachDef := &unstructured.Unstructured{}
			netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "k8s.cni.cncf.io",
				Version: "v1",
				Kind:    "NetworkAttachmentDefinition",
			})

			g.Expect(dpuClusterClient.Get(ctx, client.ObjectKey{Namespace: dpfOperatorSystemNamespace, Name: "mybrsfc"}, netAttachDef)).To(Succeed())
			config, exists, err := unstructured.NestedString(netAttachDef.Object, "spec", "config")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(exists).To(BeTrue())
			g.Expect(config).To(ContainSubstring("mtu\": 9000,"))

			g.Expect(dpuClusterClient.Get(ctx, client.ObjectKey{Namespace: dpfOperatorSystemNamespace, Name: "mybrhbn"}, netAttachDef)).To(Succeed())
			config, exists, err = unstructured.NestedString(netAttachDef.Object, "spec", "config")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(exists).To(BeTrue())
			g.Expect(config).To(ContainSubstring("mtu\": 9000,"))
		}, time.Second*30).Should(Succeed())

	})
}

func ValidateDPUService(ctx context.Context, input systemTestInput) {
	dpuServiceName := "dpu-01"
	hostDPUServiceName := "host-dpu-service"
	dpuServiceNamespace := "dpu-test-ns"
	testNSImagePullSecret := &corev1.Secret{}
	It("create a DPUService and check Objects and ImagePullSecrets are mirrored correctly", func() {
		By("create namespace for DPUService")
		testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceNamespace}}
		testNS.SetLabels(cleanupLabels)
		Expect(client.IgnoreAlreadyExists(testClient.Create(ctx, testNS))).To(Succeed())

		By("create ImagePullSecret for DPUService in user namespace")
		testNSImagePullSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      input.pullSecretNames[0],
				Namespace: dpuServiceNamespace,
				Labels: map[string]string{
					dpuservicev1.DPFImagePullSecretLabelKey: "",
				},
			},
		}
		Expect(client.IgnoreAlreadyExists(testClient.Create(ctx, testNSImagePullSecret))).ToNot(HaveOccurred())

		By("create a DPUServiceInterface")
		dsi := input.dpuServiceInterface.DeepCopy()
		dsi.SetName("net1-service")
		dsi.SetNamespace(dpuServiceNamespace)
		dsi.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dsi)).To(Succeed())

		By("create a DPUService to be deployed on the DPUCluster")
		// Create DPUCluster DPUService and check it's correctly reconciled.
		dpuService := input.dpuService.DeepCopy()
		dpuService.SetName(dpuServiceName)
		dpuService.SetNamespace(dpuServiceNamespace)
		dpuService.Spec.Interfaces = []string{"net1-service"}
		dpuService.Spec.ServiceID = ptr.To("my-service")
		dpuService.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dpuService)).To(Succeed())

		By("create a DPUService to be deployed on the host cluster")
		// Create a host DPUService and check it's correctly reconciled
		// Read the DPUService from file and create it.
		hostDPUService := input.dpuService.DeepCopy()
		hostDPUService.SetName(hostDPUServiceName)
		hostDPUService.SetNamespace(dpuServiceNamespace)
		hostDPUService.Spec.DeployInCluster = ptr.To(true)
		hostDPUService.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, hostDPUService)).To(Succeed())

		By("verify DPUServices and deployments are created")
		Eventually(func(g Gomega) {
			// Check the deployment from the DPUService can be found on the destination cluster.
			deploymentList := appsv1.DeploymentList{}
			g.Expect(dpuClusterClient.List(ctx, &deploymentList, client.HasLabels{"app", "release"})).To(Succeed())
			g.Expect(deploymentList.Items).To(HaveLen(1))
			g.Expect(deploymentList.Items[0].Name).To(ContainSubstring("helm-guestbook"))

			// Check an imagePullSecret was created in the same namespace in the destination cluster.
			g.Expect(dpuClusterClient.Get(ctx, client.ObjectKey{
				Namespace: dpuService.GetNamespace(),
				Name:      input.pullSecretNames[0]}, &corev1.Secret{})).To(Succeed())
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

	It("verify DPUService and DPUServiceInterface metrics", func() {
		if !deployKSM {
			Skip("Skip KSM metrics test due to KSM is not deployed")
		}

		By("verify DPUService metrics in KSM")
		expectedMetricsNames := map[string][]string{
			"dpuservice": {"created", "info", "status_conditions", "status_condition_last_transition_time"},
		}
		Eventually(func(g Gomega) {
			actualMetricsNames := metrics.GetKSMMetrics(ctx, testRESTClient, metricsURI)
			g.Expect(actualMetricsNames).NotTo(BeEmpty(), "Actual metrics are empty")
			g.Expect(metrics.VerifyMetrics(expectedMetricsNames, actualMetricsNames)).To(BeEmpty())
		}).WithTimeout(5 * time.Second).Should(Succeed())
	})

	It("delete the DPUServices and check that the applications are cleaned up", func() {
		if skipCleanup {
			Skip("Skip cleanup resources")
		}

		By("pause dpuservice reconciliation")
		svc := &dpuservicev1.DPUService{}
		Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceNamespace, Name: hostDPUServiceName}, svc)).To(Succeed())
		origSvc := svc.DeepCopy()
		svc.Spec.Paused = ptr.To(true)
		Eventually(testClient.Patch).WithArguments(ctx, svc, client.MergeFrom(origSvc)).Should(Succeed())

		By("delete the DPUServices")
		svc = &dpuservicev1.DPUService{}
		// Delete the DPUCluster DPUService.
		Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceNamespace, Name: dpuServiceName}, svc)).To(Succeed())
		Expect(testClient.Delete(ctx, svc)).To(Succeed())

		// Delete the host cluster DPUService.
		svc = &dpuservicev1.DPUService{}
		Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceNamespace, Name: hostDPUServiceName}, svc)).To(Succeed())
		Expect(testClient.Delete(ctx, svc)).To(Succeed())

		// Verify that the DPUServices are deleted
		By("verify DPUServices is deleted in the DPU cluster")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceNamespace, Name: dpuServiceName}, svc)).ToNot(Succeed())
		}).WithTimeout(600 * time.Second).Should(Succeed())

		By("verify DPUService is not deleted in the host cluster")
		Eventually(func(g Gomega) {
			svc = &dpuservicev1.DPUService{}
			g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceNamespace, Name: hostDPUServiceName}, svc)).To(Succeed())
		}).WithTimeout(600 * time.Second).Should(Succeed())

		By("resume dpuservice reconciliation")
		origSvc = svc.DeepCopy()
		svc.Spec.Paused = ptr.To(false)
		Eventually(testClient.Patch).WithArguments(ctx, svc, client.MergeFrom(origSvc)).Should(Succeed())

		// Verify that the DPUServices are deleted
		By("verify DPUServices is deleted in the host cluster")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceNamespace, Name: hostDPUServiceName}, svc)).ToNot(Succeed())
		}).WithTimeout(600 * time.Second).Should(Succeed())

		dsi := &dpuservicev1.DPUServiceInterface{}
		Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceNamespace, Name: "net1-service"}, dsi)).To(Succeed())
		Expect(utils.CleanupAndWait(ctx, testClient, dsi)).To(Succeed())

		// Check the DPUCluster DPUService is correctly deleted.
		Eventually(func(g Gomega) {
			deploymentList := appsv1.DeploymentList{}
			g.Expect(dpuClusterClient.List(ctx, &deploymentList, client.HasLabels{"app", "release"})).To(Succeed())
			g.Expect(deploymentList.Items).To(BeEmpty())
		}).WithTimeout(300 * time.Second).Should(Succeed())

		// Ensure the hostDPUService deployment is deleted from the host cluster.
		Eventually(func(g Gomega) {
			deploymentList := appsv1.DeploymentList{}
			g.Expect(testClient.List(ctx, &deploymentList, client.HasLabels{"app", "release"})).To(Succeed())
			g.Expect(deploymentList.Items).To(BeEmpty())
		}).WithTimeout(300 * time.Second).Should(Succeed())
	})

	It("verify that the ImagePullSecrets have been synced correctly and cleaned up", func() {
		// Verify that we have 2 secrets in the DPU Cluster.
		verifyImagePullSecretsCount(dpfOperatorSystemNamespace, 2)

		desiredConf := &operatorv1.DPFOperatorConfig{}
		Eventually(testClient.Get).WithArguments(ctx, client.ObjectKey{Namespace: dpfOperatorSystemNamespace, Name: configName}, desiredConf).Should(Succeed())
		currentConf := desiredConf.DeepCopy()

		// Patch the operatorConfig to remove the second secret. This causes the label to be removed.
		desiredConf.Spec.ImagePullSecrets = append(desiredConf.Spec.ImagePullSecrets[:1], desiredConf.Spec.ImagePullSecrets[2:]...)

		Eventually(testClient.Patch).WithArguments(ctx, desiredConf, client.MergeFrom(currentConf)).Should(Succeed())

		// Patch a DPUService to trigger a reconciliation. The DPUService should clean  this secret up from
		// clusters to which it was previously mirrored.
		Eventually(utils.ForceObjectReconcileWithAnnotation).WithArguments(ctx, testClient,
			&dpuservicev1.DPUService{ObjectMeta: metav1.ObjectMeta{Name: operatorv1.MultusName, Namespace: dpfOperatorSystemNamespace}}).Should(Succeed())
		// Verify we only have one image pull secret.
		verifyImagePullSecretsCount(dpfOperatorSystemNamespace, 1)
	})
}

func verifyImagePullSecretsCount(namespace string, count int) {
	secrets := &corev1.SecretList{}
	Expect(testClient.List(ctx, secrets,
		client.InNamespace(namespace),
		client.HasLabels{dpuservicev1.DPFImagePullSecretLabelKey}),
	).ToNot(HaveOccurred())
	Eventually(func(g Gomega) {
		// Check the imagePullSecrets has been deleted.
		secrets := &corev1.SecretList{}
		g.Expect(dpuClusterClient.List(ctx, secrets,
			client.InNamespace(namespace),
			client.HasLabels{dpuservicev1.DPFImagePullSecretLabelKey}),
		).To(Succeed())
		g.Expect(secrets.Items).To(HaveLen(count))
	}).WithTimeout(60 * time.Second).Should(Succeed())
}

func ValidateDPUDeployment(ctx context.Context, input systemTestInput) {
	It("create a DPUDeployment with its dependencies and ensure that the underlying objects are created", func() {
		By("creating the dependencies")
		dpuServiceTemplate := input.dpuServiceTemplate.DeepCopy()
		dpuServiceTemplate.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())

		dpuServiceConfiguration := input.dpuServiceConfiguration.DeepCopy()
		dpuServiceConfiguration.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dpuServiceConfiguration)).To(Succeed())

		By("creating the dpudeployment")
		dpuDeployment := input.dpuDeployment.DeepCopy()
		dpuDeployment.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())

		By("checking that the underlying objects are created")
		Eventually(func(g Gomega) {
			gotDPUSetList := &provisioningv1.DPUSetList{}
			g.Expect(testClient.List(ctx,
				gotDPUSetList,
				client.InNamespace(dpuDeployment.GetNamespace()),
				client.MatchingLabels{
					"svc.dpu.nvidia.com/owned-by-dpudeployment": fmt.Sprintf("%s_%s", dpuDeployment.GetNamespace(), dpuDeployment.GetName()),
				})).To(Succeed())
			g.Expect(gotDPUSetList.Items).To(HaveLen(1))

			gotDPUServiceList := &dpuservicev1.DPUServiceList{}
			g.Expect(testClient.List(ctx,
				gotDPUServiceList,
				client.InNamespace(dpuDeployment.GetNamespace()),
				client.MatchingLabels{
					"svc.dpu.nvidia.com/owned-by-dpudeployment": fmt.Sprintf("%s_%s", dpuDeployment.GetNamespace(), dpuDeployment.GetName()),
				})).To(Succeed())
			g.Expect(gotDPUServiceList.Items).To(HaveLen(1))

			gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
			g.Expect(testClient.List(ctx,
				gotDPUServiceChainList,
				client.InNamespace(dpuDeployment.GetNamespace()),
				client.MatchingLabels{
					"svc.dpu.nvidia.com/owned-by-dpudeployment": fmt.Sprintf("%s_%s", dpuDeployment.GetNamespace(), dpuDeployment.GetName()),
				})).To(Succeed())
			g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))

			gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
			g.Expect(testClient.List(ctx,
				gotDPUServiceInterfaceList,
				client.InNamespace(dpuDeployment.GetNamespace()),
				client.MatchingLabels{
					"svc.dpu.nvidia.com/owned-by-dpudeployment": fmt.Sprintf("%s_%s", dpuDeployment.GetNamespace(), dpuDeployment.GetName()),
				})).To(Succeed())
			g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(1))
		}).WithTimeout(180 * time.Second).Should(Succeed())
	})

	It("verify DPUDeployment and DPUServiceInterface metrics", func() {
		if !deployKSM {
			Skip("Skip KSM metrics test due to KSM is not deployed")
		}

		By("verify DPUDeployment and DPUServiceInterface metrics are in KSM")
		expectedMetricsNames := map[string][]string{
			"dpudeployment": {"created", "info", "status_conditions", "status_condition_last_transition_time"},
			//TODO implement separate test for a DPUServiceInterface and move this metrics check there
			"dpuserviceinterface": {"created", "info", "status_conditions", "status_condition_last_transition_time"},
		}
		Eventually(func(g Gomega) {
			actualMetricsNames := metrics.GetKSMMetrics(ctx, testRESTClient, metricsURI)
			g.Expect(actualMetricsNames).NotTo(BeEmpty(), "Actual metrics are empty")
			g.Expect(metrics.VerifyMetrics(expectedMetricsNames, actualMetricsNames)).To(BeEmpty())
		}).WithTimeout(5 * time.Second).Should(Succeed())

	})

	It("delete the DPUDeployment and ensure the underlying objects are gone", func() {
		if skipCleanup {
			Skip("Skip cleanup resources")
		}

		By("deleting the dpudeployment")
		dpuDeployment := input.dpuDeployment.DeepCopy()
		Expect(testClient.Delete(ctx, dpuDeployment)).To(Succeed())

		By("checking that the underlying objects are deleted")
		Eventually(func(g Gomega) {
			gotDPUSetList := &provisioningv1.DPUSetList{}
			g.Expect(testClient.List(ctx,
				gotDPUSetList,
				client.InNamespace(dpuDeployment.GetNamespace()),
				client.MatchingLabels{
					"svc.dpu.nvidia.com/owned-by-dpudeployment": fmt.Sprintf("%s_%s", dpuDeployment.GetNamespace(), dpuDeployment.GetName()),
				})).To(Succeed())
			g.Expect(gotDPUSetList.Items).To(BeEmpty())

			gotDPUServiceList := &dpuservicev1.DPUServiceList{}
			g.Expect(testClient.List(ctx,
				gotDPUServiceList,
				client.InNamespace(dpuDeployment.GetNamespace()),
				client.MatchingLabels{
					"svc.dpu.nvidia.com/owned-by-dpudeployment": fmt.Sprintf("%s_%s", dpuDeployment.GetNamespace(), dpuDeployment.GetName()),
				})).To(Succeed())
			g.Expect(gotDPUServiceList.Items).To(BeEmpty())

			gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
			g.Expect(testClient.List(ctx,
				gotDPUServiceChainList,
				client.InNamespace(dpuDeployment.GetNamespace()),
				client.MatchingLabels{
					"svc.dpu.nvidia.com/owned-by-dpudeployment": fmt.Sprintf("%s_%s", dpuDeployment.GetNamespace(), dpuDeployment.GetName()),
				})).To(Succeed())
			g.Expect(gotDPUServiceChainList.Items).To(BeEmpty())

			gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
			g.Expect(testClient.List(ctx,
				gotDPUServiceInterfaceList,
				client.InNamespace(dpuDeployment.GetNamespace()),
				client.MatchingLabels{
					"svc.dpu.nvidia.com/owned-by-dpudeployment": fmt.Sprintf("%s_%s", dpuDeployment.GetNamespace(), dpuDeployment.GetName()),
				})).To(Succeed())
			g.Expect(gotDPUServiceInterfaceList.Items).To(BeEmpty())

			// Expect the DPUDeployment to be deleted
			err := testClient.Get(ctx, client.ObjectKey{Namespace: dpuDeployment.GetNamespace(), Name: dpuDeployment.GetName()}, &dpuservicev1.DPUDeployment{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}).WithTimeout(180 * time.Second).Should(Succeed())
	})
}

func ValidateDPUServiceIPAM(ctx context.Context, input systemTestInput) {
	dpuServiceIPAMWithIPPoolName := "switched-application"
	dpuServiceIPAMWithCIDRPoolName := "routed-application"
	dpuServiceIPAMNamespace := dpfOperatorSystemNamespace

	It("create an invalid DPUServiceIPAM and ensure that the webhook rejects the request", func() {
		By("creating the invalid DPUServiceIPAM CR")
		dpuServiceIPAM := &dpuservicev1.DPUServiceIPAM{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-name",
				Namespace: dpuServiceIPAMNamespace,
			},
		}
		dpuServiceIPAM.SetGroupVersionKind(dpuservicev1.DPUServiceIPAMGroupVersionKind)
		dpuServiceIPAM.SetLabels(cleanupLabels)
		err := testClient.Create(ctx, dpuServiceIPAM)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsBadRequest(err)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("either ipv4Subnet or ipv4Network must be specified"))
	})

	It("create a DPUServiceIPAM with subnet split per node configuration and check NVIPAM IPPool is created to each cluster", func() {
		By("creating the DPUServiceIPAM CR")
		dpuServiceIPAM := input.ipPoolDPUServiceIPAM.DeepCopy()
		dpuServiceIPAM.SetName(dpuServiceIPAMWithIPPoolName)
		dpuServiceIPAM.SetNamespace(dpuServiceIPAMNamespace)
		dpuServiceIPAM.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dpuServiceIPAM)).To(Succeed())

		By("checking that NVIPAM IPPool CR is created in the DPU clusters")
		Eventually(func(g Gomega) {
			ipPools := &nvipamv1.IPPoolList{}
			g.Expect(dpuClusterClient.List(ctx, ipPools, client.MatchingLabels{
				"dpu.nvidia.com/dpuserviceipam-name":      dpuServiceIPAM.GetName(),
				"dpu.nvidia.com/dpuserviceipam-namespace": dpuServiceIPAM.GetNamespace(),
			})).To(Succeed())
			g.Expect(ipPools.Items).To(HaveLen(1))

			// TODO: Check that NVIPAM has reconciled the resources and status reflects that.
		}).WithTimeout(180 * time.Second).Should(Succeed())
	})

	It("verify DPUServiceIPAM metrics", func() {
		if !deployKSM {
			Skip("Skip KSM metrics test due to KSM is not deployed")
		}

		By("verify DPUServiceIPAM metrics in KSM")
		expectedMetricsNames := map[string][]string{
			"dpuserviceipam": {"created", "info", "status_conditions", "status_condition_last_transition_time"}, //  "network_info", "subnet_info" missed
		}
		Eventually(func(g Gomega) {
			actualMetricsNames := metrics.GetKSMMetrics(ctx, testRESTClient, metricsURI)
			g.Expect(actualMetricsNames).NotTo(BeEmpty(), "Actual metrics are empty")
			g.Expect(metrics.VerifyMetrics(expectedMetricsNames, actualMetricsNames)).To(BeEmpty())
		}).WithTimeout(5 * time.Second).Should(Succeed())
	})

	It("delete the DPUServiceIPAM with subnet split per node configuration and check NVIPAM IPPool is deleted in each cluster", func() {
		if skipCleanup {
			Skip("Skip cleanup resources")
		}
		By("deleting the DPUServiceIPAM")
		dpuServiceIPAM := &dpuservicev1.DPUServiceIPAM{}
		Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceIPAMNamespace, Name: dpuServiceIPAMWithIPPoolName}, dpuServiceIPAM)).To(Succeed())
		Expect(testClient.Delete(ctx, dpuServiceIPAM)).To(Succeed())

		By("checking that NVIPAM IPPool CR is deleted in each DPU cluster")
		Eventually(func(g Gomega) {
			ipPools := &nvipamv1.IPPoolList{}
			g.Expect(dpuClusterClient.List(ctx, ipPools, client.MatchingLabels{
				"dpu.nvidia.com/dpuserviceipam-name":      dpuServiceIPAM.GetName(),
				"dpu.nvidia.com/dpuserviceipam-namespace": dpuServiceIPAM.GetNamespace(),
			})).To(Succeed())
			g.Expect(ipPools.Items).To(BeEmpty())
		}).WithTimeout(180 * time.Second).Should(Succeed())
	})

	It("create a DPUServiceIPAM with cidr split in subnet per node configuration and check NVIPAM CIDRPool is created to each cluster", func() {
		By("creating the DPUServiceIPAM CR")
		dpuServiceIPAM := input.cidrDPUServiceIPAM.DeepCopy()
		dpuServiceIPAM.SetName(dpuServiceIPAMWithCIDRPoolName)
		dpuServiceIPAM.SetNamespace(dpuServiceIPAMNamespace)
		dpuServiceIPAM.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dpuServiceIPAM)).To(Succeed())

		By("checking that NVIPAM CIDRPool CR is created in the DPU clusters")
		Eventually(func(g Gomega) {
			cidrPools := &nvipamv1.CIDRPoolList{}
			g.Expect(dpuClusterClient.List(ctx, cidrPools, client.MatchingLabels{
				"dpu.nvidia.com/dpuserviceipam-name":      dpuServiceIPAM.GetName(),
				"dpu.nvidia.com/dpuserviceipam-namespace": dpuServiceIPAM.GetNamespace(),
			})).To(Succeed())
			g.Expect(cidrPools.Items).To(HaveLen(1))

			// TODO: Check that NVIPAM has reconciled the resources and status reflects that.
		}).WithTimeout(180 * time.Second).Should(Succeed())
	})

	It("delete the DPUServiceIPAM with cidr split in subnet per node configuration and check NVIPAM CIDRPool is deleted in each cluster", func() {
		if skipCleanup {
			Skip("Skip cleanup resources")
		}
		By("deleting the DPUServiceIPAM")
		dpuServiceIPAM := &dpuservicev1.DPUServiceIPAM{}
		Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceIPAMNamespace, Name: dpuServiceIPAMWithCIDRPoolName}, dpuServiceIPAM)).To(Succeed())
		Expect(testClient.Delete(ctx, dpuServiceIPAM)).To(Succeed())

		By("checking that NVIPAM CIDRPool CR is deleted in each DPU cluster")
		Eventually(func(g Gomega) {
			cidrPools := &nvipamv1.CIDRPoolList{}
			g.Expect(dpuClusterClient.List(ctx, cidrPools, client.MatchingLabels{
				"dpu.nvidia.com/dpuserviceipam-name":      dpuServiceIPAM.GetName(),
				"dpu.nvidia.com/dpuserviceipam-namespace": dpuServiceIPAM.GetNamespace(),
			})).To(Succeed())
			g.Expect(cidrPools.Items).To(BeEmpty())
		}).WithTimeout(180 * time.Second).Should(Succeed())
	})

}

func ValidateDPUServiceCredentialRequest(ctx context.Context, input systemTestInput) {
	hostDPUServiceCredentialRequestName := "host-dpu-credential-request"
	dpuServiceCredentialRequestName := "dpu-01-credential-request"
	dpuServiceCredentialRequestNamespace := "dpucr-test-ns"

	It("create a DPUServiceCredentialRequest and check that the credentials are created", func() {
		By("create namespace for DPUServiceCredentialRequest")
		testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceCredentialRequestNamespace}}
		testNS.SetLabels(cleanupLabels)
		Expect(client.IgnoreAlreadyExists(testClient.Create(ctx, testNS))).To(Succeed())

		By("create a DPUServiceCredentialRequest targeting the DPUCluster")
		dcr := input.dpuServiceCredentialRequest.DeepCopy()
		dcr.SetName(dpuServiceCredentialRequestName)
		dcr.SetNamespace(dpuServiceCredentialRequestNamespace)
		dcr.Spec.TargetCluster = &dpuservicev1.NamespacedName{Name: input.dpuCluster.Name, Namespace: ptr.To(dpfOperatorSystemNamespace)}
		dcr.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dcr)).To(Succeed())

		By("create a DPUServiceCredentialRequest targeting the host cluster")
		hostDcr := input.dpuServiceCredentialRequest.DeepCopy()
		hostDcr.SetName(hostDPUServiceCredentialRequestName)
		hostDcr.SetNamespace(dpuServiceCredentialRequestNamespace)
		hostDcr.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, hostDcr)).To(Succeed())

		By("verify reconciled DPUServiceCredentialRequest for DPUCluster")
		Eventually(func(g Gomega) {
			assertDPUServiceCredentialRequest(g, testClient, dcr, false)
		}).WithTimeout(300 * time.Second).Should(Succeed())

		By("verify reconciled DPUServiceCredentialRequest for host cluster")
		Eventually(func(g Gomega) {
			assertDPUServiceCredentialRequest(g, testClient, hostDcr, true)
		}).WithTimeout(600 * time.Second).Should(Succeed())

	})
	It("verify DPUServiceCredentialRequest metrics", func() {
		if !deployKSM {
			Skip("Skip KSM metrics accessibility test due to KSM is not deployed")
		}

		By("verify DPUServiceCredentialRequest metrics in KSM")
		expectedMetricsNames := map[string][]string{
			"dpuservicecredentialrequest": {"created", "info", "expiration", "issued_at", "status_conditions", "status_condition_last_transition_time"},
		}
		Eventually(func(g Gomega) {
			actualMetricsNames := metrics.GetKSMMetrics(ctx, testRESTClient, metricsURI)
			g.Expect(actualMetricsNames).NotTo(BeEmpty(), "Actual metrics are empty")
			g.Expect(metrics.VerifyMetrics(expectedMetricsNames, actualMetricsNames)).To(BeEmpty())
		}).WithTimeout(5 * time.Second).Should(Succeed())

	})

	It("delete the DPUServiceCredentialRequest and check that the credentials are deleted", func() {
		if skipCleanup {
			Skip("Skip cleanup resources")
		}
		By("delete the DPUServiceCredentialRequest")
		dcr := &dpuservicev1.DPUServiceCredentialRequest{}
		key := client.ObjectKey{Namespace: dpuServiceCredentialRequestNamespace, Name: dpuServiceCredentialRequestName}
		Expect(testClient.Get(ctx, key, dcr)).To(Succeed())
		Expect(testClient.Delete(ctx, dcr)).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, key, dcr)).NotTo(Succeed())
		}).WithTimeout(300 * time.Second).Should(Succeed())

		key = client.ObjectKey{Namespace: dpuServiceCredentialRequestNamespace, Name: hostDPUServiceCredentialRequestName}
		Expect(testClient.Get(ctx, key, dcr)).To(Succeed())
		Expect(testClient.Delete(ctx, dcr)).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, key, dcr)).NotTo(Succeed())
		}).WithTimeout(300 * time.Second).Should(Succeed())
	})

}

func assertDPUServiceCredentialRequest(g Gomega, testClient client.Client, dcr *dpuservicev1.DPUServiceCredentialRequest, host bool) {
	gotDsr := &dpuservicev1.DPUServiceCredentialRequest{}
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(dcr), gotDsr)).To(Succeed())
	g.Expect(gotDsr.Finalizers).To(ConsistOf([]string{dpuservicev1.DPUServiceCredentialRequestFinalizer}))
	g.Expect(gotDsr.Status.ServiceAccount).NotTo(BeNil())
	g.Expect(*gotDsr.Status.ServiceAccount).To(Equal(dcr.Spec.ServiceAccount.String()))
	g.Expect(gotDsr.Status.ExpirationTimestamp.Time).To(BeTemporally("~", time.Now().Add(time.Hour), time.Minute))
	g.Expect(gotDsr.Status.IssuedAt).NotTo(BeNil())
	if gotDsr.Spec.Duration != nil {
		iat := gotDsr.Status.ExpirationTimestamp.Time.Add(-1 * gotDsr.Spec.Duration.Duration)
		g.Expect(gotDsr.Status.IssuedAt.Time).To(BeTemporally("~", iat, time.Minute))
	}

	if host {
		g.Expect(gotDsr.Status.TargetCluster).To(BeNil())
	} else {
		g.Expect(gotDsr.Status.TargetCluster).To(Equal(ptr.To(dcr.Spec.TargetCluster.String())))
	}
}

func ValidateDPUServiceChain(ctx context.Context, input systemTestInput) {
	dpuServiceInterfaceName := "pf0-vf2"
	dpuServiceInterfaceNamespace := "test"
	dpuServiceChainName := "svc-chain-test"
	dpuServiceChainNamespace := "test-2"

	It("create DPUServiceInterface and check that it is mirrored to each cluster", func() {
		By("create test namespace")
		testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceInterfaceNamespace}}
		testNS.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, testNS)).To(Succeed())
		By("create DPUServiceInterface")
		dpuServiceInterface := input.dpuServiceInterface.DeepCopy()
		dpuServiceInterface.SetName(dpuServiceInterfaceName)
		dpuServiceInterface.SetNamespace(dpuServiceInterfaceNamespace)
		dpuServiceInterface.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dpuServiceInterface)).To(Succeed())
		By("verify ServiceInterfaceSet is created in DPF clusters")
		Eventually(func(g Gomega) {
			scs := &dpuservicev1.ServiceInterfaceSet{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceInterfaceName, Namespace: dpuServiceInterfaceNamespace}}
			g.Expect(dpuClusterClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
		}, time.Second*300, time.Millisecond*250).Should(Succeed())
	})

	It("create DPUServiceChain and check that it is mirrored to each cluster", func() {
		By("create test namespace")
		testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceChainNamespace}}
		testNS.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, testNS)).To(Succeed())
		By("create DPUServiceChain")
		dpuServiceChain := input.dpuServiceChain.DeepCopy()
		dpuServiceChain.SetName(dpuServiceChainName)
		dpuServiceChain.SetNamespace(dpuServiceChainNamespace)
		dpuServiceChain.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dpuServiceChain)).To(Succeed())
		By("verify ServiceChainSet is created in DPF clusters")
		Eventually(func(g Gomega) {
			scs := &dpuservicev1.ServiceChainSet{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceChainName, Namespace: dpuServiceChainNamespace}}
			g.Expect(dpuClusterClient.Get(ctx, client.ObjectKeyFromObject(scs), scs)).NotTo(HaveOccurred())
		}, time.Second*300, time.Millisecond*250).Should(Succeed())

	})
	It("verify DPUServiceChain metrics", func() {
		if !deployKSM {
			Skip("Skip KSM metrics accessibility test due to KSM is not deployed")
		}

		By("verify DPUServiceChain metrics in KSM")
		expectedMetricsNames := map[string][]string{
			"dpuservicechain": {"created", "info", "status_conditions", "status_condition_last_transition_time"},
		}
		Eventually(func(g Gomega) {
			actualMetricsNames := metrics.GetKSMMetrics(ctx, testRESTClient, metricsURI)
			g.Expect(actualMetricsNames).NotTo(BeEmpty(), "Actual metrics are empty")
			g.Expect(metrics.VerifyMetrics(expectedMetricsNames, actualMetricsNames)).To(BeEmpty())
		}).WithTimeout(5 * time.Second).Should(Succeed())
	})

	It("delete the DPUServiceChain & DPUServiceInterface and check that the Sets are cleaned up", func() {
		if skipCleanup {
			Skip("Skip cleanup resources")
		}
		dsi := &dpuservicev1.DPUServiceInterface{}
		Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceInterfaceNamespace, Name: dpuServiceInterfaceName}, dsi)).To(Succeed())
		Expect(testClient.Delete(ctx, dsi)).To(Succeed())
		dsc := &dpuservicev1.DPUServiceChain{}
		Expect(testClient.Get(ctx, client.ObjectKey{Namespace: dpuServiceChainNamespace, Name: dpuServiceChainName}, dsc)).To(Succeed())
		Expect(testClient.Delete(ctx, dsc)).To(Succeed())
		// Get the control plane secrets.
		Eventually(func(g Gomega) {
			serviceChainSetList := dpuservicev1.ServiceChainSetList{}
			g.Expect(dpuClusterClient.List(ctx, &serviceChainSetList,
				&client.ListOptions{Namespace: dpuServiceChainNamespace})).To(Succeed())
			g.Expect(serviceChainSetList.Items).To(BeEmpty())
			serviceInterfaceSetList := dpuservicev1.ServiceInterfaceSetList{}
			g.Expect(dpuClusterClient.List(ctx, &serviceInterfaceSetList,
				&client.ListOptions{Namespace: dpuServiceInterfaceNamespace})).To(Succeed())
			g.Expect(serviceInterfaceSetList.Items).To(BeEmpty())
		}).WithTimeout(300 * time.Second).Should(Succeed())
	})
}
func ValidateDPUServiceTemplate(ctx context.Context, input systemTestInput) {
	It("create a DPUServiceTemplate with a chart that doesn't include annotations and expect no versions in status", func() {
		By("creating the DPUServiceTemplate")
		dpuServiceTemplate := input.dpuServiceTemplate.DeepCopy()
		dpuServiceTemplate.SetName("dpuservice-without-annotations")
		dpuServiceTemplate.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())

		By("checking that status is ready and no versions")
		Eventually(func(g Gomega) {
			gotDPUServiceTemplate := &dpuservicev1.DPUServiceTemplate{}
			g.Expect(testClient.Get(ctx,
				types.NamespacedName{Name: dpuServiceTemplate.GetName(), Namespace: dpuServiceTemplate.GetNamespace()},
				gotDPUServiceTemplate,
			)).To(Succeed())
			g.Expect(gotDPUServiceTemplate.Status.Conditions).To(ContainElement(
				And(
					HaveField("Type", string(conditions.TypeReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
			))

			g.Expect(gotDPUServiceTemplate.Status.Versions).To(BeEmpty())
		}).WithTimeout(180 * time.Second).Should(Succeed())
	})

	It("create a DPUServiceTemplate with a chart that includes annotations and expect versions in status", func() {
		By("creating the DPUServiceTemplate")
		dpuServiceTemplate := input.dpuServiceTemplate.DeepCopy()
		dpuServiceTemplate.Spec.HelmChart.Source = dpuservicev1.ApplicationSource{
			Chart:   "dummydpuservice-chart",
			Version: tag,
			// The library is able to handle both unauthenticated and authenticated OCI and Helm Registry. As part of this
			// test we don't cover all these permutations, but based on the provided HELM_REGISTRY, it's possible to test
			// all of them.
			RepoURL: helmRegistry,
		}
		dpuServiceTemplate.SetName("dpuservice-with-annotations")
		dpuServiceTemplate.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dpuServiceTemplate)).To(Succeed())

		By("checking that status is ready and versions are set")
		Eventually(func(g Gomega) {
			gotDPUServiceTemplate := &dpuservicev1.DPUServiceTemplate{}
			g.Expect(testClient.Get(ctx,
				types.NamespacedName{Name: dpuServiceTemplate.GetName(), Namespace: dpuServiceTemplate.GetNamespace()},
				gotDPUServiceTemplate,
			)).To(Succeed())

			g.Expect(gotDPUServiceTemplate.Status.Conditions).To(ContainElement(
				And(
					HaveField("Type", string(conditions.TypeReady)),
					HaveField("Status", metav1.ConditionTrue),
					HaveField("Reason", string(conditions.ReasonSuccess)),
				),
			))

			g.Expect(gotDPUServiceTemplate.Status.Versions).To(HaveKeyWithValue("dpu.nvidia.com/doca-version", ">= 2.9"))
		}).WithTimeout(180 * time.Second).Should(Succeed())
	})

	It("verify DPUServiceTemplate metrics", func() {
		if !deployKSM {
			Skip("Skip KSM metrics accessibility test due to KSM is not deployed")
		}

		By("verify DPUServiceTemplate metrics in KSM")
		expectedMetricsNames := map[string][]string{
			"dpuservicetemplate": {"created", "info", "status_conditions", "status_condition_last_transition_time"},
		}
		Eventually(func(g Gomega) {
			actualMetricsNames := metrics.GetKSMMetrics(ctx, testRESTClient, metricsURI)
			g.Expect(actualMetricsNames).NotTo(BeEmpty(), "Actual metrics are empty")
			g.Expect(metrics.VerifyMetrics(expectedMetricsNames, actualMetricsNames)).To(BeEmpty())
		}).WithTimeout(5 * time.Second).Should(Succeed())
	})
}

func ValidateOperatorCleanup(ctx context.Context, input systemTestInput) {
	It("delete DPUs and DPUSets and ensure they are deleted", func() {
		if skipCleanup {
			Skip("Skip cleanup resources")
		}

		Eventually(func(g Gomega) {
			dpuSetList := &provisioningv1.DPUSetList{}
			dpuList := &provisioningv1.DPUList{}
			g.Expect(client.IgnoreNotFound(testClient.DeleteAllOf(ctx, &provisioningv1.DPUSet{}, client.InNamespace(dpfOperatorSystemNamespace)))).To(Succeed())
			g.Expect(testClient.List(ctx, dpuSetList)).To(Succeed())
			g.Expect(dpuSetList.Items).To(BeEmpty())

			// Expect all DPUs to have been deleted.
			g.Expect(testClient.List(ctx, dpuList)).To(Succeed())
			g.Expect(dpuList.Items).To(BeEmpty())

			nodes := &corev1.NodeList{}
			g.Expect(dpuClusterClient.List(ctx, nodes)).To(Succeed())
			By(fmt.Sprintf("Expected number of nodes %d to equal %d", len(nodes.Items), 0))
			g.Expect(nodes.Items).To(BeEmpty())
		}).WithTimeout(10 * time.Minute).Should(Succeed())
	})

	It("create a DPUDeployment with its dependencies and ensure that the underlying objects are created", func() {
		if skipCleanup {
			Skip("Skip cleanup resources")
		}
		By("creating the dpudeployment")
		dpuDeployment := input.dpuDeployment.DeepCopy()
		dpuDeployment.Name = "example-two"
		dpuDeployment.Spec.DPUs.DPUSets[0].NodeSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"feature.node.kubernetes.io/dpu-enabled": "true"},
		}
		dpuDeployment.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dpuDeployment)).To(Succeed())

		By("checking that the underlying objects are created")
		Eventually(func(g Gomega) {
			gotDPUSetList := &provisioningv1.DPUSetList{}
			g.Expect(testClient.List(ctx,
				gotDPUSetList,
				client.InNamespace(dpuDeployment.GetNamespace()),
				client.MatchingLabels{
					"svc.dpu.nvidia.com/owned-by-dpudeployment": fmt.Sprintf("%s_%s", dpuDeployment.GetNamespace(), dpuDeployment.GetName()),
				})).To(Succeed())
			g.Expect(gotDPUSetList.Items).To(HaveLen(1))

			gotDPUServiceList := &dpuservicev1.DPUServiceList{}
			g.Expect(testClient.List(ctx,
				gotDPUServiceList,
				client.InNamespace(dpuDeployment.GetNamespace()),
				client.MatchingLabels{
					"svc.dpu.nvidia.com/owned-by-dpudeployment": fmt.Sprintf("%s_%s", dpuDeployment.GetNamespace(), dpuDeployment.GetName()),
				})).To(Succeed())
			g.Expect(gotDPUServiceList.Items).To(HaveLen(1))

			gotDPUServiceChainList := &dpuservicev1.DPUServiceChainList{}
			g.Expect(testClient.List(ctx,
				gotDPUServiceChainList,
				client.InNamespace(dpuDeployment.GetNamespace()),
				client.MatchingLabels{
					"svc.dpu.nvidia.com/owned-by-dpudeployment": fmt.Sprintf("%s_%s", dpuDeployment.GetNamespace(), dpuDeployment.GetName()),
				})).To(Succeed())
			g.Expect(gotDPUServiceChainList.Items).To(HaveLen(1))

			gotDPUServiceInterfaceList := &dpuservicev1.DPUServiceInterfaceList{}
			g.Expect(testClient.List(ctx,
				gotDPUServiceInterfaceList,
				client.InNamespace(dpuDeployment.GetNamespace()),
				client.MatchingLabels{
					"svc.dpu.nvidia.com/owned-by-dpudeployment": fmt.Sprintf("%s_%s", dpuDeployment.GetNamespace(), dpuDeployment.GetName()),
				})).To(Succeed())
			g.Expect(gotDPUServiceInterfaceList.Items).To(HaveLen(1))
		}).WithTimeout(180 * time.Second).Should(Succeed())

		By(fmt.Sprintf("checking that the number of nodes is equal to %d", input.numberOfDPUNodes))
		Eventually(func(g Gomega) {
			// If we're not expecting any nodes in the cluster return with success.
			if input.numberOfDPUNodes == 0 {
				return
			}
			nodes := &corev1.NodeList{}
			g.Expect(dpuClusterClient.List(ctx, nodes)).To(Succeed())
			By(fmt.Sprintf("Expected number of nodes %d to equal %d", len(nodes.Items), input.numberOfDPUNodes))
			g.Expect(nodes.Items).To(HaveLen(input.numberOfDPUNodes))
		}).WithTimeout(30 * time.Minute).WithPolling(120 * time.Second).Should(Succeed())
	})

	It("create DPUServiceInterface and check that it is mirrored to each cluster", func() {
		if skipCleanup {
			Skip("Skip cleanup resources")
		}
		dpuServiceInterfaceName := "pf0-vf2"
		dpuServiceInterfaceNamespace := "test-dpudeployment"
		By("create test namespace")
		testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: dpuServiceInterfaceNamespace}}
		testNS.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, testNS)).To(Succeed())
		By("create DPUServiceInterface")
		dpuServiceInterface := input.dpuServiceInterface.DeepCopy()
		dpuServiceInterface.SetName(dpuServiceInterfaceName)
		dpuServiceInterface.SetNamespace(dpuServiceInterfaceNamespace)
		dpuServiceInterface.SetLabels(cleanupLabels)
		Expect(testClient.Create(ctx, dpuServiceInterface)).To(Succeed())

		By("verify ServiceInterfaceSet is created in DPF clusters")
		Eventually(func(g Gomega) {
			serviceInterfaceSetList := &dpuservicev1.ServiceInterfaceSetList{}
			g.Expect(dpuClusterClient.List(ctx, serviceInterfaceSetList)).To(Succeed())
			g.Expect(serviceInterfaceSetList.Items).To(HaveLen(2))
		}, time.Second*300, time.Millisecond*250).Should(Succeed())

		// If we're not expecting any nodes in the cluster return with success.
		if input.numberOfDPUNodes == 0 {
			return
		}

		By(fmt.Sprintf("verify ServiceInterface is created in %d nodes", input.numberOfDPUNodes))
		Eventually(func(g Gomega) {
			serviceInterfaceList := &dpuservicev1.ServiceInterfaceList{}
			g.Expect(dpuClusterClient.List(ctx, serviceInterfaceList)).To(Succeed())
			g.Expect(serviceInterfaceList.Items).To(Not(BeEmpty()))
		}).WithTimeout(30 * time.Minute).WithPolling(120 * time.Second).Should(Succeed())
	})

	// we expect all resources, the DPUCluster included to be deleted as part of the operatorConfig cleanup
	It("delete the operatorConfig and ensure it is deleted", func() {
		if skipCleanup {
			Skip("Skip cleanup resources")
		}
		// Check that all deployments and DPUServices are deleted.
		Eventually(func(g Gomega) {
			key := client.ObjectKey{Namespace: dpfOperatorSystemNamespace, Name: configName}
			g.Expect(client.IgnoreNotFound(testClient.DeleteAllOf(ctx, &operatorv1.DPFOperatorConfig{}, client.InNamespace(dpfOperatorSystemNamespace)))).To(Succeed())
			g.Expect(apierrors.IsNotFound(testClient.Get(ctx, key, &operatorv1.DPFOperatorConfig{}))).To(BeTrue())
		}).WithTimeout(600 * time.Second).Should(Succeed())
	})
}

func unstructuredFromFile(path string) *unstructured.Unstructured {
	data, err := os.ReadFile(path)
	Expect(err).ToNot(HaveOccurred())
	obj := &unstructured.Unstructured{}
	Expect(yaml.Unmarshal(data, obj)).To(Succeed())
	obj.SetLabels(cleanupLabels)
	return obj
}

func collectResourcesAndLogs(ctx context.Context) error {
	if !collectResources {
		return nil
	}
	// Run dpfctl describe to get information about the resources on a failed state.
	opts := dpfctl.ObjectTreeOptions{
		ShowOtherConditions: "failed",
		ExpandResources:     "failed",
		Output:              "table",
		Colors:              true,
	}
	t, err := dpfctl.Discover(ctx, testClient, opts, "all")
	// Only print if at least a operatorConfig is found.
	if !apierrors.IsNotFound(err) {
		if err := dpfctl.PrintObjectTree(t); err != nil {
			return err
		}
	}

	// Get the path to place artifacts in
	_, basePath, _, _ := runtime.Caller(0)
	artifactsPath := filepath.Join(filepath.Dir(basePath), "../../artifacts")
	inventoryManifestsPath := filepath.Join(filepath.Dir(basePath), "../../internal/operator/inventory/manifests")

	// Create a resourceCollector to dump logs and resources for test debugging.
	clusters, err := collector.GetClusterCollectors(ctx, testClient, artifactsPath, inventoryManifestsPath, clientset)
	Expect(err).NotTo(HaveOccurred())
	return collector.New(clusters).Run(ctx)
}

func VerifyKSMMetricsCollection(ctx context.Context, input systemTestInput) {
	It("validate DPF metrics services are accessible", func() {
		if !deployKSM {
			Skip("Skip KSM metrics accessibility test due to KSM is not deployed")
		}

		By("verify KMS metrics endpoint is accessible")
		Eventually(func(g Gomega) {
			request := testRESTClient.Get().AbsPath(metricsURI)
			response, err := request.DoRaw(ctx)
			g.Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Request %s failed with err: %v", metricsURI, err))
			g.Expect(response).NotTo(BeNil(), fmt.Sprintf("Metrics api is not accessible by url %s ", metricsURI))
		}).WithTimeout(30 * time.Second).Should(Succeed())
	})
}

func ValidateGeneralDPFMetrics(ctx context.Context, input systemTestInput) {
	hostPrometheusName := "prometheus"
	It("validate DPF metrics services are accessible", func() {
		if !deployPrometheus {
			Skip("Skip prometheus metrics tests")
		}
		By("verify prometheus is running")
		Eventually(func(g Gomega) {
			prometheusPods := &corev1.PodList{}
			g.Expect(testClient.List(ctx, prometheusPods, client.MatchingLabels{"app.kubernetes.io/name": hostPrometheusName})).To(Succeed())
			g.Expect(prometheusPods.Items).NotTo(BeEmpty(), fmt.Sprintf("Expected number of Prometheus pods %d >= %d", len(prometheusPods.Items), 0))
			for _, pod := range prometheusPods.Items {
				g.Expect(string(pod.Status.Phase)).To(Equal("Running"), "Pod %s status is %s", pod.Name, pod.Status.Phase)
			}
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("validate DPF metric on Prometheus", func() {
		if !deployPrometheus {
			Skip("Skip prometheus metrics tests due to Prometheus is not deployed")
		}
		By("verify metrics are being collected")
		expectedMetricsNames := map[string][]string{
			"bfb":               {"created", "info", "status_phase"},
			"dpfoperatorconfig": {"created", "info", "status_conditions", "status_condition_last_transition_time"}, // "paused" missed
			"dpucluster":        {"created", "info", "status_phase", "status_conditions", "status_condition_last_transition_time"},
			//"dpu":               {"created", "info", "status_phase", "status_conditions", "status_condition_last_transition_time"}, // all missed
		}

		By("verify Prometheus proxy is accessible")
		Eventually(func(g Gomega) {
			// Prepare metrics request URL and query
			query := "/api/v1/query"
			metricsURL := metrics.GetMetricsURI("dpf-operator-prometheus-server", dpfOperatorSystemNamespace, 80, query)
			request := testRESTClient.Get().AbsPath(metricsURL).Param("query", `{__name__=~"dpf.*"}`)
			// Request metrics from prometheus
			response, err := request.DoRaw(ctx)
			g.Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Request %v failed with err: %v", request, err))
			g.Expect(response).NotTo(BeNil(), fmt.Sprintf("Metrics api is not accessible by url %v ", request))
		}).WithTimeout(20 * time.Second).Should(Succeed())

		By("verify metrics keys Prometheus")
		Eventually(func(g Gomega) {
			// No checks for bfb metrics if the numberOfDPUNodes is 0
			if input.numberOfDPUNodes == 0 {
				delete(expectedMetricsNames, "bfb")
			}
			actualMetricsNames := metrics.GetPrometheusMetrics(ctx, testRESTClient, g, maps.Keys(expectedMetricsNames), dpfOperatorSystemNamespace)
			g.Expect(actualMetricsNames).NotTo(BeEmpty(), "Actual metrics are empty")
			g.Expect(metrics.VerifyMetrics(expectedMetricsNames, actualMetricsNames)).To(BeEmpty())
		}).WithTimeout(20 * time.Second).Should(Succeed())
	})
}
