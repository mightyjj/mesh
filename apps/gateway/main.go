package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/go-chi/chi/v5"
)

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func gatewayHandler(apiURL string) http.Handler {
	target, err := url.Parse(apiURL)
	if err != nil {
		panic(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	router := chi.NewRouter()
	router.Get("/health", healthHandler)
	router.Get("/api/health", http.StripPrefix("/api", proxy).ServeHTTP)
	return router
}

func main() {
	if err := http.ListenAndServe(":8081", gatewayHandler("http://localhost:8080")); err != nil {
		log.Fatal(err)
	}
}
