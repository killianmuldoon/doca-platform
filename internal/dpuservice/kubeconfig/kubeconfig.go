// Package kubeconfig contains types representing the kubeconfig from upstream Kubernetes.
package kubeconfig

// Type is a minimal struct allowing us to unmarshal what we need from a kubeconfig.
type Type struct {
	Clusters       []*ClusterWithName `json:"clusters"`
	Users          []*UserWithName    `json:"users"`
	CurrentContext string             `json:"current-context"`
	Contexts       []NamedContext     `json:"contexts"`
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
	Name    string  `yaml:"name"`
	Context Context `yaml:"context"`
}

// Context is a struct used to create a kubectl configuration YAML file
type Context struct {
	Cluster   string `yaml:"cluster"`
	Namespace string `yaml:"namespace,omitempty"`
	User      string `yaml:"user"`
}
