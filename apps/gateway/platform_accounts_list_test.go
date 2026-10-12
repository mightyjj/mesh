package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewayProxiesPlatformAccountList(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected method %s, got %s", http.MethodGet, r.Method)
		}
		if r.URL.Path != "/platform-accounts" {
			t.Errorf("expected path /platform-accounts, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer account-list-token" {
			t.Errorf("expected authorization header to pass through, got %q", got)
		}
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = w.Write([]byte(`[{"platform_account_id":42,"platform":"fake"}]`))
	}))
	defer api.Close()

	gateway := httptest.NewServer(gatewayHandler(api.URL))
	defer gateway.Close()

	request, err := http.NewRequest(http.MethodGet, gateway.URL+"/api/platform-accounts", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer account-list-token")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.StatusCode)
	}
	if got := response.Header.Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("expected cache control header to pass through, got %q", got)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `[{"platform_account_id":42,"platform":"fake"}]` {
		t.Fatalf("expected API response, got %q", body)
	}
}
