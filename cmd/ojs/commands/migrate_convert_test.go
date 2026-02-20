package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConvertSidekiqConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "sidekiq_config.yml"))
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}

	result, err := convertSidekiqConfig(data)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if result.Source != "sidekiq" {
		t.Errorf("source = %q, want sidekiq", result.Source)
	}
	if len(result.Jobs) != 3 {
		t.Fatalf("jobs count = %d, want 3", len(result.Jobs))
	}

	// EmailWorker
	found := false
	for _, job := range result.Jobs {
		if job.Type == "email.worker" {
			found = true
			if job.Queue != "mailers" {
				t.Errorf("EmailWorker queue = %q, want mailers", job.Queue)
			}
			if job.Options.MaxAttempts != 5 {
				t.Errorf("EmailWorker max_attempts = %d, want 5", job.Options.MaxAttempts)
			}
			if job.Cron != "0 */2 * * *" {
				t.Errorf("EmailWorker cron = %q, want '0 */2 * * *'", job.Cron)
			}
		}
	}
	if !found {
		t.Error("expected job type email.worker not found")
	}
}

func TestConvertSidekiqConfig_RetryTrue(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "sidekiq_config.yml"))
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}

	result, err := convertSidekiqConfig(data)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	for _, job := range result.Jobs {
		if job.Type == "hard.worker" {
			if job.Options.MaxAttempts != 25 {
				t.Errorf("HardWorker max_attempts = %d, want 25 (Sidekiq default)", job.Options.MaxAttempts)
			}
		}
	}

	// Should have a warning about default retries
	if len(result.Warnings) == 0 {
		t.Error("expected warning for default 25 retries")
	}
}

func TestConvertSidekiqConfig_InvalidYAML(t *testing.T) {
	_, err := convertSidekiqConfig([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestConvertBullMQConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "bullmq_config.json"))
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}

	result, err := convertBullMQConfig(data)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if result.Source != "bullmq" {
		t.Errorf("source = %q, want bullmq", result.Source)
	}
	if len(result.Jobs) != 3 {
		t.Fatalf("jobs count = %d, want 3", len(result.Jobs))
	}

	for _, job := range result.Jobs {
		if job.Type == "email.send" {
			if job.Queue != "email" {
				t.Errorf("email.send queue = %q, want email", job.Queue)
			}
			if job.Options.MaxAttempts != 5 {
				t.Errorf("email.send max_attempts = %d, want 5", job.Options.MaxAttempts)
			}
			if job.Options.Retry == nil {
				t.Fatal("email.send retry policy is nil")
			}
			if job.Options.Retry.Backoff != "exponential" {
				t.Errorf("email.send backoff = %q, want exponential", job.Options.Retry.Backoff)
			}
		}
		if job.Type == "report.generate" {
			if job.Options.Retry != nil && job.Options.Retry.Backoff != "fixed" {
				t.Errorf("report.generate backoff = %q, want fixed", job.Options.Retry.Backoff)
			}
		}
	}
}

func TestConvertBullMQConfig_WithDelay(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "bullmq_config.json"))
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}

	result, err := convertBullMQConfig(data)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	// email.verify has delay=5000, should generate a warning
	hasDelayWarning := false
	for _, w := range result.Warnings {
		if len(w) > 0 {
			hasDelayWarning = true
		}
	}
	if !hasDelayWarning {
		t.Error("expected warning about delay conversion")
	}
}

func TestConvertBullMQConfig_InvalidJSON(t *testing.T) {
	_, err := convertBullMQConfig([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestConvertCeleryConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "celery_config.json"))
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}

	result, err := convertCeleryConfig(data)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	if result.Source != "celery" {
		t.Errorf("source = %q, want celery", result.Source)
	}
	if len(result.Jobs) != 3 {
		t.Fatalf("jobs count = %d, want 3", len(result.Jobs))
	}

	for _, job := range result.Jobs {
		if job.Type == "myapp.tasks.send_email" {
			if job.Queue != "email" {
				t.Errorf("send_email queue = %q, want email", job.Queue)
			}
			if job.Options.MaxAttempts != 5 {
				t.Errorf("send_email max_attempts = %d, want 5", job.Options.MaxAttempts)
			}
		}
		if job.Type == "myapp.tasks.cleanup" {
			if job.Cron != "0 2 * * *" {
				t.Errorf("cleanup cron = %q, want '0 2 * * *'", job.Cron)
			}
		}
	}

	// Should have rate_limit warnings
	hasRateLimitWarning := false
	for _, w := range result.Warnings {
		if len(w) > 0 {
			hasRateLimitWarning = true
		}
	}
	if !hasRateLimitWarning {
		t.Error("expected warning about rate_limit")
	}
}

