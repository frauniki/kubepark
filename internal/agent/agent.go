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

// Package agent is the in-sandbox SSH server. It authenticates the single
// owner via a CA-signed user certificate, presents a CA-signed host
// certificate, and serves persistent (tmux-style) PTY sessions, exec, SFTP
// and TCP port forwarding.
package agent

import (
	"fmt"
	"os"
	"time"

	gliderssh "github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/frauniki/kubepark/internal/sshca"
)

// Config configures the agent server.
type Config struct {
	// Addr is the listen address (e.g. ":2222").
	Addr string
	// Owner is the certificate principal allowed to connect.
	Owner string
	// HostKeyPEM is the host private key (OpenSSH PEM).
	HostKeyPEM []byte
	// HostCertAuthorized is the host certificate in authorized_keys form.
	HostCertAuthorized []byte
	// UserCAAuthorized is the user CA public key (authorized_keys form)
	// used to verify client certificates. Only the public half is ever
	// present in the sandbox.
	UserCAAuthorized []byte
	// Command is the optional long-running main process (template command).
	Command []string
	// HomeDir is the SFTP/shell root.
	HomeDir string
	// Now is injected for tests; defaults to time.Now.
	Now func() time.Time
}

// ConfigFromEnv builds a Config from the mounted host-key secret and env.
func ConfigFromEnv(command []string) (Config, error) {
	dir := os.Getenv("KUBEPARK_HOST_DIR")
	if dir == "" {
		dir = "/etc/kubepark/host"
	}
	hostKey, err := os.ReadFile(dir + "/ssh_host_ed25519_key")
	if err != nil {
		return Config{}, fmt.Errorf("read host key: %w", err)
	}
	hostCert, err := os.ReadFile(dir + "/ssh_host_ed25519_key-cert.pub")
	if err != nil {
		return Config{}, fmt.Errorf("read host cert: %w", err)
	}
	userCA, err := os.ReadFile(dir + "/user-ca.pub")
	if err != nil {
		return Config{}, fmt.Errorf("read user CA: %w", err)
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = "/home/sandbox"
	}
	return Config{
		Addr:               ":2222",
		Owner:              os.Getenv("KUBEPARK_OWNER"),
		HostKeyPEM:         hostKey,
		HostCertAuthorized: hostCert,
		UserCAAuthorized:   userCA,
		Command:            command,
		HomeDir:            home,
	}, nil
}

// NewServer builds the gliderlabs SSH server for the sandbox agent.
func NewServer(cfg Config) (*gliderssh.Server, error) {
	if cfg.Owner == "" {
		return nil, fmt.Errorf("agent requires an owner principal")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	hostSigner, err := signerWithCert(cfg.HostKeyPEM, cfg.HostCertAuthorized)
	if err != nil {
		return nil, err
	}
	userCA, err := sshca.ParsePublicKey(cfg.UserCAAuthorized)
	if err != nil {
		return nil, fmt.Errorf("parse user CA: %w", err)
	}

	sessions := newSessionManager(cfg)
	// Launch the template's main workload (if any) as a supervised
	// background process, independent of SSH sessions.
	sessions.startMainProcess()

	srv := &gliderssh.Server{
		Addr:        cfg.Addr,
		Handler:     sessions.handle,
		HostSigners: []gliderssh.Signer{hostSigner},
		// Defense in depth: the gateway already verified the principal, but
		// the agent independently checks that the client presents a user
		// certificate signed by the user CA for exactly the owner.
		PublicKeyHandler: func(_ gliderssh.Context, key gliderssh.PublicKey) bool {
			cert, ok := key.(*gossh.Certificate)
			if !ok {
				return false
			}
			return sshca.CheckUserCert(cert, userCA, cfg.Owner, cfg.Now()) == nil
		},
		// Allow local (-L) and remote (-R) port forwarding through the
		// sandbox.
		LocalPortForwardingCallback:   func(gliderssh.Context, string, uint32) bool { return true },
		ReversePortForwardingCallback: func(gliderssh.Context, string, uint32) bool { return true },
	}

	forwardHandler := &gliderssh.ForwardedTCPHandler{}
	srv.ChannelHandlers = map[string]gliderssh.ChannelHandler{
		"session":      gliderssh.DefaultSessionHandler,
		"direct-tcpip": gliderssh.DirectTCPIPHandler,
	}
	srv.RequestHandlers = map[string]gliderssh.RequestHandler{
		"tcpip-forward":        forwardHandler.HandleSSHRequest,
		"cancel-tcpip-forward": forwardHandler.HandleSSHRequest,
	}
	srv.SubsystemHandlers = map[string]gliderssh.SubsystemHandler{
		"sftp": sftpHandler(cfg.HomeDir),
	}
	return srv, nil
}

// signerWithCert builds a host signer that presents a certificate so
// clients that trust the host CA (via @cert-authority) accept it without
// TOFU.
func signerWithCert(keyPEM, certAuthorized []byte) (gossh.Signer, error) {
	signer, err := gossh.ParsePrivateKey(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse host key: %w", err)
	}
	pub, _, _, _, err := gossh.ParseAuthorizedKey(certAuthorized)
	if err != nil {
		return nil, fmt.Errorf("parse host cert: %w", err)
	}
	cert, ok := pub.(*gossh.Certificate)
	if !ok {
		return nil, fmt.Errorf("host cert is not a certificate")
	}
	certSigner, err := gossh.NewCertSigner(cert, signer)
	if err != nil {
		return nil, fmt.Errorf("build host cert signer: %w", err)
	}
	return certSigner, nil
}
