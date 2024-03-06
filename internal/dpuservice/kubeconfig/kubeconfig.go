// Package kubeconfig contains types representing the kubeconfig from upstream Kubernetes.
package kubeconfig

// Type is a minimal struct allowing us to unmarshal what we need from a kubeconfig.
type Type struct {
	Clusters []*KubectlClusterWithName `json:"clusters"`
	Users    []*KubectlUserWithName    `json:"users"`
}

// KubectlClusterWithName contains information about the cluster.
type KubectlClusterWithName struct {
	Name    string         `json:"name"`
	Cluster KubectlCluster `json:"cluster"`
}

// KubectlUserWithName contains information about the user.
type KubectlUserWithName struct {
	Name string      `json:"name"`
	User KubectlUser `json:"user"`
}

// KubectlUser contains information about the user.
type KubectlUser struct {
	ClientCertificateData []byte `json:"client-certificate-data,omitempty"`
	ClientKeyData         []byte `json:"client-key-data,omitempty"`
}

// KubectlCluster contains information about the Cluster.
type KubectlCluster struct {
	Server                   string `json:"server,omitempty"`
	CertificateAuthorityData []byte `json:"certificate-authority-data,omitempty"`
}
