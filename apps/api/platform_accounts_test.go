package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestPlatformAccountInsertAgainstPostgres(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	suffix := time.Now().UnixNano()
	var userID int64
	if err := db.QueryRowContext(ctx, "INSERT INTO users (email, display_name) VALUES ($1, 'Platform Account Test') RETURNING id", fmt.Sprintf("platform-account-%d@example.com", suffix)).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", userID)
	})

	var accountID int64
	if err := db.QueryRowContext(ctx, "INSERT INTO platform_accounts (user_id, platform, external_account_id, display_name) VALUES ($1, 'fake', $2, 'Test Account') RETURNING platform_account_id", userID, fmt.Sprintf("account-%d", suffix)).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	if accountID < 1 {
		t.Fatalf("expected generated platform_account_id, got %d", accountID)
	}
}
