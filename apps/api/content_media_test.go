package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
)

var testMP4 = []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom<\x06t\xbfmdat")

var testWebM = []byte{0x1a, 0x45, 0xdf, 0xa3, 0x93, 0x42, 0x86, 0x81}

func TestContentMediaUploadRejectsAnonymous(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/content/1/media", nil)
	newRouter(nil).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
}

func TestContentMediaUploadAgainstPostgres(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not configured")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	mediaDirectory := t.TempDir()
	t.Setenv("MEDIA_DIR", mediaDirectory)
	suffix := time.Now().UnixNano()
	ownerClerkID := fmt.Sprintf("media_owner_%d", suffix)
	otherClerkID := fmt.Sprintf("media_other_%d", suffix)
	var ownerID, otherID int64
	for _, testUser := range []struct {
		clerkID string
		email   string
		id      *int64
	}{
		{ownerClerkID, fmt.Sprintf("media-owner-%d@example.com", suffix), &ownerID},
		{otherClerkID, fmt.Sprintf("media-other-%d@example.com", suffix), &otherID},
	} {
		if err := db.QueryRowContext(ctx, "INSERT INTO users (clerk_user_id, email, display_name) VALUES ($1, $2, $3) RETURNING id", testUser.clerkID, testUser.email, "Media Test").Scan(testUser.id); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE id IN ($1, $2)", ownerID, otherID)
	})

	var ownedContentID, otherContentID, failingContentID, limitContentID, webMContentID, concurrentContentID int64
	for _, testContent := range []struct {
		ownerID *int64
		title   string
		id      *int64
	}{
		{&ownerID, "Media upload", &ownedContentID},
		{&otherID, "Other user's media", &otherContentID},
		{&ownerID, "Failed media upload", &failingContentID},
		{&ownerID, "Exact limit media upload", &limitContentID},
		{&ownerID, "WebM media upload", &webMContentID},
		{&ownerID, "Concurrent media upload", &concurrentContentID},
	} {
		if err := db.QueryRowContext(ctx, "INSERT INTO content (creator_id, title) VALUES ($1, $2) RETURNING content_id", *testContent.ownerID, testContent.title).Scan(testContent.id); err != nil {
			t.Fatal(err)
		}
	}

	router := newRouter(db)
	ownerRequest := func(contentID int64, body []byte, filename string) *httptest.ResponseRecorder {
		t.Helper()
		return performMediaUpload(t, router, contentID, ownerClerkID, body, filename, "application/octet-stream", false)
	}

	if response := performMediaUpload(t, router, otherContentID, ownerClerkID, testMP4, "other.mp4", "video/mp4", false); response.Code != http.StatusNotFound {
		t.Fatalf("expected foreign content status %d, got %d: %s", http.StatusNotFound, response.Code, response.Body.String())
	}

	if response := ownerRequest(ownedContentID, []byte("not a video"), "notes.txt"); response.Code != http.StatusUnsupportedMediaType || !strings.Contains(response.Body.String(), "video/mp4") {
		t.Fatalf("expected unsupported media status %d and accepted types, got %d: %s", http.StatusUnsupportedMediaType, response.Code, response.Body.String())
	}

	if response := ownerRequest(ownedContentID, testMP4, "clip.mp4"); response.Code != http.StatusCreated {
		t.Fatalf("expected upload status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	} else {
		var uploaded mediaUploadResponse
		if err := json.NewDecoder(response.Body).Decode(&uploaded); err != nil {
			t.Fatal(err)
		}
		if uploaded.ContentID != ownedContentID || uploaded.OriginalFilename != "clip.mp4" || uploaded.MIMEType != "video/mp4" || uploaded.ByteSize != int64(len(testMP4)) || uploaded.StorageKey == "" {
			t.Fatalf("unexpected upload response: %+v", uploaded)
		}
		if strings.Contains(uploaded.StorageKey, "/") || strings.Contains(uploaded.StorageKey, "\\") {
			t.Fatalf("expected opaque basename storage key, got %q", uploaded.StorageKey)
		}
		stored, err := os.ReadFile(filepath.Join(mediaDirectory, uploaded.StorageKey))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(stored, testMP4) {
			t.Fatalf("stored media differs from uploaded bytes")
		}

		var originalFilename, mimeType, storageKey string
		var byteSize int64
		var updatedAt time.Time
		if err := db.QueryRowContext(ctx, "SELECT original_filename, mime_type, byte_size, storage_key, updated_at FROM content WHERE content_id = $1", ownedContentID).Scan(&originalFilename, &mimeType, &byteSize, &storageKey, &updatedAt); err != nil {
			t.Fatal(err)
		}
		if originalFilename != uploaded.OriginalFilename || mimeType != uploaded.MIMEType || byteSize != uploaded.ByteSize || storageKey != uploaded.StorageKey || updatedAt.IsZero() {
			t.Fatalf("unexpected persisted media metadata: %q %q %d %q %s", originalFilename, mimeType, byteSize, storageKey, updatedAt)
		}
	}

	fileCount := func() int {
		t.Helper()
		entries, err := os.ReadDir(mediaDirectory)
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	if got := fileCount(); got != 1 {
		t.Fatalf("expected one stored file after success, got %d", got)
	}
	if response := performMediaUpload(t, router, failingContentID, ownerClerkID, testMP4, "extra.mp4", "video/mp4", true); response.Code != http.StatusBadRequest {
		t.Fatalf("expected extra multipart part status %d, got %d: %s", http.StatusBadRequest, response.Code, response.Body.String())
	}
	if got := fileCount(); got != 1 {
		t.Fatalf("expected extra part cleanup, got %d stored files", got)
	}
	if response := performStreamingMediaUpload(t, router, limitContentID, ownerClerkID, maxMediaBytes, testMP4); response.Code != http.StatusCreated {
		t.Fatalf("expected exact size upload status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}
	if response := ownerRequest(webMContentID, testWebM, "clip.webm"); response.Code != http.StatusCreated {
		t.Fatalf("expected WebM upload status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}
	if got := fileCount(); got != 3 {
		t.Fatalf("expected three stored files after exact size and WebM uploads, got %d", got)
	}
	concurrentStatuses := concurrentMediaUpload(t, router, concurrentContentID, ownerClerkID)
	if !((concurrentStatuses[0] == http.StatusCreated && concurrentStatuses[1] == http.StatusConflict) || (concurrentStatuses[0] == http.StatusConflict && concurrentStatuses[1] == http.StatusCreated)) {
		t.Fatalf("expected one concurrent upload to succeed and one to conflict, got %v", concurrentStatuses)
	}
	if got := fileCount(); got != 4 {
		t.Fatalf("expected four stored files after concurrent uploads, got %d", got)
	}
	if response := ownerRequest(ownedContentID, testMP4, "second.mp4"); response.Code != http.StatusConflict {
		t.Fatalf("expected duplicate status %d, got %d: %s", http.StatusConflict, response.Code, response.Body.String())
	}
	if got := fileCount(); got != 4 {
		t.Fatalf("expected duplicate upload to leave four stored files, got %d", got)
	}

	if response := performStreamingMediaUpload(t, router, failingContentID, ownerClerkID, maxMediaBytes+1, testMP4); response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), "100 MiB") {
		t.Fatalf("expected oversized status %d and real limit, got %d: %s", http.StatusRequestEntityTooLarge, response.Code, response.Body.String())
	}
	if got := fileCount(); got != 4 {
		t.Fatalf("expected oversized upload cleanup, got %d stored files", got)
	}

	triggerName := fmt.Sprintf("reject_media_%d", suffix)
	_, err = db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE content ADD CONSTRAINT %s CHECK (content_id <> %d OR storage_key IS NULL)", triggerName, failingContentID))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), fmt.Sprintf("ALTER TABLE content DROP CONSTRAINT IF EXISTS %s", triggerName))
	})
	if response := ownerRequest(failingContentID, testMP4, "failed.mp4"); response.Code != http.StatusInternalServerError {
		t.Fatalf("expected metadata failure status %d, got %d: %s", http.StatusInternalServerError, response.Code, response.Body.String())
	}
	if got := fileCount(); got != 4 {
		t.Fatalf("expected failed database update cleanup, got %d stored files", got)
	}
	var storageKey sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT storage_key FROM content WHERE content_id = $1", failingContentID).Scan(&storageKey); err != nil {
		t.Fatal(err)
	}
	if storageKey.Valid {
		t.Fatalf("expected failed metadata update to leave storage key null, got %q", storageKey.String)
	}
}

