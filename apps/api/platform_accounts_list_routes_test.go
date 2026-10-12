package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
)

func TestPlatformAccountListRejectsAnonymousRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/platform-accounts", nil)
	newRouter(nil).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	if recorder.Body.String() != `{"error":"authentication required"}` {
		t.Fatalf("expected body %q, got %q", `{"error":"authentication required"}`, recorder.Body.String())
	}
}

func TestPlatformAccountListAgainstPostgres(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	suffix := time.Now().UnixNano()
	type testUser struct {
		clerkID string
		email   string
		id      int64
	}
	users := []testUser{
		{clerkID: fmt.Sprintf("platform_list_%d", suffix), email: fmt.Sprintf("platform-list-%d@example.com", suffix)},
		{clerkID: fmt.Sprintf("platform_list_other_%d", suffix), email: fmt.Sprintf("platform-list-other-%d@example.com", suffix)},
		{clerkID: fmt.Sprintf("platform_list_empty_%d", suffix), email: fmt.Sprintf("platform-list-empty-%d@example.com", suffix)},
	}
	userIDs := make([]int64, 0, len(users))
	t.Cleanup(func() {
		for _, id := range userIDs {
			_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", id)
		}
	})
	for i := range users {
		if err := db.QueryRowContext(ctx, "INSERT INTO users (clerk_user_id, email, display_name) VALUES ($1, $2, $3) RETURNING id", users[i].clerkID, users[i].email, "Platform List Test").Scan(&users[i].id); err != nil {
			t.Fatal(err)
		}
		userIDs = append(userIDs, users[i].id)
	}

	router := newRouter(db)
	request := func(method, path, clerkUserID string, body io.Reader) *httptest.ResponseRecorder {
		t.Helper()
		if body == nil {
			body = strings.NewReader("")
		}
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, body)
		claims := &clerk.SessionClaims{RegisteredClaims: clerk.RegisteredClaims{Subject: clerkUserID}}
		req = req.WithContext(clerk.ContextWithSessionClaims(req.Context(), claims))
		router.ServeHTTP(recorder, req)
		return recorder
	}

	created := request(http.MethodPost, "/platform-accounts", users[0].clerkID, nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, created.Code, created.Body.String())
	}
	var createdAccount platformAccount
	if err := json.NewDecoder(created.Body).Decode(&createdAccount); err != nil {
		t.Fatal(err)
	}

	var secondAccount platformAccount
	if err := db.QueryRowContext(ctx, "INSERT INTO platform_accounts (user_id, platform, external_account_id, display_name) VALUES ($1, 'fake', $2, $3) RETURNING "+platformAccountColumns, users[0].id, fmt.Sprintf("fake-secondary-%d", suffix), fmt.Sprintf("Secondary Account %d", suffix)).Scan(
		&secondAccount.PlatformAccountID,
		&secondAccount.Platform,
		&secondAccount.ExternalAccountID,
		&secondAccount.DisplayName,
		&secondAccount.CreatedAt,
		&secondAccount.UpdatedAt,
	); err != nil {
		t.Fatal(err)
	}

	otherCreated := request(http.MethodPost, "/platform-accounts", users[1].clerkID, nil)
	if otherCreated.Code != http.StatusCreated {
		t.Fatalf("expected other user create status %d, got %d: %s", http.StatusCreated, otherCreated.Code, otherCreated.Body.String())
	}

	listed := request(http.MethodGet, "/platform-accounts", users[0].clerkID, nil)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d: %s", http.StatusOK, listed.Code, listed.Body.String())
	}
	if got := listed.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("expected private no-store cache control, got %q", got)
	}
	listedBody := listed.Body.String()
	var accounts []platformAccount
	if err := json.NewDecoder(strings.NewReader(listedBody)).Decode(&accounts); err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected two accounts, got %d", len(accounts))
	}
	if accounts[0].PlatformAccountID >= accounts[1].PlatformAccountID {
		t.Fatalf("expected accounts in ascending ID order, got %d then %d", accounts[0].PlatformAccountID, accounts[1].PlatformAccountID)
	}
	if accounts[0].PlatformAccountID != createdAccount.PlatformAccountID || accounts[1].PlatformAccountID != secondAccount.PlatformAccountID {
		t.Fatalf("expected persisted accounts in list, got %+v", accounts)
	}
	for i, want := range []platformAccount{createdAccount, secondAccount} {
		got := accounts[i]
		if got.Platform != want.Platform || got.ExternalAccountID != want.ExternalAccountID || got.DisplayName != want.DisplayName || !got.CreatedAt.Equal(want.CreatedAt) || !got.UpdatedAt.Equal(want.UpdatedAt) {
			t.Fatalf("account %d did not match persisted data: got %+v, want %+v", i, got, want)
		}
	}

	var fields []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(listedBody), &fields); err != nil {
		t.Fatal(err)
	}
	for i, fieldSet := range fields {
		for field := range fieldSet {
			switch field {
			case "platform_account_id", "platform", "external_account_id", "display_name", "created_at", "updated_at":
			default:
				t.Fatalf("account %d returned unexpected field %q", i, field)
			}
		}
		if len(fieldSet) != 6 {
			t.Fatalf("account %d returned %d fields: %v", i, len(fieldSet), fieldSet)
		}
	}

	other := request(http.MethodGet, "/platform-accounts", users[1].clerkID, nil)
	if other.Code != http.StatusOK {
		t.Fatalf("expected other user list status %d, got %d: %s", http.StatusOK, other.Code, other.Body.String())
	}
	var otherAccounts []platformAccount
	if err := json.NewDecoder(other.Body).Decode(&otherAccounts); err != nil {
		t.Fatal(err)
	}
	if len(otherAccounts) != 1 || otherAccounts[0].ExternalAccountID == accounts[0].ExternalAccountID {
		t.Fatalf("expected only the other user's account, got %+v", otherAccounts)
	}

	for _, clerkUserID := range []string{users[2].clerkID, fmt.Sprintf("unmapped_%d", suffix)} {
		empty := request(http.MethodGet, "/platform-accounts", clerkUserID, nil)
		if empty.Code != http.StatusOK || empty.Body.String() != "[]" {
			t.Fatalf("expected empty list for %q, got %d: %s", clerkUserID, empty.Code, empty.Body.String())
		}
	}
}

func TestPlatformAccountListDatabaseFailure(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://mesh:mesh@127.0.0.1:1/mesh?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/platform-accounts", nil)
	claims := &clerk.SessionClaims{RegisteredClaims: clerk.RegisteredClaims{Subject: "database_failure"}}
	request = request.WithContext(clerk.ContextWithSessionClaims(request.Context(), claims))
	recorder := httptest.NewRecorder()
	newRouter(db).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}
	if recorder.Body.String() != `{"error":"could not list platform accounts"}` {
		t.Fatalf("expected database error body, got %q", recorder.Body.String())
	}
}
