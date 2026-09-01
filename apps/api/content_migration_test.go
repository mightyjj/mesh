package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestContentConstraintsAgainstPostgres(t *testing.T) {
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

	var userID int64
	suffix := time.Now().UnixNano()
	if err := db.QueryRowContext(ctx, "INSERT INTO users (email, display_name) VALUES ($1, 'Content Test') RETURNING id", fmt.Sprintf("content-%d@example.com", suffix)).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", userID)
	})

	if _, err := db.ExecContext(ctx, "INSERT INTO content (creator_id, title) VALUES ($1, 'First upload')", userID); err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title) VALUES ($1, '   ')", userID)
	assertPostgresCode(t, err, "23514")
	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title) VALUES ($1, 'No owner')", userID+1_000_000_000)
	assertPostgresCode(t, err, "23503")
}

func assertPostgresCode(t *testing.T, err error, code string) {
	t.Helper()
	if pgErr, ok := err.(*pgconn.PgError); !ok || pgErr.Code != code {
		t.Fatalf("expected PostgreSQL error %s, got %v", code, err)
	}
}
