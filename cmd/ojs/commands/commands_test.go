package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/config"
	"github.com/openjobspec/ojs-cli/internal/output"
)

func newTestClient(handler http.HandlerFunc) *client.Client {
	server := httptest.NewServer(handler)
	return client.New(&config.Config{ServerURL: server.URL})
}

func init() {
	output.Format = "json"
}

func TestEnqueue_MissingType(t *testing.T) {
	c := newTestClient(nil)
	err := Enqueue(c, []string{})
	if err == nil {
		t.Fatal("expected error for missing --type")
	}
}

func TestEnqueue_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["type"] != "email.send" {
			t.Errorf("type = %v, want email.send", body["type"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "job-1", "type": "email.send", "state": "available", "queue": "default",
		})
	})
	err := Enqueue(c, []string{"--type", "email.send", "--args", `["hello"]`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnqueue_InvalidArgs(t *testing.T) {
	c := newTestClient(nil)
	err := Enqueue(c, []string{"--type", "test", "--args", "not-json"})
	if err == nil {
		t.Fatal("expected error for invalid --args")
	}
}

func TestStatus_MissingID(t *testing.T) {
	c := newTestClient(nil)
	err := Status(c, []string{})
	if err == nil {
		t.Fatal("expected error for missing job ID")
	}
}

func TestStatus_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "job-1", "type": "test", "state": "active", "queue": "default",
		})
	})
	err := Status(c, []string{"job-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancel_MissingID(t *testing.T) {
	c := newTestClient(nil)
	err := Cancel(c, []string{})
	if err == nil {
		t.Fatal("expected error for missing job ID")
	}
}

func TestCancel_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"id": "job-1", "state": "cancelled"})
	})
	err := Cancel(c, []string{"job-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHealth_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok", "version": "1.0", "uptime_seconds": 42,
			"backend": map[string]any{"type": "redis", "status": "connected"},
		})
	})
	err := Health(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHealth_ServerError(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "internal_error", "message": "down"},
		})
	})
	err := Health(c, []string{})
	if err == nil {
		t.Fatal("expected error for unhealthy server")
	}
}

func TestQueues_List(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"queues": []map[string]any{
				{"name": "default", "status": "active"},
			},
		})
	})
	err := Queues(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQueues_Stats(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"queue": "default", "status": "active",
			"stats": map[string]any{
				"available": 5, "active": 2, "completed": 10,
				"scheduled": 1, "retryable": 0, "dead": 0,
			},
		})
	})
	err := Queues(c, []string{"--stats", "default"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeadLetter_List(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"jobs": []any{}, "total": 0,
		})
	})
	err := DeadLetter(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCron_List(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"cron_jobs": []any{},
		})
	})
	err := Cron(c, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCron_Register_MissingFields(t *testing.T) {
	c := newTestClient(nil)
	err := Cron(c, []string{"--register"})
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}

func TestCron_Register_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"name": "daily-report", "expression": "0 9 * * *",
		})
	})
	err := Cron(c, []string{
		"--register", "--name", "daily-report",
		"--expression", "0 9 * * *", "--type", "report.generate",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkflow_NoSubcommand(t *testing.T) {
	c := newTestClient(nil)
	err := Workflow(c, []string{})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestWorkflow_Create_MissingFields(t *testing.T) {
	c := newTestClient(nil)
	err := Workflow(c, []string{"create"})
	if err == nil {
		t.Fatal("expected error for missing --name and --steps")
	}
}

func TestWorkflow_Create_InvalidSteps(t *testing.T) {
	c := newTestClient(nil)
	err := Workflow(c, []string{"create", "--name", "test", "--steps", "not-json"})
	if err == nil {
		t.Fatal("expected error for invalid --steps JSON")
	}
}

func TestWorkflow_Create_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "my-workflow" {
			t.Errorf("name = %v, want my-workflow", body["name"])
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "wf-1", "name": "my-workflow", "state": "running",
		})
	})
	err := Workflow(c, []string{
		"create", "--name", "my-workflow",
		"--steps", `[{"id":"step1","type":"test.job","args":[]}]`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkflow_Status_MissingID(t *testing.T) {
	c := newTestClient(nil)
	err := Workflow(c, []string{"status"})
	if err == nil {
		t.Fatal("expected error for missing workflow ID")
	}
}

func TestWorkflow_Status_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "wf-1", "name": "my-workflow", "state": "running",
			"steps": []map[string]any{
				{"id": "step1", "type": "test.job", "state": "completed", "job_id": "job-1"},
			},
			"created_at": "2026-01-01T00:00:00Z",
		})
	})
	err := Workflow(c, []string{"status", "wf-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkflow_Cancel_MissingID(t *testing.T) {
	c := newTestClient(nil)
	err := Workflow(c, []string{"cancel"})
	if err == nil {
		t.Fatal("expected error for missing workflow ID")
	}
}

func TestWorkflow_Cancel_Success(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "wf-1", "state": "cancelled", "cancelled_steps": 2,
		})
	})
	err := Workflow(c, []string{"cancel", "wf-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkflow_List(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"workflows": []map[string]any{
				{"id": "wf-1", "name": "pipeline", "state": "running", "step_count": 3},
			},
			"total": 1,
		})
	})
	err := Workflow(c, []string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkflow_List_Empty(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"workflows": []any{},
			"total":     0,
		})
	})
	err := Workflow(c, []string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkers_Quiet(t *testing.T) {
	c := newTestClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["directive"] != "quiet" {
			t.Errorf("directive = %v, want quiet", body["directive"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"directive": "quiet"})
	})
	err := Workers(c, []string{"--quiet"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkers_Resume(t *testing.T) {
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
		json.NewEncoder(w).Encode(map[string]any{"directive": "resume"})
	})
	err := Workers(c, []string{"--resume"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompletion_NoShell(t *testing.T) {
	err := Completion([]string{})
	if err == nil {
		t.Fatal("expected error for missing shell type")
	}
}

func TestCompletion_UnsupportedShell(t *testing.T) {
	err := Completion([]string{"powershell"})
	if err == nil {
		t.Fatal("expected error for unsupported shell")
	}
}

func TestCompletion_Bash(t *testing.T) {
	err := Completion([]string{"bash"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompletion_Zsh(t *testing.T) {
	err := Completion([]string{"zsh"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompletion_Fish(t *testing.T) {
	err := Completion([]string{"fish"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
