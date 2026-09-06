package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

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
	apiProxy := http.StripPrefix("/api", proxy)
	router := chi.NewRouter()
	router.Get("/health", healthHandler)
	router.Get("/api/health", apiProxy.ServeHTTP)
	router.Get("/api/me", apiProxy.ServeHTTP)
	router.Get("/api/content", apiProxy.ServeHTTP)
	router.Post("/api/content", apiProxy.ServeHTTP)
	router.Post("/api/content/{contentID}/media", apiProxy.ServeHTTP)
	return router
}

func apiURL() string {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		return "http://localhost:8080"
	}
	return apiURL
}

func main() {
	if err := http.ListenAndServe(":8081", gatewayHandler(apiURL())); err != nil {
		log.Fatal(err)
	}
}
