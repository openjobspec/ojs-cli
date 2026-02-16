package commands

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Result command tests ---

func TestResult_MissingID(t *testing.T) {
	c := newTestClient(nil)
	err := Result(c, []string{})
	if err == nil {
		t.Fatal("expected error for missing job ID")
	}
}

func TestResult_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/result") {
			t.Errorf("path = %s, want suffix /result", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"state":  "completed",
			"result": map[string]any{"output": "done"},
		})
	})
	err := Result(c, []string{"job-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResult_Wait(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "wait=true") {
			t.Errorf("query = %s, want wait=true", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"state":  "completed",
			"result": "ok",
		})
	})
	err := Result(c, []string{"--wait", "--timeout", "10", "job-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Jobs command tests ---

func TestJobs_List(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/ojs/v1/jobs") {
			t.Errorf("path = %s, want /ojs/v1/jobs", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"jobs":  []any{},
			"total": 0,
		})
	})
	err := Jobs(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestJobs_ListWithFilters(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != "active" {
			t.Errorf("state = %s, want active", q.Get("state"))
		}
		if q.Get("queue") != "billing" {
			t.Errorf("queue = %s, want billing", q.Get("queue"))
		}
		if q.Get("type") != "email.send" {
			t.Errorf("type = %s, want email.send", q.Get("type"))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"jobs": []map[string]any{
				{"id": "j-1", "type": "email.send", "state": "active", "queue": "billing"},
			},
			"total": 1,
		})
	})
	err := Jobs(c, []string{"--state", "active", "--queue", "billing", "--type", "email.send"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Bulk command tests ---

func TestBulk_NoSubcommand(t *testing.T) {
	c := newTestClient(nil)
	err := Bulk(c, []string{})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestBulk_Cancel_MissingIDs(t *testing.T) {
	c := newTestClient(nil)
	err := Bulk(c, []string{"cancel"})
	if err == nil {
		t.Fatal("expected error for missing --ids")
	}
}

func TestBulk_Cancel_Success(t *testing.T) {
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
		json.NewEncoder(w).Encode(map[string]any{"cancelled": 2, "failed": 0})
	})
	err := Bulk(c, []string{"cancel", "--ids", "job-1,job-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBulk_Retry_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"retried": 3, "failed": 0})
	})
	err := Bulk(c, []string{"retry", "--ids", "job-1,job-2,job-3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBulk_Cancel_ByState(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		filter := body["filter"].(map[string]any)
		if filter["state"] != "available" {
			t.Errorf("state = %v, want available", filter["state"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"cancelled": 5, "failed": 0})
	})
	err := Bulk(c, []string{"cancel", "--state", "available", "--queue", "default"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Priority command tests ---

func TestPriority_MissingID(t *testing.T) {
	c := newTestClient(nil)
	err := Priority(c, []string{})
	if err == nil {
		t.Fatal("expected error for missing job ID")
	}
}

func TestPriority_MissingSet(t *testing.T) {
	c := newTestClient(nil)
	err := Priority(c, []string{"job-1"})
	if err == nil {
		t.Fatal("expected error for missing --set")
	}
}

func TestPriority_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["priority"].(float64) != 5 {
			t.Errorf("priority = %v, want 5", body["priority"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"id": "job-1", "priority": 5})
	})
	err := Priority(c, []string{"--set", "5", "job-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Retries command tests ---

func TestRetries_MissingID(t *testing.T) {
	c := newTestClient(nil)
	err := Retries(c, []string{})
	if err == nil {
		t.Fatal("expected error for missing job ID")
	}
}

func TestRetries_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/retries") {
			t.Errorf("path = %s, want suffix /retries", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"job_id": "job-1",
			"retries": []map[string]any{
				{"attempt": 1, "state": "failed", "error": "timeout", "started_at": "2026-01-01T00:00:00Z", "failed_at": "2026-01-01T00:01:00Z"},
			},
			"policy": map[string]any{"max_attempts": 3, "backoff_strategy": "exponential", "initial_interval": "1s"},
		})
	})
	err := Retries(c, []string{"job-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Metrics command tests ---

func TestMetrics_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"uptime_seconds":       3600,
			"jobs_enqueued_total":  100,
			"jobs_completed_total": 95,
			"jobs_failed_total":    5,
			"jobs_active":          3,
			"queues_active":        2,
			"workers_active":       4,
			"avg_latency_ms":       12.5,
			"throughput_per_second": 2.3,
		})
	})
	err := Metrics(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMetrics_Prometheus(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("format") != "prometheus" {
			t.Errorf("format query = %s, want prometheus", r.URL.Query().Get("format"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# HELP ojs_jobs_total Total jobs\nojs_jobs_total 100\n"))
	})
	err := Metrics(c, []string{"--format", "prometheus"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RateLimits command tests ---

func TestRateLimits_List(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"rate_limits": []map[string]any{
				{"key": "email", "concurrency": 10, "active": 3, "available": 7},
			},
		})
	})
	err := RateLimits(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRateLimits_Inspect(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/email") {
			t.Errorf("path = %s, want suffix /email", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"key": "email", "concurrency": 10, "active": 3, "available": 7,
		})
	})
	err := RateLimits(c, []string{"--inspect", "email"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRateLimits_Override(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"key": "email", "concurrency": 20})
	})
	err := RateLimits(c, []string{"--override", "email", "--concurrency", "20"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRateLimits_Override_MissingConcurrency(t *testing.T) {
	c := newTestClient(nil)
	err := RateLimits(c, []string{"--override", "email"})
	if err == nil {
		t.Fatal("expected error for missing --concurrency")
	}
}

// --- System command tests ---

func TestSystem_NoSubcommand(t *testing.T) {
	c := newTestClient(nil)
	err := System(c, []string{})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestSystem_Maintenance_Enable(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["enabled"] != true {
			t.Errorf("enabled = %v, want true", body["enabled"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"enabled": true})
	})
	err := System(c, []string{"maintenance", "--enable", "--reason", "scheduled upgrade"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSystem_Maintenance_Disable(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["enabled"] != false {
			t.Errorf("enabled = %v, want false", body["enabled"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"enabled": false})
	})
	err := System(c, []string{"maintenance", "--disable"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSystem_Maintenance_Status(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"enabled": false,
		})
	})
	err := System(c, []string{"maintenance"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSystem_Config(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"max_retry_attempts": 3,
			"default_queue":      "default",
		})
	})
	err := System(c, []string{"config"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Dead letter extended tests (purge, stats) ---

func TestDeadLetter_Stats(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/stats") {
			t.Errorf("path = %s, want suffix /stats", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"total":    5,
			"by_queue": map[string]any{"default": 3, "billing": 2},
			"by_type":  map[string]any{"email.send": 4, "report.gen": 1},
		})
	})
	err := DeadLetter(c, []string{"--stats"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeadLetter_Purge(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/purge") {
			t.Errorf("path = %s, want suffix /purge", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"deleted": 10})
	})
	err := DeadLetter(c, []string{"--purge"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeadLetter_Purge_OlderThan(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("older_than") != "7d" {
			t.Errorf("older_than = %s, want 7d", r.URL.Query().Get("older_than"))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"deleted": 3})
	})
	err := DeadLetter(c, []string{"--purge", "--older-than", "7d"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Cron extended tests (trigger, history, pause, resume) ---

func TestCron_Trigger(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/trigger") {
			t.Errorf("path = %s, want suffix /trigger", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"job_id": "triggered-job-1"})
	})
	err := Cron(c, []string{"--trigger", "daily-report"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCron_History(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/history") {
			t.Errorf("path = %s, want contains /history", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"executions": []map[string]any{
				{"job_id": "j-1", "state": "completed", "scheduled_at": "2026-01-01T09:00:00Z"},
			},
		})
	})
	err := Cron(c, []string{"--history", "daily-report"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCron_Pause(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/pause") {
			t.Errorf("path = %s, want suffix /pause", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"paused": true})
	})
	err := Cron(c, []string{"--pause", "daily-report"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCron_Resume(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/resume") {
			t.Errorf("path = %s, want suffix /resume", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"paused": false})
	})
	err := Cron(c, []string{"--resume", "daily-report"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Queue extended tests (create, delete, purge) ---

func TestQueues_Create(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "billing" {
			t.Errorf("name = %v, want billing", body["name"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"name": "billing", "status": "active"})
	})
	err := Queues(c, []string{"--create", "billing", "--concurrency", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueues_Delete(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"deleted": true})
	})
	err := Queues(c, []string{"--delete", "old-queue"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueues_Purge(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/purge") {
			t.Errorf("path = %s, want suffix /purge", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"deleted": 42})
	})
	err := Queues(c, []string{"--purge", "default", "--states", "completed,discarded"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Enqueue extended tests (unique, batch) ---

func TestEnqueue_UniqueKey(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		opts := body["options"].(map[string]any)
		unique := opts["unique"].(map[string]any)
		if unique["key"] != "user-123" {
			t.Errorf("unique key = %v, want user-123", unique["key"])
		}
		if unique["within"] != "1h" {
			t.Errorf("unique within = %v, want 1h", unique["within"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "job-1", "type": "email.send", "state": "available", "queue": "default",
		})
	})
	err := Enqueue(c, []string{
		"--type", "email.send",
		"--unique-key", "user-123",
		"--unique-within", "1h",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnqueue_Batch(t *testing.T) {
	// Create a temp NDJSON file
	dir := t.TempDir()
	path := filepath.Join(dir, "batch.ndjson")
	content := `{"type":"email.send","args":["hello"],"options":{"queue":"default"}}
{"type":"report.gen","args":["world"],"options":{"queue":"reports"}}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/batch") {
			t.Errorf("path = %s, want suffix /batch", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		jobs := body["jobs"].([]any)
		if len(jobs) != 2 {
			t.Errorf("jobs len = %d, want 2", len(jobs))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"enqueued": 2, "failed": 0})
	})
	err := Enqueue(c, []string{"--batch", path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnqueue_Batch_MissingFile(t *testing.T) {
	c := newTestClient(nil)
	err := Enqueue(c, []string{"--batch", "/nonexistent/file.ndjson"})
	if err == nil {
		t.Fatal("expected error for missing batch file")
	}
}

// --- Status progress test ---

func TestStatus_WithProgress(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "job-1", "type": "test", "state": "active", "queue": "default",
			"progress":      0.75,
			"progress_data": map[string]any{"step": "processing"},
		})
	})
	err := Status(c, []string{"job-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
