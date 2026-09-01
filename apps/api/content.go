package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
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
