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
	"flag"
	"os"
	"time"

	provisioningdpfv1alpha1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/provisioning/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/bfb"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpu/util"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/provisioning/controllers/dpuset"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	nodeMaintenancev1beta1 "github.com/medik8s/node-maintenance-operator/api/v1beta1"
	nmstatev1 "github.com/nmstate/kubernetes-nmstate/api/v1"
	nmstatev1alpha1 "github.com/nmstate/kubernetes-nmstate/api/v1alpha1"
	nmstatev1beta1 "github.com/nmstate/kubernetes-nmstate/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(provisioningdpfv1alpha1.AddToScheme(scheme))

	utilruntime.Must(nmstatev1.AddToScheme(scheme))
	utilruntime.Must(nmstatev1beta1.AddToScheme(scheme))
	utilruntime.Must(nmstatev1alpha1.AddToScheme(scheme))

	utilruntime.Must(nodeMaintenancev1beta1.AddToScheme(scheme))
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var dmsImage string
	var hostnetworkImage string
	var dhcrelayImage string
	var parprouterdImage string
	var imagePullSecret string
	var bfbPVC string
	var dhcp string
	var dmsTimeout int
	var dmsPodTimeout time.Duration

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&dmsImage, "dms-image", "", "The image for DMS pod.")
	flag.StringVar(&hostnetworkImage, "hostnetwork-image", "", "The image for DMS pod.")
	flag.StringVar(&dhcrelayImage, "dhcrelay-image", "", "The image for DMS pod.")
	flag.StringVar(&parprouterdImage, "parprouterd-image", "", "The image for DMS pod.")
	flag.StringVar(&imagePullSecret, "image-pull-secret", "", "The image pull secret for pulling DMS image.")
	flag.StringVar(&bfbPVC, "bfb-pvc", "", "The pvc to storage bfb.")
	flag.StringVar(&dhcp, "dhcp", "", "The DHCP server address.")
	flag.IntVar(&dmsTimeout, "dms-timeout", 900, "The max timeout execution in seconds of a command if not responding, 0 is unlimited.")
	flag.DurationVar(&dmsPodTimeout, "dms-pod-timeout", 5*time.Minute, "Timeout for DMS pods")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                server.Options{BindAddress: metricsAddr},
		WebhookServer:          &webhook.DefaultServer{},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "19f9f38b.nvidia.com",
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

	dpuOptions := util.DPUOptions{
		DMSImageWithTag:          dmsImage,
		HostnetworkImageWithTag:  hostnetworkImage,
		DHCRelayImageWithTag:     dhcrelayImage,
		PrarprouterdImageWithTag: parprouterdImage,
		ImagePullSecret:          imagePullSecret,
		BfbPvc:                   bfbPVC,
		DHCP:                     dhcp,
		DMSTimeout:               dmsTimeout,
		DMSPodTimeout:            dmsPodTimeout,
	}
	setupLog.Info("DPU", "options", dpuOptions)
	if err = (&dpu.DpuReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Recorder:   mgr.GetEventRecorderFor(dpu.DpuControllerName),
		DPUOptions: dpuOptions,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Dpu")
		os.Exit(1)
	}
	if err = (&dpuset.DpuSetReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor(dpuset.DpuSetControllerName),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DpuSet")
		os.Exit(1)
	}
	if err = (&bfb.BfbReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor(bfb.BfbControllerName),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Bfb")
		os.Exit(1)
	}
	if err = (&provisioningdpfv1alpha1.Bfb{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Bfb")
		os.Exit(1)
	}
	if err = (&provisioningdpfv1alpha1.Dpu{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Dpu")
		os.Exit(1)
	}
	if err = (&provisioningdpfv1alpha1.DpuSet{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "DpuSet")
		os.Exit(1)
	}
	if err = (&provisioningdpfv1alpha1.DPUFlavor{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "DPUFlavor")
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
