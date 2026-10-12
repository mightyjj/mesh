package main

import (
	"database/sql"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/jackc/pgx/v5/pgconn"
)

const platformAccountColumns = "platform_account_id, platform, external_account_id, display_name, created_at, updated_at"
const platformAccountIdentityConstraint = "platform_accounts_platform_external_account_id_key"

type platformAccount struct {
	PlatformAccountID int64     `json:"platform_account_id"`
	Platform          string    `json:"platform"`
	ExternalAccountID string    `json:"external_account_id"`
	DisplayName       string    `json:"display_name"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func scanPlatformAccount(scanner rowScanner, target *platformAccount) error {
	return scanner.Scan(
		&target.PlatformAccountID,
		&target.Platform,
		&target.ExternalAccountID,
		&target.DisplayName,
		&target.CreatedAt,
		&target.UpdatedAt,
	)
}

func createPlatformAccountHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")

		claims, ok := clerk.SessionClaimsFromContext(r.Context())
		if !ok || claims == nil || claims.Subject == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1))
		if err != nil || len(body) != 0 {
			writeError(w, http.StatusBadRequest, "request body must be empty")
			return
		}

		var created platformAccount
		err = scanPlatformAccount(db.QueryRowContext(
			r.Context(),
			"INSERT INTO platform_accounts (user_id, platform, external_account_id, display_name) SELECT id, 'fake', 'fake-user-' || id::text, 'Fake Account ' || id::text FROM users WHERE clerk_user_id = $1 RETURNING "+platformAccountColumns,
			claims.Subject,
		), &created)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusConflict, "current user is not initialized")
			return
		}
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == platformAccountIdentityConstraint {
				writeError(w, http.StatusConflict, "fake platform account already exists")
				return
			}
			writeError(w, http.StatusInternalServerError, "could not create fake platform account")
			return
		}

		writeJSON(w, http.StatusCreated, created)
	}
}
