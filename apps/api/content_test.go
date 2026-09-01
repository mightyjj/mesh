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

func TestContentRoutesRejectAnonymousRequests(t *testing.T) {
	router := newRouter(nil)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(method, "/content", strings.NewReader(`{"title":"ignored"}`))
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
			}
		})
	}
}

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
	emptyClerkUserID := fmt.Sprintf("user_content_empty_%d", suffix)
	var userID, otherUserID, emptyUserID int64
	for _, user := range []struct {
		clerkID string
		email   string
		name    string
		id      *int64
	}{
		{clerkUserID, fmt.Sprintf("content-%d@example.com", suffix), "Content Test", &userID},
		{otherClerkUserID, fmt.Sprintf("content-other-%d@example.com", suffix), "Other Test", &otherUserID},
		{emptyClerkUserID, fmt.Sprintf("content-empty-%d@example.com", suffix), "Empty Test", &emptyUserID},
	} {
		if err := db.QueryRowContext(ctx, "INSERT INTO users (clerk_user_id, email, display_name) VALUES ($1, $2, $3) RETURNING id", user.clerkID, user.email, user.name).Scan(user.id); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE id IN ($1, $2, $3)", userID, otherUserID, emptyUserID)
	})
	if _, err := db.ExecContext(ctx, "INSERT INTO content (creator_id, title) VALUES ($1, 'Other user content')", otherUserID); err != nil {
		t.Fatal(err)
	}

	router := newRouter(db)
	claims := func(clerkUserID string) *clerk.SessionClaims {
		return &clerk.SessionClaims{RegisteredClaims: clerk.RegisteredClaims{Subject: clerkUserID}}
	}
	request := func(method, path, body, clerkUserID string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request = request.WithContext(clerk.ContextWithSessionClaims(request.Context(), claims(clerkUserID)))
		router.ServeHTTP(recorder, request)
		return recorder
	}

	for _, body := range []string{`{"title":"   "}`, `{"title":"First upload"}{}`, `{"title":"First upload","creator_id":999}`} {
		recorder := request(http.MethodPost, "/content", body, clerkUserID)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected invalid body status %d for %s, got %d: %s", http.StatusBadRequest, body, recorder.Code, recorder.Body.String())
		}
	}

	created := make([]content, 0, 2)
	for _, title := range []string{"  First upload  ", "Second upload"} {
		recorder := request(http.MethodPost, "/content", fmt.Sprintf(`{"title":%q}`, title), clerkUserID)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, recorder.Code, recorder.Body.String())
		}
		if got := recorder.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Fatalf("expected private no-store cache control, got %q", got)
		}

		body := recorder.Body.String()
		var item content
		if err := json.NewDecoder(strings.NewReader(body)).Decode(&item); err != nil {
			t.Fatal(err)
		}
		if item.ContentID < 1 || item.Title == "" {
			t.Fatalf("unexpected created content: %+v", item)
		}
		if strings.Contains(body, "creator_id") {
			t.Fatal("response exposed creator ID")
		}
		created = append(created, item)
	}
	if created[0].Title != "First upload" {
		t.Fatalf("expected title to be trimmed, got %q", created[0].Title)
	}

	recorder := request(http.MethodGet, "/content", "", clerkUserID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("expected private no-store cache control, got %q", got)
	}
	body := recorder.Body.String()
	var items []content
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != len(created) || items[0] != created[0] || items[1] != created[1] {
		t.Fatalf("expected current user's content in stable order %+v, got %+v", created, items)
	}
	if strings.Contains(body, "creator_id") {
		t.Fatal("response exposed creator ID")
	}

	recorder = request(http.MethodGet, "/content", "", otherClerkUserID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected other user's list status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	body = recorder.Body.String()
	var otherItems []content
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&otherItems); err != nil {
		t.Fatal(err)
	}
	if len(otherItems) != 1 || otherItems[0].Title != "Other user content" {
		t.Fatalf("expected only other user's content, got %+v", otherItems)
	}

	recorder = request(http.MethodGet, "/content", "", emptyClerkUserID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected empty list status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if body := recorder.Body.String(); body != "[]" {
		t.Fatalf("expected empty list [], got %q", body)
	}
}
