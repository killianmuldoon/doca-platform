/*
COPYRIGHT 2024 NVIDIA

Licensed under the Apache License, Version 2.0 (the License);
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an AS IS BASIS,
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
	"time"

	ovnkubernetesoperatorv1 "github.com/nvidia/doca-platform/api/ovnkubernetesoperator/v1alpha1"
	ovnkubernetesoperatorcontroller "github.com/nvidia/doca-platform/internal/ovnkubernetesoperator/controllers"
	"github.com/nvidia/doca-platform/internal/ovnkubernetesoperator/webhooks"
	"github.com/nvidia/doca-platform/internal/release"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
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

	utilruntime.Must(ovnkubernetesoperatorv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

var (
	customOVNKubernetesDPUImage    string
	customOVNKubernetesNonDPUImage string
	enableWebhook                  bool
)

func main() {
	defaults := release.NewDefaults()
	err := defaults.Parse()
	if err != nil {
		setupLog.Error(err, "unable to parse release defaults")
		os.Exit(1)
	}
	// TODO: This doesn't work as these should be distinct images.
	customOVNKubernetesDPUImage = defaults.CustomOVNKubernetesImage
	customOVNKubernetesNonDPUImage = defaults.CustomOVNKubernetesImage

	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var syncPeriod time.Duration

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.BoolVar(&enableWebhook, "enable-webhook", true,
		"Enable the webhook that is handling the network annotation injection for workload pods")
	flag.DurationVar(&syncPeriod, "sync-period", 10*time.Minute,
		"The minimum interval at which watched resources are reconciled.")

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
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "8a3114c5.dpu.nvidia.com",
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

	settings, err := getSettings()
	if err != nil {
		setupLog.Error(err, "unable to get settings")
	}

	if err = (&ovnkubernetesoperatorcontroller.DPFOVNKubernetesOperatorConfigReconciler{
		Client:   mgr.GetClient(),
		Settings: settings,
		Scheme:   mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DPFOVNKubernetesOperatorConfig")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if enableWebhook {
		if err = (&webhooks.NetworkInjector{
			Client: mgr.GetClient(),
		}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "DPFOperatorConfig")
			os.Exit(1)
		}
	}

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

func getSettings() (*ovnkubernetesoperatorcontroller.DPFOVNKubernetesOperatorConfigReconcilerSettings, error) {
	if customOVNKubernetesDPUImage == "" {
		return nil, errors.New("flag ovn-kubernetes-dpu-image can't be empty")
	}
	if customOVNKubernetesNonDPUImage == "" {
		return nil, errors.New("flag ovn-kubernetes-dpu-image can't be empty")
	}

	return &ovnkubernetesoperatorcontroller.DPFOVNKubernetesOperatorConfigReconcilerSettings{
		CustomOVNKubernetesDPUImage:    customOVNKubernetesDPUImage,
		CustomOVNKubernetesNonDPUImage: customOVNKubernetesNonDPUImage,
		WebhookEnabled:                 enableWebhook,
	}, nil
}
