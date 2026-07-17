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
	"fmt"
	"net"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
	"github.com/frauniki/kubepark/internal/controller/podspec"
)

// Dialer opens a TCP connection to a sandbox's agent. The interface is the
// seam a future reverse-tunnel transport slots into without touching auth
// or routing; v1 dials the pod IP directly (the gateway runs in-cluster).
type Dialer interface {
	DialSandbox(ctx context.Context, sb *kubeparkv1alpha1.Sandbox) (net.Conn, error)
}

// directDialer connects straight to status.podIP:2222.
type directDialer struct {
	dial func(ctx context.Context, network, addr string) (net.Conn, error)
}

// NewDirectDialer returns a Dialer that dials pod IPs directly.
func NewDirectDialer() Dialer {
	var d net.Dialer
	return &directDialer{dial: d.DialContext}
}

func (d *directDialer) DialSandbox(ctx context.Context, sb *kubeparkv1alpha1.Sandbox) (net.Conn, error) {
	if sb.Status.PodIP == "" {
		return nil, ErrNoRoute{Reason: "sandbox has no pod IP yet"}
	}
	addr := net.JoinHostPort(sb.Status.PodIP, fmt.Sprintf("%d", podspec.AgentPort))
	conn, err := d.dial(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial sandbox agent %s: %w", addr, err)
	}
	return conn, nil
}
