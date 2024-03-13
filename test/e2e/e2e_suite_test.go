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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// kubeconfig path to be used for this test.
	kubeconfig string
)

func init() {
	flag.StringVar(&kubeconfig, "e2e.kubeconfig", "", "path to the kubeconfig file")
}

var (
	testClient client.Client
	ctx        = ctrl.SetupSignalHandler()
)

// Run e2e tests using the Ginkgo runner.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	g := NewWithT(t)
	defer GinkgoRecover()
	fmt.Fprintf(GinkgoWriter, "Starting dpf-operator suite\n")

	// If kubeconfig is not set default it to $HOME/.kube/config
	if kubeconfig == "" {
		home, exists := os.LookupEnv("HOME")
		g.Expect(exists).To(BeTrue())
		kubeconfig = filepath.Join(home, ".kube/config")
	}

	// Create a client to use throughout the test.
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	g.Expect(err).NotTo(HaveOccurred())
	testClient, err = client.New(config, client.Options{})
	g.Expect(err).NotTo(HaveOccurred())
	RunSpecs(t, "e2e suite")
}
