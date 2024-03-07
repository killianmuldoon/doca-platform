//nolint:unused
package argocd

import (
	"fmt"

	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	argoapplication "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application"
	argocdv1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func NewAppProject(namespacedName types.NamespacedName, clusterNames []string) *argocdv1.AppProject {
	project := argocdv1.AppProject{
		TypeMeta: metav1.TypeMeta{
			Kind:       argoapplication.AppProjectKind,
			APIVersion: argoapplication.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        namespacedName.Name,
			Namespace:   namespacedName.Namespace,
			Labels:      nil,
			Annotations: nil,
		},
		Spec: argocdv1.AppProjectSpec{
			SourceRepos:                nil,
			Destinations:               nil,
			Description:                "Installing DPU Services",
			Roles:                      nil,
			ClusterResourceWhitelist:   nil,
			NamespaceResourceBlacklist: nil,
			OrphanedResources: &argocdv1.OrphanedResourcesMonitorSettings{
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
	for _, cluster := range clusterNames {
		project.Spec.Destinations = append(project.Spec.Destinations, argocdv1.ApplicationDestination{
			Server:    fmt.Sprintf("%v-argocd", cluster),
			Namespace: "*",
		})
	}
	return &project
}

func NewApplication(projectName string, namespacedName types.NamespacedName, clusterName string, service *dpuservicev1.DPUService) *argocdv1.Application {
	return &argocdv1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:        namespacedName.Name,
			Namespace:   namespacedName.Namespace,
			Labels:      nil,
			Annotations: nil,
		},
		Spec: argocdv1.ApplicationSpec{
			Source: &argocdv1.ApplicationSource{
				RepoURL:        "half",
				Path:           "big ",
				TargetRevision: "1.5",
			},
			Destination: argocdv1.ApplicationDestination{
				Server:    fmt.Sprintf("%v-argocd", clusterName),
				Namespace: "*",
			},
			Project: projectName,
		},
	}
}
