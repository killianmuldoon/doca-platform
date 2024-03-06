//nolint:unused
package argocd

import (
	"fmt"

	argoapplication "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func NewAppProject(namespacedName types.NamespacedName, clusters []types.NamespacedName) *argov1.AppProject {
	project := argov1.AppProject{
		TypeMeta: metav1.TypeMeta{
			Kind:       argoapplication.AppProjectKind,
			APIVersion: fmt.Sprintf("%v/%v", argoapplication.Group, argoapplication.Version),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespacedName.Name,
			Namespace: namespacedName.Namespace,
			// TODO: Consider a common set of labels for all objects.
			Labels:          nil,
			Annotations:     nil,
			OwnerReferences: nil,
		},
		Spec: argov1.AppProjectSpec{
			SourceRepos:                nil,
			Destinations:               nil,
			Description:                "Installing DPU Services",
			Roles:                      nil,
			ClusterResourceWhitelist:   nil,
			NamespaceResourceBlacklist: nil,
			OrphanedResources: &argov1.OrphanedResourcesMonitorSettings{
				Warn:   nil,
				Ignore: nil,
			},
			SyncWindows:                     nil,
			NamespaceResourceWhitelist:      nil,
			SignatureKeys:                   nil,
			ClusterResourceBlacklist:        nil,
			SourceNamespaces:                nil,
			PermitOnlyProjectScopedClusters: false,
		},
	}
	for _, cluster := range clusters {
		project.Spec.Destinations = append(project.Spec.Destinations, argov1.ApplicationDestination{
			Server:    cluster.Name,
			Namespace: "*",
		})
	}
	return &project
}
