//go:build linux

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
	"context"
	"net"
	"os"
	"os/signal"
	"sync"

	dpucniprovisioner "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/dpu"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/networkhelper"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/ovsclient"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/cniprovisioner/utils/readyz"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	kexec "k8s.io/utils/exec"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func main() {
	klog.Info("Starting DPU CNI Provisioner")

	node := os.Getenv("NODE_NAME")
	if node == "" {
		klog.Fatal("NODE_NAME environment variable is not found. This is supposed to be configured via Kubernetes Downward API in production")
	}

	ovsClient, err := ovsclient.New()
	if err != nil {
		klog.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := clock.RealClock{}

	config, err := config.GetConfig()
	if err != nil {
		klog.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	provisioner := dpucniprovisioner.New(ctx, c, ovsClient, networkhelper.New(), kexec.New(), clientset, nil, net.IP{}, nil, nil, node)

	err = provisioner.RunOnce()
	if err != nil {
		klog.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		provisioner.EnsureConfiguration()
	}()

	err = readyz.ReportReady()
	if err != nil {
		klog.Fatal(err)
	}

	klog.Info("DPU CNI Provisioner is ready")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
	klog.Info("Received termination signal, terminating.")
	cancel()
	wg.Wait()
}
