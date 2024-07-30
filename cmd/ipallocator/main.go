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
	"errors"
	"fmt"
	"os"
	"os/signal"

	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/cniprovisioner/utils/readyz"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/ipallocator"

	"github.com/containernetworking/cni/libcni"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

// requiredEnvVariables are environment variables that the program needs in order to run. Boolean indicates if this env
// variable should be configured by Kubernetes downward API (easier for user to understand the mistake).
var requiredEnvVariables = map[string]bool{
	"POOL":              false,
	"POOL_TYPE":         false,
	"K8S_POD_NAME":      true,
	"K8S_POD_NAMESPACE": true,
	"K8S_POD_UID":       true,
}

func main() {
	klog.Info("Starting IP Allocator")
	env, err := parseEnv()
	if err != nil {
		klog.Fatalf("error while parsing environment: %s", err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	allocator := ipallocator.New(
		libcni.NewCNIConfig([]string{ipallocator.CNIBinDir}, nil),
		env["POOL"],
		env["POOL_TYPE"],
		env["K8S_POD_NAME"],
		env["K8S_POD_NAMESPACE"],
		env["K8S_POD_UID"],
	)

	err = allocator.Allocate(ctx)
	if err != nil {
		klog.Fatal(err)
	}

	err = readyz.ReportReady()
	if err != nil {
		klog.Fatal(err)
	}

	klog.Info("IP Allocator job has finished")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
	klog.Info("Received termination signal, terminating.")
	cancel()
}

// parseEnv parses the required environment variables
func parseEnv() (map[string]string, error) {
	var errs []error
	m := make(map[string]string)
	for key, expectedViaDownwardAPI := range requiredEnvVariables {
		value := os.Getenv(key)
		if value == "" {
			err := fmt.Errorf("required %s environment variable is not found", key)
			if expectedViaDownwardAPI {
				err = errors.Join(err, errors.New("this is supposed to be configured via Kubernetes Downward API in production"))
			}
			errs = append(errs, err)
		}
		m[key] = value
	}

	return m, kerrors.NewAggregate(errs)
}
