/*
Copyright 2026.

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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NamespacedGrant grants RBAC rules in an explicit list of namespaces.
// v1 deliberately supports namespaced grants only (no cluster-wide rules).
type NamespacedGrant struct {
	// Namespaces is the explicit list of namespaces the rules apply in.
	// Wildcards are not supported.
	// +kubebuilder:validation:MinItems=1
	Namespaces []string `json:"namespaces"`

	// Rules are standard RBAC policy rules, applied verbatim as a Role in
	// each listed namespace.
	// +kubebuilder:validation:MinItems=1
	Rules []rbacv1.PolicyRule `json:"rules"`
}

// AccessProfileSpec defines the desired state of AccessProfile.
//
// AccessProfiles are the trust boundary of kubepark: whoever can create or
// modify them controls what sandboxes may do in the cluster. Their creation
// must be restricted to administrators.
type AccessProfileSpec struct {
	// Grants are the permissions this profile bestows on a sandbox's
	// ServiceAccount.
	// +kubebuilder:validation:MinItems=1
	Grants []NamespacedGrant `json:"grants"`

	// AllowedNamespaces is the explicit list of namespaces whose Sandboxes
	// may reference this profile. A Sandbox in any other namespace is
	// refused (RBACReady=False, reason ProfileNotPermitted). Empty means no
	// namespace may use the profile — referencing, not creation, is the
	// escalation surface, so the default is deny.
	// +optional
	AllowedNamespaces []string `json:"allowedNamespaces,omitempty"`
}

// Condition types and reasons for AccessProfile.
const (
	ConditionValid = "Valid"

	ReasonValid            = "Valid"
	ReasonMissingNamespace = "MissingNamespace"
)

// AccessProfileStatus defines the observed state of AccessProfile.
type AccessProfileStatus struct {
	// conditions represent the current state of the AccessProfile resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=ap
// +kubebuilder:printcolumn:name="Valid",type=string,JSONPath=`.status.conditions[?(@.type=="Valid")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AccessProfile declares which Kubernetes operations a sandbox may perform.
// The controller translates it into a per-namespace Role plus a per-sandbox
// RoleBinding for the sandbox's ServiceAccount.
type AccessProfile struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of AccessProfile
	// +required
	Spec AccessProfileSpec `json:"spec"`

	// status defines the observed state of AccessProfile
	// +optional
	Status AccessProfileStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AccessProfileList contains a list of AccessProfile
type AccessProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AccessProfile `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AccessProfile{}, &AccessProfileList{})
}
