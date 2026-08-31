package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type createUserRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

const userColumns = "id, email, display_name, created_at, updated_at"

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

func createUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input createUserRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		if err := decoder.Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		input.Email = strings.TrimSpace(input.Email)
		input.DisplayName = strings.TrimSpace(input.DisplayName)
		if input.Email == "" || input.DisplayName == "" {
			writeError(w, http.StatusBadRequest, "email and display_name are required")
			return
		}

		var created user
		err := db.QueryRowContext(
			r.Context(),
			"INSERT INTO users (email, display_name) VALUES ($1, $2) RETURNING "+userColumns,
			input.Email,
			input.DisplayName,
		).Scan(&created.ID, &created.Email, &created.DisplayName, &created.CreatedAt, &created.UpdatedAt)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeError(w, http.StatusConflict, "email already exists")
				return
			}
			writeError(w, http.StatusInternalServerError, "could not create user")
			return
		}

		writeJSON(w, http.StatusCreated, created)
	}
}

func getUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id < 1 {
			writeError(w, http.StatusBadRequest, "invalid user id")
			return
		}

		var found user
		err = db.QueryRowContext(
			r.Context(),
			"SELECT "+userColumns+" FROM users WHERE id = $1",
			id,
		).Scan(&found.ID, &found.Email, &found.DisplayName, &found.CreatedAt, &found.UpdatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not fetch user")
			return
		}

		writeJSON(w, http.StatusOK, found)
	}
}

func newRouter(db *sql.DB) http.Handler {
	router := chi.NewRouter()
	router.Get("/health", healthHandler(db))
	router.Post("/users", createUserHandler(db))
	router.Get("/users/{id}", getUserHandler(db))
	return router
}

func databaseURL() string {
	if value := os.Getenv("DATABASE_URL"); value != "" {
		return value
	}
	return "postgres://mesh:mesh@localhost:5432/mesh?sslmode=disable"
}

func main() {
	db, err := sql.Open("pgx", databaseURL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := http.ListenAndServe(":8080", newRouter(db)); err != nil {
		log.Fatal(err)
	}
}
