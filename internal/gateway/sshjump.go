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
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"time"

	gliderssh "github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
	"github.com/frauniki/kubepark/internal/sshca"
)

const (
	ctxKeyPrincipal  = "kubepark-principal"
	ctxKeyCertSerial = "kubepark-cert-serial"

	defaultWakeTimeout = 180 * time.Second
	wakePollInterval   = 2 * time.Second
)

// SSHConfig configures the gateway SSH jump host.
type SSHConfig struct {
	Addr string
	// HostKeyPEM is the gateway's own host key (OpenSSH PEM).
	HostKeyPEM []byte
	// UserCAAuthorized is the user CA public key that must have signed
	// client certificates.
	UserCAAuthorized []byte
	// DefaultNamespace is used when a target omits its namespace.
	DefaultNamespace string
	// WakeTimeout bounds the wake-on-connect stall (default 180s).
	WakeTimeout time.Duration
	// Now is injected for tests.
	Now func() time.Time

	Store  Store
	Dialer Dialer
}

// NewSSHServer builds the jump host: it accepts only certificate auth and
// only direct-tcpip channels (no shell or exec on the gateway itself).
func NewSSHServer(cfg SSHConfig) (*gliderssh.Server, error) {
	if cfg.Store == nil || cfg.Dialer == nil {
		return nil, fmt.Errorf("gateway requires a store and a dialer")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.WakeTimeout == 0 {
		cfg.WakeTimeout = defaultWakeTimeout
	}

	hostSigner, err := gossh.ParsePrivateKey(cfg.HostKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse gateway host key: %w", err)
	}
	userCA, err := sshca.ParsePublicKey(cfg.UserCAAuthorized)
	if err != nil {
		return nil, fmt.Errorf("parse user CA: %w", err)
	}

	h := &jumpHandler{cfg: cfg}

	srv := &gliderssh.Server{
		Addr:        cfg.Addr,
		HostSigners: []gliderssh.Signer{hostSigner},
		// Certificate auth only. The principal is stashed for the routing
		// authz check.
		PublicKeyHandler: func(ctx gliderssh.Context, key gliderssh.PublicKey) bool {
			cert, ok := key.(*gossh.Certificate)
			if !ok || len(cert.ValidPrincipals) == 0 {
				return false
			}
			principal := cert.ValidPrincipals[0]
			if err := sshca.CheckUserCert(cert, userCA, principal, cfg.Now()); err != nil {
				return false
			}
			ctx.SetValue(ctxKeyPrincipal, principal)
			ctx.SetValue(ctxKeyCertSerial, fmt.Sprintf("%d", cert.Serial))
			return true
		},
		// No shell/exec/sftp on the gateway: only direct-tcpip.
		ChannelHandlers: map[string]gliderssh.ChannelHandler{
			"direct-tcpip": h.handleDirectTCPIP,
		},
	}
	return srv, nil
}

// jumpHandler carries the config for the direct-tcpip channel handler.
type jumpHandler struct {
	cfg SSHConfig
}

// localForwardChannelData is the payload of a direct-tcpip channel open
// (RFC 4254 section 7.2).
type localForwardChannelData struct {
	DestAddr   string
	DestPort   uint32
	OriginAddr string
	OriginPort uint32
}

// handleDirectTCPIP is the whole gateway data plane: it resolves the target
// sandbox from the requested host, authorizes it against the certificate
// principal, wakes a suspended sandbox and stalls until it is ready, then
// bridges the channel to the sandbox agent. Session bookkeeping records the
// connection for audit.
func (h *jumpHandler) handleDirectTCPIP(_ *gliderssh.Server, _ *gossh.ServerConn, newChan gossh.NewChannel, ctx gliderssh.Context) {
	logger := log.FromContext(ctx)

	var payload localForwardChannelData
	if err := gossh.Unmarshal(newChan.ExtraData(), &payload); err != nil {
		_ = newChan.Reject(gossh.ConnectionFailed, "invalid direct-tcpip payload")
		return
	}

	principal, _ := ctx.Value(ctxKeyPrincipal).(string)
	target, err := ParseSSHTarget(payload.DestAddr, h.cfg.DefaultNamespace)
	if err != nil {
		_ = newChan.Reject(gossh.ConnectionFailed, err.Error())
		return
	}

	sb, err := h.authorize(ctx, target, principal)
	if err != nil {
		logger.Info("rejected ssh route", "target", payload.DestAddr, "principal", principal, "reason", err.Error())
		_ = newChan.Reject(gossh.Prohibited, "not authorized for this sandbox")
		return
	}

	// Record the session; close it when the channel ends.
	serial, _ := ctx.Value(ctxKeyCertSerial).(string)
	_, closeSession := h.openSession(ctx, sb, principal, ctx.RemoteAddr().String(), serial)
	defer closeSession(kubeparkv1alpha1.ExitReasonDisconnected)

	sb, err = h.wakeAndWait(ctx, sb)
	if err != nil {
		logger.Info("wake failed", "sandbox", sb.Name, "reason", err.Error())
		_ = newChan.Reject(gossh.ConnectionFailed, "sandbox did not become ready")
		return
	}

	upstream, err := h.cfg.Dialer.DialSandbox(ctx, sb)
	if err != nil {
		_ = newChan.Reject(gossh.ConnectionFailed, "cannot reach sandbox")
		return
	}
	defer func() { _ = upstream.Close() }()

	ch, reqs, err := newChan.Accept()
	if err != nil {
		return
	}
	go gossh.DiscardRequests(reqs)
	bridge(ch, upstream)
}

// authorize resolves the sandbox and enforces principal == owner, the whole
// SSH authorization model.
func (h *jumpHandler) authorize(ctx context.Context, target SSHTarget, principal string) (*kubeparkv1alpha1.Sandbox, error) {
	if principal == "" {
		return nil, fmt.Errorf("no principal")
	}
	sb, err := h.cfg.Store.GetSandbox(ctx, target.Namespace, target.Sandbox)
	if err != nil {
		return nil, fmt.Errorf("sandbox lookup: %w", err)
	}
	if sb.Spec.Owner.Name != principal {
		return nil, fmt.Errorf("principal %q is not the owner of sandbox %s/%s", principal, target.Namespace, target.Sandbox)
	}
	return sb, nil
}

// wakeAndWait resumes a suspended sandbox and stalls until it reports a pod
// IP, bounded by WakeTimeout.
func (h *jumpHandler) wakeAndWait(ctx context.Context, sb *kubeparkv1alpha1.Sandbox) (*kubeparkv1alpha1.Sandbox, error) {
	if sb.Status.PodIP != "" && sb.Spec.DesiredState == kubeparkv1alpha1.DesiredStateRunning {
		return sb, nil
	}
	if err := h.cfg.Store.SetDesiredRunning(ctx, sb); err != nil {
		return sb, fmt.Errorf("resume sandbox: %w", err)
	}

	deadline := h.cfg.Now().Add(h.cfg.WakeTimeout)
	for {
		fresh, err := h.cfg.Store.GetSandbox(ctx, sb.Namespace, sb.Name)
		if err == nil && fresh.Status.PodIP != "" {
			return fresh, nil
		}
		if h.cfg.Now().After(deadline) {
			return sb, fmt.Errorf("timed out waiting for sandbox %s to become ready", sb.Name)
		}
		select {
		case <-ctx.Done():
			return sb, ctx.Err()
		case <-time.After(wakePollInterval):
		}
	}
}

// openSession creates the SandboxSession audit record and returns a closer.
func (h *jumpHandler) openSession(ctx context.Context, sb *kubeparkv1alpha1.Sandbox, principal, clientAddr, certSerial string) (string, func(reason string)) {
	logger := log.FromContext(ctx)
	name := fmt.Sprintf("%s-%s", sb.Name, randomSuffix(ctx))
	session := &kubeparkv1alpha1.SandboxSession{
		ObjectMeta: metav1.ObjectMeta{Namespace: sb.Namespace, Name: name},
		Spec: kubeparkv1alpha1.SandboxSessionSpec{
			SandboxName: sb.Name,
			User:        principal,
			ClientAddr:  clientAddr,
			Kind:        kubeparkv1alpha1.SessionKindSSH,
			CertSerial:  certSerial,
		},
	}
	if err := h.cfg.Store.CreateSession(ctx, session); err != nil {
		logger.Error(err, "failed to record session")
		return "", func(string) {}
	}
	logger.Info("session opened", "sandbox", sb.Name, "user", principal, "client", clientAddr)
	closed := false
	return name, func(reason string) {
		if closed {
			return
		}
		closed = true
		// Use a detached context: the connection context is already done.
		if err := h.cfg.Store.CloseSession(context.WithoutCancel(ctx), sb.Namespace, name, reason); err != nil {
			logger.Error(err, "failed to close session")
		}
		logger.Info("session closed", "sandbox", sb.Name, "user", principal, "reason", reason)
	}
}

// randomSuffix returns a short random hex string for session names.
func randomSuffix(_ context.Context) string {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "000000000a"
	}
	return hex.EncodeToString(b[:])
}

// bridge copies bytes both ways until either side closes.
func bridge(a io.ReadWriteCloser, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(a, b); done <- struct{}{} }()
	go func() { _, _ = io.Copy(b, a); done <- struct{}{} }()
	<-done
	_ = a.Close()
	_ = b.Close()
}
