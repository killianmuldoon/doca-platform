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
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var testClient client.Client
var testEnv *envtest.Environment
var ctx, testManagerCancelFunc = context.WithCancel(ctrl.SetupSignalHandler())
var reconciler *DPFOperatorConfigReconciler

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "operator", "crd", "bases")},
		ErrorIfCRDPathMissing: true,

		// The BinaryAssetsDirectory is only required if you want to run the tests directly
		// without call the makefile target test. If not informed it will look for the
		// default path defined in controller-runtime which is /usr/local/kubebuilder/.
		// Note that you must have the required binaries setup under the bin directory to perform
		// the tests directly. When we run make test it will be setup and used automatically.
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.29.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = operatorv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	s := scheme.Scheme
	//+kubebuilder:scaffold:scheme

	testClient, err = client.New(cfg, client.Options{Scheme: s})
	Expect(err).NotTo(HaveOccurred())
	Expect(testClient).NotTo(BeNil())

	By("setting up and running the test reconciler")
	testManager, err := ctrl.NewManager(cfg,
		ctrl.Options{
			Scheme: scheme.Scheme,
			// Set metrics server bind address to 0 to disable it.
			Metrics: server.Options{
				BindAddress: "0",
			}})
	Expect(err).ToNot(HaveOccurred())

	reconciler = &DPFOperatorConfigReconciler{
		Client: testClient,
		Scheme: testManager.GetScheme(),
		Settings: &DPFOperatorConfigReconcilerSettings{
			CustomOVNKubernetesImage: "nvidia.com/ovn-kubernetes:dev",
		},
	}
	err = reconciler.SetupWithManager(testManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = testManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	if testManagerCancelFunc != nil {
		testManagerCancelFunc()
	}
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
