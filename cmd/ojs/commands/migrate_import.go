package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// MigrateImportFlags holds the flags for the migrate import command.
type MigrateImportFlags struct {
	InputFile string
	DryRun    bool
	BatchSize int
}

// RunMigrateImport imports jobs from a migration export file into an OJS server.
func RunMigrateImport(serverURL string, flags MigrateImportFlags) error {
	data, err := os.ReadFile(flags.InputFile)
	if err != nil {
		return fmt.Errorf("reading import file: %w", err)
	}

	var export migrationExport
	if err := json.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parsing import file: %w", err)
	}

	if export.Version != "1.0" {
		return fmt.Errorf("unsupported export version: %s (expected 1.0)", export.Version)
	}

	fmt.Fprintf(os.Stderr, "📦 Import file: %s\n", flags.InputFile)
	fmt.Fprintf(os.Stderr, "   Source: %s (%s)\n", export.Source.Backend, export.Source.URL)
	fmt.Fprintf(os.Stderr, "   Exported at: %s\n", export.ExportedAt)
	fmt.Fprintf(os.Stderr, "   Total jobs: %d\n", len(export.Jobs))
	for state, count := range export.Stats.ByState {
		fmt.Fprintf(os.Stderr, "   %s: %d\n", state, count)
	}

	if flags.DryRun {
		fmt.Fprintf(os.Stderr, "\n🔍 Dry run — no jobs will be imported.\n")
		return nil
	}

	// Filter to only importable states (available, scheduled, retryable)
	var importable []migrationJob
	for _, j := range export.Jobs {
		switch j.State {
		case "available", "scheduled", "retryable":
			importable = append(importable, j)
		}
	}

	if len(importable) == 0 {
		fmt.Fprintf(os.Stderr, "\n⚠️  No importable jobs (only available/scheduled/retryable can be imported).\n")
		return nil
	}

	fmt.Fprintf(os.Stderr, "\n🚀 Importing %d jobs to %s...\n", len(importable), serverURL)

	batchSize := flags.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	imported := 0
	failed := 0
	start := time.Now()

	for i := 0; i < len(importable); i += batchSize {
		end := i + batchSize
		if end > len(importable) {
			end = len(importable)
		}
		batch := importable[i:end]

		// Build batch enqueue request
		var batchReq struct {
			Jobs []map[string]any `json:"jobs"`
		}
		for _, j := range batch {
			job := map[string]any{
				"type":  j.Type,
				"queue": j.Queue,
				"args":  j.Args,
			}
			if j.Priority != 0 {
				job["priority"] = j.Priority
			}
			if j.ScheduledAt != "" {
				job["scheduled_at"] = j.ScheduledAt
			}
			batchReq.Jobs = append(batchReq.Jobs, job)
		}

		body, _ := json.Marshal(batchReq)
		resp, err := http.Post(
			serverURL+"/ojs/v1/jobs/batch",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "   ❌ Batch %d-%d failed: %v\n", i+1, end, err)
			failed += len(batch)
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			imported += len(batch)
			fmt.Fprintf(os.Stderr, "   ✅ Batch %d-%d imported (%d/%d)\n", i+1, end, imported, len(importable))
		} else {
			failed += len(batch)
			fmt.Fprintf(os.Stderr, "   ❌ Batch %d-%d failed: HTTP %d — %s\n", i+1, end, resp.StatusCode, string(respBody))
		}
	}

	elapsed := time.Since(start)
	fmt.Fprintf(os.Stderr, "\n📊 Import complete in %s\n", elapsed.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "   Imported: %d\n", imported)
	fmt.Fprintf(os.Stderr, "   Failed: %d\n", failed)
	fmt.Fprintf(os.Stderr, "   Skipped: %d (terminal state)\n", len(export.Jobs)-len(importable))

	if failed > 0 {
		return fmt.Errorf("%d jobs failed to import", failed)
	}
	return nil
}
