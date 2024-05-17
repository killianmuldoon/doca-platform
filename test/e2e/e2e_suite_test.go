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
	"runtime"
	"testing"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"
	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application/v1alpha1"
	nvipamv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/nvipam/api/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/test/utils/collector"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// testKubeconfig path to be used for this test.
	testKubeconfig string
)

func init() {
	flag.StringVar(&testKubeconfig, "e2e.testKubeconfig", "", "path to the testKubeconfig file")
}

var (
	testClient        client.Client
	resourceCollector *collector.Collector
	ctx               = ctrl.SetupSignalHandler()
)

// Run e2e tests using the Ginkgo runner.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	g := NewWithT(t)
	defer GinkgoRecover()
	fmt.Fprintf(GinkgoWriter, "Starting dpf-operator suite\n")
	ctrl.SetLogger(klog.Background())

	Expect(dpuservicev1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(operatorv1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(argov1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(sfcv1.AddToScheme(scheme.Scheme)).To(Succeed())
	Expect(nvipamv1.AddToScheme(scheme.Scheme)).To(Succeed())
	s := scheme.Scheme

	// If testKubeconfig is not set default it to $HOME/.kube/config
	home, exists := os.LookupEnv("HOME")
	g.Expect(exists).To(BeTrue())
	if testKubeconfig == "" {
		testKubeconfig = filepath.Join(home, ".kube/config")
	}

	// Create a client to use throughout the test.
	config, err := clientcmd.BuildConfigFromFlags("", testKubeconfig)
	g.Expect(err).NotTo(HaveOccurred())
	testClient, err = client.New(config, client.Options{Scheme: s})
	g.Expect(err).NotTo(HaveOccurred())

	// Get the path to place artifacts in
	_, basePath, _, _ := runtime.Caller(0)
	artifactsPath := filepath.Join(filepath.Dir(basePath), "../../artifacts")
	inventoryManifestsPath := filepath.Join(filepath.Dir(basePath), "../../internal/operator/inventory/manifests")

	// Create a resourceCollector to dump logs and resources for test debugging.
	clusters, err := collector.GetClusterCollectors(ctx, testClient, artifactsPath, inventoryManifestsPath, config)
	g.Expect(err).NotTo(HaveOccurred())
	resourceCollector = collector.New(clusters)
	RunSpecs(t, "e2e suite")
}
