/*
Copyright 2024.

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

package controlplane

import (
	"context"
	"fmt"

	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/controlplane/metadata"
	"gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/dpuservice/kubeconfig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DPFCluster represents a single Kubernetes cluster in DPF implemented using Kamaji.
// TODO: Consider if this should be a more complex type carrying more data about the cluster.
type DPFCluster types.NamespacedName

func (d *DPFCluster) String() string {
	return fmt.Sprintf("%s-%s", d.Namespace, d.Name)
}

// GetKubeconfig returns the kubeconfig with admin permissions for the given cluster
func (d *DPFCluster) GetKubeconfig(ctx context.Context, c client.Client) (*kubeconfig.Type, error) {
	// Get the control plane secret using the naming convention - $CLUSTERNAME-admin-kubeconfig.
	secret := &corev1.Secret{}
	key := client.ObjectKey{Namespace: d.Namespace, Name: fmt.Sprintf("%v-admin-kubeconfig", d.Name)}
	if err := c.Get(ctx, key, secret); err != nil {
		return nil, fmt.Errorf("error while getting secret for cluster %s: %w", d.String(), err)
	}

	adminSecret, ok := secret.Data["admin.conf"]
	if !ok {
		return nil, fmt.Errorf("secret %v/%v not in the expected format: data.admin.conf not found", secret.Namespace, secret.Name)
	}
	var adminConfig kubeconfig.Type
	if err := yaml.Unmarshal(adminSecret, &adminConfig); err != nil {
		return nil, err
	}
	if len(adminConfig.Users) != 1 {
		return nil, fmt.Errorf("secret %v/%v not in the expected format: user list should have one member", secret.Namespace, secret.Name)
	}
	if len(adminConfig.Clusters) != 1 {
		return nil, fmt.Errorf("secret %v/%v not in the expected format: cluster list should have one member", secret.Namespace, secret.Name)
	}
	return &adminConfig, nil
}

// NewClient returns a client for the given cluster
func (d *DPFCluster) NewClient(ctx context.Context, c client.Client) (client.Client, error) {
	kubeconfig, err := d.GetKubeconfig(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("error while getting kubeconfig: %w", err)
	}
	kubeconfigBytes, err := kubeconfig.Bytes()
	if err != nil {
		return nil, fmt.Errorf("error while converting kubeconfig to bytes: %w", err)
	}
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("error while getting client config: %w", err)
	}
	dpfClusterClient, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("error while getting client: %w", err)
	}
	return dpfClusterClient, nil
}

// GetDPFClusters returns a list of DPFCluster available in the given Kubernetes cluster
func GetDPFClusters(ctx context.Context, c client.Client) ([]DPFCluster, error) {
	var errs []error
	secrets := &corev1.SecretList{}
	err := c.List(ctx, secrets, client.MatchingLabels(controlplanemeta.DPFClusterSecretLabels))
	if err != nil {
		return nil, err
	}
	clusters := []DPFCluster{}
	for _, secret := range secrets.Items {
		clusterName, found := secret.GetLabels()[controlplanemeta.DPFClusterSecretClusterNameLabelKey]
		if !found {
			errs = append(errs, fmt.Errorf("could not identify cluster name for secret %v/%v", secret.Namespace, secret.Name))
			continue
		}
		clusters = append(clusters, DPFCluster{
			Namespace: secret.Namespace,
			Name:      clusterName,
		})
	}
	return clusters, kerrors.NewAggregate(errs)
}
