package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
)

func TestUsersAgainstPostgres(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	email := fmt.Sprintf("user-%d@example.com", time.Now().UnixNano())
	_, _ = db.ExecContext(ctx, "DELETE FROM users WHERE email = $1", email)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE email = $1", email)
	})

	router := newRouter(db)
	createResponse := requestJSON(t, router, http.MethodPost, "/users", fmt.Sprintf(`{"email":%q,"display_name":"Test User"}`, email))
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, createResponse.Code, createResponse.Body.String())
	}

	var created user
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID < 1 || created.Email != email || created.DisplayName != "Test User" {
		t.Fatalf("unexpected created user: %+v", created)
	}

	fetchResponse := requestJSON(t, router, http.MethodGet, fmt.Sprintf("/users/%d", created.ID), "")
	if fetchResponse.Code != http.StatusOK {
		t.Fatalf("expected fetch status %d, got %d: %s", http.StatusOK, fetchResponse.Code, fetchResponse.Body.String())
	}

	var fetched user
	if err := json.NewDecoder(fetchResponse.Body).Decode(&fetched); err != nil {
		t.Fatal(err)
	}
	if fetched != created {
		t.Fatalf("expected fetched user %+v, got %+v", created, fetched)
	}
}

func TestMeMapsClerkUserAgainstPostgres(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	clerkUserID := fmt.Sprintf("user_test_%d", time.Now().UnixNano())
	email := fmt.Sprintf("clerk-%d@example.com", time.Now().UnixNano())
	_, _ = db.ExecContext(ctx, "DELETE FROM users WHERE clerk_user_id = $1 OR email = $2", clerkUserID, email)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE clerk_user_id = $1 OR email = $2", clerkUserID, email)
	})

	clerkAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/"+clerkUserID {
			t.Errorf("expected Clerk user path /users/%s, got %s", clerkUserID, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"id":%q,"primary_email_address_id":"email_1","email_addresses":[{"id":"email_1","email_address":%q}],"first_name":"Ada","last_name":"Lovelace"}`, clerkUserID, email)
	}))
	defer clerkAPI.Close()

	previousBackend := clerk.GetBackend()
	clerk.SetBackend(clerk.NewBackend(&clerk.BackendConfig{
		HTTPClient: clerkAPI.Client(),
		URL:        &clerkAPI.URL,
		Key:        clerk.String("test"),
	}))
	t.Cleanup(func() { clerk.SetBackend(previousBackend) })

	claims := &clerk.SessionClaims{
		RegisteredClaims: clerk.RegisteredClaims{Subject: clerkUserID},
	}
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request = request.WithContext(clerk.ContextWithSessionClaims(request.Context(), claims))
	recorder := httptest.NewRecorder()
	meHandler(db).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var mapped user
	if err := json.NewDecoder(recorder.Body).Decode(&mapped); err != nil {
		t.Fatal(err)
	}
	if mapped.ClerkUserID != clerkUserID || mapped.Email != email || mapped.DisplayName != "Ada Lovelace" {
		t.Fatalf("unexpected mapped user: %+v", mapped)
	}
}

func requestJSON(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	handler.ServeHTTP(recorder, request)
	return recorder
}
