//nolint:unused
package argocd

import (
	"fmt"
	"strings"

	controlplanev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/controlplane/v1alpha1"
	dpuservicev1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/api/dpuservice/v1alpha1"
	argoapplication "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application"
	argov1 "gitlab-master.nvidia.com/doca-platform-foundation/dpf-operator/internal/argocd/api/application/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
)

var (
	// TODO: (killianmuldoon) this is duplicated from the dpuService package.
	argoCDNamespace = "default"
)

func NewAppProject(name string, clusters []types.NamespacedName) *argov1.AppProject {
	project := argov1.AppProject{
		TypeMeta: metav1.TypeMeta{
			Kind:       argoapplication.AppProjectKind,
			APIVersion: fmt.Sprintf("%v/%v", argoapplication.Group, argoapplication.Version),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: argoCDNamespace,
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
			Server:    fmt.Sprintf("%s-%s", cluster.Namespace, cluster.Name),
			Namespace: "*",
		})
	}
	return &project
}

func NewApplication(projectName string, cluster types.NamespacedName, dpuService *dpuservicev1.DPUService, values *runtime.RawExtension) *argov1.Application {
	uid := string(uuid.NewUUID())
	suffix := strings.Split(uid, "-")[0]
	return &argov1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       argoapplication.ApplicationKind,
			APIVersion: fmt.Sprintf("%v/%v", argoapplication.Group, argoapplication.Version),
		},
		ObjectMeta: metav1.ObjectMeta{
			// TODO: Revisit this naming.
			Name:      fmt.Sprintf("%v-%v", dpuService.Name, suffix),
			Namespace: argoCDNamespace,
			// TODO: Consider adding labels for the Application.
			Labels: map[string]string{
				controlplanev1.DPFClusterLabelKey:        fmt.Sprintf("%s-%s", cluster.Namespace, cluster.Name),
				dpuservicev1.DPUServiceNameLabelKey:      dpuService.Name,
				dpuservicev1.DPUServiceNamespaceLabelKey: dpuService.Namespace,
			},
			Annotations: nil,
		},
		Spec: argov1.ApplicationSpec{
			Source: &argov1.ApplicationSource{
				RepoURL:        dpuService.Spec.Source.RepoURL,
				Chart:          dpuService.Spec.Source.Chart,
				TargetRevision: dpuService.Spec.Source.Version,
				Helm: &argov1.ApplicationSourceHelm{
					ReleaseName:  dpuService.Spec.Source.ReleaseName,
					ValuesObject: values,
				},
			},
			Destination: argov1.ApplicationDestination{
				Server:    fmt.Sprintf("%s-%s", cluster.Namespace, cluster.Name),
				Namespace: "*",
			},
			Project: projectName,
		},
	}
}
