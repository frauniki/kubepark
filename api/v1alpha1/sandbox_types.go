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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DesiredState is the state the owner wants the sandbox to be in.
// +kubebuilder:validation:Enum=Running;Stopped
type DesiredState string

const (
	// DesiredStateRunning requests a schedulable, connectable sandbox pod.
	DesiredStateRunning DesiredState = "Running"
	// DesiredStateStopped requests suspension: the pod is deleted while the
	// home volume, permissions and host key are kept for a fast resume.
	DesiredStateStopped DesiredState = "Stopped"
)

// AuthMode selects how an exposed HTTP port is authenticated at the gateway.
// +kubebuilder:validation:Enum=oidc;none
type AuthMode string

const (
	// AuthModeOIDC requires an OIDC login at the gateway; only the sandbox
	// owner (and any allowedUsers/allowedGroups) may access the port.
	AuthModeOIDC AuthMode = "oidc"
	// AuthModeNone proxies without authentication. Unauthenticated traffic
	// never wakes a suspended sandbox and never creates SandboxSessions.
	AuthModeNone AuthMode = "none"
)

// RetainPolicy controls what happens to the home volume when the Sandbox is
// deleted.
// +kubebuilder:validation:Enum=Retain;Delete
type RetainPolicy string

const (
	// RetainPolicyRetain keeps the PVC after Sandbox deletion (default). The
	// controller strips its owner linkage and labels it as an orphaned home.
	RetainPolicyRetain RetainPolicy = "Retain"
	// RetainPolicyDelete deletes the PVC together with the Sandbox. Only
	// valid for PVCs created by kubepark (not with home.existingClaim).
	RetainPolicyDelete RetainPolicy = "Delete"
)

// OwnerSpec identifies the human owner of the sandbox. The name is the OIDC
// claim value (default: email) that must appear as the principal of the SSH
// certificate presented at the gateway.
type OwnerSpec struct {
	// Name is the OIDC identity (certificate principal) of the owner.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Groups optionally records the owner's OIDC groups.
	// +optional
	Groups []string `json:"groups,omitempty"`
}

// ExposedPort declares an HTTP port on the sandbox that the gateway routes
// to via host-based routing (<port>--<sandbox>--<namespace>.<baseDomain>).
type ExposedPort struct {
	// Name is the routing key; it becomes the first label segment of the
	// hostname. Must be a DNS label without consecutive hyphens so the
	// hostname parse stays unambiguous.
	// +kubebuilder:validation:MaxLength=15
	// +kubebuilder:validation:Pattern=`^[a-z]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="!self.contains('--')",message="port name must not contain '--'"
	Name string `json:"name"`

	// Port is the container port to proxy to.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Auth selects gateway authentication for this port.
	Auth AuthMode `json:"auth"`

	// AllowedUsers optionally grants access to OIDC identities besides the
	// owner. Only meaningful with auth: oidc.
	// +optional
	AllowedUsers []string `json:"allowedUsers,omitempty"`

	// AllowedGroups optionally grants access to OIDC groups besides the
	// owner. Only meaningful with auth: oidc.
	// +optional
	AllowedGroups []string `json:"allowedGroups,omitempty"`
}

// HomeSpec configures the sandbox home volume.
// +kubebuilder:validation:XValidation:rule="!(has(self.existingClaim) && size(self.existingClaim) > 0 && has(self.retainPolicy) && self.retainPolicy == 'Delete')",message="retainPolicy Delete cannot be combined with existingClaim: kubepark never deletes PVCs it did not create"
type HomeSpec struct {
	// Size overrides the template's homeSize for the created PVC.
	// +optional
	Size *resource.Quantity `json:"size,omitempty"`

	// StorageClassName overrides the template's storage class.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// ExistingClaim mounts an existing PVC as the home instead of creating
	// one. The claim must not be in use by another non-suspended Sandbox.
	// +optional
	ExistingClaim string `json:"existingClaim,omitempty"`

	// RetainPolicy controls the PVC's fate on Sandbox deletion.
	// Defaults to Retain.
	// +optional
	// +kubebuilder:default=Retain
	RetainPolicy RetainPolicy `json:"retainPolicy,omitempty"`
}

