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

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/frauniki/kubepark/internal/sshca"
)

// configDir is where the CLI keeps its keypair, certificate and generated
// ssh_config.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".kubepark")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// ensureKeyPair returns the paths to the CLI's ed25519 private key and
// authorized-keys public key, generating them on first use.
func ensureKeyPair() (privPath, pubPath string, err error) {
	dir, err := configDir()
	if err != nil {
		return "", "", err
	}
	privPath = filepath.Join(dir, "id_ed25519")
	pubPath = privPath + ".pub"

	if _, statErr := os.Stat(privPath); statErr == nil {
		return privPath, pubPath, nil
	}

	kp, err := sshca.GenerateKeyPair("kubepark-cli")
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(privPath, kp.PrivatePEM, 0o600); err != nil {
		return "", "", fmt.Errorf("write private key: %w", err)
	}
	if err := os.WriteFile(pubPath, kp.PublicAuthorized, 0o644); err != nil {
		return "", "", fmt.Errorf("write public key: %w", err)
	}
	return privPath, pubPath, nil
}

// certPath returns the path the signed certificate is written to.
func certPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "id_ed25519-cert.pub"), nil
}
