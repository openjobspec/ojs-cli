package commands

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// --- Webhooks command tests ---

func TestWebhooks_NoSubcommand(t *testing.T) {
	c := newTestClient(nil)
	err := Webhooks(c, []string{})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestWebhooks_Create_MissingFields(t *testing.T) {
	c := newTestClient(nil)
	err := Webhooks(c, []string{"create"})
	if err == nil {
		t.Fatal("expected error for missing --url and --events")
	}
}

func TestWebhooks_Create_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["url"] != "https://example.com/hooks" {
			t.Errorf("url = %v, want https://example.com/hooks", body["url"])
		}
		events := body["events"].([]any)
		if len(events) != 2 {
			t.Errorf("events len = %d, want 2", len(events))
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "wh-1", "url": "https://example.com/hooks",
		})
	})
	err := Webhooks(c, []string{"create", "--url", "https://example.com/hooks", "--events", "job.completed,job.failed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebhooks_List(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"subscriptions": []map[string]any{
				{"id": "wh-1", "url": "https://example.com", "events": []string{"job.completed"}, "active": true},
			},
			"total": 1,
		})
	})
	err := Webhooks(c, []string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebhooks_Get_MissingID(t *testing.T) {
	c := newTestClient(nil)
	err := Webhooks(c, []string{"get"})
	if err == nil {
		t.Fatal("expected error for missing subscription ID")
	}
}

func TestWebhooks_Get_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "wh-1", "url": "https://example.com", "events": []string{"job.completed"},
			"active": true, "success_count": 10, "failure_count": 1,
		})
	})
	err := Webhooks(c, []string{"get", "wh-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebhooks_Delete_MissingID(t *testing.T) {
	c := newTestClient(nil)
	err := Webhooks(c, []string{"delete"})
	if err == nil {
		t.Fatal("expected error for missing subscription ID")
	}
}

func TestWebhooks_Delete_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"deleted": true})
	})
	err := Webhooks(c, []string{"delete", "wh-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebhooks_Test_MissingID(t *testing.T) {
	c := newTestClient(nil)
	err := Webhooks(c, []string{"test"})
	if err == nil {
		t.Fatal("expected error for missing subscription ID")
	}
}

func TestWebhooks_Test_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"status_code": 200, "success": true})
	})
	err := Webhooks(c, []string{"test", "wh-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebhooks_RotateSecret_MissingID(t *testing.T) {
	c := newTestClient(nil)
	err := Webhooks(c, []string{"rotate-secret"})
	if err == nil {
		t.Fatal("expected error for missing subscription ID")
	}
}

func TestWebhooks_RotateSecret_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"new_secret": "whsec_newkey123"})
	})
	err := Webhooks(c, []string{"rotate-secret", "wh-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Stats command tests ---

func TestStats_Overview(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/ojs/v1/admin/stats") {
			t.Errorf("path = %s, want prefix /ojs/v1/admin/stats", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"queues":  map[string]any{"total": 3, "active": 2, "paused": 1},
			"workers": map[string]any{"total": 5, "running": 4, "quiet": 1, "stale": 0},
			"jobs":    map[string]any{"available": 10, "active": 3, "completed": 100, "retryable": 2, "scheduled": 5, "discarded": 1, "cancelled": 0},
			"throughput": map[string]any{"enqueued_per_min": 20, "completed_per_min": 18, "failed_per_min": 2, "avg_latency_ms": 15.5},
		})
	})
	err := Stats(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStats_History(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/history") {
			t.Errorf("path = %s, want contains /history", r.URL.Path)
		}
		if r.URL.Query().Get("period") != "5m" {
			t.Errorf("period = %s, want 5m", r.URL.Query().Get("period"))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"period": "5m",
			"data_points": []map[string]any{
				{"timestamp": "2026-01-01T10:00:00Z", "enqueued": 5, "completed": 4, "failed": 1, "avg_latency_ms": 12.0},
			},
		})
	})
	err := Stats(c, []string{"--history", "--period", "5m"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStats_WithQueueFilter(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("queue") != "billing" {
			t.Errorf("queue = %s, want billing", r.URL.Query().Get("queue"))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"queues": map[string]any{"total": 1, "active": 1, "paused": 0},
			"workers": map[string]any{"total": 0, "running": 0, "quiet": 0, "stale": 0},
			"jobs": map[string]any{"available": 0, "active": 0, "completed": 0, "retryable": 0, "scheduled": 0, "discarded": 0, "cancelled": 0},
			"throughput": map[string]any{"enqueued_per_min": 0, "completed_per_min": 0, "failed_per_min": 0, "avg_latency_ms": 0},
		})
	})
	err := Stats(c, []string{"--queue", "billing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Retry command tests ---

func TestRetry_MissingID(t *testing.T) {
	c := newTestClient(nil)
	err := Retry(c, []string{})
	if err == nil {
		t.Fatal("expected error for missing job ID")
	}
}

func TestRetry_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/retry") {
			t.Errorf("path = %s, want suffix /retry", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"id": "job-1", "state": "available"})
	})
	err := Retry(c, []string{"job-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Bulk delete tests ---

func TestBulk_Delete_MissingIDsOrState(t *testing.T) {
	c := newTestClient(nil)
	err := Bulk(c, []string{"delete"})
	if err == nil {
		t.Fatal("expected error for missing --ids or --state")
	}
}

func TestBulk_Delete_ByIDs(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		ids := body["job_ids"].([]any)
		if len(ids) != 2 {
			t.Errorf("job_ids len = %d, want 2", len(ids))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"deleted": 2, "failed": 0})
	})
	err := Bulk(c, []string{"delete", "--ids", "job-1,job-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBulk_Delete_ByState(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		filter := body["filter"].(map[string]any)
		if filter["state"] != "completed" {
			t.Errorf("state = %v, want completed", filter["state"])
		}
		if filter["older_than"] != "7d" {
			t.Errorf("older_than = %v, want 7d", filter["older_than"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"deleted": 50, "failed": 0})
	})
	err := Bulk(c, []string{"delete", "--state", "completed", "--older-than", "7d"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Worker management tests ---

func TestWorkers_Detail(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "wk-1", "state": "running", "directive": "none",
			"active_jobs": 3, "queues": []string{"default", "billing"},
			"hostname": "worker-01", "pid": 12345,
			"last_heartbeat": "2026-01-01T10:00:00Z", "started_at": "2026-01-01T09:00:00Z",
		})
	})
	err := Workers(c, []string{"--detail", "wk-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkers_QuietWorker(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/quiet") {
			t.Errorf("path = %s, want suffix /quiet", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"directive": "quiet"})
	})
	err := Workers(c, []string{"--quiet-worker", "wk-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkers_Deregister(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"deregistered": true})
	})
	err := Workers(c, []string{"--deregister", "wk-stale"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Cron detail and update tests ---

func TestCron_Detail(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"name": "daily-report", "expression": "0 9 * * *", "enabled": true,
			"next_run_at": "2026-01-02T09:00:00Z", "created_at": "2026-01-01T00:00:00Z",
			"job_template": map[string]any{"type": "report.gen", "options": map[string]any{"queue": "default"}},
			"run_count": 42,
		})
	})
	err := Cron(c, []string{"--detail", "daily-report"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCron_Update(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["expression"] != "0 10 * * *" {
			t.Errorf("expression = %v, want 0 10 * * *", body["expression"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"name": "daily-report", "expression": "0 10 * * *"})
	})
	err := Cron(c, []string{"--update", "daily-report", "--expression", "0 10 * * *"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCron_List_EnabledFilter(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("enabled") != "true" {
			t.Errorf("enabled = %s, want true", r.URL.Query().Get("enabled"))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"cron_jobs": []map[string]any{
				{"name": "daily-report", "expression": "0 9 * * *", "enabled": true},
			},
		})
	})
	err := Cron(c, []string{"--enabled", "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Queue config update tests ---

func TestQueues_Config(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["concurrency"].(float64) != 10 {
			t.Errorf("concurrency = %v, want 10", body["concurrency"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"name": "default", "concurrency": 10})
	})
	err := Queues(c, []string{"--config", "default", "--concurrency", "10"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueues_Config_NoOptions(t *testing.T) {
	c := newTestClient(nil)
	err := Queues(c, []string{"--config", "default"})
	if err == nil {
		t.Fatal("expected error for no config options")
	}
}

// --- Status --detail tests ---

func TestStatus_Detail(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/admin/jobs/") {
			t.Errorf("path = %s, want contains /admin/jobs/", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "job-1", "type": "email.send", "state": "completed", "queue": "default",
			"args": []string{"hello@example.com"},
			"meta": map[string]any{"user_id": "u-123"},
			"options": map[string]any{"priority": 5, "max_attempts": 3},
			"result": map[string]any{"sent": true},
		})
	})
	err := Status(c, []string{"--detail", "job-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
