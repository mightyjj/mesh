package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
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
		"https://app.mesh.com":     true,
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
	publicKey, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKey}))

	middleware := clerkhttp.WithHeaderAuthorization(
		clerkhttp.JSONWebKey(pemKey),
		clerkhttp.AuthorizedPartyMatches(clerkAuthorizedParties...),
		clerkhttp.AuthorizationFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})),
	)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := clerk.SessionClaimsFromContext(r.Context()); !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, test := range []struct {
		name, token string
		status      int
	}{
		{"valid", signedToken(t, privateKey, "http://localhost:3000", time.Now().Add(time.Minute)), http.StatusNoContent},
		{"expired", signedToken(t, privateKey, "http://localhost:3000", time.Now().Add(-time.Minute)), http.StatusUnauthorized},
		{"malformed", "not-a-jwt", http.StatusUnauthorized},
		{"wrong authorized party", signedToken(t, privateKey, "https://attacker.example", time.Now().Add(time.Minute)), http.StatusUnauthorized},
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

func signedToken(t *testing.T, privateKey *rsa.PrivateKey, authorizedParty string, expiresAt time.Time) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: privateKey}, nil)
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
