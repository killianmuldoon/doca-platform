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

package v1alpha1

import (
	"strings"

	sfcv1 "gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/api/servicechain/v1alpha1"
	"gitlab-master.nvidia.com/doca-platform-foundation/doca-platform-foundation/internal/conditions"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// DPUCredentialRequestFinalizer is the finalizer that will be added to the
	// DPUServiceCredentialRequest
	DPUServiceCredentialRequestFinalizer = "dpf.nvidia.com/dpuservicecredentialrequest"
)

const (
	// ConditionServiceAccountReconciled is the condition type that indicates that the
	// service account is ready.
	ConditionServiceAccountReconciled conditions.ConditionType = "ServiceAccountReconciled"
	// ConditionSecretReconciled is the condition type that indicates that the secret
	// is ready.
	ConditionSecretReconciled conditions.ConditionType = "SecretReconciled"
)

var (
	// DPUCredentialRequestConditions is the list of conditions that the DPUServiceCredentialRequest
	// can have.
	DPUCredentialRequestConditions = []conditions.ConditionType{
		conditions.TypeReady,
		ConditionServiceAccountReconciled,
		ConditionSecretReconciled,
	}
)

var _ conditions.GetSet = &DPUServiceCredentialRequest{}

// DPUServiceCredentialRequestSpec defines the desired state of DPUServiceCredentialRequest
type DPUServiceCredentialRequestSpec struct {
	// ServiceAccount defines the needed information to create the service account.
	// +required
	ServiceAccount NamespacedName `json:"serviceAccount"`

	// Duration is the duration for which the token will be valid.
	// Value must be in units accepted by Go time.ParseDuration https://golang.org/pkg/time/#ParseDuration.
	// e.g. "1h", "1m", "1s", "1ms", "1.5h", "2h45m".
	// Value duration must not be less than 10 minutes.
	// **Note:** The maximum TTL for a token is 24 hours, after which the token
	// will be rotated.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// TargetClusterName defines the target cluster where the service account will
	// be created and a token for that service account will be requested.
	// If not provided, the token will be requested for the same cluster where
	// the DPUServiceCredentialRequest object is created.
	// +optional
	TargetClusterName *string `json:"targetClusterName,omitempty"`

	// Type is the type of the secret that will be created.
	// +kubebuilder:validation:Enum=kubeconfig;
	// +required
	Type string `json:"type"`

	// Secret defines the needed information to create the secret.
	// The secret will be of the type specified in the `spec.type` field.
	// +required
	Secret NamespacedName `json:"secret"`

	// ObjectMeta defines the metadata of the secret.
	// +optional
	ObjectMeta *sfcv1.ObjectMeta `json:"metadata,omitempty"`
}

// NamespacedObjectReference contains enough information to locate the referenced Kubernetes resource object in any
// namespace.
type NamespacedName struct {
	// Name of the object.
	// +required
	Name string `json:"name"`

	// Namespace of the object, if not provided the object will be looked up in
	// the same namespace as the referring object
	// +kubebuilder:Default=default
	// +optional
	Namespace *string `json:"namespace,omitempty"`
}

// String returns the string representation of the NamespacedName.
func (n *NamespacedName) String() string {
	return n.GetNamespace() + string(types.Separator) + n.Name
}

// GetNamespace returns the namespace of the NamespacedName.
func (n *NamespacedName) GetNamespace() string {
	if n.Namespace == nil {
		return "default"
	}
	return *n.Namespace
}

// DPUServiceStatus defines the observed state of DPUServiceCredentialRequest
type DPUServiceCredentialRequestStatus struct {
	// Conditions defines current service state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ServiceAccount is the namespaced name of the ServiceAccount resource created by
	// the controller for the DPUServiceCredentialRequest.
	ServiceAccount *string `json:"serviceAccount,omitempty"`

	// TargetClusterName is the name of the cluster where the service account was created.
	// It has to be persisted in the status to be able to delete the service account
	// when the DPUServiceCredentialRequest is updated.
	// +optional
	TargetClusterName *string `json:"targetClusterName,omitempty"`

	// ExpirationTimestamp is the time when the token will expire.
	// +optional
	ExpirationTimestamp *metav1.Time `json:"expirationTimestamp,omitempty"`

	// IssuedAt is the time when the token was issued.
	// +optional
	IssuedAt *metav1.Time `json:"issuedAt,omitempty"`

	// Sercet is the namespaced name of the Secret resource created by the controller for
	// the DPUServiceCredentialRequest.
	Secret *string `json:"secret,omitempty"`
}

// GetServiceAccount returns the namespace and name of the ServiceAccount.
func (n *DPUServiceCredentialRequestStatus) GetServiceAccount() (string, string) {
	if n.ServiceAccount == nil {
		return "", ""
	}
	if split := strings.Split(*n.ServiceAccount, string(types.Separator)); len(split) > 1 {
		return split[0], split[1]
	}
	return "", ""
}

// GetSecret returns the namespace and name of the Secret.
func (n *DPUServiceCredentialRequestStatus) GetSecret() (string, string) {
	if n.Secret == nil {
		return "", ""
	}
	if split := strings.Split(*n.Secret, string(types.Separator)); len(split) > 1 {
		return split[0], split[1]
	}
	return "", ""
}

// DPUServiceCredentialRequest is the Schema for the dpuserviceCredentialRequests API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type DPUServiceCredentialRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DPUServiceCredentialRequestSpec   `json:"spec,omitempty"`
	Status DPUServiceCredentialRequestStatus `json:"status,omitempty"`
}

// GetConditions returns the conditions of the DPUServiceCredentialRequest
func (c *DPUServiceCredentialRequest) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions sets the conditions of the DPUServiceCredentialRequest
func (c *DPUServiceCredentialRequest) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// DPUServiceList contains a list of DPUServiceCredentialRequest
// +kubebuilder:object:root=true
type DPUServiceCredentialRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DPUServiceCredentialRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DPUServiceCredentialRequest{}, &DPUServiceCredentialRequestList{})
}
