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

package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane/kubeconfig"
	controlplanemeta "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/controlplane/metadata"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CleanupAndWait deletes an object and waits for it to be removed before exiting.
func CleanupAndWait(ctx context.Context, c client.Client, objs ...client.Object) error {
	// Ensure each object is deleted by checking that each object returns an IsNotFound error in the api server.
	errs := []error{}
	for _, o := range objs {
		if err := c.Delete(ctx, o); err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		key := client.ObjectKeyFromObject(o)
		err := wait.ExponentialBackoff(
			wait.Backoff{
				Duration: 100 * time.Millisecond,
				Factor:   1.5,
				Steps:    15,
				Jitter:   0.4,
			},
			func() (done bool, err error) {
				if err := c.Get(ctx, key, o); err != nil {
					if apierrors.IsNotFound(err) {
						return true, nil
					}
					return false, err
				}
				return false, nil
			})
		if err != nil {
			errs = append(errs, fmt.Errorf("key %s, %s is not being deleted: %s", o.GetObjectKind().GroupVersionKind().String(), key, err))
		}
	}
	return kerrors.NewAggregate(errs)
}

// CreateResourceIfNotExist creates a resource if it doesn't exist
func CreateResourceIfNotExist(ctx context.Context, c client.Client, obj client.Object) error {
	err := c.Create(ctx, obj)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// GetFakeKamajiClusterSecretFromEnvtest creates a kamaji secret using the envtest information to simulate that we have
// a kamaji cluster. In reality, this is the same envtest Kubernetes API.
func GetFakeKamajiClusterSecretFromEnvtest(cluster controlplane.DPFCluster, cfg *rest.Config) (*corev1.Secret, error) {
	adminConfig := &kubeconfig.Type{
		Clusters: []*kubeconfig.ClusterWithName{
			{
				Name: cluster.Name,
				Cluster: kubeconfig.Cluster{
					Server:                   cfg.Host,
					CertificateAuthorityData: cfg.CAData,
				},
			},
		},
		Users: []*kubeconfig.UserWithName{
			{
				Name: "user",
				User: kubeconfig.User{
					ClientKeyData:         cfg.KeyData,
					ClientCertificateData: cfg.CertData,
				},
			},
		},
		Contexts: []*kubeconfig.NamedContext{
			{
				Name: "default",
				Context: kubeconfig.Context{
					Cluster: cluster.Name,
					User:    "user",
				},
			},
		},
		CurrentContext: "default",
	}
	confData, err := json.Marshal(adminConfig)
	if err != nil {
		return nil, err
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-admin-kubeconfig", cluster.Name),
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				controlplanemeta.DPFClusterSecretClusterNameLabelKey: cluster.Name,
				"kamaji.clastix.io/component":                        "admin-kubeconfig",
				"kamaji.clastix.io/project":                          "kamaji",
			},
		},
		Data: map[string][]byte{
			"admin.conf": confData,
		},
	}, nil
}

func GetTestLabels() map[string]string {
	return map[string]string{"some": "label", "color": "blue", "lab": "santa-clara"}
}
