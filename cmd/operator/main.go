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

package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"os"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	operatorv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/operator/v1alpha1"
	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/servicechain/v1alpha1"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application/v1alpha1"
	operatorcontroller "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/controllers"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/inventory"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/operator/release"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(operatorv1.AddToScheme(scheme))

	utilruntime.Must(dpuservicev1.AddToScheme(scheme))

	utilruntime.Must(argov1.AddToScheme(scheme))

	// This is not required by the client the operator has to the host cluster, but rather the clients used to access
	// the DPU clusters. By default, the schema for each client is parsed from the global context, therefore it's
	// shared across all clients. Registering this type here prevents concurrent map writes detected sporadically while
	// running the DPF Operator testing suite.
	// TODO: Revisit this approach and check if it makes sense to have new schema for each DPU cluster client we create.
	utilruntime.Must(sfcv1.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

var (
	metricsAddr                    string
	enableLeaderElection           bool
	probeAddr                      string
	secureMetrics                  bool
	enableHTTP2                    bool
	customOVNKubernetesDPUImage    string
	customOVNKubernetesNonDPUImage string
	configSingletonNamespace       string
	configSingletonName            string
	reconcileOVNKubernetes         bool
)

func initFlags(defaults *release.Defaults) {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.BoolVar(&reconcileOVNKubernetes, "reconcileOVNKubernetes", true,
		"Enable OVN Kubernetes offload to DPU. OpenShift only.")
	flag.StringVar(&customOVNKubernetesDPUImage, "ovn-kubernetes-dpu-image", defaults.CustomOVNKubernetesDPUImage,
		"The custom OVN Kubernetes image deployed by the operator to the DPU enabled nodes (workers)")
	flag.StringVar(&customOVNKubernetesNonDPUImage, "ovn-kubernetes-non-dpu-image", defaults.CustomOVNKubernetesNonDPUImage,
		"The custom OVN Kubernetes image deployed by the operator to the non DPU enabled nodes (control plane)")
	flag.StringVar(&configSingletonNamespace, "config-namespace",
		operatorcontroller.DefaultDPFOperatorConfigSingletonNamespace, "The namespace of the DPFOperatorConfig the operator will reconcile")
	flag.StringVar(&configSingletonName, "config-name",
		operatorcontroller.DefaultDPFOperatorConfigSingletonName, "The name of the DPFOperatorConfig the operator will reconcile")
}

func main() {
	defaults := release.NewDefaults()
	err := defaults.Parse()
	if err != nil {
		setupLog.Error(err, "unable to parse release defaults")
		os.Exit(1)
	}

	initFlags(defaults)
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancelation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "8a3114c5.dpf.nvidia.com",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// ParseAll manifest yamls to ensure Kubernetes objects are available before starting.
	inventory := inventory.New()
	if err := inventory.ParseAll(); err != nil {
		setupLog.Error(err, "unable to parse inventory")
		os.Exit(1)
	}

	settings, err := getSettings()
	if err != nil {
		setupLog.Error(err, "unable to get settings")
	}

	if err = (&operatorcontroller.DPFOperatorConfigReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Settings:  settings,
		Inventory: inventory,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DPFOperatorConfig")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getSettings() (*operatorcontroller.DPFOperatorConfigReconcilerSettings, error) {
	if customOVNKubernetesDPUImage == "" {
		return nil, errors.New("flag ovn-kubernetes-dpu-image can't be empty")
	}
	if customOVNKubernetesNonDPUImage == "" {
		return nil, errors.New("flag ovn-kubernetes-dpu-image can't be empty")
	}

	configSingletonNamespaceName := &types.NamespacedName{
		Namespace: configSingletonNamespace,
		Name:      configSingletonName,
	}
	if configSingletonNamespace == "" && configSingletonName == "" {
		configSingletonNamespaceName = nil
	}
	return &operatorcontroller.DPFOperatorConfigReconcilerSettings{
		CustomOVNKubernetesDPUImage:    customOVNKubernetesDPUImage,
		CustomOVNKubernetesNonDPUImage: customOVNKubernetesNonDPUImage,
		ConfigSingletonNamespaceName:   configSingletonNamespaceName,
		ReconcileOVNKubernetes:         reconcileOVNKubernetes,
	}, nil
}
