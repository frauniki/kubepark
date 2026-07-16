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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsolationLevel selects how strongly the sandbox pod is isolated from the
// node.
// +kubebuilder:validation:Enum=standard;strong
type IsolationLevel string

const (
	// IsolationStandard uses the baseline: non-root, seccomp RuntimeDefault,
	// per-sandbox NetworkPolicy.
	IsolationStandard IsolationLevel = "standard"
	// IsolationStrong additionally runs the pod under the template's
	// RuntimeClass (e.g. gVisor or Kata).
	IsolationStrong IsolationLevel = "strong"
)

// EgressRule is a small vocabulary mapping 1:1 onto a
// NetworkPolicyEgressRule. Template egress is additive on top of the
// built-in allowances (DNS and the Kubernetes API server).
type EgressRule struct {
	// To lists the destinations this rule allows.
	// +optional
	To []networkingv1.NetworkPolicyPeer `json:"to,omitempty"`

	// Ports restricts the rule to specific ports.
	// +optional
	Ports []networkingv1.NetworkPolicyPort `json:"ports,omitempty"`
}

// SandboxTemplateSpec defines the desired state of SandboxTemplate.
// +kubebuilder:validation:XValidation:rule="self.isolationLevel != 'strong' || (has(self.runtimeClassName) && size(self.runtimeClassName) > 0)",message="isolationLevel strong requires runtimeClassName"
type SandboxTemplateSpec struct {
	// Image is the sandbox container image. Its ENTRYPOINT is not used: the
	// operator wraps Command with the kubepark agent (see Command).
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Command is the long-running main process, executed as a child of the
	// kubepark agent (which is PID 1). If empty, the agent runs alone and
	// spawns a login shell per connection.
	// +optional
	Command []string `json:"command,omitempty"`

	// Env is added to the sandbox container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources are the container resource requirements.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// IsolationLevel defaults to standard.
	// +optional
	// +kubebuilder:default=standard
	IsolationLevel IsolationLevel `json:"isolationLevel,omitempty"`

	// RuntimeClassName is required when isolationLevel is strong.
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// HomeSize is the default size of the per-sandbox home PVC.
	HomeSize resource.Quantity `json:"homeSize"`

	// StorageClassName is the default storage class for home PVCs.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// Egress is rendered into the sandbox NetworkPolicy in addition to the
	// built-in DNS and API-server allowances. Everything else is denied.
	// +optional
	Egress []EgressRule `json:"egress,omitempty"`

	// DefaultIdleTimeout applies to sandboxes that do not set idleTimeout.
	// Zero or unset disables idle suspension by default.
	// +optional
	DefaultIdleTimeout *metav1.Duration `json:"defaultIdleTimeout,omitempty"`

	// RunAsUser is the UID of the sandbox user. Defaults to 1000; root is
	// not allowed.
	// +optional
	// +kubebuilder:default=1000
	// +kubebuilder:validation:Minimum=1
	RunAsUser *int64 `json:"runAsUser,omitempty"`
}

// SandboxTemplateStatus defines the observed state of SandboxTemplate.
type SandboxTemplateStatus struct {
	// conditions represent the current state of the SandboxTemplate resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=sbt
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:printcolumn:name="Isolation",type=string,JSONPath=`.spec.isolationLevel`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SandboxTemplate is an admin-defined sandbox class. All use-case diversity
// (ops bastion, MLOps client, DB operations) is expressed here; the
// controller has no use-case-specific logic.
type SandboxTemplate struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of SandboxTemplate
	// +required
	Spec SandboxTemplateSpec `json:"spec"`

	// status defines the observed state of SandboxTemplate
	// +optional
	Status SandboxTemplateStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SandboxTemplateList contains a list of SandboxTemplate
type SandboxTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SandboxTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SandboxTemplate{}, &SandboxTemplateList{})
}
