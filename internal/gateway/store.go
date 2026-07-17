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

// Package gateway implements the kubepark SSH/HTTP gateway: a single,
// certificate-authenticated entry point that routes connections to sandbox
// pods.
package gateway

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

// Store is the gateway's read/write view of sandboxes and their sessions.
// It is deliberately small so the gateway stays stateless (reconstructable
// entirely from the API server).
type Store interface {
	// GetSandbox returns the sandbox by name and namespace.
	GetSandbox(ctx context.Context, namespace, name string) (*kubeparkv1alpha1.Sandbox, error)
	// SetDesiredRunning flips a suspended sandbox back to Running so the
	// operator resumes it (wake-on-connect).
	SetDesiredRunning(ctx context.Context, sb *kubeparkv1alpha1.Sandbox) error
	// CreateSession records an audit session (marking it Active) and returns
	// its name.
	CreateSession(ctx context.Context, session *kubeparkv1alpha1.SandboxSession) error
	// Heartbeat refreshes a session's last-activity time so the stale
	// reaper does not close it while the connection lives.
	Heartbeat(ctx context.Context, namespace, name string) error
	// CloseSession marks a session Closed with the given reason.
	CloseSession(ctx context.Context, namespace, name, reason string) error
}

// clientStore implements Store against a controller-runtime client.
type clientStore struct {
	c client.Client
}

// NewStore builds a Store backed by the given client (typically a cached
// client from a manager).
func NewStore(c client.Client) Store {
	return &clientStore{c: c}
}

func (s *clientStore) GetSandbox(ctx context.Context, namespace, name string) (*kubeparkv1alpha1.Sandbox, error) {
	var sb kubeparkv1alpha1.Sandbox
	if err := s.c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &sb); err != nil {
		return nil, err
	}
	return &sb, nil
}

func (s *clientStore) SetDesiredRunning(ctx context.Context, sb *kubeparkv1alpha1.Sandbox) error {
	if sb.Spec.DesiredState == kubeparkv1alpha1.DesiredStateRunning {
		return nil
	}
	patch := client.MergeFrom(sb.DeepCopy())
	sb.Spec.DesiredState = kubeparkv1alpha1.DesiredStateRunning
	return s.c.Patch(ctx, sb, patch)
}

func (s *clientStore) CreateSession(ctx context.Context, session *kubeparkv1alpha1.SandboxSession) error {
	if err := s.c.Create(ctx, session); err != nil {
		return err
	}
	now := metav1.Now()
	session.Status.State = kubeparkv1alpha1.SessionStateActive
	session.Status.StartTime = &now
	session.Status.LastActivityTime = &now
	return s.c.Status().Update(ctx, session)
}

func (s *clientStore) Heartbeat(ctx context.Context, namespace, name string) error {
	var session kubeparkv1alpha1.SandboxSession
	if err := s.c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &session); err != nil {
		return err
	}
	if session.Status.State != kubeparkv1alpha1.SessionStateActive {
		return nil
	}
	now := metav1.Now()
	session.Status.LastActivityTime = &now
	return s.c.Status().Update(ctx, &session)
}

func (s *clientStore) CloseSession(ctx context.Context, namespace, name, reason string) error {
	var session kubeparkv1alpha1.SandboxSession
	if err := s.c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &session); err != nil {
		return err
	}
	if session.Status.State == kubeparkv1alpha1.SessionStateClosed {
		return nil
	}
	now := metav1.Now()
	session.Status.State = kubeparkv1alpha1.SessionStateClosed
	session.Status.EndTime = &now
	session.Status.ExitReason = reason
	return s.c.Status().Update(ctx, &session)
}

// ErrNoRoute is returned when a connection names a sandbox that cannot be
// routed (missing, wrong owner, unreachable).
type ErrNoRoute struct{ Reason string }

func (e ErrNoRoute) Error() string { return fmt.Sprintf("no route: %s", e.Reason) }
