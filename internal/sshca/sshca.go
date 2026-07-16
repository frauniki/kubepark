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

// Package sshca is the shared SSH certificate authority core. Both the
// gateway and the in-sandbox agent validate certificates through
// CheckUserCert; keeping the checks in one place is the security keystone
// of kubepark.
package sshca

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

// KeyPair holds a generated ed25519 key in SSH wire formats.
type KeyPair struct {
	// PrivatePEM is the OpenSSH PEM encoding of the private key.
	PrivatePEM []byte
	// PublicAuthorized is the authorized_keys encoding of the public key.
	PublicAuthorized []byte
}

// GenerateKeyPair creates a new ed25519 key pair.
func GenerateKeyPair(comment string) (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519 key: %w", err)
	}
	pemBlock, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("convert public key: %w", err)
	}
	return &KeyPair{
		PrivatePEM:       pemEncode(pemBlock),
		PublicAuthorized: ssh.MarshalAuthorizedKey(sshPub),
	}, nil
}

// ParseSigner parses an OpenSSH PEM private key into a signer.
func ParseSigner(privatePEM []byte) (ssh.Signer, error) {
	signer, err := ssh.ParsePrivateKey(privatePEM)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return signer, nil
}

// ParsePublicKey parses an authorized_keys-format public key.
func ParsePublicKey(authorized []byte) (ssh.PublicKey, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey(authorized)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	return pub, nil
}

// NewSerial returns a random certificate serial for audit correlation.
func NewSerial() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("generate serial: %w", err)
	}
	return binary.BigEndian.Uint64(b[:]), nil
}

// SignHostCert signs a host public key. Principals are the hostnames
// clients connect to (e.g. "<sandbox>.<namespace>").
func SignHostCert(ca ssh.Signer, hostPub ssh.PublicKey, principals []string,
	validFor time.Duration, now time.Time) (*ssh.Certificate, error) {
	serial, err := NewSerial()
	if err != nil {
		return nil, err
	}
	cert := &ssh.Certificate{
		Key:             hostPub,
		Serial:          serial,
		CertType:        ssh.HostCert,
		KeyId:           principals[0],
		ValidPrincipals: principals,
		ValidAfter:      uint64(now.Add(-time.Minute).Unix()),
		ValidBefore:     validBefore(now, validFor),
	}
	if err := cert.SignCert(rand.Reader, ca); err != nil {
		return nil, fmt.Errorf("sign host certificate: %w", err)
	}
	return cert, nil
}

// SignUserCert signs a user public key for exactly one principal (the
// sandbox owner identity). Extensions enable the standard interactive
// features (pty, port forwarding).
func SignUserCert(ca ssh.Signer, userPub ssh.PublicKey, principal string,
	validFor time.Duration, now time.Time) (*ssh.Certificate, error) {
	if principal == "" {
		return nil, errors.New("refusing to sign a certificate without a principal")
	}
	serial, err := NewSerial()
	if err != nil {
		return nil, err
	}
	cert := &ssh.Certificate{
		Key:             userPub,
		Serial:          serial,
		CertType:        ssh.UserCert,
		KeyId:           principal,
		ValidPrincipals: []string{principal},
		ValidAfter:      uint64(now.Add(-time.Minute).Unix()),
		ValidBefore:     validBefore(now, validFor),
		Permissions: ssh.Permissions{
			Extensions: map[string]string{
				"permit-pty":              "",
				"permit-port-forwarding":  "",
				"permit-agent-forwarding": "",
			},
		},
	}
	if err := cert.SignCert(rand.Reader, ca); err != nil {
		return nil, fmt.Errorf("sign user certificate: %w", err)
	}
	return cert, nil
}

// CheckUserCert validates that cert is a user certificate signed by caPub,
// currently valid, and issued for exactly the expected principal. Both the
// gateway and the agent call this; every rejection reason is explicit.
func CheckUserCert(cert *ssh.Certificate, caPub ssh.PublicKey, principal string, now time.Time) error {
	if cert == nil {
		return errors.New("no certificate presented")
	}
	if cert.CertType != ssh.UserCert {
		return fmt.Errorf("certificate is not a user certificate (type %d)", cert.CertType)
	}
	if len(cert.ValidPrincipals) == 0 {
		return errors.New("certificate has no principals")
	}
	// Explicitly pin the signing authority: CheckCert validates the
	// signature and validity window but its authority gating is only
	// consulted for host certs, so the user-cert CA must be checked here.
	if !keysEqual(cert.SignatureKey, caPub) {
		return errors.New("certificate not signed by the expected CA")
	}
	checker := &ssh.CertChecker{
		IsUserAuthority: func(auth ssh.PublicKey) bool {
			return keysEqual(auth, caPub)
		},
		Clock: func() time.Time { return now },
	}
	if err := checker.CheckCert(principal, cert); err != nil {
		return fmt.Errorf("certificate check failed: %w", err)
	}
	return nil
}

// keysEqual compares two public keys by wire encoding in constant time.
func keysEqual(a, b ssh.PublicKey) bool {
	if a == nil || b == nil {
		return false
	}
	return subtle.ConstantTimeCompare(a.Marshal(), b.Marshal()) == 1
}

func validBefore(now time.Time, validFor time.Duration) uint64 {
	if validFor <= 0 {
		return uint64(ssh.CertTimeInfinity)
	}
	return uint64(now.Add(validFor).Unix())
}

func pemEncode(block *pem.Block) []byte {
	return pem.EncodeToMemory(block)
}
