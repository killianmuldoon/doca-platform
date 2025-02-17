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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	dpuservicev1 "github.com/nvidia/doca-platform/api/dpuservice/v1alpha1"
	operatorv1 "github.com/nvidia/doca-platform/api/operator/v1alpha1"
	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/test/utils"
	"github.com/nvidia/doca-platform/test/utils/metrics"
	argov1 "github.com/nvidia/doca-platform/third_party/api/argocd/api/application/v1alpha1"
	kamajiv1 "github.com/nvidia/doca-platform/third_party/api/kamaji/api/v1alpha1"
	nvipamv1 "github.com/nvidia/doca-platform/third_party/api/nvipam/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	flag.StringVar(&testKubeconfig, "e2e.testKubeconfig", "", "path to the testKubeconfig file")
	flag.StringVar(&configPath, "e2e.config", "", "path to the configuration file")

	getEnvVariables()
}

const (
	configName                 = "dpfoperatorconfig"
	dpfOperatorSystemNamespace = "dpf-operator-system"
)

// These variables can be set from the environment when running the DPF tests.
var (
	configPath string
	// testKubeconfig path to be used for this test.
	testKubeconfig string
	// skipCleanup indicates whether to skip the cleanup of resources created during the e2e test run.
	// When set to true, resources will not be removed after the test completes.
	skipCleanup = false
	// collectResources indicates whether to collect logs an objects after an e2e test run.
	collectResources = true

	// bfbImageURL can be used to override the default BFB image URL used in the tests.
	bfbImageURL = ""

	// deployPrometheus indicates whether metrics are collected by Prometheus.
	// When set to true, Prometheus metrics will be verified.
	deployPrometheus = false
	// deployKSM indicates whether metrics are collected by KSM.
	// When set to true, KMS metrics will be verified for the resources.
	deployKSM           = false
	argoCDInstanceLabel = "argocd.argoproj.io/instance"

	// Labels and resources targeted for cleanup before running our e2e tests.
	// This cleanup is typically handled by cleanupObjs, but if an e2e test fails, the standard cleanup may not be executed.
	cleanupLabels     = map[string]string{"dpf-operator-e2e-test-cleanup": "true"}
	labelSelector     = labels.SelectorFromSet(cleanupLabels)
	resourcesToDelete = []client.ObjectList{
		&dpuservicev1.DPUDeploymentList{},
		&dpuservicev1.DPUServiceCredentialRequestList{},
		&dpuservicev1.DPUServiceList{},
		&dpuservicev1.DPUServiceConfigurationList{},
		&dpuservicev1.DPUServiceTemplateList{},
		&provisioningv1.DPUSetList{},
		&provisioningv1.DPUList{},
		&provisioningv1.BFBList{},
		&provisioningv1.DPUClusterList{},
		&dpuservicev1.DPUServiceIPAMList{},
		&dpuservicev1.DPUServiceChainList{},
		&dpuservicev1.DPUServiceInterfaceList{},
		&kamajiv1.TenantControlPlaneList{},
		&operatorv1.DPFOperatorConfigList{},
		&appsv1.DeploymentList{},
		&appsv1.DaemonSetList{},
		&corev1.PersistentVolumeClaimList{},
		&corev1.NamespaceList{},
		&corev1.NodeList{},
		&corev1.ServiceList{},
	}
)

func getEnvVariables() {
	if v, found := os.LookupEnv("E2E_SKIP_CLEANUP"); found {
		var err error
		skipCleanup, err = strconv.ParseBool(v)
		if err != nil {
			panic(fmt.Errorf("string must be a bool: %v", err))
		}
	}
	if url, found := os.LookupEnv("BFB_IMAGE_URL"); found {
		var err error
		bfbImageURL, err = utils.ResolveBFBImageURL(url)
		if err != nil {
			panic(err)
		}
	}
	if v, found := os.LookupEnv("DPF_E2E_COLLECT_RESOURCES"); found {
		var err error
		collectResources, err = strconv.ParseBool(v)
		if err != nil {
			panic(fmt.Errorf("string must be a bool: %v", err))
		}
	}

	if v, found := os.LookupEnv("DEPLOY_PROMETHEUS"); found {
		var err error
		deployPrometheus, err = strconv.ParseBool(v)
		if err != nil {
			panic(fmt.Errorf("string must be a bool: %v", err))
		}
	}
	if v, found := os.LookupEnv("DEPLOY_KSM"); found {
		var err error
		deployKSM, err = strconv.ParseBool(v)
		if err != nil {
			panic(fmt.Errorf("string must be a bool: %v", err))
		}
	}
}

var (
	testClient       client.Client
	restConfig       *rest.Config
	clientset        *kubernetes.Clientset
	ctx              = ctrl.SetupSignalHandler()
	dpuClusterClient client.Client
	testRESTClient   *rest.RESTClient
	metricsURI       string
	conf             *config
)

// Run e2e tests using the Ginkgo runner.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	g := NewWithT(t)
	defer GinkgoRecover()
	var err error
	_, err = fmt.Fprintf(GinkgoWriter, "Starting dpf-operator suite\n")
	Expect(err).ToNot(HaveOccurred())
	ctrl.SetLogger(klog.Background())

	Expect(dpuservicev1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(operatorv1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(argov1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(provisioningv1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(nvipamv1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(kamajiv1.AddToScheme(scheme.Scheme)).To(Succeed())
	s := scheme.Scheme

	// SchemeGroupVersion is group version used to register these objects
	var SchemeGroupVersion = schema.GroupVersion{Group: "", Version: "v1"}

	conf, err = readConfig(configPath)
	g.Expect(err).NotTo(HaveOccurred())

	// If testKubeconfig is not set default it to $HOME/.kube/config
	home, exists := os.LookupEnv("HOME")
	g.Expect(exists).To(BeTrue())
	if testKubeconfig == "" {
		testKubeconfig = filepath.Join(home, ".kube/config")
	}

	// Create a client to use throughout the test.
	restConfig, err = clientcmd.BuildConfigFromFlags("", testKubeconfig)
	g.Expect(err).NotTo(HaveOccurred())
	clientset, err = kubernetes.NewForConfig(restConfig)
	g.Expect(err).NotTo(HaveOccurred())
	testClient, err = client.New(restConfig, client.Options{Scheme: s})
	g.Expect(err).NotTo(HaveOccurred())

	// Extend configs to restConfig for testRESTClient
	restConfig.GroupVersion = &SchemeGroupVersion
	restConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	testRESTClient, err = rest.RESTClientFor(restConfig)
	g.Expect(err).NotTo(HaveOccurred())
	if deployKSM {
		metricsURI = metrics.GetMetricsURI("dpf-operator-kube-state-metrics", dpfOperatorSystemNamespace, 8080, "/metrics")
		g.Expect(metricsURI).NotTo(BeEmpty())
	}

	RunSpecs(t, "e2e suite")
}
