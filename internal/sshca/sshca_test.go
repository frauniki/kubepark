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

package sshca

import (
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// signerAndPub generates a CA and returns its signer plus public key.
func signerAndPub(t *testing.T) (ssh.Signer, ssh.PublicKey) {
	t.Helper()
	kp, err := GenerateKeyPair("ca")
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	signer, err := ParseSigner(kp.PrivatePEM)
	if err != nil {
		t.Fatalf("ParseSigner: %v", err)
	}
	pub, err := ParsePublicKey(kp.PublicAuthorized)
	if err != nil {
		t.Fatalf("ParsePublicKey: %v", err)
	}
	return signer, pub
}

func userPub(t *testing.T) ssh.PublicKey {
	t.Helper()
	kp, err := GenerateKeyPair("user")
	if err != nil {
		t.Fatalf("GenerateKeyPair user: %v", err)
	}
	pub, err := ParsePublicKey(kp.PublicAuthorized)
	if err != nil {
		t.Fatalf("ParsePublicKey user: %v", err)
	}
	return pub
}

func TestCheckUserCert_ValidCert(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	caSigner, caPub := signerAndPub(t)
	cert, err := SignUserCert(caSigner, userPub(t), "alice@example.com", time.Hour, now)
	if err != nil {
		t.Fatalf("SignUserCert: %v", err)
	}
	if err := CheckUserCert(cert, caPub, "alice@example.com", now.Add(time.Minute)); err != nil {
		t.Fatalf("expected valid cert to pass, got: %v", err)
	}
}

func TestCheckUserCert_WrongPrincipal(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	caSigner, caPub := signerAndPub(t)
	cert, _ := SignUserCert(caSigner, userPub(t), "alice@example.com", time.Hour, now)
	if err := CheckUserCert(cert, caPub, "bob@example.com", now.Add(time.Minute)); err == nil {
		t.Fatal("expected principal mismatch to be rejected")
	}
}

func TestCheckUserCert_WrongCA(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	caSigner, _ := signerAndPub(t)
	_, otherPub := signerAndPub(t)
	cert, _ := SignUserCert(caSigner, userPub(t), "alice@example.com", time.Hour, now)
	if err := CheckUserCert(cert, otherPub, "alice@example.com", now.Add(time.Minute)); err == nil {
		t.Fatal("expected cert signed by a different CA to be rejected")
	}
}

func TestCheckUserCert_Expired(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	caSigner, caPub := signerAndPub(t)
	cert, _ := SignUserCert(caSigner, userPub(t), "alice@example.com", time.Hour, now)
	if err := CheckUserCert(cert, caPub, "alice@example.com", now.Add(2*time.Hour)); err == nil {
		t.Fatal("expected expired cert to be rejected")
	}
}

func TestCheckUserCert_HostCertRejected(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	caSigner, caPub := signerAndPub(t)
	// A host certificate must never authenticate a user.
	hostCert, err := SignHostCert(caSigner, userPub(t), []string{"alice@example.com"}, time.Hour, now)
	if err != nil {
		t.Fatalf("SignHostCert: %v", err)
	}
	if err := CheckUserCert(hostCert, caPub, "alice@example.com", now.Add(time.Minute)); err == nil {
		t.Fatal("expected host cert to be rejected as a user cert")
	}
}

func TestSignUserCert_EmptyPrincipal(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	caSigner, _ := signerAndPub(t)
	if _, err := SignUserCert(caSigner, userPub(t), "", time.Hour, now); err == nil {
		t.Fatal("expected signing with an empty principal to fail")
	}
}

func TestCheckUserCert_Nil(t *testing.T) {
	_, caPub := signerAndPub(t)
	if err := CheckUserCert(nil, caPub, "alice@example.com", time.Now()); err == nil {
		t.Fatal("expected nil cert to be rejected")
	}
}
