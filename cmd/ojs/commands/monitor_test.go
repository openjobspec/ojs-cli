package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/config"
)

func newMonitorTestClient(handler http.Handler) *client.Client {
	server := httptest.NewServer(handler)
	return client.New(&config.Config{ServerURL: server.URL})
}

func TestRenderDashboard_HealthResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ojs/v1/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok", "version": "1.0.0", "uptime_seconds": 120,
			"backend": map[string]any{"type": "redis", "status": "connected"},
		})
	})
	mux.HandleFunc("/ojs/v1/queues", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"queues": []any{}})
	})
	mux.HandleFunc("/ojs/v1/dead-letter", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"jobs": []any{}, "total": 0})
	})

	c := newMonitorTestClient(mux)
	err := renderDashboard(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderDashboard_QueueStats(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ojs/v1/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok", "version": "1.0.0", "uptime_seconds": 60,
		})
	})
	mux.HandleFunc("/ojs/v1/queues", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"queues": []map[string]any{
				{"name": "default", "status": "active"},
				{"name": "priority", "status": "active"},
			},
		})
	})
	mux.HandleFunc("/ojs/v1/queues/default/stats", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"stats": map[string]any{"available": 10, "active": 3, "scheduled": 2, "retryable": 1, "dead": 0},
		})
	})
	mux.HandleFunc("/ojs/v1/queues/priority/stats", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"stats": map[string]any{"available": 5, "active": 1, "scheduled": 0, "retryable": 0, "dead": 2},
		})
	})
	mux.HandleFunc("/ojs/v1/dead-letter", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"jobs": []any{}, "total": 0})
	})

	c := newMonitorTestClient(mux)
	err := renderDashboard(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderDashboard_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ojs/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "internal_error", "message": "db down"},
		})
	})

	c := newMonitorTestClient(mux)
	// renderDashboard prints error inline but returns nil for health errors
	err := renderDashboard(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderDashboard_EmptyQueueList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ojs/v1/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok", "version": "2.0.0", "uptime_seconds": 300,
		})
	})
	mux.HandleFunc("/ojs/v1/queues", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"queues": []any{}})
	})
	mux.HandleFunc("/ojs/v1/dead-letter", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"jobs": []any{}, "total": 0})
	})

	c := newMonitorTestClient(mux)
	err := renderDashboard(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderDashboard_WithDeadLetterItems(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ojs/v1/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok", "version": "1.0.0", "uptime_seconds": 60,
		})
	})
	mux.HandleFunc("/ojs/v1/queues", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"queues": []any{}})
	})
	mux.HandleFunc("/ojs/v1/dead-letter", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"jobs":  []map[string]any{{"id": "dead-1", "type": "email.send"}},
			"total": 5,
		})
	})

	c := newMonitorTestClient(mux)
	err := renderDashboard(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderDashboard_QueueStatsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ojs/v1/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok", "version": "1.0.0", "uptime_seconds": 10,
		})
	})
	mux.HandleFunc("/ojs/v1/queues", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"queues": []map[string]any{{"name": "broken", "status": "active"}},
		})
	})
	mux.HandleFunc("/ojs/v1/queues/broken/stats", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "internal_error", "message": "stats unavailable"},
		})
	})
	mux.HandleFunc("/ojs/v1/dead-letter", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"jobs": []any{}, "total": 0})
	})

	c := newMonitorTestClient(mux)
	err := renderDashboard(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
