package commands

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/config"
	"github.com/openjobspec/ojs-cli/internal/migrate"
	"github.com/openjobspec/ojs-cli/internal/output"
)

type staticMigrationSource struct {
	jobs []migrate.ExportedJob
	err  error
}

func (s *staticMigrationSource) Analyze() (*migrate.AnalysisResult, error) {
	return nil, nil
}

func (s *staticMigrationSource) Export() ([]migrate.ExportedJob, error) {
	return s.jobs, s.err
}

func (s *staticMigrationSource) Close() error {
	return nil
}

func TestExportSourceDoesNotReplaceOutputOnPartialFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.ndjson")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	partial := &migrate.PartialExportError{
		Exported: 1,
		Failures: []migrate.FailedRecord{{Source: "test", Index: 2, Error: "malformed"}},
	}
	source := &staticMigrationSource{
		jobs: []migrate.ExportedJob{{Type: "good.job", Queue: "default", Args: json.RawMessage("[]")}},
		err:  partial,
	}

	err := exportSource(source, path, false)
	var gotPartial *migrate.PartialExportError
	if !errors.As(err, &gotPartial) {
		t.Fatalf("exportSource() error = %v, want PartialExportError", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "original" {
		t.Fatalf("partial export replaced existing output with %q", data)
	}
}

func TestExportSourceAllowPartialWritesThenReturnsTypedError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.ndjson")
	partial := &migrate.PartialExportError{
		Exported: 1,
		Failures: []migrate.FailedRecord{{Source: "test", Index: 2, Error: "malformed"}},
	}
	source := &staticMigrationSource{
		jobs: []migrate.ExportedJob{{Type: "good.job", Queue: "default", Args: json.RawMessage("[]")}},
		err:  partial,
	}

	err := exportSource(source, path, true)
	var gotPartial *migrate.PartialExportError
	if !errors.As(err, &gotPartial) {
		t.Fatalf("exportSource() error = %v, want PartialExportError", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !json.Valid(data) {
		t.Fatalf("partial output is not valid NDJSON JSON record: %q", data)
	}
}

func TestRunMigrateImportReturnsPartialErrorForMixedBatches(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 2 {
			http.Error(w, "rejected", http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"count":1}`))
	}))
	defer server.Close()

	path := writeMigrationImportFixture(t, []migrationJob{
		{ID: "1", Type: "one", Queue: "default", State: "available", Args: json.RawMessage("[]")},
		{ID: "2", Type: "two", Queue: "default", State: "available", Args: json.RawMessage("[]")},
	})
	err := RunMigrateImport(server.URL, MigrateImportFlags{InputFile: path, BatchSize: 1})
	var partial *migrate.PartialFailureError
	if !errors.As(err, &partial) {
		t.Fatalf("RunMigrateImport() error = %v, want PartialFailureError", err)
	}
	if partial.Total != 2 || partial.Failed != 1 {
		t.Fatalf("partial = %+v, want total=2 failed=1", partial)
	}
}

func TestRunMigrateImportReturnsPartialErrorOnConnectionFailure(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	serverURL := server.URL
	server.Close()

	path := writeMigrationImportFixture(t, []migrationJob{
		{ID: "1", Type: "one", Queue: "default", State: "available", Args: json.RawMessage("[]")},
	})
	err := RunMigrateImport(serverURL, MigrateImportFlags{InputFile: path, BatchSize: 1})
	var partial *migrate.PartialFailureError
	if !errors.As(err, &partial) {
		t.Fatalf("RunMigrateImport() error = %v, want PartialFailureError", err)
	}
	if partial.Failed != 1 {
		t.Fatalf("failed = %d, want 1", partial.Failed)
	}
}

func TestRunMigrateExportPartialPolicyAndRedaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/ojs/v1/health":
			_ = json.NewEncoder(w).Encode(map[string]string{"backend": "redis", "version": "1"})
		case "/ojs/v1/admin/jobs":
			_, _ = w.Write([]byte(`{"jobs":[{"id":"1","type":"ok","queue":"default","state":"available","args":[]},"bad"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	serverWithSecret := strings.Replace(server.URL, "http://", "http://user:topsecret@", 1)
	err := RunMigrateExport(serverWithSecret, MigrateExportFlags{OutputFile: path})
	var partial *migrate.PartialExportError
	if !errors.As(err, &partial) {
		t.Fatalf("RunMigrateExport() error = %v, want PartialExportError", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "original" {
		t.Fatal("export replaced output without explicit partial policy")
	}

	err = RunMigrateExport(serverWithSecret, MigrateExportFlags{OutputFile: path, AllowPartial: true})
	if !errors.As(err, &partial) {
		t.Fatalf("RunMigrateExport(allow partial) error = %v, want PartialExportError", err)
	}
	data, readErr = os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) == "original" {
		t.Fatal("explicit partial export did not replace output")
	}
	if len(data) == 0 || strings.Contains(string(data), "topsecret") {
		t.Fatalf("export leaked source credentials: %s", data)
	}
}

func TestMigrateShadowIsUnsupported(t *testing.T) {
	if err := migrateShadow(nil, nil); !errors.Is(err, ErrShadowNotImplemented) {
		t.Fatalf("migrateShadow() error = %v, want ErrShadowNotImplemented", err)
	}
}

func TestMigrationFileCommandPaths(t *testing.T) {
	previous := output.Format
	output.Format = "table"
	t.Cleanup(func() { output.Format = previous })

	validPath := filepath.Join(t.TempDir(), "valid.ndjson")
	if err := os.WriteFile(validPath, []byte(`{"type":"good.job","queue":"default","args":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := migrateImportDryRun(validPath); err != nil {
		t.Fatalf("valid dry run: %v", err)
	}
	if err := migrateValidate([]string{"--file", validPath}); err != nil {
		t.Fatalf("valid migration file: %v", err)
	}

	invalidPath := filepath.Join(t.TempDir(), "invalid.ndjson")
	if err := os.WriteFile(invalidPath, []byte(`{"type":"","queue":"","args":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var partial *migrate.PartialFailureError
	if err := migrateImportDryRun(invalidPath); !errors.As(err, &partial) {
		t.Fatalf("invalid dry run error = %v, want PartialFailureError", err)
	}
	if err := migrateValidate([]string{"--file", invalidPath}); err != nil {
		t.Fatalf("migrate validate renders invalid records without transport error: %v", err)
	}

	dryRun, outputFile, remaining := parseMigrateFlags([]string{"--dry-run", "--output", "out.json", "config.yml"})
	if !dryRun || outputFile != "out.json" || len(remaining) != 1 {
		t.Fatalf("parseMigrateFlags = %v %q %v", dryRun, outputFile, remaining)
	}
}

func TestMigrateImportJobsRendersThenReturnsPartial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/jobs") {
			http.Error(w, `{"error":{"code":"rejected","message":"bad"}}`, http.StatusUnprocessableEntity)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	apiClient := client.New(&config.Config{ServerURL: server.URL})
	path := filepath.Join(t.TempDir(), "jobs.ndjson")
	if err := os.WriteFile(path, []byte(`{"type":"job","queue":"default","args":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := output.Format
	output.Format = "table"
	t.Cleanup(func() { output.Format = previous })

	var partial *migrate.PartialFailureError
	if err := migrateImportJobs(apiClient, path); !errors.As(err, &partial) {
		t.Fatalf("migrateImportJobs() error = %v, want PartialFailureError", err)
	}
}

func TestMigrationTemplateAndSourceDispatch(t *testing.T) {
	for _, sourceName := range []string{"sidekiq", "bullmq", "celery", "faktory", "river"} {
		if getMigrationTemplates(sourceName) == nil {
			t.Errorf("missing migration template for %s", sourceName)
		}
		source, err := newSource(sourceName, "redis://localhost:6379")
		if err != nil {
			t.Errorf("newSource(%s): %v", sourceName, err)
			continue
		}
		_ = source.Close()
	}
	if getMigrationTemplates("unknown") != nil {
		t.Error("unknown migration template should be nil")
	}
	if _, err := newSource("unknown", "redis://localhost"); err == nil {
		t.Error("newSource accepted unknown source")
	}
	if !contains([]string{"one", "two"}, "two") || contains([]string{"one"}, "two") {
		t.Error("contains helper returned incorrect result")
	}
}

func TestSetupCommandDispatch(t *testing.T) {
	if err := RunSetupCommand(nil); err != nil {
		t.Fatal(err)
	}
	if err := RunSetupCommand([]string{"unknown"}); err == nil {
		t.Fatal("unknown setup subcommand should fail")
	}
}

func writeMigrationImportFixture(t *testing.T, jobs []migrationJob) string {
	t.Helper()
	export := migrationExport{
		Version:    "1.0",
		Source:     migrationSource{Backend: "redis", URL: "redis://user:pass@example"},
		ExportedAt: "2026-01-01T00:00:00Z",
		Jobs:       jobs,
		Stats:      migrationStats{TotalJobs: len(jobs), ByState: map[string]int{"available": len(jobs)}},
	}
	data, err := json.Marshal(export)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "migration.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
