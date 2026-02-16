package commands

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/openjobspec/ojs-cli/internal/migrate"
)

func TestParseSidekiqJob(t *testing.T) {
	raw := `{"class":"EmailWorker","args":["user@example.com"],"queue":"default","retry":true,"jid":"abc123"}`

	job, err := migrate.ParseSidekiqJob(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Type != "email.worker" {
		t.Errorf("type = %q, want %q", job.Type, "email.worker")
	}
	if job.Queue != "default" {
		t.Errorf("queue = %q, want %q", job.Queue, "default")
	}

	var args []any
	json.Unmarshal(job.Args, &args)
	if len(args) != 1 || args[0] != "user@example.com" {
		t.Errorf("args = %v, want [user@example.com]", args)
	}

	if job.Meta["sidekiq_jid"] != "abc123" {
		t.Errorf("meta.sidekiq_jid = %v, want abc123", job.Meta["sidekiq_jid"])
	}
	if job.Meta["sidekiq_class"] != "EmailWorker" {
		t.Errorf("meta.sidekiq_class = %v, want EmailWorker", job.Meta["sidekiq_class"])
	}
}

func TestParseSidekiqJob_NamespacedClass(t *testing.T) {
	raw := `{"class":"Mailers::WelcomeEmail","args":[],"queue":"mailers","jid":"def456"}`

	job, err := migrate.ParseSidekiqJob(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Type != "mailers.welcome.email" {
		t.Errorf("type = %q, want %q", job.Type, "mailers.welcome.email")
	}
	if job.Queue != "mailers" {
		t.Errorf("queue = %q, want %q", job.Queue, "mailers")
	}
}

func TestParseSidekiqJob_DefaultQueue(t *testing.T) {
	raw := `{"class":"MyJob","args":[1,2],"jid":"ghi789"}`

	job, err := migrate.ParseSidekiqJob(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Queue != "default" {
		t.Errorf("queue = %q, want %q", job.Queue, "default")
	}
}

func TestParseBullMQJob(t *testing.T) {
	raw := `{"name":"email.send","data":{"to":"user@example.com"},"opts":{"priority":5}}`

	job, err := migrate.ParseBullMQJob("notifications", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Type != "email.send" {
		t.Errorf("type = %q, want %q", job.Type, "email.send")
	}
	if job.Queue != "notifications" {
		t.Errorf("queue = %q, want %q", job.Queue, "notifications")
	}

	// Args should wrap data in an array
	var args []json.RawMessage
	json.Unmarshal(job.Args, &args)
	if len(args) != 1 {
		t.Fatalf("args length = %d, want 1", len(args))
	}

	var data map[string]any
	json.Unmarshal(args[0], &data)
	if data["to"] != "user@example.com" {
		t.Errorf("args[0].to = %v, want user@example.com", data["to"])
	}

	if job.Priority == nil || *job.Priority != 5 {
		t.Errorf("priority = %v, want 5", job.Priority)
	}
}

func TestParseBullMQJob_WithDelay(t *testing.T) {
	raw := `{"name":"delayed.task","data":{},"opts":{"delay":5000}}`

	job, err := migrate.ParseBullMQJob("default", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.ScheduledAt == "" {
		t.Error("expected scheduled_at to be set for delayed job")
	}
}

func TestParseBullMQJob_NoPriority(t *testing.T) {
	raw := `{"name":"simple.task","data":{"key":"value"},"opts":{}}`

	job, err := migrate.ParseBullMQJob("default", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Priority != nil {
		t.Errorf("priority = %v, want nil", job.Priority)
	}
}

func TestParseCeleryMessage(t *testing.T) {
	// Build a Celery message with base64-encoded body
	bodyContent := `[["arg1", "arg2"], {"key": "value"}, {}]`
	encodedBody := base64.StdEncoding.EncodeToString([]byte(bodyContent))

	msg := map[string]any{
		"body": encodedBody,
		"headers": map[string]any{
			"task": "app.tasks.send_email",
			"id":   "task-uuid-123",
		},
	}
	raw, _ := json.Marshal(msg)

	job, err := migrate.ParseCeleryMessage("celery", string(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Type != "app.tasks.send_email" {
		t.Errorf("type = %q, want %q", job.Type, "app.tasks.send_email")
	}
	if job.Queue != "celery" {
		t.Errorf("queue = %q, want %q", job.Queue, "celery")
	}

	var args []any
	json.Unmarshal(job.Args, &args)
	if len(args) != 2 || args[0] != "arg1" {
		t.Errorf("args = %v, want [arg1, arg2]", args)
	}

	if job.Meta["celery_task_id"] != "task-uuid-123" {
		t.Errorf("meta.celery_task_id = %v, want task-uuid-123", job.Meta["celery_task_id"])
	}

	// Kwargs should be preserved in meta
	kwargs, ok := job.Meta["celery_kwargs"].(map[string]any)
	if !ok {
		t.Fatal("expected celery_kwargs in meta")
	}
	if kwargs["key"] != "value" {
		t.Errorf("kwargs.key = %v, want value", kwargs["key"])
	}
}

func TestParseCeleryMessage_MissingTask(t *testing.T) {
	msg := `{"body":"W10sIHt9LCB7fV0=","headers":{"id":"uuid"}}`

	_, err := migrate.ParseCeleryMessage("celery", msg)
	if err == nil {
		t.Fatal("expected error for missing task header")
	}
}

func TestParseCeleryMessage_EmptyBody(t *testing.T) {
	msg := `{"body":"","headers":{"task":"my.task","id":"uuid"}}`

	job, err := migrate.ParseCeleryMessage("celery", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(job.Args) != "[]" {
		t.Errorf("args = %s, want []", job.Args)
	}
}

func TestMigrate_NoSubcommand(t *testing.T) {
	c := newTestClient(nil)
	err := Migrate(c, []string{})
	if err == nil {
		t.Fatal("expected error for missing subcommand")
	}
}

func TestMigrate_UnknownSubcommand(t *testing.T) {
	c := newTestClient(nil)
	err := Migrate(c, []string{"unknown"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

func TestMigrate_AnalyzeMissingSource(t *testing.T) {
	c := newTestClient(nil)
	err := Migrate(c, []string{"analyze"})
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestMigrate_ExportMissingSource(t *testing.T) {
	c := newTestClient(nil)
	err := Migrate(c, []string{"export"})
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestMigrate_ImportMissingFile(t *testing.T) {
	c := newTestClient(nil)
	err := Migrate(c, []string{"import"})
	if err == nil {
		t.Fatal("expected error for missing --file")
	}
}

func TestMigrate_UnsupportedSource(t *testing.T) {
	c := newTestClient(nil)
	err := Migrate(c, []string{"analyze", "unknown-source"})
	if err == nil {
		t.Fatal("expected error for unsupported source")
	}
}

func TestSidekiqToOJSConversion(t *testing.T) {
	// Verify full conversion pipeline
	raw := `{"class":"Reports::DailyDigest","args":[42,true],"queue":"reports","retry":5,"jid":"xyz"}`

	job, err := migrate.ParseSidekiqJob(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify OJS-compatible output
	data, _ := json.Marshal(job)
	var result map[string]any
	json.Unmarshal(data, &result)

	if result["type"] != "reports.daily.digest" {
		t.Errorf("type = %v, want reports.daily.digest", result["type"])
	}
	if result["queue"] != "reports" {
		t.Errorf("queue = %v, want reports", result["queue"])
	}
}
