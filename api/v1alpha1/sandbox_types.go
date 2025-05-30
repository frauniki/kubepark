/*
Copyright 2025.

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
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SandboxSpec defines the desired state of Sandbox.
type SandboxSpec struct {
	// ServiceAccountName is the name of the ServiceAccount to use to run this sandbox.
	ServiceAccountName string `json:"serviceAccountName,omitempty" protobuf:"bytes,1,opt,name=serviceAccountName"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	NodeSelector map[string]string `json:"nodeSelector,omitempty" protobuf:"bytes,2,opt,name=nodeSelector"`

	Affinity *apiv1.Affinity `json:"affinity,omitempty" protobuf:"bytes,3,opt,name=affinity"`

	// Tolerations are the pod's tolerations.
	// +patchStrategy=merge
	// +patchMergeKey=key
	Tolerations []apiv1.Toleration `json:"tolerations,omitempty" patchStrategy:"merge" patchMergeKey:"key" protobuf:"bytes,4,opt,name=tolerations"`

	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images.
	// +patchStrategy=merge
	// +patchMergeKey=name
	ImagePullSecrets []apiv1.LocalObjectReference `json:"imagePullSecrets,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,5,opt,name=imagePullSecrets"`

	// HostNetwork indicates if the pod should use the host network namespace.
	HostNetwork *bool `json:"hostNetwork,omitempty" protobuf:"bytes,6,opt,name=hostNetwork"`

	// Container defines the container configuration for the sandbox.
	Container *apiv1.Container `json:"container,omitempty" protobuf:"bytes,7,opt,name=container"`

	// SandboxTemplateRef is a reference to a SandboxTemplate resource.
	SandboxTemplateRef *SandboxTemplateRef `json:"sandboxTemplateRef,omitempty" protobuf:"bytes,8,opt,name=sandboxTemplateRef"`

	// Image is the Docker image to use for the sandbox.
	Image string `json:"image,omitempty" protobuf:"bytes,9,opt,name=image"`

	// SSH defines the SSH configuration for the sandbox.
	SSH *SSHConfig `json:"ssh,omitempty" protobuf:"bytes,10,opt,name=ssh"`

	// TerminationGracePeriodSeconds is the duration in seconds the pod needs to terminate gracefully.
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,11,opt,name=terminationGracePeriodSeconds"`
}

// SSHConfig defines the SSH configuration for the sandbox.
type SSHConfig struct {
	// PublicKey is the SSH public key to use for accessing the sandbox.
	PublicKey string `json:"publicKey,omitempty" protobuf:"bytes,1,opt,name=publicKey"`
}

type SandboxTemplateRef struct {
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
}

// SandboxStatus defines the observed state of Sandbox.
type SandboxStatus struct {
	Phase SandboxPhase `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase"`

	StartedAt *metav1.Time `json:"startedAt,omitempty" protobuf:"bytes,2,opt,name=startedAt"`

	FinishedAt *metav1.Time `json:"finishedAt,omitempty" protobuf:"bytes,3,opt,name=finishedAt"`

	Message string `json:"message,omitempty" protobuf:"bytes,4,opt,name=message"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Sandbox is the Schema for the sandboxes API.
type Sandbox struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SandboxSpec   `json:"spec,omitempty"`
	Status SandboxStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SandboxList contains a list of Sandbox.
type SandboxList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Sandbox `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Sandbox{}, &SandboxList{})
}
