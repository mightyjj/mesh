package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
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

	router := chi.NewRouter()
	router.Get("/health", healthHandler(db))

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
