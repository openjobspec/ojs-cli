package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/openjobspec/ojs-cli/internal/migrate"
	"github.com/openjobspec/ojs-cli/internal/redact"
)

const (
	defaultMigrationImportBatchSize = 50
	maxMigrationImportFileBytes     = 64 << 20
	maxMigrationImportResponseBytes = 8 << 20
)

// MigrateImportFlags holds the flags for the migrate import command.
type MigrateImportFlags struct {
	InputFile string
	DryRun    bool
	BatchSize int
}

// RunMigrateImport imports jobs from a migration export file into an OJS server.
func RunMigrateImport(serverURL string, flags MigrateImportFlags) error {
	export, err := readMigrationExport(flags.InputFile)
	if err != nil {
		return err
	}
	printMigrationImportHeader(export, flags.InputFile)
	if flags.DryRun {
		fmt.Fprintln(os.Stderr, "\n🔍 Dry run — no jobs will be imported.")
		return nil
	}

	importable := filterImportableJobs(export.Jobs)
	if len(importable) == 0 {
		fmt.Fprintln(os.Stderr, "\n⚠️  No importable jobs (only available/scheduled/retryable can be imported).")
		return nil
	}

	fmt.Fprintf(os.Stderr, "\n🚀 Importing %d jobs to %s...\n", len(importable), redact.URL(serverURL))
	result := importMigrationBatches(serverURL, importable, flags.BatchSize)
	printMigrationImportResult(result, len(export.Jobs)-len(importable))
	if result.Failed > 0 {
		return &migrate.PartialFailureError{
			Operation: "migration batch import",
			Total:     len(importable),
			Failed:    result.Failed,
			Details:   result.Failures,
		}
	}
	return nil
}

type migrationImportResult struct {
	Imported int
	Failed   int
	Elapsed  time.Duration
	Failures []migrate.FailureDetail
}

func readMigrationExport(path string) (*migrationExport, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading import file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxMigrationImportFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading import file: %w", err)
	}
	if len(data) > maxMigrationImportFileBytes {
		return nil, fmt.Errorf("import file exceeds %d bytes", maxMigrationImportFileBytes)
	}

	var export migrationExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("parsing import file: %w", err)
	}
	if export.Version != "1.0" {
		return nil, fmt.Errorf("unsupported export version: %s (expected 1.0)", export.Version)
	}
	return &export, nil
}

func printMigrationImportHeader(export *migrationExport, inputFile string) {
	fmt.Fprintf(os.Stderr, "📦 Import file: %s\n", inputFile)
	fmt.Fprintf(os.Stderr, "   Source: %s (%s)\n", export.Source.Backend, redact.URL(export.Source.URL))
	fmt.Fprintf(os.Stderr, "   Exported at: %s\n", export.ExportedAt)
	fmt.Fprintf(os.Stderr, "   Total jobs: %d\n", len(export.Jobs))
	for state, count := range export.Stats.ByState {
		fmt.Fprintf(os.Stderr, "   %s: %d\n", state, count)
	}
}

func filterImportableJobs(jobs []migrationJob) []migrationJob {
	importable := make([]migrationJob, 0, len(jobs))
	for i := range jobs {
		switch jobs[i].State {
		case "available", "scheduled", "retryable":
			importable = append(importable, jobs[i])
		}
	}
	return importable
}

func importMigrationBatches(serverURL string, jobs []migrationJob, batchSize int) migrationImportResult {
	if batchSize <= 0 {
		batchSize = defaultMigrationImportBatchSize
	}

	start := time.Now()
	result := migrationImportResult{}
	client := &http.Client{Timeout: 30 * time.Second}
	for startIndex := 0; startIndex < len(jobs); startIndex += batchSize {
		end := startIndex + batchSize
		if end > len(jobs) {
			end = len(jobs)
		}
		batch := jobs[startIndex:end]
		err := postMigrationBatch(client, serverURL, batch)
		if err != nil {
			result.Failed += len(batch)
			result.Failures = append(result.Failures, migrate.FailureDetail{
				Index: startIndex + 1,
				Error: fmt.Sprintf("batch %d-%d: %v", startIndex+1, end, err),
			})
			fmt.Fprintf(os.Stderr, "   ❌ Batch %d-%d failed: %v\n", startIndex+1, end, err)
			continue
		}

		result.Imported += len(batch)
		fmt.Fprintf(os.Stderr, "   ✅ Batch %d-%d imported (%d/%d)\n",
			startIndex+1, end, result.Imported, len(jobs))
	}
	result.Elapsed = time.Since(start)
	return result
}

func postMigrationBatch(client *http.Client, serverURL string, batch []migrationJob) error {
	requestBody := struct {
		Jobs []map[string]any `json:"jobs"`
	}{Jobs: make([]map[string]any, 0, len(batch))}
	for i := range batch {
		job := map[string]any{
			"type":  batch[i].Type,
			"queue": batch[i].Queue,
			"args":  batch[i].Args,
		}
		if batch[i].Priority != 0 {
			job["priority"] = batch[i].Priority
		}
		if batch[i].ScheduledAt != "" {
			job["scheduled_at"] = batch[i].ScheduledAt
		}
		requestBody.Jobs = append(requestBody.Jobs, job)
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("encode batch: %w", err)
	}
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		serverURL+"/ojs/v1/jobs/batch",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create batch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send batch to %s: %w", redact.URL(serverURL), err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxMigrationImportResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read batch response: %w", err)
	}
	if len(responseBody) > maxMigrationImportResponseBytes {
		return fmt.Errorf("batch response exceeds %d bytes", maxMigrationImportResponseBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d — %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

func printMigrationImportResult(result migrationImportResult, skipped int) {
	fmt.Fprintf(os.Stderr, "\n📊 Import complete in %s\n", result.Elapsed.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "   Imported: %d\n", result.Imported)
	fmt.Fprintf(os.Stderr, "   Failed: %d\n", result.Failed)
	fmt.Fprintf(os.Stderr, "   Skipped: %d (terminal state)\n", skipped)
}
