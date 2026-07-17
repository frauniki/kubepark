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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// randomString returns a URL-safe random token for OIDC state.
func randomString() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

const (
	sessionCookie = "kubepark_session"
	stateCookie   = "kubepark_oidc_state"
	// oidcCallbackPath is where the IdP redirects back to; it lives under a
	// reserved prefix so it never collides with a sandbox route.
	oidcCallbackPath = "/kubepark/oidc/callback"
)

// CookieAuthenticator implements Authenticator with an OIDC auth-code flow
// and an HMAC-signed session cookie. It is the browser counterpart to the
// CLI's PKCE flow.
type CookieAuthenticator struct {
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
	claim    string
	hmacKey  []byte
	ttl      time.Duration
	secure   bool
}

// NewCookieAuthenticator builds the authenticator against the given
// provider. hmacKey signs session cookies; it should be stable across
// gateway replicas (sourced from a Secret).
func NewCookieAuthenticator(ctx context.Context, issuer, clientID, clientSecret, redirectBase, claim string, hmacKey []byte, ttl time.Duration, secure bool) (*CookieAuthenticator, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	if claim == "" {
		claim = defaultPrincipalClaim
	}
	return &CookieAuthenticator{
		oauth: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  strings.TrimSuffix(redirectBase, "/") + oidcCallbackPath,
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile", "groups"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		claim:    claim,
		hmacKey:  hmacKey,
		ttl:      ttl,
		secure:   secure,
	}, nil
}

type sessionClaims struct {
	Principal string   `json:"p"`
	Groups    []string `json:"g,omitempty"`
	Expiry    int64    `json:"e"`
}

// Identify verifies the signed session cookie.
func (a *CookieAuthenticator) Identify(r *http.Request) (string, []string, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", nil, false
	}
	claims, ok := a.verifyCookie(cookie.Value)
	if !ok || time.Now().Unix() > claims.Expiry {
		return "", nil, false
	}
	return claims.Principal, claims.Groups, true
}

// StartLogin redirects into the OIDC flow, remembering the original URL.
func (a *CookieAuthenticator) StartLogin(w http.ResponseWriter, r *http.Request) {
	state := randomString()
	http.SetCookie(w, &http.Cookie{
		Name: stateCookie, Value: state + "|" + r.URL.String(),
		Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode,
		MaxAge: 600,
	})
	http.Redirect(w, r, a.oauth.AuthCodeURL(state), http.StatusFound)
}

// Callback handles the OIDC redirect: it verifies the ID token, sets the
// session cookie, and returns the user to their original URL.
func (a *CookieAuthenticator) Callback(w http.ResponseWriter, r *http.Request) {
	stateRaw, err := r.Cookie(stateCookie)
	if err != nil {
		http.Error(w, "missing state", http.StatusBadRequest)
		return
	}
	state, returnTo, _ := strings.Cut(stateRaw.Value, "|")
	if subtle.ConstantTimeCompare([]byte(state), []byte(r.URL.Query().Get("state"))) != 1 {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	token, err := a.oauth.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusBadGateway)
		return
	}
	rawID, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token", http.StatusBadGateway)
		return
	}
	idToken, err := a.verifier.Verify(r.Context(), rawID)
	if err != nil {
		http.Error(w, "invalid id_token", http.StatusUnauthorized)
		return
	}
	principal, groups := identityFromToken(idToken, a.claim)
	if principal == "" {
		http.Error(w, "no principal claim", http.StatusBadRequest)
		return
	}

	value := a.signCookie(sessionClaims{
		Principal: principal, Groups: groups, Expiry: time.Now().Add(a.ttl).Unix(),
	})
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode,
		MaxAge: int(a.ttl.Seconds()),
	})
	if returnTo == "" {
		returnTo = "/"
	}
	http.Redirect(w, r, returnTo, http.StatusFound)
}

// IsCallback reports whether a request targets the OIDC callback path.
func (a *CookieAuthenticator) IsCallback(r *http.Request) bool {
	return r.URL.Path == oidcCallbackPath
}

func (a *CookieAuthenticator) signCookie(claims sessionClaims) string {
	payload, _ := json.Marshal(claims)
	b := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, a.hmacKey)
	mac.Write([]byte(b))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return b + "." + sig
}

func (a *CookieAuthenticator) verifyCookie(value string) (sessionClaims, bool) {
	b, sig, ok := strings.Cut(value, ".")
	if !ok {
		return sessionClaims{}, false
	}
	mac := hmac.New(sha256.New, a.hmacKey)
	mac.Write([]byte(b))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(sig), []byte(want)) != 1 {
		return sessionClaims{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(b)
	if err != nil {
		return sessionClaims{}, false
	}
	var claims sessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return sessionClaims{}, false
	}
	return claims, true
}

// identityFromToken extracts the principal claim and any groups.
func identityFromToken(idToken *oidc.IDToken, claim string) (string, []string) {
	var raw map[string]any
	if err := idToken.Claims(&raw); err != nil {
		return "", nil
	}
	principal, _ := raw[claim].(string)
	var groups []string
	if gs, ok := raw["groups"].([]any); ok {
		for _, g := range gs {
			if s, ok := g.(string); ok {
				groups = append(groups, s)
			}
		}
	}
	return principal, groups
}
