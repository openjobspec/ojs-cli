package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/openjobspec/ojs-cli/internal/fileutil"
	"github.com/openjobspec/ojs-cli/internal/migrate"
	"github.com/openjobspec/ojs-cli/internal/redact"
)

const maxMigrationExportResponseBytes = 64 << 20

// MigrateExportFlags holds the flags for the migrate export command.
type MigrateExportFlags struct {
	OutputFile        string
	IncludeCompleted  bool
	IncludeDeadLetter bool
	Queues            []string
	Since             string
	AllowPartial      bool
}

// migrationExport matches the migration-export.schema.json format.
type migrationExport struct {
	Version    string           `json:"version"`
	Source     migrationSource  `json:"source"`
	ExportedAt string           `json:"exported_at"`
	Options    migrationOptions `json:"options"`
	Jobs       []migrationJob   `json:"jobs"`
	Stats      migrationStats   `json:"stats"`
}

type migrationSource struct {
	Backend string `json:"backend"`
	URL     string `json:"url"`
	Version string `json:"version"`
}

type migrationOptions struct {
	IncludeCompleted  bool     `json:"include_completed"`
	IncludeDeadLetter bool     `json:"include_dead_letter"`
	Queues            []string `json:"queues,omitempty"`
	Since             string   `json:"since,omitempty"`
}

type migrationJob struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Queue       string          `json:"queue"`
	State       string          `json:"state"`
	Args        json.RawMessage `json:"args"`
	Priority    int             `json:"priority,omitempty"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	CreatedAt   string          `json:"created_at,omitempty"`
	EnqueuedAt  string          `json:"enqueued_at,omitempty"`
	ScheduledAt string          `json:"scheduled_at,omitempty"`
	CompletedAt string          `json:"completed_at,omitempty"`
}

type migrationStats struct {
	TotalJobs int            `json:"total_jobs"`
	ByState   map[string]int `json:"by_state"`
	ByQueue   map[string]int `json:"by_queue"`
}

// RunMigrateExport exports jobs from an OJS server to the migration format.
func RunMigrateExport(serverURL string, flags MigrateExportFlags) error {
	client := &http.Client{Timeout: 30 * time.Second}
	health, err := fetchMigrationHealth(client, serverURL)
	if err != nil {
		return err
	}
	rawJobs, err := fetchMigrationJobs(client, serverURL, flags.IncludeCompleted)
	if err != nil {
		return err
	}

	export, partialErr := buildMigrationExport(serverURL, health, rawJobs, flags)
	if partialErr != nil && !flags.AllowPartial {
		return fmt.Errorf("export aborted; set AllowPartial to write valid jobs: %w", partialErr)
	}
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling export: %w", err)
	}

	if err := writeMigrationExport(flags.OutputFile, data); err != nil {
		return err
	}
	printMigrationExportStats(export, flags.OutputFile)
	if partialErr != nil {
		for _, failure := range partialErr.Failures {
			fmt.Fprintf(os.Stderr, "   skipped job %d: %s\n", failure.Index, failure.Error)
		}
		return partialErr
	}
	return nil
}

type migrationHealth struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Backend string `json:"backend"`
}

func fetchMigrationHealth(client *http.Client, serverURL string) (*migrationHealth, error) {
	var health migrationHealth
	if err := getMigrationJSON(client, serverURL+"/ojs/v1/health", &health); err != nil {
		return nil, fmt.Errorf("fetch health from %s: %w", redact.URL(serverURL), err)
	}
	return &health, nil
}

func fetchMigrationJobs(client *http.Client, serverURL string, includeCompleted bool) ([]json.RawMessage, error) {
	endpoint := serverURL + "/ojs/v1/admin/jobs?limit=10000"
	if !includeCompleted {
		endpoint += "&exclude_terminal=true"
	}
	var result struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := getMigrationJSON(client, endpoint, &result); err != nil {
		return nil, fmt.Errorf("fetch jobs from %s: %w", redact.URL(serverURL), err)
	}
	return result.Jobs, nil
}

func getMigrationJSON(client *http.Client, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMigrationExportResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxMigrationExportResponseBytes {
		return fmt.Errorf("response exceeds %d bytes", maxMigrationExportResponseBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func buildMigrationExport(
	serverURL string,
	health *migrationHealth,
	rawJobs []json.RawMessage,
	flags MigrateExportFlags,
) (*migrationExport, *migrate.PartialExportError) {
	jobs := make([]migrationJob, 0, len(rawJobs))
	byState := make(map[string]int)
	byQueue := make(map[string]int)
	failures := make([]migrate.FailedRecord, 0)
	for i, raw := range rawJobs {
		var job migrationJob
		if err := json.Unmarshal(raw, &job); err != nil {
			failures = append(failures, migrate.FailedRecord{
				Source: "ojs", Structure: "admin/jobs", Index: i + 1, Error: err.Error(),
			})
			continue
		}
		if len(flags.Queues) > 0 && !contains(flags.Queues, job.Queue) {
			continue
		}
		jobs = append(jobs, job)
		byState[job.State]++
		byQueue[job.Queue]++
	}

	export := &migrationExport{
		Version: "1.0",
		Source: migrationSource{
			Backend: health.Backend,
			URL:     redact.URL(serverURL),
			Version: health.Version,
		},
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Options: migrationOptions{
			IncludeCompleted:  flags.IncludeCompleted,
			IncludeDeadLetter: flags.IncludeDeadLetter,
			Queues:            flags.Queues,
			Since:             flags.Since,
		},
		Jobs: jobs,
		Stats: migrationStats{
			TotalJobs: len(jobs),
			ByState:   byState,
			ByQueue:   byQueue,
		},
	}
	if len(failures) > 0 {
		return export, &migrate.PartialExportError{Exported: len(jobs), Failures: failures}
	}
	return export, nil
}

func writeMigrationExport(path string, data []byte) error {
	if path == "" || path == "-" {
		fmt.Println(string(data))
		return nil
	}
	if err := fileutil.WriteAtomic(path, 0o644, func(writer io.Writer) error {
		_, err := writer.Write(data)
		return err
	}); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}

func printMigrationExportStats(export *migrationExport, outputFile string) {
	if outputFile != "" && outputFile != "-" {
		fmt.Fprintf(os.Stderr, "✅ Exported %d jobs to %s\n", len(export.Jobs), outputFile)
	}
	fmt.Fprintf(os.Stderr, "   Backend: %s\n", export.Source.Backend)
	fmt.Fprintf(os.Stderr, "   Total: %d jobs\n", len(export.Jobs))
	for state, count := range export.Stats.ByState {
		fmt.Fprintf(os.Stderr, "   %s: %d\n", state, count)
	}
}

func contains(slice []string, item string) bool {
	for _, value := range slice {
		if value == item {
			return true
		}
	}
	return false
}
