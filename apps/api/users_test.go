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

func TestUserRoutesAreNotPublic(t *testing.T) {
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

	for _, test := range []struct{ method, path string }{
		{http.MethodPost, "/users"},
		{http.MethodGet, "/users/1"},
	} {
		response := requestJSON(t, newRouter(db), test.method, test.path, "")
		if response.Code != http.StatusNotFound {
			t.Fatalf("expected status %d for %s %s, got %d", http.StatusNotFound, test.method, test.path, response.Code)
		}
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
		_, _ = fmt.Fprintf(w, `{"id":%q,"primary_email_address_id":"email_1","email_addresses":[{"id":"email_1","email_address":%q,"verification":{"status":"verified"}}],"first_name":"Ada","last_name":"Lovelace"}`, clerkUserID, email)
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
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "private, no-store" {
		t.Fatalf("expected private no-store cache control, got %q", cacheControl)
	}

	var mapped user
	if err := json.NewDecoder(recorder.Body).Decode(&mapped); err != nil {
		t.Fatal(err)
	}
	if mapped.Email != email || mapped.DisplayName != "Ada Lovelace" {
		t.Fatalf("unexpected mapped user: %+v", mapped)
	}
	if strings.Contains(recorder.Body.String(), "clerk_user_id") {
		t.Fatal("response exposed Clerk user ID")
	}
	var storedClerkUserID string
	if err := db.QueryRowContext(ctx, "SELECT clerk_user_id FROM users WHERE id = $1", mapped.ID).Scan(&storedClerkUserID); err != nil {
		t.Fatal(err)
	}
	if storedClerkUserID != clerkUserID {
		t.Fatalf("expected stored Clerk user ID %q, got %q", clerkUserID, storedClerkUserID)
	}
}

func requestJSON(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	handler.ServeHTTP(recorder, request)
	return recorder
}
