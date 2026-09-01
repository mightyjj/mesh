package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	clerkuser "github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type dbPinger interface {
	PingContext(context.Context) error
}

func healthHandler(db dbPinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := db.PingContext(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"error"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

type user struct {
	ID          int64     `json:"id"`
	ClerkUserID string    `json:"-"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

const userColumns = "id, clerk_user_id, email, display_name, created_at, updated_at"

var clerkAuthorizedParties = []string{"http://localhost:3000"}

type rowScanner interface {
	Scan(...any) error
}

func scanUser(scanner rowScanner, target *user) error {
	var clerkUserID sql.NullString
	err := scanner.Scan(&target.ID, &clerkUserID, &target.Email, &target.DisplayName, &target.CreatedAt, &target.UpdatedAt)
	target.ClerkUserID = clerkUserID.String
	return err
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func clerkUserProfile(ctx context.Context, clerkUserID string) (string, string, error) {
	profile, err := clerkuser.Get(ctx, clerkUserID)
	if err != nil {
		return "", "", err
	}

	email := ""
	if profile.PrimaryEmailAddressID != nil {
		for _, address := range profile.EmailAddresses {
			if address != nil && address.ID == *profile.PrimaryEmailAddressID && address.Verification != nil && address.Verification.Status == "verified" {
				email = address.EmailAddress
				break
			}
		}
	}
	if email == "" {
		for _, address := range profile.EmailAddresses {
			if address != nil && address.Verification != nil && address.Verification.Status == "verified" {
				email = address.EmailAddress
				break
			}
		}
	}

	parts := make([]string, 0, 2)
	if profile.FirstName != nil {
		parts = append(parts, strings.TrimSpace(*profile.FirstName))
	}
	if profile.LastName != nil {
		parts = append(parts, strings.TrimSpace(*profile.LastName))
	}
	displayName := strings.TrimSpace(strings.Join(parts, " "))
	if displayName == "" && profile.Username != nil {
		displayName = strings.TrimSpace(*profile.Username)
	}
	if displayName == "" {
		displayName = email
	}
	if email == "" {
		return "", "", fmt.Errorf("Clerk user %q has no email", clerkUserID)
	}

	return email, displayName, nil
}

func meHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")

		claims, ok := clerk.SessionClaimsFromContext(r.Context())
		if !ok || claims == nil || claims.Subject == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		var found user
		err := scanUser(db.QueryRowContext(
			r.Context(),
			"SELECT "+userColumns+" FROM users WHERE clerk_user_id = $1",
			claims.Subject,
		), &found)
		if err == nil {
			writeJSON(w, http.StatusOK, found)
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, "could not fetch current user")
			return
		}

		email, displayName, err := clerkUserProfile(r.Context(), claims.Subject)
		if err != nil {
			writeError(w, http.StatusBadGateway, "could not load Clerk user")
			return
		}

		err = scanUser(db.QueryRowContext(
			r.Context(),
			"INSERT INTO users (clerk_user_id, email, display_name) VALUES ($1, $2, $3) ON CONFLICT (clerk_user_id) DO UPDATE SET email = EXCLUDED.email, display_name = EXCLUDED.display_name, updated_at = CURRENT_TIMESTAMP RETURNING "+userColumns,
			claims.Subject,
			email,
			displayName,
		), &found)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeError(w, http.StatusConflict, "could not map Clerk user")
				return
			}
			writeError(w, http.StatusInternalServerError, "could not save current user")
			return
		}

		writeJSON(w, http.StatusOK, found)
	}
}

func newRouter(db *sql.DB) http.Handler {
	router := chi.NewRouter()
	router.Get("/health", healthHandler(db))
	router.Get("/me", requireClerkAuthorization(meHandler(db)).ServeHTTP)
	router.Get("/content", requireClerkAuthorization(listContentHandler(db)).ServeHTTP)
	router.Post("/content", requireClerkAuthorization(createContentHandler(db)).ServeHTTP)
	return router
}

func requireClerkAuthorization(next http.Handler) http.Handler {
	return clerkhttp.WithHeaderAuthorization(
		clerkhttp.AuthorizedPartyMatches(clerkAuthorizedParties...),
		clerkhttp.AuthorizationFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusUnauthorized, "invalid authentication")
		})),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := clerk.SessionClaimsFromContext(r.Context()); !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func databaseURL() string {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		return value
	}
	return "postgres://mesh:mesh@localhost:5432/mesh?sslmode=disable"
}

func main() {
	secretKey := os.Getenv("CLERK_SECRET_KEY")
	if secretKey == "" {
		log.Fatal("CLERK_SECRET_KEY is required")
	}
	clerk.SetKey(secretKey)

	db, err := sql.Open("pgx", databaseURL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := http.ListenAndServe(":8080", newRouter(db)); err != nil {
		log.Fatal(err)
	}
}
