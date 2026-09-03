package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/config"
	"github.com/openjobspec/ojs-cli/internal/output"
	"gopkg.in/yaml.v3"
)

func TestDebugCommandPaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ojs/v1/jobs/j1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "j1", "type": "email.send", "queue": "default", "state": "discarded",
				"priority": 2, "attempt": 3, "max_attempts": 3, "args": []any{"a"},
				"meta": map[string]any{"tenant": "one"}, "error": "failed",
				"errors": []any{"first", "second"}, "created_at": "2026-01-01T00:00:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/ojs/v1/jobs":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "j2", "queue": "priority"})
		case r.URL.Path == "/ojs/v1/admin/jobs/j1/trace":
			_ = json.NewEncoder(w).Encode(map[string]any{"snapshots": []map[string]any{
				{"phase": "queued", "duration": "1ms", "state": "available"},
				{"phase": "run", "duration": "2ms", "state": "discarded", "error": "failed"},
			}})
		case r.URL.Path == "/ojs/v1/events/j1/history":
			_ = json.NewEncoder(w).Encode(map[string]any{"events": []map[string]any{
				{"type": "job.created", "state": "available", "timestamp": "2026-01-01T00:00:00Z"},
				{"type": "job.failed", "state": "discarded", "timestamp": "2026-01-01T00:01:00Z"},
			}})
		case r.URL.Path == "/ojs/v1/admin/stats":
			_ = json.NewEncoder(w).Encode(map[string]any{"queues": []map[string]any{
				{"name": "default", "depth": 2, "avg_duration_ms": 12.5, "error_rate": 0.1},
			}})
		case r.URL.Path == "/ojs/v1/queues/default":
			_ = json.NewEncoder(w).Encode(map[string]any{"depth": 2, "running": 1, "paused": false})
		case r.URL.Path == "/ojs/v1/jobs":
			_ = json.NewEncoder(w).Encode(map[string]any{"jobs": []map[string]any{
				{"id": "short", "type": "bad.job", "error": strings.Repeat("e", 70), "queue": "default"},
			}})
		case r.URL.Path == "/ojs/v1/admin/observability/health":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"score": 72, "status": "degraded", "slo_violations": 1, "anomaly_count": 2,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	apiClient := client.New(&config.Config{ServerURL: server.URL})

	previous := output.Format
	output.Format = "table"
	t.Cleanup(func() { output.Format = previous })
	tests := [][]string{
		{"inspect", "j1"},
		{"trace", "j1"},
		{"replay", "--queue", "priority", "--priority", "4", "j1"},
		{"history", "j1"},
		{"bottleneck", "--limit", "1"},
		{"queue", "default"},
		{"failures", "--limit", "1"},
		{"health"},
	}
	for _, args := range tests {
		if err := Debug(apiClient, args); err != nil {
			t.Errorf("Debug(%v): %v", args, err)
		}
	}
	if err := Debug(apiClient, []string{"unknown"}); err == nil {
		t.Error("unknown debug subcommand should fail")
	}
	if err := Debug(apiClient, nil); err != nil {
		t.Errorf("debug help: %v", err)
	}
}

func TestDebugFallbackPaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ojs/v1/jobs/j1" || r.URL.Path == "/ojs/v1/health" {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "j1", "state": "available", "status": "ok"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	apiClient := client.New(&config.Config{ServerURL: server.URL})
	previous := output.Format
	output.Format = "json"
	t.Cleanup(func() { output.Format = previous })
	for _, args := range [][]string{{"trace", "j1"}, {"history", "j1"}, {"health"}} {
		if err := Debug(apiClient, args); err != nil {
			t.Errorf("Debug fallback %v: %v", args, err)
		}
	}
}

func TestDoctorProductionAndJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/ojs/v1/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "version": "1.0"})
		case "/ojs/v1/queues":
			if r.Header.Get("Authorization") == "" {
				http.Error(w, `{"error":{"code":"unauthorized","message":"token required"}}`, http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"queues": []map[string]any{{"name": "default"}}})
		case "/ojs/v1/dead-letter":
			_ = json.NewEncoder(w).Encode(map[string]any{"jobs": []any{}})
		case "/ojs/v1/workers":
			_ = json.NewEncoder(w).Encode(map[string]any{"workers": []map[string]any{{"id": "w1"}}})
		case "/metrics":
			_, _ = w.Write([]byte("ojs_up 1\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	apiClient := client.New(&config.Config{ServerURL: server.URL, AuthToken: "token"})

	previous := output.Format
	output.Format = "table"
	t.Cleanup(func() { output.Format = previous })
	if err := Doctor(apiClient, []string{"--production", "--verbose"}); err == nil {
		t.Fatal("HTTP production doctor should fail the TLS check")
	}
	if checkTLS(client.New(&config.Config{ServerURL: "https://jobs.example"})).Status != "pass" {
		t.Error("HTTPS URL should pass TLS check")
	}

	output.Format = "json"
	if err := Doctor(apiClient, nil); err != nil {
		t.Fatalf("JSON doctor: %v", err)
	}
}

func TestEventsCommandEOFAndHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("queue") == "broken" {
			http.Error(w, "broken", http.StatusBadGateway)
			return
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: {\"type\":\"job\",\"event\":\"created\",\"job_id\":\"j1\",\"queue\":%q}\n\n",
			strings.Repeat("q", 70*1024))
	}))
	defer server.Close()
	cfg := &config.Config{ServerURL: server.URL, AuthToken: "token"}
	previous := output.Format
	output.Format = "json"
	t.Cleanup(func() { output.Format = previous })

	if err := Events(cfg, []string{"--types", "job.created,job.failed", "--queue", "default"}); err != nil {
		t.Fatal(err)
	}
	if err := Events(cfg, []string{"--queue", "broken"}); err == nil {
		t.Fatal("Events() accepted non-200 response")
	}
}

func TestCodegenCommandAndCreateJSON(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "jobs.yaml")
	content := `version: "1.0"
package: generated
job_types:
  - type: email.send
    queue: email
    args:
      - name: to
        type: string
        required: true
`
	if err := os.WriteFile(manifest, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"go", "typescript", "python", "java", "rust", "ruby", "dotnet"} {
		outDir := filepath.Join(dir, language)
		if err := Codegen([]string{"--manifest", manifest, "--lang", language, "--out", outDir}); err != nil {
			t.Errorf("Codegen(%s): %v", language, err)
		}
	}
	if err := Codegen([]string{"--manifest", manifest, "--lang", "unknown"}); err == nil {
		t.Error("Codegen() accepted unknown language")
	}
	if err := CreateProjectJSON([]string{"demo", "--backend=lite", "--language=go"}); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateGenerateJSONAndYAML(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		dir := filepath.Join(t.TempDir(), format)
		if err := MigrateGenerate([]string{"--source", "sidekiq", "--output", dir, "--format", format}); err != nil {
			t.Fatalf("MigrateGenerate(%s): %v", format, err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "migration-plan."+format))
		if err != nil {
			t.Fatal(err)
		}
		var decoded any
		if format == "json" {
			err = json.Unmarshal(data, &decoded)
		} else {
			err = yaml.Unmarshal(data, &decoded)
		}
		if err != nil {
			t.Fatalf("decode %s output: %v", format, err)
		}
	}
	if err := MigrateGenerate([]string{"--source", "sidekiq", "--format", "toml"}); err == nil {
		t.Fatal("MigrateGenerate() accepted unsupported format")
	}
}

func TestContractInitWrapperAndRendering(t *testing.T) {
	if err := RunContractCommand([]string{"init", "--service", "billing", "--role", "producer"}); err != nil {
		t.Fatal(err)
	}
	previous := output.Format
	t.Cleanup(func() { output.Format = previous })
	suite := &ContractTestSuite{
		Results: []ContractTestResult{
			{Service: "a", Role: "producer", JobType: "one", Passed: true},
			{Service: "b", Role: "consumer", JobType: "two", Passed: false, Errors: []string{"bad"}, Warnings: []string{"warn"}},
		},
		Passed: 1,
		Failed: 1,
	}
	output.Format = "table"
	if err := renderContractTestResults(suite); err == nil {
		t.Fatal("failed contract suite should return an error")
	}
	output.Format = "json"
	if err := renderContractTestResults(&ContractTestSuite{Passed: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForReadyAndDevErrors(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 2 {
			http.Error(w, "starting", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if !waitForReady(server.URL, 2*time.Second) {
		t.Fatal("waitForReady() did not observe ready server")
	}
	if waitForReady("://bad", 10*time.Millisecond) {
		t.Fatal("waitForReady() accepted malformed URL")
	}
	if err := Dev([]string{"--port", "bad"}); err == nil {
		t.Fatal("Dev() accepted invalid port")
	}
}
