package commands

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestWorkers_List_MultipleWorkers(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/ojs/v1/admin/workers" {
			t.Errorf("path = %s, want /ojs/v1/admin/workers", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{
					"id": "worker-1", "state": "running", "directive": "none",
					"active_jobs": 3, "last_heartbeat": "2026-01-01T00:00:00Z",
				},
				{
					"id": "worker-2", "state": "running", "directive": "quiet",
					"active_jobs": 1, "last_heartbeat": "2026-01-01T00:00:05Z",
				},
				{
					"id": "worker-3", "state": "stale", "directive": "none",
					"active_jobs": 0, "last_heartbeat": "2025-12-31T23:50:00Z",
				},
			},
			"summary": map[string]any{
				"total": 3, "running": 2, "quiet": 1, "stale": 1,
			},
		})
	})
	err := Workers(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkers_Quiet_Directive(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/ojs/v1/admin/workers/directive" {
			t.Errorf("path = %s, want /ojs/v1/admin/workers/directive", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["directive"] != "quiet" {
			t.Errorf("directive = %v, want quiet", body["directive"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"directive": "quiet", "affected_workers": 3})
	})
	err := Workers(c, []string{"--quiet"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkers_Resume_Directive(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["directive"] != "resume" {
			t.Errorf("directive = %v, want resume", body["directive"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"directive": "resume", "affected_workers": 2})
	})
	err := Workers(c, []string{"--resume"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkers_List_Empty(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"items": []any{},
			"summary": map[string]any{
				"total": 0, "running": 0, "quiet": 0, "stale": 0,
			},
		})
	})
	err := Workers(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkers_List_CorrectFields(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{
					"id":             "wk-abc123",
					"state":          "running",
					"directive":      "none",
					"active_jobs":    5,
					"last_heartbeat": "2026-06-15T10:30:00Z",
				},
			},
			"summary": map[string]any{
				"total": 1, "running": 1, "quiet": 0, "stale": 0,
			},
		})
	})
	err := Workers(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkers_List_ServerError(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "internal_error", "message": "service unavailable"},
		})
	})
	err := Workers(c, []string{})
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestWorkers_Quiet_ServerError(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "internal_error", "message": "failed to set directive"},
		})
	})
	err := Workers(c, []string{"--quiet"})
	if err == nil {
		t.Fatal("expected error for server error on quiet")
	}
}
