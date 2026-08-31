package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHealth(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/health", healthHandler)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if body := recorder.Body.String(); body != `{"status":"ok"}` {
		t.Fatalf("expected body %q, got %q", `{"status":"ok"}`, body)
	}
}

func TestGatewayProxiesHealthToAPI(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected method %s, got %s", http.MethodGet, r.Method)
		}
		if r.URL.Path != "/health" {
			t.Errorf("expected path /health, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"from-api"}`))
	}))
	defer api.Close()

	gateway := httptest.NewServer(gatewayHandler(api.URL))
	defer gateway.Close()

	response, err := http.Get(gateway.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"status":"from-api"}` {
		t.Fatalf("expected API response, got %q", body)
	}
}

func TestGatewayProxiesUsersToAPI(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected method %s, got %s", http.MethodPost, r.Method)
		}
		if r.URL.Path != "/users" {
			t.Errorf("expected path /users, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer api.Close()

	gateway := httptest.NewServer(gatewayHandler(api.URL))
	defer gateway.Close()

	request, err := http.NewRequest(http.MethodPost, gateway.URL+"/api/users", strings.NewReader(`{"email":"test@example.com"}`))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"id":1}` {
		t.Fatalf("expected API response, got %q", body)
	}
}

func TestAPIURL(t *testing.T) {
	t.Setenv("API_URL", "http://api:8080")
	if got := apiURL(); got != "http://api:8080" {
		t.Fatalf("expected Compose API URL, got %q", got)
	}
}
