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

package gateway_test

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
	"github.com/frauniki/kubepark/internal/agent"
	"github.com/frauniki/kubepark/internal/gateway"
	"github.com/frauniki/kubepark/internal/sshca"
)

const (
	testSandboxKey = "alice/demo"
	testTarget     = "demo.alice:2222"
)

// --- test fixtures -------------------------------------------------------

type testCA struct {
	signer gossh.Signer
	pub    []byte
}

func newCA(t *testing.T, comment string) testCA {
	t.Helper()
	kp, err := sshca.GenerateKeyPair(comment)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := sshca.ParseSigner(kp.PrivatePEM)
	if err != nil {
		t.Fatal(err)
	}
	return testCA{signer: signer, pub: kp.PublicAuthorized}
}

// userCert signs a client key and returns a gossh.Signer presenting the
// certificate (what an SSH client offers).
func userCert(t *testing.T, ca testCA, principal string) gossh.Signer {
	t.Helper()
	kp, err := sshca.GenerateKeyPair("client")
	if err != nil {
		t.Fatal(err)
	}
	clientSigner, err := sshca.ParseSigner(kp.PrivatePEM)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := sshca.ParsePublicKey(kp.PublicAuthorized)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := sshca.SignUserCert(ca.signer, pub, principal, time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	certSigner, err := gossh.NewCertSigner(cert, clientSigner)
	if err != nil {
		t.Fatal(err)
	}
	return certSigner
}

// fakeStore serves a fixed set of sandboxes and records sessions.
type fakeStore struct {
	mu        sync.Mutex
	sandboxes map[string]*kubeparkv1alpha1.Sandbox
	opened    int
	closed    int
}

func (s *fakeStore) key(ns, name string) string { return ns + "/" + name }

func (s *fakeStore) GetSandbox(_ context.Context, ns, name string) (*kubeparkv1alpha1.Sandbox, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sb, ok := s.sandboxes[s.key(ns, name)]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return sb.DeepCopy(), nil
}

func (s *fakeStore) SetDesiredRunning(_ context.Context, _ *kubeparkv1alpha1.Sandbox) error {
	return nil
}

func (s *fakeStore) CreateSession(_ context.Context, _ *kubeparkv1alpha1.SandboxSession) error {
	s.mu.Lock()
	s.opened++
	s.mu.Unlock()
	return nil
}

func (s *fakeStore) Heartbeat(_ context.Context, _, _ string) error { return nil }

func (s *fakeStore) CloseSession(_ context.Context, _, _, _ string) error {
	s.mu.Lock()
	s.closed++
	s.mu.Unlock()
	return nil
}

// fakeDialer dials a fixed address regardless of the sandbox pod IP, so the
// test's in-process agent stands in for the pod.
type fakeDialer struct{ addr string }

func (d fakeDialer) DialSandbox(ctx context.Context, _ *kubeparkv1alpha1.Sandbox) (net.Conn, error) {
	var nd net.Dialer
	return nd.DialContext(ctx, "tcp", d.addr)
}

// startAgent runs an in-process agent and returns its address.
func startAgent(t *testing.T, owner string, userCA, hostCA testCA) string {
	t.Helper()
	hostKP, err := sshca.GenerateKeyPair("host")
	if err != nil {
		t.Fatal(err)
	}
	hostPub, err := sshca.ParsePublicKey(hostKP.PublicAuthorized)
	if err != nil {
		t.Fatal(err)
	}
	hostCert, err := sshca.SignHostCert(hostCA.signer, hostPub, []string{owner}, time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	server, err := agent.NewServer(agent.Config{
		Owner:              owner,
		HostKeyPEM:         hostKP.PrivatePEM,
		HostCertAuthorized: gossh.MarshalAuthorizedKey(hostCert),
		UserCAAuthorized:   userCA.pub,
		HomeDir:            t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().String()
}

// startGateway runs an in-process gateway and returns its address.
func startGateway(t *testing.T, userCA testCA, store gateway.Store, dialer gateway.Dialer) string {
	t.Helper()
	gwHost, err := sshca.GenerateKeyPair("gw-host")
	if err != nil {
		t.Fatal(err)
	}
	server, err := gateway.NewSSHServer(gateway.SSHConfig{
		HostKeyPEM:       gwHost.PrivatePEM,
		UserCAAuthorized: userCA.pub,
		DefaultNamespace: "alice",
		WakeTimeout:      5 * time.Second,
		Store:            store,
		Dialer:           dialer,
	})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().String()
}

func sandbox(ns, name, owner string) *kubeparkv1alpha1.Sandbox {
	return &kubeparkv1alpha1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec: kubeparkv1alpha1.SandboxSpec{
			Owner:        kubeparkv1alpha1.OwnerSpec{Name: owner},
			DesiredState: kubeparkv1alpha1.DesiredStateRunning,
		},
		Status: kubeparkv1alpha1.SandboxStatus{PodIP: "10.0.0.1"},
	}
}

// dialGatewayJump connects to the gateway as principal and opens a
// direct-tcpip channel to target, returning the tunneled net.Conn.
func dialGatewayJump(t *testing.T, gwAddr string, cert gossh.Signer, target string) (net.Conn, *gossh.Client, error) {
	t.Helper()
	cfg := &gossh.ClientConfig{
		User:            "jump",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(cert)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	client, err := gossh.Dial("tcp", gwAddr, cfg)
	if err != nil {
		return nil, nil, err
	}
	conn, err := client.Dial("tcp", target)
	if err != nil {
		_ = client.Close()
		return nil, nil, err
	}
	return conn, client, nil
}

// --- tests ---------------------------------------------------------------

// TestEndToEndExec is the keystone: client -> gateway (cert auth, routing) ->
// agent (cert auth) -> exec, all in-process.
func TestEndToEndExec(t *testing.T) {
	userCA := newCA(t, "user-ca")
	hostCA := newCA(t, "host-ca")
	agentAddr := startAgent(t, "alice@example.com", userCA, hostCA)

	store := &fakeStore{sandboxes: map[string]*kubeparkv1alpha1.Sandbox{
		testSandboxKey: sandbox("alice", "demo", "alice@example.com"),
	}}
	gwAddr := startGateway(t, userCA, store, fakeDialer{addr: agentAddr})

	cert := userCert(t, userCA, "alice@example.com")
	tunnel, jump, err := dialGatewayJump(t, gwAddr, cert, testTarget)
	if err != nil {
		t.Fatalf("jump dial failed: %v", err)
	}
	defer func() { _ = jump.Close() }()

	// Second hop: SSH to the agent over the tunnel.
	agentConn, chans, reqs, err := gossh.NewClientConn(tunnel, testTarget, &gossh.ClientConfig{
		User:            "sandbox",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(cert)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatalf("agent handshake failed: %v", err)
	}
	agentClient := gossh.NewClient(agentConn, chans, reqs)
	defer func() { _ = agentClient.Close() }()

	sess, err := agentClient.NewSession()
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	defer func() { _ = sess.Close() }()
	out, err := sess.Output("echo kubepark-ok")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if got := string(out); got != "kubepark-ok\n" {
		t.Fatalf("unexpected exec output: %q", got)
	}

	if store.opened == 0 {
		t.Error("expected a session to be recorded")
	}
}

// TestGatewayRejectsWrongPrincipal proves the principal==owner check: a
// valid cert for a different user cannot open the channel.
func TestGatewayRejectsWrongPrincipal(t *testing.T) {
	userCA := newCA(t, "user-ca")
	store := &fakeStore{sandboxes: map[string]*kubeparkv1alpha1.Sandbox{
		testSandboxKey: sandbox("alice", "demo", "alice@example.com"),
	}}
	gwAddr := startGateway(t, userCA, store, fakeDialer{addr: "127.0.0.1:1"})

	cert := userCert(t, userCA, "mallory@example.com")
	_, jump, err := dialGatewayJump(t, gwAddr, cert, testTarget)
	if jump != nil {
		_ = jump.Close()
	}
	if err == nil {
		t.Fatal("expected the channel open to be rejected for a non-owner principal")
	}
}

// TestGatewayRejectsUntrustedCA proves a cert from an unknown CA cannot even
// authenticate to the gateway.
func TestGatewayRejectsUntrustedCA(t *testing.T) {
	userCA := newCA(t, "user-ca")
	otherCA := newCA(t, "rogue-ca")
	store := &fakeStore{sandboxes: map[string]*kubeparkv1alpha1.Sandbox{
		testSandboxKey: sandbox("alice", "demo", "alice@example.com"),
	}}
	gwAddr := startGateway(t, userCA, store, fakeDialer{addr: "127.0.0.1:1"})

	cert := userCert(t, otherCA, "alice@example.com")
	_, jump, err := dialGatewayJump(t, gwAddr, cert, testTarget)
	if jump != nil {
		_ = jump.Close()
	}
	if err == nil {
		t.Fatal("expected authentication to fail for a cert signed by an untrusted CA")
	}
}
