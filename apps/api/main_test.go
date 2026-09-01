package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
)

type fakePinger struct {
	err    error
	called bool
}

func TestAuthorizedParties(t *testing.T) {
	params := &clerkhttp.AuthorizationParams{}
	if err := clerkhttp.AuthorizedPartyMatches(clerkAuthorizedParties...)(params); err != nil {
		t.Fatal(err)
	}
	for party, want := range map[string]bool{
		"http://localhost:3000":    true,
		"https://app.mesh.com":     false,
		"https://attacker.example": false,
	} {
		if got := params.AuthorizedPartyHandler(party); got != want {
			t.Errorf("authorized party %q: expected %t, got %t", party, want, got)
		}
	}
}

func TestAuthorizationBoundary(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	const keyID = "test-key"
	jwk := jose.JSONWebKey{Key: &privateKey.PublicKey, KeyID: keyID, Algorithm: string(jose.RS256), Use: "sig"}
	clerkAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jwks" {
			t.Errorf("expected Clerk JWKS path /jwks, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []jose.JSONWebKey{jwk}})
	}))
	defer clerkAPI.Close()

	previousBackend := clerk.GetBackend()
	clerk.SetBackend(clerk.NewBackend(&clerk.BackendConfig{
		HTTPClient: clerkAPI.Client(),
		URL:        &clerkAPI.URL,
		Key:        clerk.String("test"),
	}))
	t.Cleanup(func() { clerk.SetBackend(previousBackend) })

	handler := requireClerkAuthorization(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, test := range []struct {
		name, token string
		status      int
	}{
		{"valid", signedToken(t, privateKey, keyID, "http://localhost:3000", time.Now().Add(time.Minute)), http.StatusNoContent},
		{"expired", signedToken(t, privateKey, keyID, "http://localhost:3000", time.Now().Add(-time.Minute)), http.StatusUnauthorized},
		{"malformed", "not-a-jwt", http.StatusUnauthorized},
		{"wrong authorized party", signedToken(t, privateKey, keyID, "https://attacker.example", time.Now().Add(time.Minute)), http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/me", nil)
			request.Header.Set("Authorization", "Bearer "+test.token)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("expected status %d, got %d", test.status, recorder.Code)
			}
		})
	}
}

func signedToken(t *testing.T, privateKey *rsa.PrivateKey, keyID, authorizedParty string, expiresAt time.Time) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithHeader(jose.HeaderKey("kid"), keyID),
	)
	if err != nil {
		t.Fatal(err)
	}
	token, err := jwt.Signed(signer).Claims(struct {
		jwt.Claims
		AuthorizedParty string `json:"azp"`
	}{
		Claims: jwt.Claims{
			Issuer:   "https://clerk.test",
			Subject:  "user_test",
			IssuedAt: jwt.NewNumericDate(time.Now()),
			Expiry:   jwt.NewNumericDate(expiresAt),
		},
		AuthorizedParty: authorizedParty,
	}).CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func (p *fakePinger) PingContext(context.Context) error {
	p.called = true
	return p.err
}

func TestHealthChecksDatabase(t *testing.T) {
	for _, test := range []struct {
		name       string
		err        error
		statusCode int
		body       string
	}{
		{name: "healthy", statusCode: http.StatusOK, body: `{"status":"ok"}`},
		{name: "unavailable", err: errors.New("database unavailable"), statusCode: http.StatusServiceUnavailable, body: `{"status":"error"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			pinger := &fakePinger{err: test.err}
			router := chi.NewRouter()
			router.Get("/health", healthHandler(pinger))

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			router.ServeHTTP(recorder, request)

			if !pinger.called {
				t.Fatal("expected health check to ping the database")
			}
			if recorder.Code != test.statusCode {
				t.Fatalf("expected status %d, got %d", test.statusCode, recorder.Code)
			}
			if body := recorder.Body.String(); body != test.body {
				t.Fatalf("expected body %q, got %q", test.body, body)
			}
		})
	}
}

func TestMeRejectsAnonymousRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	newRouter(nil).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	if body := recorder.Body.String(); body != `{"error":"authentication required"}` {
		t.Fatalf("expected body %q, got %q", `{"error":"authentication required"}`, body)
	}
}

func TestMeRejectsNonGetRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/me", nil)
	newRouter(nil).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, recorder.Code)
	}
}
