package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func main() {
	router := chi.NewRouter()
	router.Get("/health", healthHandler)

	if err := http.ListenAndServe(":8081", router); err != nil {
		log.Fatal(err)
	}
}
