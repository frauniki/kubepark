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

package gateway

import (
	"fmt"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/frauniki/kubepark/internal/sshca"
)

// Signer mints short-lived user certificates from the user CA private key.
// It is the only component that holds the private key; it is isolated in
// its own type so a future move to a separate deployment is mechanical.
type Signer struct {
	ca  gossh.Signer
	ttl time.Duration
	now func() time.Time
}

// NewSigner builds a Signer from the user CA private key (OpenSSH PEM).
func NewSigner(userCAPrivPEM []byte, ttl time.Duration) (*Signer, error) {
	ca, err := sshca.ParseSigner(userCAPrivPEM)
	if err != nil {
		return nil, fmt.Errorf("parse user CA: %w", err)
	}
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &Signer{ca: ca, ttl: ttl, now: time.Now}, nil
}

// Sign issues a certificate for the given public key and principal. The
// caller is responsible for having authenticated the principal (offline
// admin signing, or a verified OIDC identity in M4).
func (s *Signer) Sign(pubAuthorized []byte, principal string) ([]byte, error) {
	pub, err := sshca.ParsePublicKey(pubAuthorized)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	cert, err := sshca.SignUserCert(s.ca, pub, principal, s.ttl, s.now())
	if err != nil {
		return nil, err
	}
	return gossh.MarshalAuthorizedKey(cert), nil
}
