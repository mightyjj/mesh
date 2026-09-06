package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/go-chi/chi/v5"
)

type content struct {
	ContentID int64     `json:"content_id"`
	CreatorID int64     `json:"-"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type createContentRequest struct {
	Title string `json:"title"`
}

const contentColumns = "content_id, creator_id, title, created_at, updated_at"

const (
	maxMediaBytes         int64 = 100 << 20
	maxMultipartOverhead  int64 = 1 << 20
	mediaProbeBytes             = 512
	defaultMediaDirectory       = "./media"
	mediaFieldName              = "file"
)

func isMaxBytesError(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

type mediaUploadResponse struct {
	ContentID        int64     `json:"content_id"`
	OriginalFilename string    `json:"original_filename"`
	MIMEType         string    `json:"mime_type"`
	ByteSize         int64     `json:"byte_size"`
	StorageKey       string    `json:"storage_key"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func mediaDirectory() string {
	if value := os.Getenv("MEDIA_DIR"); value != "" {
		return value
	}
	return defaultMediaDirectory
}

func originalFilename(filename string) string {
	filename = strings.ReplaceAll(filename, "\\", "/")
	filename = path.Base(filename)
	filename = strings.Map(func(r rune) rune {
		if r == 0 {
			return -1
		}
		return r
	}, filename)
	if filename == "" || filename == "." || filename == "/" {
		return "upload"
	}
	return filename
}

func uploadContentMediaHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")

		claims, ok := clerk.SessionClaimsFromContext(r.Context())
		if !ok || claims == nil || claims.Subject == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		contentID, err := strconv.ParseInt(chi.URLParam(r, "contentID"), 10, 64)
		if err != nil || contentID < 1 {
			writeError(w, http.StatusBadRequest, "invalid content ID")
			return
		}

		var existingStorageKey sql.NullString
		err = db.QueryRowContext(
			r.Context(),
			"SELECT c.storage_key FROM content c JOIN users u ON u.id = c.creator_id WHERE c.content_id = $1 AND u.clerk_user_id = $2",
			contentID,
			claims.Subject,
		).Scan(&existingStorageKey)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "content not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not verify content ownership")
			return
		}
		if existingStorageKey.Valid {
			writeError(w, http.StatusConflict, "content already has media")
			return
		}

		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			writeError(w, http.StatusBadRequest, `request must be multipart/form-data with one file field named "file"`)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxMediaBytes+maxMultipartOverhead)
		multipartReader, err := r.MultipartReader()
		if err != nil {
			writeError(w, http.StatusBadRequest, `request must be multipart/form-data with one file field named "file"`)
			return
		}
		part, err := multipartReader.NextPart()
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, `request must contain exactly one file field named "file"`)
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "malformed multipart request")
			return
		}
		if part.FormName() != mediaFieldName || part.FileName() == "" {
			writeError(w, http.StatusBadRequest, `request must contain exactly one file field named "file"`)
			return
		}
		filename := originalFilename(part.FileName())
		if strings.TrimSpace(filename) == "" {
			writeError(w, http.StatusBadRequest, "file name is required")
			return
		}

		directory := mediaDirectory()
		if err := os.MkdirAll(directory, 0750); err != nil {
			writeError(w, http.StatusInternalServerError, "could not prepare media storage")
			return
		}
		file, err := os.CreateTemp(directory, "media-")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not prepare media storage")
			return
		}
		storageKey := filepath.Base(file.Name())
		filePath := filepath.Join(directory, storageKey)
		keepFile := false
		fileClosed := false
		closeFile := func() error {
			if fileClosed {
				return nil
			}
			fileClosed = true
			return file.Close()
		}
		defer func() {
			if err := closeFile(); err != nil {
				log.Printf("close media file %q: %v", filePath, err)
			}
			if !keepFile {
				if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
					log.Printf("remove media file %q: %v", filePath, err)
				}
			}
		}()

		probe := make([]byte, mediaProbeBytes)
		probeLength, probeErr := io.ReadFull(part, probe)
		if probeErr != nil && !errors.Is(probeErr, io.EOF) && !errors.Is(probeErr, io.ErrUnexpectedEOF) {
			if isMaxBytesError(probeErr) {
				writeError(w, http.StatusRequestEntityTooLarge, "file exceeds the 100 MiB limit")
				return
			}
			writeError(w, http.StatusBadRequest, "malformed multipart request")
			return
		}
		probe = probe[:probeLength]
		written, err := file.Write(probe)
		if err != nil || written != len(probe) {
			writeError(w, http.StatusInternalServerError, "could not store media")
			return
		}
		mediaSize := int64(written)
		remaining := maxMediaBytes - mediaSize
		copied, err := io.Copy(file, io.LimitReader(part, remaining+1))
		mediaSize += copied
		if isMaxBytesError(err) {
			writeError(w, http.StatusRequestEntityTooLarge, "file exceeds the 100 MiB limit")
			return
		}
		if err != nil {
			var pathError *os.PathError
			if errors.As(err, &pathError) || errors.Is(err, io.ErrShortWrite) {
				writeError(w, http.StatusInternalServerError, "could not store media")
				return
			}
			writeError(w, http.StatusBadRequest, "malformed multipart request")
			return
		}
		if mediaSize > maxMediaBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "file exceeds the 100 MiB limit")
			return
		}

		if _, err := multipartReader.NextPart(); err == nil {
			writeError(w, http.StatusBadRequest, `request must contain exactly one file field named "file"`)
			return
		} else if !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "malformed multipart request")
			return
		}

		detectedMIMEType := http.DetectContentType(probe)
		if detectedMIMEType != "video/mp4" && detectedMIMEType != "video/webm" {
			writeError(w, http.StatusUnsupportedMediaType, "file type must be video/mp4 or video/webm")
			return
		}
		if err := closeFile(); err != nil {
			writeError(w, http.StatusInternalServerError, "could not store media")
			return
		}

		var response mediaUploadResponse
		err = db.QueryRowContext(
			r.Context(),
			`UPDATE content
			 SET original_filename = $1, mime_type = $2, byte_size = $3, storage_key = $4, updated_at = CURRENT_TIMESTAMP
			 WHERE content_id = $5
			   AND creator_id = (SELECT id FROM users WHERE clerk_user_id = $6)
			   AND storage_key IS NULL
			 RETURNING content_id, original_filename, mime_type, byte_size, storage_key, updated_at`,
			filename,
			detectedMIMEType,
			mediaSize,
			storageKey,
			contentID,
			claims.Subject,
		).Scan(
			&response.ContentID,
			&response.OriginalFilename,
			&response.MIMEType,
			&response.ByteSize,
			&response.StorageKey,
			&response.UpdatedAt,
		)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusConflict, "content already has media")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not save media metadata")
			return
		}

		keepFile = true
		writeJSON(w, http.StatusCreated, response)
	}
}

