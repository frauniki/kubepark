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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SessionKind is the transport of a session.
// +kubebuilder:validation:Enum=ssh;http
type SessionKind string

const (
	SessionKindSSH  SessionKind = "ssh"
	SessionKindHTTP SessionKind = "http"
)

// SessionState is the lifecycle state of a session.
// +kubebuilder:validation:Enum=Active;Closed
type SessionState string

const (
	SessionStateActive SessionState = "Active"
	SessionStateClosed SessionState = "Closed"
)

// Session exit reasons.
const (
	ExitReasonDisconnected   = "Disconnected"
	ExitReasonStaleHeartbeat = "StaleHeartbeat"
	ExitReasonSandboxDeleted = "SandboxDeleted"
)

// SandboxSessionSpec defines the desired state of SandboxSession.
// Sessions are created by the gateway, one per authenticated connection
// (ssh) or per (sandbox, user) sliding window (http). They are the audit
// record of who reached which sandbox from where.
type SandboxSessionSpec struct {
	// SandboxName is the sandbox this session connects to (same namespace).
	// +kubebuilder:validation:MinLength=1
	SandboxName string `json:"sandboxName"`

	// User is the authenticated identity (SSH certificate principal or OIDC
	// claim).
	// +kubebuilder:validation:MinLength=1
	User string `json:"user"`

	// ClientAddr is the remote address the connection came from.
	// +optional
	ClientAddr string `json:"clientAddr,omitempty"`

	// Kind is ssh or http.
	Kind SessionKind `json:"kind"`

	// CertSerial is the serial of the SSH certificate used, for joining
	// with signing audit logs.
	// +optional
	CertSerial string `json:"certSerial,omitempty"`

	// HeartbeatInterval is stamped by the gateway at creation so the stale
	// reaper can compute its threshold without re-deriving the sandbox's
	// idle timeout.
	// +optional
	HeartbeatInterval *metav1.Duration `json:"heartbeatInterval,omitempty"`
}

// SandboxSessionStatus defines the observed state of SandboxSession.
type SandboxSessionStatus struct {
	// State is Active while the connection lives, then Closed.
	// +optional
	State SessionState `json:"state,omitempty"`

	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// LastActivityTime is refreshed by gateway heartbeats.
	// +optional
	LastActivityTime *metav1.Time `json:"lastActivityTime,omitempty"`

	// +optional
	EndTime *metav1.Time `json:"endTime,omitempty"`

	// ExitReason records why the session closed (Disconnected,
	// StaleHeartbeat, SandboxDeleted).
	// +optional
	ExitReason string `json:"exitReason,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=sbs
// +kubebuilder:printcolumn:name="Sandbox",type=string,JSONPath=`.spec.sandboxName`
// +kubebuilder:printcolumn:name="User",type=string,JSONPath=`.spec.user`
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=`.spec.kind`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SandboxSession is the short-lived audit record of one connection to a
// sandbox through the gateway.
type SandboxSession struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of SandboxSession
	// +required
	Spec SandboxSessionSpec `json:"spec"`

	// status defines the observed state of SandboxSession
	// +optional
	Status SandboxSessionStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SandboxSessionList contains a list of SandboxSession
type SandboxSessionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SandboxSession `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SandboxSession{}, &SandboxSessionList{})
}
