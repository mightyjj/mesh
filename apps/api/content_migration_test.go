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

	var contentID int64
	if err := db.QueryRowContext(ctx, "INSERT INTO content (creator_id, title) VALUES ($1, 'First upload') RETURNING content_id", userID).Scan(&contentID); err != nil {
		t.Fatal(err)
	}
	if contentID < 1 {
		t.Fatalf("expected generated content_id, got %d", contentID)
	}
	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title) VALUES ($1, '   ')", userID)
	assertPostgresCode(t, err, "23514")
	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title) VALUES ($1, 'No owner')", userID+1_000_000_000)
	assertPostgresCode(t, err, "23503")
}

func TestContentMediaMetadataConstraintsAgainstPostgres(t *testing.T) {
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

	suffix := time.Now().UnixNano()
	storageKey := fmt.Sprintf("media/%d-video.mp4", suffix)
	var userID int64
	if err := db.QueryRowContext(ctx, "INSERT INTO users (email, display_name) VALUES ($1, 'Media Metadata Test') RETURNING id", fmt.Sprintf("media-metadata-%d@example.com", suffix)).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", userID)
	})

	var contentID int64
	if err := db.QueryRowContext(ctx, "INSERT INTO content (creator_id, title) VALUES ($1, 'Content without media') RETURNING content_id", userID).Scan(&contentID); err != nil {
		t.Fatal(err)
	}
	if contentID < 1 {
		t.Fatalf("expected generated content_id, got %d", contentID)
	}
	var originalFilenameNull, mimeTypeNull, byteSizeNull, storageKeyNull bool
	if err := db.QueryRowContext(ctx, "SELECT original_filename IS NULL, mime_type IS NULL, byte_size IS NULL, storage_key IS NULL FROM content WHERE content_id = $1", contentID).Scan(&originalFilenameNull, &mimeTypeNull, &byteSizeNull, &storageKeyNull); err != nil {
		t.Fatal(err)
	}
	if !originalFilenameNull || !mimeTypeNull || !byteSizeNull || !storageKeyNull {
		t.Fatal("expected content without media to have null media metadata")
	}

	// The unique storage key must not stop a second content item from having no media.
	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title) VALUES ($1, 'Another item without media')", userID)
	if err != nil {
		t.Fatalf("expected content without media to coexist: %v", err)
	}

	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title, storage_key) VALUES ($1, 'Partial metadata', 'media/partial.mp4')", userID)
	assertPostgresCode(t, err, "23514")

	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title, original_filename, mime_type, byte_size, storage_key) VALUES ($1, 'Valid metadata', 'video.mp4', 'video/mp4', 100, $2)", userID, storageKey)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title, original_filename, mime_type, byte_size, storage_key) VALUES ($1, 'Zero size', 'zero.mp4', 'video/mp4', 0, 'media/zero.mp4')", userID)
	assertPostgresCode(t, err, "23514")
	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title, original_filename, mime_type, byte_size, storage_key) VALUES ($1, 'Negative size', 'negative.mp4', 'video/mp4', -1, 'media/negative.mp4')", userID)
	assertPostgresCode(t, err, "23514")
	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title, original_filename, mime_type, byte_size, storage_key) VALUES ($1, 'Blank filename', '   ', 'video/mp4', 100, 'media/blank.mp4')", userID)
	assertPostgresCode(t, err, "23514")
	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title, original_filename, mime_type, byte_size, storage_key) VALUES ($1, 'Blank MIME type', 'mime.mp4', '   ', 100, 'media/mime.mp4')", userID)
	assertPostgresCode(t, err, "23514")
	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title, original_filename, mime_type, byte_size, storage_key) VALUES ($1, 'Blank storage key', 'key.mp4', 'video/mp4', 100, '   ')", userID)
	assertPostgresCode(t, err, "23514")
	_, err = db.ExecContext(ctx, "INSERT INTO content (creator_id, title, original_filename, mime_type, byte_size, storage_key) VALUES ($1, 'Duplicate key', 'duplicate.mp4', 'video/mp4', 100, $2)", userID, storageKey)
	assertPostgresCode(t, err, "23505")
}

func assertPostgresCode(t *testing.T, err error, code string) {
	t.Helper()
	if pgErr, ok := err.(*pgconn.PgError); !ok || pgErr.Code != code {
		t.Fatalf("expected PostgreSQL error %s, got %v", code, err)
	}
}
