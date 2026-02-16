package commands

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestDeadLetter_RetryByID(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/ojs/v1/dead-letter/dead-job-1/retry" {
			t.Errorf("path = %s, want /ojs/v1/dead-letter/dead-job-1/retry", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "dead-job-1", "state": "available", "attempt": 1,
		})
	})
	err := DeadLetter(c, []string{"--retry", "dead-job-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeadLetter_DeleteByID(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/ojs/v1/dead-letter/dead-job-2" {
			t.Errorf("path = %s, want /ojs/v1/dead-letter/dead-job-2", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"deleted": true})
	})
	err := DeadLetter(c, []string{"--delete", "dead-job-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeadLetter_List_CustomLimit(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "limit=10" {
			t.Errorf("query = %s, want limit=10", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"jobs": []map[string]any{
				{"id": "dj-1", "type": "email.send", "queue": "default", "attempt": 3, "discarded_at": "2026-01-01T00:00:00Z"},
				{"id": "dj-2", "type": "report.gen", "queue": "reports", "attempt": 5, "discarded_at": "2026-01-02T00:00:00Z"},
			},
			"total": 2,
		})
	})
	err := DeadLetter(c, []string{"--limit", "10"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeadLetter_List_EmptyQueue(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"jobs":  []any{},
			"total": 0,
		})
	})
	err := DeadLetter(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeadLetter_Retry_ServerError(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "internal_error", "message": "retry failed"},
		})
	})
	err := DeadLetter(c, []string{"--retry", "dead-job-1"})
	if err == nil {
		t.Fatal("expected error for server error on retry")
	}
}

func TestDeadLetter_Delete_ServerError(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "internal_error", "message": "delete failed"},
		})
	})
	err := DeadLetter(c, []string{"--delete", "dead-job-1"})
	if err == nil {
		t.Fatal("expected error for server error on delete")
	}
}

func TestDeadLetter_List_ServerError(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "not_found", "message": "endpoint not found"},
		})
	})
	err := DeadLetter(c, []string{})
	if err == nil {
		t.Fatal("expected error for server error on list")
	}
}
