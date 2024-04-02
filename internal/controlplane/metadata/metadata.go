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

package metadata

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TODO: Review this package when implementing control plane provisioning.
var (
	// DPFClusterSecretLabels are the labels that identify the admin kubeconfig of a DPF cluster.
	DPFClusterSecretLabels = map[string]string{"kamaji.clastix.io/component": "admin-kubeconfig", "kamaji.clastix.io/project": "kamaji"}

	// DPFClusterSecretClusterNameLabelKey is the key of the label linking a DPFClusterSecret to the name of the cluster.
	// TODO: When implementing control plane provisioning replace this with a DPF-specific label.
	DPFClusterSecretClusterNameLabelKey = "kamaji.clastix.io/name"

	// DPFClusterLabelKey is the key of the label linking objects to a specific DPF Cluster.
	DPFClusterLabelKey = "dpf.nvidia.com/cluster"
)

var (
	TenantControlPlaneGVK = schema.GroupVersion{Group: "kamaji.clastix.io", Version: "v1alpha1"}.WithKind("TenantControlPlane")
)
