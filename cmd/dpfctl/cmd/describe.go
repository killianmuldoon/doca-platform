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

package cmd

import (
	"flag"

	dpuservicev1 "github.com/nvidia/doca-platform/api/dpuservice/v1alpha1"
	operatorv1 "github.com/nvidia/doca-platform/api/operator/v1alpha1"
	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	argov1 "github.com/nvidia/doca-platform/internal/argocd/api/application/v1alpha1"

	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// describeCmd represents the describe command
var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Describe DPF resources",
	Long:  "Describe the overall status of DPF resources in your cluster.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDescribe()
	},
}

func init() {
	rootCmd.AddCommand(describeCmd)

	// TODO: decide if we want to use Kubernetes cli-runtime here instead of the controller-runtime flags.
	// The cli-runtime has alot dependencies, but brings several generic flags that can be useful.
	//
	// Load the go flagset (i.e. controller-runtimes kubeconfig).
	describeCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
}

func runDescribe() error {
	_, err := newClient()
	if err != nil {
		return err
	}

	return nil
}

func newClient() (client.Client, error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}

	c, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}

	_ = operatorv1.AddToScheme(c.Scheme())
	_ = provisioningv1.AddToScheme(c.Scheme())
	_ = dpuservicev1.AddToScheme(c.Scheme())
	_ = argov1.AddToScheme(c.Scheme())

	return c, nil
}
