/*
Copyright 2025 NVIDIA

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

package controllers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/allocator"
	"github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu"
	dutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/dpu/util"
	cutil "github.com/nvidia/doca-platform/internal/provisioning/controllers/util"
	"github.com/nvidia/doca-platform/test/mock/dms/pkg/config"
	dmsserver "github.com/nvidia/doca-platform/test/mock/dms/pkg/server"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const dmsServerPodName = "dms-server-pod-name"

var (
	cfg                        *rest.Config
	testClient                 client.Client
	testEnv                    *envtest.Environment
	ctx, testManagerCancelFunc = context.WithCancel(ctrl.SetupSignalHandler())
	key                        *rsa.PrivateKey
	cert                       *x509.Certificate
)

func TestMain(m *testing.M) {
	setupLogger := ctrl.Log.WithName("dms-server-controller-test-setup")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "..", "..", "deploy", "helm", "dpf-operator", "templates", "crds"),
			filepath.Join("..", "..", "..", "..", "..", "test", "objects", "crd", "cert-manager"),
			filepath.Join("..", "..", "..", "..", "..", "test", "objects", "crd", "argocd"),
			filepath.Join("..", "..", "..", "..", "..", "test", "objects", "crd", "kamaji"),
		},
		ErrorIfCRDPathMissing: true,

		// The BinaryAssetsDirectory is only required if you want to run the tests directly
		// without call the makefile target test. If not informed it will look for the
		// default path defined in controller-runtime which is /usr/local/kubebuilder/.
		// Note that you must have the required binaries setup under the bin directory to perform
		// the tests directly. When we run make test it will be setup and used automatically.
		BinaryAssetsDirectory: filepath.Join("..", "..", "..", "hack", "tools", "bin", "k8s",
			fmt.Sprintf("1.29.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}
	ctrl.SetLogger(klog.Background())
	var err error

	s := scheme.Scheme

	if err := provisioningv1.AddToScheme(s); err != nil {
		panic(fmt.Sprintf("Failed to add provisioningv1 scheme: %v", err))
	}

	// cfg is defined in this file globally in this package. This allows the resource collector to use this
	// config when it runs.
	cfg, err = testEnv.Start()
	if err != nil {
		panic(fmt.Sprintf("Failed to set up test environment: %v", err))
	}

	// create the certs required for communication between the DPU controller and the mock DMS server
	key, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Sprintf("Failed to set up test certificate private key: %v", err))
	}
	cert, err = newCert(key)
	if err != nil {
		panic(fmt.Sprintf("Failed to set up test certificate: %v", err))
	}
	// testClient is defined globally in this package so it can be used by the resource.
	testClient, err = client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		panic(fmt.Sprintf("Failed to create client: %v", err))
	}

	testManager, err := ctrl.NewManager(cfg,
		ctrl.Options{
			Scheme: s,
			// Set metrics server bind address to 0 to disable it.
			Metrics: server.Options{
				BindAddress: "0",
			}})
	if err != nil {
		panic(fmt.Sprintf("Failed to create test manager: %v", err))
	}

	// Set base directory for BFB and kubeconfig on the DPU controller.
	// This overwrites hardcoded variables in order to enable testing.
	cutil.BFBBaseDir = os.TempDir()
	cutil.KubeconfigBaseDir = os.TempDir()

	err = os.MkdirAll(cutil.BFBBaseDir, 0755)
	if err != nil {
		panic(err)
	}

	dpuReconciler := dpu.NewDPUReconciler(
		testManager,
		allocator.NewAllocator(testClient),
		&mockKubeadmJoinCommandGenerator{},
		&mockHostUptimeReporter{},
		dutil.DPUOptions{DPUInstallInterface: string(provisioningv1.InstallViaHost)},
	)
	if err := dpuReconciler.SetupWithManager(testManager); err != nil {
		panic(fmt.Sprintf("Failed to setup DPU reconciler: %v", err))
	}

	dmsServerReconciler := DMSServerReconciler{
		Client:  testClient,
		PodName: dmsServerPodName,
		Server:  dmsserver.NewDMSServerMux(20000, 22000, "127.0.0.1", cert, key, config.Config{}),
	}
	if err := dmsServerReconciler.SetupWithManager(testManager); err != nil {
		panic(fmt.Sprintf("Failed to setup DMSServer reconciler: %v", err))
	}

	go func() {
		if err := testManager.Start(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start test manager: %v", err))
		}
	}()

	// run the test suite in this package.
	if code := m.Run(); code != 0 {
		setupLogger.Info("error running tests: ", code)
	}

	if testManagerCancelFunc != nil {
		testManagerCancelFunc()
	}

	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop test environment: %v", err))
	}
}
