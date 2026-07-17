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

package controller

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/frauniki/kubepark/internal/sshca"
)

const (
	// CASecretName is the CA key material Secret in the operator namespace.
	// It holds BOTH private keys; it must never be mounted into sandbox
	// pods. Sandboxes only ever receive the public halves.
	CASecretName = "kubepark-ca"

	// CA secret keys.
	KeyUserCAPrivate = "user-ca.key"
	KeyUserCAPublic  = "user-ca.pub"
	KeyHostCAPrivate = "host-ca.key"
	KeyHostCAPublic  = "host-ca.pub"
	// KeyCookieHMAC signs browser session cookies; kept with the CA so it is
	// stable across gateway replicas and restarts.
	KeyCookieHMAC = "cookie-hmac"
)

// OperatorNamespace returns the namespace the operator (and gateway) run
// in, from the downward-API POD_NAMESPACE env, defaulting for dev runs.
func OperatorNamespace() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}
	return "kubepark-system"
}

// EnsureCASecret returns the CA secret, generating user and host CAs on
// first use so the install has zero manual bootstrap steps.
func EnsureCASecret(ctx context.Context, c client.Client, namespace string) (*corev1.Secret, error) {
	var secret corev1.Secret
	err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: CASecretName}, &secret)
	if err == nil {
		return &secret, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("get CA secret: %w", err)
	}

	userCA, err := sshca.GenerateKeyPair("kubepark-user-ca")
	if err != nil {
		return nil, err
	}
	hostCA, err := sshca.GenerateKeyPair("kubepark-host-ca")
	if err != nil {
		return nil, err
	}
	hmacKey := make([]byte, 32)
	if _, err := rand.Read(hmacKey); err != nil {
		return nil, fmt.Errorf("generate cookie HMAC key: %w", err)
	}
	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CASecretName,
			Namespace: namespace,
			Labels:    map[string]string{"app.kubernetes.io/managed-by": "kubepark"},
		},
		Data: map[string][]byte{
			KeyUserCAPrivate: userCA.PrivatePEM,
			KeyUserCAPublic:  userCA.PublicAuthorized,
			KeyHostCAPrivate: hostCA.PrivatePEM,
			KeyHostCAPublic:  hostCA.PublicAuthorized,
			KeyCookieHMAC:    hmacKey,
		},
	}
	if err := c.Create(ctx, &secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Lost a create race with another reconcile; fetch the winner.
			var existing corev1.Secret
			if getErr := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: CASecretName}, &existing); getErr == nil {
				return &existing, nil
			}
		}
		return nil, fmt.Errorf("create CA secret: %w", err)
	}
	return &secret, nil
}
