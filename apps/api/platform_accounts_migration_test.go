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

func TestPlatformAccountsMigrationPreservesExistingData(t *testing.T) {
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

	applyUp := func(filename string) {
		t.Helper()

		contents, err := os.ReadFile(filepath.Join("migrations", filename))
		if err != nil {
			t.Fatal(err)
		}

		const upMarker = "-- +goose Up"
		const downMarker = "-- +goose Down"
		upStart := strings.Index(string(contents), upMarker)
		downStart := strings.Index(string(contents), downMarker)
		if upStart < 0 || downStart < 0 || upStart >= downStart {
			t.Fatalf("migration %s is missing valid goose markers", filename)
		}

		upSQL := strings.TrimSpace(string(contents[upStart+len(upMarker) : downStart]))
		if upSQL == "" {
			t.Fatalf("migration %s has an empty up section", filename)
		}
		if _, err := tx.ExecContext(ctx, upSQL); err != nil {
			t.Fatalf("apply migration %s: %v", filename, err)
		}
	}

	for _, filename := range []string{
		"20260830000000_create_users.sql",
		"20260831000000_add_clerk_user_id.sql",
		"20260901000000_create_content.sql",
		"20260903000000_add_content_media_metadata.sql",
	} {
		applyUp(filename)
	}

	var beforeUser migrationCompatibilityUser
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO users (email, clerk_user_id, display_name)
		VALUES ('migration-compat@example.com', 'user_migration_compat', 'Migration Compatibility')
		RETURNING id, email, clerk_user_id, display_name, created_at, updated_at
	`).Scan(
		&beforeUser.ID,
		&beforeUser.Email,
		&beforeUser.ClerkUserID,
		&beforeUser.DisplayName,
		&beforeUser.CreatedAt,
		&beforeUser.UpdatedAt,
	); err != nil {
		t.Fatal(err)
	}

	var beforeContent migrationCompatibilityContent
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO content (creator_id, title, original_filename, mime_type, byte_size, storage_key)
		VALUES ($1, 'Migration compatibility video', 'migration.mp4', 'video/mp4', 12345, 'media/migration-compat.mp4')
		RETURNING content_id, creator_id, title, original_filename, mime_type, byte_size, storage_key, created_at, updated_at
	`, beforeUser.ID).Scan(
		&beforeContent.ID,
		&beforeContent.CreatorID,
		&beforeContent.Title,
		&beforeContent.OriginalFilename,
		&beforeContent.MIMEType,
		&beforeContent.ByteSize,
		&beforeContent.StorageKey,
		&beforeContent.CreatedAt,
		&beforeContent.UpdatedAt,
	); err != nil {
		t.Fatal(err)
	}

	applyUp("20261012000000_create_platform_accounts.sql")

	var afterUser migrationCompatibilityUser
	if err := tx.QueryRowContext(ctx, `
		SELECT id, email, clerk_user_id, display_name, created_at, updated_at
		FROM users WHERE id = $1
	`, beforeUser.ID).Scan(
		&afterUser.ID,
		&afterUser.Email,
		&afterUser.ClerkUserID,
		&afterUser.DisplayName,
		&afterUser.CreatedAt,
		&afterUser.UpdatedAt,
	); err != nil {
		t.Fatal(err)
	}
	if beforeUser != afterUser {
		t.Fatalf("user changed during migration: before=%+v after=%+v", beforeUser, afterUser)
	}

	var afterContent migrationCompatibilityContent
	if err := tx.QueryRowContext(ctx, `
		SELECT content_id, creator_id, title, original_filename, mime_type, byte_size, storage_key, created_at, updated_at
		FROM content WHERE content_id = $1
	`, beforeContent.ID).Scan(
		&afterContent.ID,
		&afterContent.CreatorID,
		&afterContent.Title,
		&afterContent.OriginalFilename,
		&afterContent.MIMEType,
		&afterContent.ByteSize,
		&afterContent.StorageKey,
		&afterContent.CreatedAt,
		&afterContent.UpdatedAt,
	); err != nil {
		t.Fatal(err)
	}
	if beforeContent != afterContent {
		t.Fatalf("content changed during migration: before=%+v after=%+v", beforeContent, afterContent)
	}

	var accountCount int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM platform_accounts").Scan(&accountCount); err != nil {
		t.Fatal(err)
	}
	if accountCount != 0 {
		t.Fatalf("expected new platform accounts table to be empty, got %d rows", accountCount)
	}
}

type migrationCompatibilityUser struct {
	ID          int64
	Email       string
	ClerkUserID string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type migrationCompatibilityContent struct {
	ID               int64
	CreatorID        int64
	Title            string
	OriginalFilename string
	MIMEType         string
	ByteSize         int64
	StorageKey       string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
