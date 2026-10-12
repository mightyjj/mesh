package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlatformAccountsMigrationLifecycle(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	schemaName := fmt.Sprintf("migration_compat_%d", time.Now().UnixNano())
	if _, err := tx.ExecContext(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "SET LOCAL search_path TO "+schemaName); err != nil {
		t.Fatal(err)
	}

	applyMigration := func(filename, direction string) {
		t.Helper()

		contents, err := os.ReadFile(filepath.Join("migrations", filename))
		if err != nil {
			t.Fatal(err)
		}

		const upMarker = "-- +goose Up"
		const downMarker = "-- +goose Down"
		migration := string(contents)
		upStart := strings.Index(migration, upMarker)
		downStart := strings.Index(migration, downMarker)
		if upStart < 0 || downStart < 0 || upStart >= downStart {
			t.Fatalf("migration %s is missing valid goose markers", filename)
		}

		var migrationSQL string
		switch direction {
		case "Up":
			migrationSQL = migration[upStart+len(upMarker) : downStart]
		case "Down":
			migrationSQL = migration[downStart+len(downMarker):]
		default:
			t.Fatalf("migration %s has unsupported direction %q", filename, direction)
		}

		migrationSQL = strings.TrimSpace(migrationSQL)
		if migrationSQL == "" {
			t.Fatalf("migration %s has an empty %s section", filename, strings.ToLower(direction))
		}
		if _, err := tx.ExecContext(ctx, migrationSQL); err != nil {
			t.Fatalf("apply %s migration %s: %v", strings.ToLower(direction), filename, err)
		}
	}

	for _, filename := range []string{
		"20260830000000_create_users.sql",
		"20260831000000_add_clerk_user_id.sql",
		"20260901000000_create_content.sql",
		"20260903000000_add_content_media_metadata.sql",
	} {
		applyMigration(filename, "Up")
	}

	var userID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO users (email, clerk_user_id, display_name)
		VALUES ('migration-compat@example.com', 'user_migration_compat', 'Migration Compatibility')
		RETURNING id
	`).Scan(&userID); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content (creator_id, title, original_filename, mime_type, byte_size, storage_key)
		VALUES ($1, 'Migration compatibility video', 'migration.mp4', 'video/mp4', 12345, 'media/migration-compat.mp4')
	`, userID); err != nil {
		t.Fatal(err)
	}

	snapshot := func() (string, string) {
		t.Helper()

		var userRow, contentRow string
		if err := tx.QueryRowContext(ctx, `
			SELECT row_to_json(u)::text, row_to_json(c)::text
			FROM users u JOIN content c ON c.creator_id = u.id
			WHERE u.id = $1
		`, userID).Scan(&userRow, &contentRow); err != nil {
			t.Fatal(err)
		}
		return userRow, contentRow
	}

	beforeUser, beforeContent := snapshot()
	applyMigration("20261012000000_create_platform_accounts.sql", "Up")

	afterUser, afterContent := snapshot()
	if beforeUser != afterUser || beforeContent != afterContent {
		t.Fatalf("existing rows changed during migration: user before=%s after=%s, content before=%s after=%s", beforeUser, afterUser, beforeContent, afterContent)
	}

	var accountCount int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM platform_accounts").Scan(&accountCount); err != nil {
		t.Fatal(err)
	}
	if accountCount != 0 {
		t.Fatalf("expected new platform accounts table to be empty, got %d rows", accountCount)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO platform_accounts (user_id, platform, external_account_id, display_name)
		VALUES ($1, 'fake', 'migration-account', 'Migration Account')
	`, userID); err != nil {
		t.Fatal(err)
	}

	applyMigration("20261012000000_create_platform_accounts.sql", "Down")

	var tableName sql.NullString
	if err := tx.QueryRowContext(ctx, "SELECT to_regclass($1::text)", schemaName+".platform_accounts").Scan(&tableName); err != nil {
		t.Fatal(err)
	}
	if tableName.Valid {
		t.Fatalf("expected platform accounts table to be removed, got %q", tableName.String)
	}

	afterDownUser, afterDownContent := snapshot()
	if beforeUser != afterDownUser || beforeContent != afterDownContent {
		t.Fatalf("existing rows changed during migration reversal: user before=%s after=%s, content before=%s after=%s", beforeUser, afterDownUser, beforeContent, afterDownContent)
	}

	applyMigration("20261012000000_create_platform_accounts.sql", "Up")

	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM platform_accounts").Scan(&accountCount); err != nil {
		t.Fatal(err)
	}
	if accountCount != 0 {
		t.Fatalf("expected reapplied platform accounts table to be empty, got %d rows", accountCount)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO platform_accounts (user_id, platform, external_account_id, display_name)
		VALUES ($1, 'fake', 'migration-account-reapplied', 'Reapplied Migration Account')
	`, userID); err != nil {
		t.Fatal(err)
	}

	afterReapplyUser, afterReapplyContent := snapshot()
	if beforeUser != afterReapplyUser || beforeContent != afterReapplyContent {
		t.Fatalf("existing rows changed after migration reapplication: user before=%s after=%s, content before=%s after=%s", beforeUser, afterReapplyUser, beforeContent, afterReapplyContent)
	}
}
