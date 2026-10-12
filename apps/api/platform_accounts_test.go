package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestPlatformAccountPersistenceAgainstPostgres(t *testing.T) {
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

	expectedAccounts := []struct {
		externalID  string
		displayName string
	}{
		{fmt.Sprintf("account-%d-primary", suffix), "Test Account"},
		{fmt.Sprintf("account-%d-secondary", suffix), "Second Account"},
	}

	var before time.Time
	if err := db.QueryRowContext(ctx, "SELECT CURRENT_TIMESTAMP").Scan(&before); err != nil {
		t.Fatal(err)
	}
	accountIDs := make([]int64, len(expectedAccounts))
	for i, account := range expectedAccounts {
		if err := db.QueryRowContext(ctx, "INSERT INTO platform_accounts (user_id, platform, external_account_id, display_name) VALUES ($1, 'fake', $2, $3) RETURNING platform_account_id", userID, account.externalID, account.displayName).Scan(&accountIDs[i]); err != nil {
			t.Fatal(err)
		}
		if accountIDs[i] < 1 {
			t.Fatalf("expected generated platform_account_id, got %d", accountIDs[i])
		}
	}
	_, err = db.ExecContext(ctx, "INSERT INTO platform_accounts (user_id, platform, external_account_id, display_name) VALUES ($1, 'fake', $2, 'Missing Owner')", -1, fmt.Sprintf("account-%d-missing-owner", suffix))
	assertPostgresCode(t, err, "23503")
	var after time.Time
	if err := db.QueryRowContext(ctx, "SELECT CURRENT_TIMESTAMP").Scan(&after); err != nil {
		t.Fatal(err)
	}

	rows, err := db.QueryContext(ctx, "SELECT platform_account_id, user_id, platform, external_account_id, display_name, created_at, updated_at FROM platform_accounts WHERE user_id = $1 ORDER BY platform_account_id", userID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for i, expected := range expectedAccounts {
		if !rows.Next() {
			t.Fatalf("expected persisted platform account %d", i+1)
		}
		var accountID, storedUserID int64
		var platform, externalID, displayName string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&accountID, &storedUserID, &platform, &externalID, &displayName, &createdAt, &updatedAt); err != nil {
			t.Fatal(err)
		}
		if accountID != accountIDs[i] || storedUserID != userID || platform != "fake" || externalID != expected.externalID || displayName != expected.displayName {
			t.Fatalf("unexpected persisted platform account: id=%d user_id=%d platform=%q external_account_id=%q display_name=%q", accountID, storedUserID, platform, externalID, displayName)
		}
		if createdAt.IsZero() || updatedAt.IsZero() || createdAt.Before(before) || createdAt.After(after) || updatedAt.Before(before) || updatedAt.After(after) || updatedAt.Before(createdAt) {
			t.Fatalf("unexpected platform account timestamps: created_at=%s updated_at=%s, expected between %s and %s", createdAt, updatedAt, before, after)
		}
	}
	if rows.Next() {
		t.Fatal("expected only the two persisted platform accounts for the test user")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = db.ExecContext(ctx, "INSERT INTO platform_accounts (user_id, platform, external_account_id, display_name) VALUES ($1, 'fake', $2, $3)", userID, expectedAccounts[0].externalID, "Duplicate Account")
	assertPostgresCode(t, err, "23505")

	var secondUserID int64
	if err := db.QueryRowContext(ctx, "INSERT INTO users (email, display_name) VALUES ($1, 'Second Platform Account Test') RETURNING id", fmt.Sprintf("platform-account-second-%d@example.com", suffix)).Scan(&secondUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", secondUserID)
	})
	_, err = db.ExecContext(ctx, "INSERT INTO platform_accounts (user_id, platform, external_account_id, display_name) VALUES ($1, 'fake', $2, $3)", secondUserID, expectedAccounts[0].externalID, "Duplicate Account")
	assertPostgresCode(t, err, "23505")

	if _, err := db.ExecContext(ctx, "INSERT INTO platform_accounts (user_id, platform, external_account_id, display_name) VALUES ($1, 'fake', $2, $3)", userID, fmt.Sprintf("account-%d-same-display", suffix), expectedAccounts[0].displayName); err != nil {
		t.Fatal(err)
	}
	var accountCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM platform_accounts WHERE user_id = $1", userID).Scan(&accountCount); err != nil {
		t.Fatal(err)
	}
	if accountCount != len(expectedAccounts)+1 {
		t.Fatalf("expected %d persisted platform accounts for the test user, found %d", len(expectedAccounts)+1, accountCount)
	}

	if _, err := db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM platform_accounts WHERE user_id = $1", userID).Scan(&accountCount); err != nil {
		t.Fatal(err)
	}
	if accountCount != 0 {
		t.Fatalf("expected platform accounts to be deleted with owner, found %d", accountCount)
	}
}
