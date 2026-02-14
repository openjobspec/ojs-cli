package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openjobspec/ojs-cli/internal/config"
)

func TestClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/ojs/v1/health" {
			t.Errorf("path = %s, want /ojs/v1/health", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/openjobspec+json" {
			t.Errorf("missing Accept header")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	c := New(&config.Config{ServerURL: server.URL})
	data, status, err := c.Get("/health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
	var result map[string]string
	json.Unmarshal(data, &result)
	if result["status"] != "ok" {
		t.Errorf("status = %q, want %q", result["status"], "ok")
	}
}

func TestClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/openjobspec+json" {
			t.Errorf("missing Content-Type header")
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["type"] != "email.send" {
			t.Errorf("type = %q, want %q", body["type"], "email.send")
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "job-123", "state": "available"})
	}))
	defer server.Close()

	c := New(&config.Config{ServerURL: server.URL})
	data, status, err := c.Post("/jobs", map[string]string{"type": "email.send"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 201 {
		t.Errorf("status = %d, want 201", status)
	}
	var result map[string]string
	json.Unmarshal(data, &result)
	if result["id"] != "job-123" {
		t.Errorf("id = %q, want %q", result["id"], "job-123")
	}
}

func TestClient_Delete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"state": "cancelled"})
	}))
	defer server.Close()

	c := New(&config.Config{ServerURL: server.URL})
	_, status, err := c.Delete("/jobs/abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
}

func TestClient_AuthToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer server.Close()

	c := New(&config.Config{ServerURL: server.URL, AuthToken: "test-token"})
	_, _, err := c.Get("/health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    "not_found",
				"message": "job 'xyz' not found",
			},
		})
	}))
	defer server.Close()

	c := New(&config.Config{ServerURL: server.URL})
	_, status, err := c.Get("/jobs/xyz")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status != 404 {
		t.Errorf("status = %d, want 404", status)
	}
	if err.Error() != "not_found: job 'xyz' not found" {
		t.Errorf("error = %q", err.Error())
	}
}

func TestClient_ConnectionError(t *testing.T) {
	c := New(&config.Config{ServerURL: "http://localhost:1"})
	_, _, err := c.Get("/health")
	if err == nil {
		t.Fatal("expected connection error")
	}
}