// SandboxSpec defines the desired state of Sandbox.
type SandboxSpec struct {
	// Template names the cluster-scoped SandboxTemplate this sandbox is
	// built from.
	// +kubebuilder:validation:MinLength=1
	Template string `json:"template"`

	// AccessProfile optionally names a cluster-scoped AccessProfile whose
	// grants are translated into RBAC for this sandbox's ServiceAccount.
	// Empty means the sandbox gets no Kubernetes API credentials.
	// +optional
	AccessProfile string `json:"accessProfile,omitempty"`

	// Owner is the identity allowed to connect to this sandbox.
	Owner OwnerSpec `json:"owner"`

	// DesiredState is Running (default) or Stopped (suspended: pod deleted,
	// home and permissions kept).
	// +optional
	// +kubebuilder:default=Running
	DesiredState DesiredState `json:"desiredState,omitempty"`

	// IdleTimeout suspends the sandbox after this duration without active
	// sessions. Unset inherits the template default; 0 disables idle
	// suspension.
	// +optional
	IdleTimeout *metav1.Duration `json:"idleTimeout,omitempty"`

	// ExposedPorts are HTTP ports routed by the gateway.
	// +optional
	// +listType=map
	// +listMapKey=name
	ExposedPorts []ExposedPort `json:"exposedPorts,omitempty"`

	// Home configures the home volume.
	// +optional
	Home *HomeSpec `json:"home,omitempty"`
}

// SandboxPhase is a coarse, derived summary of the sandbox state. The
// conditions are the source of truth.
// +kubebuilder:validation:Enum=Pending;Provisioning;Running;Suspending;Suspended;Resuming;Failed;Terminating
type SandboxPhase string

const (
	SandboxPhasePending      SandboxPhase = "Pending"
	SandboxPhaseProvisioning SandboxPhase = "Provisioning"
	SandboxPhaseRunning      SandboxPhase = "Running"
	SandboxPhaseSuspending   SandboxPhase = "Suspending"
	SandboxPhaseSuspended    SandboxPhase = "Suspended"
	SandboxPhaseResuming     SandboxPhase = "Resuming"
	SandboxPhaseFailed       SandboxPhase = "Failed"
	SandboxPhaseTerminating  SandboxPhase = "Terminating"
)

// Condition types surfaced on Sandbox.status.conditions.
const (
	ConditionReady            = "Ready"
	ConditionPodReady         = "PodReady"
	ConditionHomeReady        = "HomeReady"
	ConditionRBACReady        = "RBACReady"
	ConditionTemplateOutdated = "TemplateOutdated"
)

// Condition reasons.
const (
	ReasonInvalidRef          = "InvalidRef"
	ReasonClaimInUse          = "ClaimInUse"
	ReasonProfileNotPermitted = "ProfileNotPermitted"
	ReasonProfileDeleted      = "ProfileDeleted"
	ReasonProvisioning        = "Provisioning"
	ReasonSuspended           = "Suspended"
	ReasonRunning             = "Running"
	ReasonUpToDate            = "UpToDate"
	ReasonOutdated            = "Outdated"
)

// SandboxStatus defines the observed state of Sandbox.
type SandboxStatus struct {
	// Phase is a derived one-word summary; conditions are authoritative.
	// +optional
	Phase SandboxPhase `json:"phase,omitempty"`

	// conditions represent the current state of the Sandbox resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// PodName is the current executor pod, if any.
	// +optional
	PodName string `json:"podName,omitempty"`

	// PVCName is the home volume claim in use.
	// +optional
	PVCName string `json:"pvcName,omitempty"`

	// ServiceAccountName is the per-sandbox SA carrying AccessProfile
	// grants.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// PodIP is the routing target for the gateway. Cleared while the
	// sandbox is suspending or suspended.
	// +optional
	PodIP string `json:"podIP,omitempty"`

	// TemplateHash pins the hash of the template spec the current pod was
	// built from. Template changes never restart a running pod; they apply
	// on the next resume.
	// +optional
	TemplateHash string `json:"templateHash,omitempty"`

	// LastActivityTime is initialized when the sandbox becomes Running and
	// updated when sessions close. It drives idle suspension.
	// +optional
	LastActivityTime *metav1.Time `json:"lastActivityTime,omitempty"`

	// ActiveSessions is display-only; the suspend decision is always
	// computed from the live SandboxSession list.
	// +optional
	ActiveSessions int32 `json:"activeSessions,omitempty"`

	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=sb
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Desired",type=string,JSONPath=`.spec.desiredState`
// +kubebuilder:printcolumn:name="Owner",type=string,JSONPath=`.spec.owner.name`
// +kubebuilder:printcolumn:name="Template",type=string,JSONPath=`.spec.template`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="!self.metadata.name.contains('--')",message="sandbox name must not contain '--' (reserved as the gateway hostname separator)"
// +kubebuilder:validation:XValidation:rule="self.metadata.name.size() <= 30",message="sandbox name must be at most 30 characters so gateway hostnames fit in a DNS label"

// Sandbox is a persistent, declarative workspace. Its pod is a disposable
// executor: the home volume, permissions and gateway route outlive it.
type Sandbox struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Sandbox
	// +required
	Spec SandboxSpec `json:"spec"`

	// status defines the observed state of Sandbox
	// +optional
	Status SandboxStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SandboxList contains a list of Sandbox
type SandboxList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Sandbox `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Sandbox{}, &SandboxList{})
}
