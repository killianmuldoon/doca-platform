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

package utils

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	dpuservicev1 "github.com/nvidia/doca-platform/api/dpuservice/v1alpha1"
	operatorv1 "github.com/nvidia/doca-platform/api/operator/v1alpha1"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ChartHelper interface {
	// GetAnnotationsFromChart returns the annotations for a given chart
	GetAnnotationsFromChart(ctx context.Context, c client.Client, source dpuservicev1.ApplicationSource) (map[string]string, error)
}

type chartHelper struct{}

var _ ChartHelper = &chartHelper{}

// NewChartHelper returns a ChartHelper that is able to do operations with charts. It is able to authenticate to registries
// using secrets that conform with the ArgoCD way of providing credentials for registries.
func NewChartHelper() ChartHelper {
	return &chartHelper{}
}

// GetAnnotationsFromChart returns the annotations found in the specified chart by pulling the chart locally
func (u *chartHelper) GetAnnotationsFromChart(ctx context.Context, c client.Client, source dpuservicev1.ApplicationSource) (_ map[string]string, reterr error) {
	username, password, err := getChartPullCredentials(ctx, c, source)
	if err != nil {
		return nil, err
	}

	dir, err := os.MkdirTemp("", source.Chart)
	defer func() {
		reterr = kerrors.NewAggregate([]error{reterr, os.RemoveAll(dir)})
	}()
	if err != nil {
		return nil, fmt.Errorf("failed to create directory to pull chart: %w", err)
	}

	chartPath, err := pullChart(source, username, password, dir)
	if err != nil {
		return nil, err
	}

	chart, err := loader.LoadFile(chartPath)
	if err != nil {
		return nil, err
	}

	return chart.Metadata.Annotations, nil
}

// getChartPullCredentials returns the username and the password to pull the given chart based on Secrets that exist in the
// cluster. Empty values are returned if no secret is found for the relevant chart.
// TODO: Consider abstracting the secret we use today to be DPF specific instead of ArgoCD specific.
func getChartPullCredentials(ctx context.Context, c client.Client, source dpuservicev1.ApplicationSource) (string, string, error) {
	dpfOperatorConfigList := operatorv1.DPFOperatorConfigList{}
	if err := c.List(ctx, &dpfOperatorConfigList); err != nil {
		return "", "", fmt.Errorf("listing DPFOperatorConfigs: %w", err)
	}
	if len(dpfOperatorConfigList.Items) == 0 || len(dpfOperatorConfigList.Items) > 1 {
		return "", "", fmt.Errorf("exactly one DPFOperatorConfig must exist")
	}
	dpfOperatorConfig := &dpfOperatorConfigList.Items[0]

	secrets := &corev1.SecretList{}
	if err := c.List(ctx,
		secrets,
		client.MatchingLabels{
			"argocd.argoproj.io/secret-type": "repository",
		},
		client.InNamespace(dpfOperatorConfig.Namespace)); err != nil {
		return "", "", fmt.Errorf("listing secrets: %w", err)
	}

	dpuServiceTemplateRepoURL := source.RepoURL
	dpuServiceTemplateRepoURL, _ = strings.CutPrefix(dpuServiceTemplateRepoURL, "oci://")

	var username string
	var password string
	for _, secret := range secrets.Items {
		repositoryType, ok := secret.Data["type"]
		if !ok {
			continue
		}
		if string(repositoryType) != "helm" {
			continue
		}

		repoURL, ok := secret.Data["url"]
		if !ok {
			continue
		}
		if string(repoURL) != dpuServiceTemplateRepoURL {
			continue
		}

		usernameParsed, ok := secret.Data["username"]
		if !ok {
			continue
		}
		username = string(usernameParsed)

		passwordParsed, ok := secret.Data["password"]
		if !ok {
			continue
		}
		password = string(passwordParsed)

		break
	}

	return username, password, nil
}

// pullChart pulls a chart in a local directory and returns the path of the archive
func pullChart(source dpuservicev1.ApplicationSource, username string, password string, destDir string) (string, error) {
	repoURL := source.RepoURL
	chart := source.Chart
	if strings.HasPrefix(source.RepoURL, "oci://") {
		repoURL = ""
		chart = strings.Join([]string{source.RepoURL, source.Chart}, "/")
	}

	registryClient, err := registry.NewClient(registry.ClientOptEnableCache(true), registry.ClientOptBasicAuth(username, password))
	if err != nil {
		return "", err
	}
	pull := action.NewPullWithOpts(action.WithConfig(&action.Configuration{}), func(pull *action.Pull) {
		// TODO: Remove this one when upstream removes it. Removing this now leads to nil pointer dereference
		pull.Settings = &cli.EnvSettings{}
		pull.SetRegistryClient(registryClient)
		pull.DestDir = destDir
		pull.RepoURL = repoURL
		pull.Version = source.Version
		pull.Username = username
		pull.Password = password
	})

	_, err = pull.Run(chart)
	if err != nil {
		return "", fmt.Errorf("failed to pull chart: %w", err)
	}
	var chartArchivePath string
	err = filepath.WalkDir(destDir, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		chartArchivePath = path
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk the chart directory: %w", err)
	}

	if len(chartArchivePath) == 0 {
		return "", fmt.Errorf("no chart exists in local directory")
	}

	return chartArchivePath, nil
}
