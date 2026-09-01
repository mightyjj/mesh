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

func TestGatewayProxiesMeToAPI(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected method %s, got %s", http.MethodGet, r.Method)
		}
		if r.URL.Path != "/me" {
			t.Errorf("expected path /me, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"authentication required"}`))
	}))
	defer api.Close()

	gateway := httptest.NewServer(gatewayHandler(api.URL))
	defer gateway.Close()

	response, err := http.Get(gateway.URL + "/api/me")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"error":"authentication required"}` {
		t.Fatalf("expected API response, got %q", body)
	}

	request, err := http.NewRequest(http.MethodPost, gateway.URL+"/api/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, response.StatusCode)
	}
}

func TestGatewayProxiesContentMethodsAndRequest(t *testing.T) {
	var calls int
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/content" {
			t.Errorf("expected path /content, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("expected authorization header to pass through, got %q", got)
		}
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != `{"title":"through gateway"}` {
				t.Errorf("expected request body to pass through, got %q", body)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"title":"through gateway"}`))
	}))
	defer api.Close()

	gateway := httptest.NewServer(gatewayHandler(api.URL))
	defer gateway.Close()

	for _, test := range []struct {
		method string
		body   string
	}{
		{http.MethodGet, ""},
		{http.MethodPost, `{"title":"through gateway"}`},
	} {
		request, err := http.NewRequest(test.method, gateway.URL+"/api/content", strings.NewReader(test.body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer test-token")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("expected status %d for %s, got %d", http.StatusOK, test.method, response.StatusCode)
		}
		if string(body) != `{"id":1,"title":"through gateway"}` {
			t.Fatalf("expected API response for %s, got %q", test.method, body)
		}
	}

	request, err := http.NewRequest(http.MethodPut, gateway.URL+"/api/content", strings.NewReader(`{"title":"rejected"}`))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for unsupported method, got %d", http.StatusMethodNotAllowed, response.StatusCode)
	}
	if calls != 2 {
		t.Fatalf("expected only GET and POST to reach API, got %d calls", calls)
	}
}

func TestAPIURL(t *testing.T) {
	t.Setenv("API_URL", "http://api:8080")
	if got := apiURL(); got != "http://api:8080" {
		t.Fatalf("expected Compose API URL, got %q", got)
	}
}
