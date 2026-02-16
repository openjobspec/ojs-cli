package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openjobspec/ojs-cli/internal/config"
)

func TestClient_CustomTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	c := New(&config.Config{ServerURL: server.URL})
	if c.http.Timeout <= 0 {
		t.Errorf("timeout should be positive, got %v", c.http.Timeout)
	}
	_, status, err := c.Get("/health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
}

func TestClient_QueryParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "limit=10&state=active" {
			t.Errorf("query = %q, want %q", r.URL.RawQuery, "limit=10&state=active")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	}))
	defer server.Close()

	c := New(&config.Config{ServerURL: server.URL})
	_, status, err := c.Get("/jobs?limit=10&state=active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
}

func TestClient_RateLimiting_429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    "rate_limited",
				"message": "too many requests",
			},
		})
	}))
	defer server.Close()

	c := New(&config.Config{ServerURL: server.URL})
	_, status, err := c.Get("/jobs")
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
	if status != 429 {
		t.Errorf("status = %d, want 429", status)
	}
	if !strings.Contains(err.Error(), "rate_limited") {
		t.Errorf("error = %q, want to contain 'rate_limited'", err.Error())
	}
}

func TestClient_Various4xx5xxErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errorCode  string
		message    string
	}{
		{"bad_request", 400, "bad_request", "invalid json"},
		{"unauthorized", 401, "unauthorized", "invalid token"},
		{"forbidden", 403, "forbidden", "insufficient permissions"},
		{"conflict", 409, "conflict", "job already exists"},
		{"service_unavailable", 503, "service_unavailable", "backend down"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    tt.errorCode,
						"message": tt.message,
					},
				})
			}))
			defer server.Close()

			c := New(&config.Config{ServerURL: server.URL})
			_, status, err := c.Get("/test")
			if err == nil {
				t.Fatalf("expected error for %d response", tt.statusCode)
			}
			if status != tt.statusCode {
				t.Errorf("status = %d, want %d", status, tt.statusCode)
			}
			if !strings.Contains(err.Error(), tt.errorCode) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.errorCode)
			}
		})
	}
}

func TestClient_ConnectionReset(t *testing.T) {
	c := New(&config.Config{ServerURL: "http://127.0.0.1:1"})
	_, _, err := c.Get("/health")
	if err == nil {
		t.Fatal("expected connection error")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("error = %q, want to contain 'request failed'", err.Error())
	}
}

func TestClient_TrailingSlashHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ojs/v1/health" {
			t.Errorf("path = %s, want /ojs/v1/health", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	// ServerURL with trailing slash — BaseURL() appends /ojs/v1 directly
	c := New(&config.Config{ServerURL: server.URL})
	_, status, err := c.Get("/health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
}

func TestClient_Post_NilBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"result": "ok"})
	}))
	defer server.Close()

	c := New(&config.Config{ServerURL: server.URL})
	_, status, err := c.Post("/action", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
}

func TestClient_ErrorResponse_NoBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("bad gateway"))
	}))
	defer server.Close()

	c := New(&config.Config{ServerURL: server.URL})
	_, status, err := c.Get("/test")
	if err == nil {
		t.Fatal("expected error for 502 response")
	}
	if status != 502 {
		t.Errorf("status = %d, want 502", status)
	}
	if !strings.Contains(err.Error(), "HTTP 502") {
		t.Errorf("error = %q, want to contain 'HTTP 502'", err.Error())
	}
}