func performMediaUpload(t *testing.T, router http.Handler, contentID int64, clerkUserID string, data []byte, filename, partContentType string, extraPart bool) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := mediaUploadBody(t, data, filename, partContentType, extraPart)
	request := newMediaUploadRequest(contentID, clerkUserID, bytes.NewReader(body), contentType)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func mediaUploadBody(t *testing.T, data []byte, filename, partContentType string, extraPart bool) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	header.Set("Content-Type", partContentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if extraPart {
		extra, err := writer.CreateFormField("unexpected")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := extra.Write([]byte("extra")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func newMediaUploadRequest(contentID int64, clerkUserID string, body io.Reader, contentType string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/content/%d/media", contentID), body)
	request.Header.Set("Content-Type", contentType)
	claims := &clerk.SessionClaims{RegisteredClaims: clerk.RegisteredClaims{Subject: clerkUserID}}
	return request.WithContext(clerk.ContextWithSessionClaims(request.Context(), claims))
}

func performStreamingMediaUpload(t *testing.T, router http.Handler, contentID int64, clerkUserID string, size int64, prefix []byte) *httptest.ResponseRecorder {
	t.Helper()
	if size < int64(len(prefix)) {
		t.Fatalf("media stream size %d is smaller than prefix %d", size, len(prefix))
	}
	boundary := "mesh-test-boundary"
	contentType := "multipart/form-data; boundary=" + boundary
	header := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"clip.mp4\"\r\nContent-Type: application/octet-stream\r\n\r\n", boundary)
	footer := fmt.Sprintf("\r\n--%s--\r\n", boundary)
	body := io.MultiReader(strings.NewReader(header), bytes.NewReader(prefix), &repeatByteReader{remaining: size - int64(len(prefix))}, strings.NewReader(footer))
	request := newMediaUploadRequest(contentID, clerkUserID, body, contentType)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

type repeatByteReader struct {
	remaining int64
}

func (r *repeatByteReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > r.remaining {
		n = r.remaining
	}
	for i := range p[:n] {
		p[i] = 'x'
	}
	r.remaining -= n
	return int(n), nil
}

func concurrentMediaUpload(t *testing.T, router http.Handler, contentID int64, clerkUserID string) [2]int {
	t.Helper()
	body, contentType := mediaUploadBody(t, testMP4, "concurrent.mp4", "video/mp4", false)
	release := make(chan struct{})
	started := []chan struct{}{make(chan struct{}), make(chan struct{})}
	responses := [2]*httptest.ResponseRecorder{httptest.NewRecorder(), httptest.NewRecorder()}
	done := make(chan struct{}, len(responses))
	for i := range responses {
		requestBody := &gatedReader{reader: bytes.NewReader(body), started: started[i], release: release}
		request := newMediaUploadRequest(contentID, clerkUserID, requestBody, contentType)
		go func(i int) {
			router.ServeHTTP(responses[i], request)
			done <- struct{}{}
		}(i)
	}
	for _, signal := range started {
		select {
		case <-signal:
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent upload did not reach media read")
		}
	}
	close(release)
	for range responses {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent upload did not finish")
		}
	}
	return [2]int{responses[0].Code, responses[1].Code}
}

type gatedReader struct {
	reader  io.Reader
	started chan<- struct{}
	release <-chan struct{}
	once    sync.Once
}

func (r *gatedReader) Read(p []byte) (int, error) {
	r.once.Do(func() {
		close(r.started)
		<-r.release
	})
	return r.reader.Read(p)
}

func TestMediaUploadHelpers(t *testing.T) {
	if got := http.DetectContentType(testMP4); got != "video/mp4" {
		t.Fatalf("expected test fixture to sniff as video/mp4, got %q", got)
	}
	if got := originalFilename(`C:\\uploads\\clip.mp4`); got != "clip.mp4" {
		t.Fatalf("expected basename filename, got %q", got)
	}
}
