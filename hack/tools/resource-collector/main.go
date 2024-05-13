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
	"flag"
	"fmt"
	"path/filepath"
	"runtime"

	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/test/utils/collector"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	artifactsDirectory string
	kubeconfig         string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "~/.kube/config", "absolute path to the kubeconfig file")
	flag.StringVar(&artifactsDirectory, "artifactsDirectory", "", "absolute path to the directory where artifacts are stored")

}
func main() {
	flag.Parse()

	ctx := context.Background()
	log := klog.FromContext(ctx)

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clusterClient, err := client.New(config, client.Options{})
	if err != nil {
		panic(err)
	}

	// If the artifacts directory is not set default to the project root.
	if artifactsDirectory == "" {
		// Get the path to place artifacts in
		_, basePath, _, _ := runtime.Caller(0)
		artifactsDirectory = filepath.Join(filepath.Dir(basePath), "../../../artifacts")
	}

	clusters, err := collector.GetClusterCollectors(ctx, clusterClient, artifactsDirectory, config)

	if err != nil {
		log.Error(err, "Failed to get cluster collectors")
	}
	resourceCollector := collector.New(clusters)
	log.Info(fmt.Sprintf("Running the resource resourceCollector. Artifacts will be stored in %s", artifactsDirectory))
	// Run the resourceCollector for the main cluster.

	if err := resourceCollector.Run(ctx); err != nil {
		panic(err)
	}
}
