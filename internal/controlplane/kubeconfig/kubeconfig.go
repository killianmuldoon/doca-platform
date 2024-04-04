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

// Package kubeconfig contains types representing the kubeconfig from upstream Kubernetes.
package kubeconfig

import (
	"encoding/json"
	"fmt"
)

// Type is a minimal struct allowing us to unmarshal what we need from a kubeconfig.
type Type struct {
	Clusters       []*ClusterWithName `json:"clusters"`
	Users          []*UserWithName    `json:"users"`
	CurrentContext string             `json:"current-context"`
	Contexts       []*NamedContext    `json:"contexts"`
}

// Bytes converts Kubeconfig to bytes
func (t *Type) Bytes() ([]byte, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("error converting kubeconfig to bytes: %w", err)
	}
	return b, err
}

// ClusterWithName contains information about the cluster.
type ClusterWithName struct {
	Name    string  `json:"name"`
	Cluster Cluster `json:"cluster"`
}

// UserWithName contains information about the user.
type UserWithName struct {
	Name string `json:"name"`
	User User   `json:"user"`
}

// User contains information about the user.
type User struct {
	ClientCertificateData []byte `json:"client-certificate-data,omitempty"`
	ClientKeyData         []byte `json:"client-key-data,omitempty"`
}

// Cluster contains information about the Cluster.
type Cluster struct {
	Server                   string `json:"server,omitempty"`
	CertificateAuthorityData []byte `json:"certificate-authority-data,omitempty"`
}

// NamedContext is a struct used to create a kubectl configuration YAML file
type NamedContext struct {
	Name    string  `json:"name"`
	Context Context `json:"context"`
}

// Context is a struct used to create a kubectl configuration YAML file
type Context struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace,omitempty"`
	User      string `json:"user"`
}
