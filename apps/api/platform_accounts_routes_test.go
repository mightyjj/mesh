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
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
)

func TestPlatformAccountCreateRejectsAnonymousRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/platform-accounts", nil)
	newRouter(nil).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	if recorder.Body.String() != `{"error":"authentication required"}` {
		t.Fatalf("expected body %q, got %q", `{"error":"authentication required"}`, recorder.Body.String())
	}
}

func TestPlatformAccountCreateAgainstPostgres(t *testing.T) {
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
		{clerkID: fmt.Sprintf("platform_create_%d", suffix), email: fmt.Sprintf("platform-create-%d@example.com", suffix)},
		{clerkID: fmt.Sprintf("platform_create_other_%d", suffix), email: fmt.Sprintf("platform-create-other-%d@example.com", suffix)},
		{clerkID: fmt.Sprintf("platform_create_race_%d", suffix), email: fmt.Sprintf("platform-create-race-%d@example.com", suffix)},
	}
	for i := range users {
		if err := db.QueryRowContext(ctx, "INSERT INTO users (clerk_user_id, email, display_name) VALUES ($1, $2, $3) RETURNING id", users[i].clerkID, users[i].email, "Platform Create Test").Scan(&users[i].id); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, user := range users {
			_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.id)
		}
	})

	router := newRouter(db)
	request := func(method, body, clerkUserID string, reader io.Reader) *httptest.ResponseRecorder {
		t.Helper()
		if reader == nil {
			reader = strings.NewReader(body)
		}
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/platform-accounts", reader)
		claims := &clerk.SessionClaims{RegisteredClaims: clerk.RegisteredClaims{Subject: clerkUserID}}
		req = req.WithContext(clerk.ContextWithSessionClaims(req.Context(), claims))
		router.ServeHTTP(recorder, req)
		return recorder
	}

	for _, test := range []struct {
		name   string
		body   string
		reader io.Reader
	}{
		{name: "owner field", body: `{"owner_id":999}`},
		{name: "provider field", body: `{"platform":"youtube"}`},
		{name: "one byte body", body: "x"},
		{name: "whitespace", body: " \t\n"},
		{name: "chunked body", reader: &oneByteReader{data: []byte(`{"platform":"fake"}`)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := request(http.MethodPost, test.body, users[0].clerkID, test.reader)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
			}
			if recorder.Body.String() != `{"error":"request body must be empty"}` {
				t.Fatalf("expected body %q, got %q", `{"error":"request body must be empty"}`, recorder.Body.String())
			}
		})
	}

	created := request(http.MethodPost, "", users[0].clerkID, nil)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, created.Code, created.Body.String())
	}
	if got := created.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("expected private no-store cache control, got %q", got)
	}
	createdBody := created.Body.String()
	var account platformAccount
	if err := json.NewDecoder(strings.NewReader(createdBody)).Decode(&account); err != nil {
		t.Fatal(err)
	}
	if account.PlatformAccountID < 1 || account.Platform != "fake" || account.ExternalAccountID != fmt.Sprintf("fake-user-%d", users[0].id) || account.DisplayName != fmt.Sprintf("Fake Account %d", users[0].id) || account.CreatedAt.IsZero() || account.UpdatedAt.IsZero() {
		t.Fatalf("unexpected created account: %+v", account)
	}

	var responseFields map[string]any
	if err := json.Unmarshal([]byte(createdBody), &responseFields); err != nil {
		t.Fatal(err)
	}
	if _, ok := responseFields["user_id"]; ok {
		t.Fatal("response exposed user ID")
	}

	var storedUserID int64
	var storedPlatform, storedExternalID, storedDisplayName string
	if err := db.QueryRowContext(ctx, "SELECT user_id, platform, external_account_id, display_name FROM platform_accounts WHERE platform_account_id = $1", account.PlatformAccountID).Scan(&storedUserID, &storedPlatform, &storedExternalID, &storedDisplayName); err != nil {
		t.Fatal(err)
	}
	if storedUserID != users[0].id || storedPlatform != "fake" || storedExternalID != account.ExternalAccountID || storedDisplayName != account.DisplayName {
		t.Fatalf("unexpected persisted account: user_id=%d platform=%q external_account_id=%q display_name=%q", storedUserID, storedPlatform, storedExternalID, storedDisplayName)
	}

	repeated := request(http.MethodPost, "", users[0].clerkID, nil)
	if repeated.Code != http.StatusConflict || repeated.Body.String() != `{"error":"fake platform account already exists"}` {
		t.Fatalf("expected duplicate conflict, got %d: %s", repeated.Code, repeated.Body.String())
	}
	if got := repeated.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("expected private no-store cache control, got %q", got)
	}

	unmapped := request(http.MethodPost, "", fmt.Sprintf("unmapped_%d", suffix), nil)
	if unmapped.Code != http.StatusConflict || unmapped.Body.String() != `{"error":"current user is not initialized"}` {
		t.Fatalf("expected unmapped conflict, got %d: %s", unmapped.Code, unmapped.Body.String())
	}

	other := request(http.MethodPost, "", users[1].clerkID, nil)
	if other.Code != http.StatusCreated {
		t.Fatalf("expected distinct user status %d, got %d: %s", http.StatusCreated, other.Code, other.Body.String())
	}
	var otherAccount platformAccount
	if err := json.NewDecoder(other.Body).Decode(&otherAccount); err != nil {
		t.Fatal(err)
	}
	if otherAccount.ExternalAccountID == account.ExternalAccountID || otherAccount.DisplayName == account.DisplayName {
		t.Fatalf("distinct users received colliding fake account data: %+v and %+v", account, otherAccount)
	}

	statuses := make(chan int, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			statuses <- request(http.MethodPost, "", users[2].clerkID, nil).Code
		}()
	}
	wait.Wait()
	close(statuses)
	gotStatuses := make([]int, 0, 2)
	for status := range statuses {
		gotStatuses = append(gotStatuses, status)
	}
	sort.Ints(gotStatuses)
	if want := []int{http.StatusCreated, http.StatusConflict}; fmt.Sprint(gotStatuses) != fmt.Sprint(want) {
		t.Fatalf("expected one concurrent create and one conflict, got %v", gotStatuses)
	}

	var raceCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM platform_accounts WHERE user_id = $1", users[2].id).Scan(&raceCount); err != nil {
		t.Fatal(err)
	}
	if raceCount != 1 {
		t.Fatalf("expected one account after concurrent creation, got %d", raceCount)
	}
}

type oneByteReader struct {
	data []byte
}

func (r *oneByteReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	p[0] = r.data[0]
	r.data = r.data[1:]
	return 1, nil
}
