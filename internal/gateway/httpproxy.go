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
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeparkv1alpha1 "github.com/frauniki/kubepark/api/v1alpha1"
)

// Authenticator resolves the authenticated OIDC identity of an HTTP request
// (via a session cookie), or returns ok=false to trigger a login redirect.
// It is an interface so the cookie/OIDC machinery can evolve independently
// of routing and authorization.
type Authenticator interface {
	// Identify returns the caller's principal and groups, or ok=false when
	// the request is unauthenticated.
	Identify(r *http.Request) (principal string, groups []string, ok bool)
	// StartLogin redirects an unauthenticated request into the OIDC flow.
	StartLogin(w http.ResponseWriter, r *http.Request)
}

// HTTPProxyConfig configures the HTTP reverse proxy.
type HTTPProxyConfig struct {
	BaseDomain string
	Store      Store
	// Auth is optional; without it, only auth:none ports are reachable.
	Auth Authenticator
	// DialAddr resolves a sandbox and its resolved numeric container port to
	// an upstream base URL; defaults to http://<podIP>:<port>.
	DialAddr func(sb *kubeparkv1alpha1.Sandbox, port int32) string
}

// HTTPProxy routes authenticated browser traffic to a sandbox's exposed
// ports by host name.
type HTTPProxy struct {
	cfg HTTPProxyConfig
}

// NewHTTPProxy builds the proxy handler.
func NewHTTPProxy(cfg HTTPProxyConfig) *HTTPProxy {
	if cfg.DialAddr == nil {
		cfg.DialAddr = func(sb *kubeparkv1alpha1.Sandbox, port int32) string {
			return fmt.Sprintf("http://%s:%d", sb.Status.PodIP, port)
		}
	}
	return &HTTPProxy{cfg: cfg}
}

func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := log.FromContext(r.Context())

	// The OIDC callback is handled before routing so it works regardless of
	// which sandbox host the browser is on.
	if ca, ok := p.cfg.Auth.(*CookieAuthenticator); ok && ca.IsCallback(r) {
		ca.Callback(w, r)
		return
	}

	target, err := ParseHTTPHost(r.Host, p.cfg.BaseDomain)
	if err != nil {
		http.Error(w, "unknown route", http.StatusNotFound)
		return
	}

	sb, err := p.cfg.Store.GetSandbox(r.Context(), target.Namespace, target.Sandbox)
	if err != nil {
		http.Error(w, "sandbox not found", http.StatusNotFound)
		return
	}
	port := findExposedPort(sb, target.Port)
	if port == nil {
		http.Error(w, "port not exposed", http.StatusNotFound)
		return
	}

	// Authorization depends on the port's auth mode.
	if port.Auth == kubeparkv1alpha1.AuthModeOIDC {
		if p.cfg.Auth == nil {
			http.Error(w, "OIDC is not configured on this gateway", http.StatusServiceUnavailable)
			return
		}
		principal, groups, ok := p.cfg.Auth.Identify(r)
		if !ok {
			p.cfg.Auth.StartLogin(w, r)
			return
		}
		if !authorizedForPort(sb, port, principal, groups) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	// Unauthenticated (auth:none) traffic must never wake a suspended
	// sandbox or create session records; return a clear 503 instead.
	if sb.Status.PodIP == "" {
		http.Error(w, "sandbox is suspended", http.StatusServiceUnavailable)
		return
	}

	upstream, err := url.Parse(p.cfg.DialAddr(sb, port.Port))
	if err != nil {
		http.Error(w, "bad upstream", http.StatusInternalServerError)
		return
	}
	logger.V(1).Info("proxying http", "sandbox", sb.Name, "port", target.Port)

	// httputil.ReverseProxy transparently supports WebSocket upgrades.
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.ServeHTTP(w, r)
}

// findExposedPort returns the exposed port with the given name.
func findExposedPort(sb *kubeparkv1alpha1.Sandbox, name string) *kubeparkv1alpha1.ExposedPort {
	for i := range sb.Spec.ExposedPorts {
		if sb.Spec.ExposedPorts[i].Name == name {
			return &sb.Spec.ExposedPorts[i]
		}
	}
	return nil
}

// authorizedForPort enforces owner-match by default, widened by the port's
// optional allowedUsers/allowedGroups. Authentication alone is never
// sufficient — the request must map to the owner or an explicit allowee.
func authorizedForPort(sb *kubeparkv1alpha1.Sandbox, port *kubeparkv1alpha1.ExposedPort, principal string, groups []string) bool {
	if principal == sb.Spec.Owner.Name {
		return true
	}
	if slices.Contains(port.AllowedUsers, principal) {
		return true
	}
	for _, g := range groups {
		if slices.Contains(port.AllowedGroups, g) {
			return true
		}
	}
	return false
}
