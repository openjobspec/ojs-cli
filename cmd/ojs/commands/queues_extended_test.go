package commands

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestQueues_Pause(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/ojs/v1/queues/emails/pause" {
			t.Errorf("path = %s, want /ojs/v1/queues/emails/pause", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"queue": "emails", "status": "paused"})
	})
	err := Queues(c, []string{"--pause", "emails"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueues_Resume(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/ojs/v1/queues/emails/resume" {
			t.Errorf("path = %s, want /ojs/v1/queues/emails/resume", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"queue": "emails", "status": "active"})
	})
	err := Queues(c, []string{"--resume", "emails"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueues_Stats_AllCounts(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"queue": "default", "status": "active",
			"stats": map[string]any{
				"available": 15, "active": 7, "completed": 100,
				"scheduled": 3, "retryable": 2, "dead": 1,
			},
		})
	})
	err := Queues(c, []string{"--stats", "default"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueues_List_Empty(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"queues": []any{}})
	})
	err := Queues(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueues_List_ServerError(t *testing.T) {
	t.Run("500", func(t *testing.T) {
		c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": "internal_error", "message": "server down"},
			})
		})
		err := Queues(c, []string{})
		if err == nil {
			t.Fatal("expected error for 500 response")
		}
	})

	t.Run("404", func(t *testing.T) {
		c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": "not_found", "message": "resource not found"},
			})
		})
		err := Queues(c, []string{"--stats", "nonexistent"})
		if err == nil {
			t.Fatal("expected error for 404 response")
		}
	})
}

func TestQueues_Pause_ServerError(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "internal_error", "message": "failed"},
		})
	})
	err := Queues(c, []string{"--pause", "default"})
	if err == nil {
		t.Fatal("expected error for server error on pause")
	}
}

func TestQueues_List_JSONOutput(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"queues": []map[string]any{
				{"name": "default", "status": "active"},
				{"name": "priority", "status": "paused"},
			},
		})
	})
	// output.Format is already "json" from init()
	err := Queues(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
