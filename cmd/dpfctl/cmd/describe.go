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
	"context"
	"flag"
	"fmt"

	dpuservicev1 "github.com/nvidia/doca-platform/api/dpuservice/v1alpha1"
	operatorv1 "github.com/nvidia/doca-platform/api/operator/v1alpha1"
	provisioningv1 "github.com/nvidia/doca-platform/api/provisioning/v1alpha1"
	argov1 "github.com/nvidia/doca-platform/internal/argocd/api/application/v1alpha1"
	"github.com/nvidia/doca-platform/internal/dpfctl"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type describeOptions struct {
	showOtherConditions string
	showResources       string
	expandResources     string
	grouping            bool // TODO: Currently not working as expected
	color               bool
	wrapLines           bool
}

var opts describeOptions

// describeCmd represents the describe command
var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Describe DPF resources",
	Long:  "Describe the overall status of DPF resources in your cluster.",
	Example: fmt.Sprintf(`# Show all conditions for DPUService resources
%[1]s describe --show-conditions=DPUService

# Show all resources for a specific DPU
%[1]s describe --show-resources=DPU/dpf-test-0000-08-00

# Show all conditions for DPUService and DPU resources
%[1]s describe --show-conditions=DPUService,DPU

# Expand the resources for a DPUService
%[1]s describe --expand-resources=DPUService

# Display conditions for all resources
%[1]s describe --show-conditions=all

# Wrap long lines for better readability
%[1]s describe --wrap-lines

# Run %[1]s for a different cluster
%[1]s describe --kubeconfig /path/to/your/kubeconfig
# or
KUBECONFIG=/path/to/your/kubeconfig %[1]s describe
`, rootCmd.Root().Name()),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDescribe(cmd)
	},
}

func init() {
	rootCmd.AddCommand(describeCmd)

	describeCmd.Flags().StringVar(&opts.showOtherConditions, "show-conditions", "",
		"list of comma separated kind or kind/name for which the command should show all the object's conditions (use 'all' to show conditions for everything).")

	// TODO: add also support for kind/name. Currently this is not implemented as we return early without knowing the kind name.
	describeCmd.Flags().StringVar(&opts.showResources, "show-resources", "",
		"list of comma separated kind for which the command should show all the object's resources (default value is 'all').")

	describeCmd.Flags().StringVar(&opts.expandResources, "expand-resources", "",
		"list of comma separated kind or kind/name for which the command should show all the object's child resources (default value is '').")

	describeCmd.Flags().BoolVar(&opts.grouping, "grouping", false,
		"enable grouping of objects by kind.")

	describeCmd.Flags().BoolVar(&opts.wrapLines, "wrap-lines", false,
		"wrap long lines in the output.")

	describeCmd.Flags().BoolVarP(&opts.color, "color", "c", false,
		"Enable or disable color output; if not set color is enabled by default only if using tty. The flag is overridden by the NO_COLOR env variable if set.")

	// TODO: decide if we want to use Kubernetes cli-runtime here instead of the controller-runtime flags.
	// The cli-runtime has alot dependencies, but brings several generic flags that can be useful.
	//
	// Load the go flagset (i.e. controller-runtimes kubeconfig).
	describeCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
}

func runDescribe(cmd *cobra.Command) error {
	ctx := context.Background()

	c, err := newClient()
	if err != nil {
		return err
	}

	t, err := dpfctl.TreeDiscovery(ctx, c, dpfctl.ObjectTreeOptions{
		ShowResources:       opts.showResources,
		ShowOtherConditions: opts.showOtherConditions,
		ExpandResources:     opts.expandResources,
		Grouping:            opts.grouping,
		WrapLines:           opts.wrapLines,
		Colors:              opts.color,
	})
	if err != nil {
		return err
	}

	if cmd.Flags().Changed("color") {
		color.NoColor = !opts.color
	}

	dpfctl.PrintObjectTree(t)
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