func scanContent(scanner rowScanner, target *content) error {
	return scanner.Scan(&target.ContentID, &target.CreatorID, &target.Title, &target.CreatedAt, &target.UpdatedAt)
}

func createContentHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")

		claims, ok := clerk.SessionClaimsFromContext(r.Context())
		if !ok || claims == nil || claims.Subject == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		var input *createContentRequest
		if err := decoder.Decode(&input); err != nil || input == nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		input.Title = strings.TrimSpace(input.Title)
		if input.Title == "" {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}

		var created content
		err := scanContent(db.QueryRowContext(
			r.Context(),
			"INSERT INTO content (creator_id, title) SELECT id, $2 FROM users WHERE clerk_user_id = $1 RETURNING "+contentColumns,
			claims.Subject,
			input.Title,
		), &created)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusConflict, "current user is not initialized")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create content")
			return
		}

		writeJSON(w, http.StatusCreated, created)
	}
}

func listContentHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")

		claims, ok := clerk.SessionClaimsFromContext(r.Context())
		if !ok || claims == nil || claims.Subject == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		rows, err := db.QueryContext(
			r.Context(),
			"SELECT "+contentColumns+" FROM content WHERE creator_id = (SELECT id FROM users WHERE clerk_user_id = $1) ORDER BY content_id",
			claims.Subject,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not list content")
			return
		}
		defer rows.Close()

		items := []content{}
		for rows.Next() {
			var item content
			if err := scanContent(rows, &item); err != nil {
				writeError(w, http.StatusInternalServerError, "could not list content")
				return
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "could not list content")
			return
		}

		writeJSON(w, http.StatusOK, items)
	}
}
