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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// defaultPrincipalClaim is the ID-token claim used for the certificate
// principal when none is configured.
const defaultPrincipalClaim = "email"

// OIDCConfig configures the OIDC verification the sign endpoint performs.
type OIDCConfig struct {
	Issuer   string
	ClientID string
	// PrincipalClaim is the ID-token claim mapped to the certificate
	// principal (default "email").
	PrincipalClaim string
	// BaseDomain is advertised to clients for HTTP routing (M5).
	BaseDomain string
	// GatewaySSHAddr is advertised to clients as the jump host.
	GatewaySSHAddr string
}

// SignServer serves /v1/config and /v1/sign: the CLI fetches the OIDC
// configuration, runs the login flow itself, then posts its public key and
// ID token here to receive a short-lived certificate.
type SignServer struct {
	signer   *Signer
	oidc     OIDCConfig
	verifier *oidc.IDTokenVerifier
}

// NewSignServer builds the sign server, discovering the OIDC provider.
func NewSignServer(ctx context.Context, signer *Signer, cfg OIDCConfig) (*SignServer, error) {
	if cfg.PrincipalClaim == "" {
		cfg.PrincipalClaim = defaultPrincipalClaim
	}
	s := &SignServer{signer: signer, oidc: cfg}
	if cfg.Issuer != "" {
		provider, err := oidc.NewProvider(ctx, cfg.Issuer)
		if err != nil {
			return nil, fmt.Errorf("discover OIDC provider: %w", err)
		}
		s.verifier = provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	}
	return s, nil
}

// Handler returns the HTTP handler for the sign endpoints.
func (s *SignServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/config", s.handleConfig)
	mux.HandleFunc("/v1/sign", s.handleSign)
	return mux
}

type configResponse struct {
	Issuer         string `json:"issuer"`
	ClientID       string `json:"clientId"`
	PrincipalClaim string `json:"principalClaim"`
	BaseDomain     string `json:"baseDomain,omitempty"`
	GatewaySSHAddr string `json:"gatewaySshAddr,omitempty"`
}

func (s *SignServer) handleConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, configResponse{
		Issuer:         s.oidc.Issuer,
		ClientID:       s.oidc.ClientID,
		PrincipalClaim: s.oidc.PrincipalClaim,
		BaseDomain:     s.oidc.BaseDomain,
		GatewaySSHAddr: s.oidc.GatewaySSHAddr,
	})
}

type signRequest struct {
	PublicKey string `json:"publicKey"`
	IDToken   string `json:"idToken"`
}

type signResponse struct {
	Certificate string `json:"certificate"`
	Principal   string `json:"principal"`
}

func (s *SignServer) handleSign(w http.ResponseWriter, r *http.Request) {
	logger := log.FromContext(r.Context())
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.verifier == nil {
		http.Error(w, "OIDC is not configured on this gateway", http.StatusServiceUnavailable)
		return
	}

	var req signRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Verify the ID token (issuer, audience, signature, expiry).
	idToken, err := s.verifier.Verify(r.Context(), req.IDToken)
	if err != nil {
		logger.Info("rejected sign request", "reason", err.Error())
		http.Error(w, "invalid ID token", http.StatusUnauthorized)
		return
	}
	principal, err := principalFromClaims(idToken, s.oidc.PrincipalClaim)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cert, err := s.signer.Sign([]byte(req.PublicKey), principal)
	if err != nil {
		http.Error(w, "signing failed", http.StatusInternalServerError)
		return
	}
	logger.Info("signed certificate", "principal", principal, "remote", r.RemoteAddr)
	writeJSON(w, http.StatusOK, signResponse{Certificate: string(cert), Principal: principal})
}

// principalFromClaims extracts the configured claim as the principal.
func principalFromClaims(idToken *oidc.IDToken, claim string) (string, error) {
	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return "", fmt.Errorf("parse claims: %w", err)
	}
	value, ok := claims[claim].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("claim %q missing or not a string", claim)
	}
	return value, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
