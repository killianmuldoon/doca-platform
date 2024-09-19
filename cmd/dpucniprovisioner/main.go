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

	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	kexec "k8s.io/utils/exec"
)

func main() {
	klog.Info("Starting DPU CNI Provisioner")
	ovsClient, err := ovsclient.New()
	if err != nil {
		klog.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := clock.RealClock{}
	provisioner := dpucniprovisioner.New(ctx, c, ovsClient, networkhelper.New(), kexec.New(), nil, net.IP{}, nil, nil)

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
