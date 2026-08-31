package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type fakePinger struct {
	err    error
	called bool
}

func (p *fakePinger) PingContext(context.Context) error {
	p.called = true
	return p.err
}

func TestHealthChecksDatabase(t *testing.T) {
	for _, test := range []struct {
		name       string
		err        error
		statusCode int
		body       string
	}{
		{name: "healthy", statusCode: http.StatusOK, body: `{"status":"ok"}`},
		{name: "unavailable", err: errors.New("database unavailable"), statusCode: http.StatusServiceUnavailable, body: `{"status":"error"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			pinger := &fakePinger{err: test.err}
			router := chi.NewRouter()
			router.Get("/health", healthHandler(pinger))

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			router.ServeHTTP(recorder, request)

			if !pinger.called {
				t.Fatal("expected health check to ping the database")
			}
			if recorder.Code != test.statusCode {
				t.Fatalf("expected status %d, got %d", test.statusCode, recorder.Code)
			}
			if body := recorder.Body.String(); body != test.body {
				t.Fatalf("expected body %q, got %q", test.body, body)
			}
		})
	}
}

func TestMeRejectsAnonymousRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	newRouter(nil).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	if body := recorder.Body.String(); body != `{"error":"authentication required"}` {
		t.Fatalf("expected body %q, got %q", `{"error":"authentication required"}`, body)
	}
}
