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

func TestContentCreateAndListAgainstPostgres(t *testing.T) {
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

	suffix := time.Now().UnixNano()
	clerkUserID := fmt.Sprintf("user_content_%d", suffix)
	otherClerkUserID := fmt.Sprintf("user_content_other_%d", suffix)
	var userID, otherUserID int64
	if err := db.QueryRowContext(ctx, "INSERT INTO users (clerk_user_id, email, display_name) VALUES ($1, $2, 'Content Test') RETURNING id", clerkUserID, fmt.Sprintf("content-%d@example.com", suffix)).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "INSERT INTO users (clerk_user_id, email, display_name) VALUES ($1, $2, 'Other Test') RETURNING id", otherClerkUserID, fmt.Sprintf("content-other-%d@example.com", suffix)).Scan(&otherUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE id IN ($1, $2)", userID, otherUserID)
	})
	if _, err := db.ExecContext(ctx, "INSERT INTO content (creator_id, title) VALUES ($1, 'Other user content')", otherUserID); err != nil {
		t.Fatal(err)
	}

	claims := &clerk.SessionClaims{RegisteredClaims: clerk.RegisteredClaims{Subject: clerkUserID}}
	request := httptest.NewRequest(http.MethodPost, "/content", strings.NewReader(`{"title":"   "}`))
	request = request.WithContext(clerk.ContextWithSessionClaims(request.Context(), claims))
	recorder := httptest.NewRecorder()
	createContentHandler(db).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected blank title status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/content", strings.NewReader(`{"title":"  First upload  "}`))
	request = request.WithContext(clerk.ContextWithSessionClaims(request.Context(), claims))
	createContentHandler(db).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}

	var created content
	if err := json.NewDecoder(recorder.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID < 1 || created.Title != "First upload" {
		t.Fatalf("unexpected created content: %+v", created)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/content", nil)
	request = request.WithContext(clerk.ContextWithSessionClaims(request.Context(), claims))
	listContentHandler(db).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var items []content
	if err := json.NewDecoder(recorder.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0] != created {
		t.Fatalf("expected only created content %+v, got %+v", created, items)
	}
}