func TestConvertCeleryConfig_InvalidJSON(t *testing.T) {
	_, err := convertCeleryConfig([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMigrateDetect(t *testing.T) {
	// Create a temp directory with a package.json referencing bullmq
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"bullmq":"^4.0.0"}}`), 0o644)

	detections := detectFrameworks(dir)
	if len(detections) != 1 {
		t.Fatalf("detections = %d, want 1", len(detections))
	}
	if detections[0].Name != "bullmq" {
		t.Errorf("detection name = %q, want bullmq", detections[0].Name)
	}
}

func TestMigrateDetect_Sidekiq(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sidekiq.yml"), []byte(":queues:\n  - default\n"), 0o644)

	detections := detectFrameworks(dir)
	found := false
	for _, d := range detections {
		if d.Name == "sidekiq" {
			found = true
			if d.Confidence != "high" {
				t.Errorf("confidence = %q, want high", d.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected sidekiq detection")
	}
}

func TestMigrateDetect_Celery(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("celery==5.3.0\nredis==5.0.0\n"), 0o644)

	detections := detectFrameworks(dir)
	found := false
	for _, d := range detections {
		if d.Name == "celery" {
			found = true
		}
	}
	if !found {
		t.Error("expected celery detection")
	}
}

func TestMigrateDetect_NoFramework(t *testing.T) {
	dir := t.TempDir()
	detections := detectFrameworks(dir)
	if len(detections) != 0 {
		t.Errorf("detections = %d, want 0", len(detections))
	}
}

func TestMigrateValidateConfig(t *testing.T) {
	validConfig := ojsMigrateOutput{
		Source: "test",
		Jobs: []ojsJobDefinition{
			{Type: "email.send", Queue: "email", Options: ojsJobOptions{Queue: "email"}},
		},
	}

	f := filepath.Join(t.TempDir(), "valid.json")
	data, _ := json.MarshalIndent(validConfig, "", "  ")
	os.WriteFile(f, data, 0o644)

	err := migrateValidateConfig([]string{f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMigrateValidateConfig_MissingFile(t *testing.T) {
	err := migrateValidateConfig([]string{"/nonexistent/file.json"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMigrateValidateConfig_NoArgs(t *testing.T) {
	err := migrateValidateConfig([]string{})
	if err == nil {
		t.Fatal("expected error for no args")
	}
}

func TestMigrateSidekiq_MissingFile(t *testing.T) {
	err := migrateSidekiq([]string{}, false, "")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMigrateBullMQ_MissingFile(t *testing.T) {
	err := migrateBullMQ([]string{}, false, "")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMigrateCelery_MissingFile(t *testing.T) {
	err := migrateCelery([]string{}, false, "")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMigrateDetect_MissingDir(t *testing.T) {
	err := migrateDetect([]string{})
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestMigrateDetect_NotADir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(f, []byte("hello"), 0o644)
	err := migrateDetect([]string{f})
	if err == nil {
		t.Fatal("expected error for file instead of directory")
	}
}

func TestMigrate_NewSubcommands(t *testing.T) {
	c := newTestClient(nil)

	// Test that new subcommands are recognized (even if they fail due to missing args)
	for _, sub := range []string{"sidekiq", "bullmq", "celery", "detect", "validate-config"} {
		err := Migrate(c, []string{sub})
		if err == nil {
			t.Errorf("migrate %s with no args should return error", sub)
		}
	}
}

func TestSidekiqClassToOJSType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"EmailWorker", "email.worker"},
		{"HardWorker", "hard.worker"},
		{"Mailers::WelcomeEmail", "mailers.welcome.email"},
		{"Reports::DailyDigest", "reports.daily.digest"},
		{"SimpleJob", "simple.job"},
	}
	for _, tt := range tests {
		got := sidekiqClassToOJSType(tt.input)
		if got != tt.want {
			t.Errorf("sidekiqClassToOJSType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMigrateSidekiq_DryRun(t *testing.T) {
	configFile := filepath.Join("..", "..", "..", "testdata", "sidekiq_config.yml")
	err := migrateSidekiq([]string{configFile}, true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMigrateBullMQ_DryRun(t *testing.T) {
	configFile := filepath.Join("..", "..", "..", "testdata", "bullmq_config.json")
	err := migrateBullMQ([]string{configFile}, true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMigrateCelery_DryRun(t *testing.T) {
	configFile := filepath.Join("..", "..", "..", "testdata", "celery_config.json")
	err := migrateCelery([]string{configFile}, true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMigrateSidekiq_OutputFile(t *testing.T) {
	configFile := filepath.Join("..", "..", "..", "testdata", "sidekiq_config.yml")
	outFile := filepath.Join(t.TempDir(), "output.json")

	err := migrateSidekiq([]string{configFile}, false, outFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var result ojsMigrateOutput
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if len(result.Jobs) != 3 {
		t.Errorf("output jobs = %d, want 3", len(result.Jobs))
	}
}
