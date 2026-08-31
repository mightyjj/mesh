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

func requestJSON(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	handler.ServeHTTP(recorder, request)
	return recorder
}
