package v1alpha1

import (
	"github.com/nvidia/doca-platform/internal/argocd/api/application"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion                   = schema.GroupVersion{Group: application.Group, Version: "v1alpha1"}
	ApplicationSchemaGroupVersionKind    = schema.GroupVersionKind{Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationKind}
	AppProjectSchemaGroupVersionKind     = schema.GroupVersionKind{Group: application.Group, Version: "v1alpha1", Kind: application.AppProjectKind}
	ApplicationSetSchemaGroupVersionKind = schema.GroupVersionKind{Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationSetKind}
)

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// addKnownTypes adds the set of types defined in this package to the supplied scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Application{},
		&ApplicationList{},
		&AppProject{},
		&AppProjectList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
