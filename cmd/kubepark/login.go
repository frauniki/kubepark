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
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// newLoginCommand runs the OIDC auth-code + PKCE flow against the IdP, then
// exchanges the ID token at the gateway for a short-lived SSH certificate.
func newLoginCommand() *cobra.Command {
	var gatewayURL string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Obtain a short-lived SSH certificate via OIDC",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if gatewayURL == "" {
				return fmt.Errorf("--gateway-url is required (e.g. https://gateway.example.com:8080)")
			}
			return runLogin(cmd.Context(), gatewayURL)
		},
	}
	cmd.Flags().StringVar(&gatewayURL, "gateway-url",
		envOr("KUBEPARK_GATEWAY_URL", ""), "Base URL of the gateway sign endpoint.")
	return cmd
}

type gatewayConfig struct {
	Issuer         string `json:"issuer"`
	ClientID       string `json:"clientId"`
	PrincipalClaim string `json:"principalClaim"`
}

func runLogin(ctx context.Context, gatewayURL string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, err := fetchGatewayConfig(ctx, gatewayURL)
	if err != nil {
		return err
	}
	if cfg.Issuer == "" {
		return fmt.Errorf("gateway has no OIDC issuer configured")
	}

	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return fmt.Errorf("discover OIDC provider: %w", err)
	}

	idToken, err := oidcAuthCodeFlow(ctx, provider, cfg.ClientID)
	if err != nil {
		return err
	}

	// Ensure a keypair, then exchange the ID token for a certificate.
	_, pubPath, err := ensureKeyPair()
	if err != nil {
		return err
	}
	pub, err := os.ReadFile(pubPath)
	if err != nil {
		return err
	}
	cert, principal, err := requestCert(ctx, gatewayURL, string(pub), idToken)
	if err != nil {
		return err
	}

	out, err := certPath()
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, []byte(cert), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "logged in as %q; certificate written to %s\n", principal, out)
	return nil
}

func fetchGatewayConfig(ctx context.Context, gatewayURL string) (gatewayConfig, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gatewayURL+"/v1/config", nil)
	if err != nil {
		return gatewayConfig{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return gatewayConfig{}, fmt.Errorf("fetch gateway config: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var cfg gatewayConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return gatewayConfig{}, err
	}
	return cfg, nil
}

// oidcAuthCodeFlow runs a localhost auth-code + PKCE flow and returns the raw
// ID token.
func oidcAuthCodeFlow(ctx context.Context, provider *oidc.Provider, clientID string) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer func() { _ = listener.Close() }()
	redirectURL := fmt.Sprintf("http://%s/callback", listener.Addr().String())

	oauthCfg := oauth2.Config{
		ClientID:    clientID,
		Endpoint:    provider.Endpoint(),
		RedirectURL: redirectURL,
		Scopes:      []string{oidc.ScopeOpenID, "email", "profile"},
	}
	verifier := oauth2.GenerateVerifier()
	state := randomString()

	authURL := oauthCfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
	fmt.Fprintf(os.Stderr, "Open this URL to log in:\n\n    %s\n\n", authURL)

	type result struct {
		token string
		err   error
	}
	resultCh := make(chan result, 1)
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			resultCh <- result{err: fmt.Errorf("state mismatch")}
			return
		}
		oauth2Token, exErr := oauthCfg.Exchange(r.Context(), r.URL.Query().Get("code"),
			oauth2.VerifierOption(verifier))
		if exErr != nil {
			http.Error(w, "token exchange failed", http.StatusBadGateway)
			resultCh <- result{err: exErr}
			return
		}
		rawID, ok := oauth2Token.Extra("id_token").(string)
		if !ok {
			http.Error(w, "no id_token in response", http.StatusBadGateway)
			resultCh <- result{err: fmt.Errorf("no id_token returned")}
			return
		}
		_, _ = fmt.Fprintln(w, "kubepark login complete; you may close this tab.")
		resultCh <- result{token: rawID}
	})
	go func() { _ = srv.Serve(listener) }()
	defer func() { _ = srv.Close() }()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-resultCh:
		return res.token, res.err
	case <-time.After(5 * time.Minute):
		return "", fmt.Errorf("timed out waiting for OIDC callback")
	}
}

func requestCert(ctx context.Context, gatewayURL, publicKey, idToken string) (cert, principal string, err error) {
	body, _ := json.Marshal(map[string]string{"publicKey": publicKey, "idToken": idToken})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gatewayURL+"/v1/sign", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("sign request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("sign request failed: %s", resp.Status)
	}
	var out struct {
		Certificate string `json:"certificate"`
		Principal   string `json:"principal"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", err
	}
	return out.Certificate, out.Principal, nil
}

func randomString() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}
