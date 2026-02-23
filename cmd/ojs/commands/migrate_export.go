package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// MigrateExportFlags holds the flags for the migrate export command.
type MigrateExportFlags struct {
	OutputFile       string
	IncludeCompleted bool
	IncludeDeadLetter bool
	Queues           []string
	Since            string
}

// migrationExport matches the migration-export.schema.json format.
type migrationExport struct {
	Version    string              `json:"version"`
	Source     migrationSource     `json:"source"`
	ExportedAt string             `json:"exported_at"`
	Options    migrationOptions    `json:"options"`
	Jobs       []migrationJob      `json:"jobs"`
	Stats      migrationStats      `json:"stats"`
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
	// Fetch health to get backend info
	healthResp, err := http.Get(serverURL + "/ojs/v1/health")
	if err != nil {
		return fmt.Errorf("cannot reach server at %s: %w", serverURL, err)
	}
	defer healthResp.Body.Close()

	var health struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		Backend string `json:"backend"`
	}
	if err := json.NewDecoder(healthResp.Body).Decode(&health); err != nil {
		return fmt.Errorf("parsing health response: %w", err)
	}

	// Fetch jobs from the admin API
	jobsURL := serverURL + "/ojs/v1/admin/jobs?limit=10000"
	if !flags.IncludeCompleted {
		jobsURL += "&exclude_terminal=true"
	}

	jobsResp, err := http.Get(jobsURL)
	if err != nil {
		return fmt.Errorf("fetching jobs: %w", err)
	}
	defer jobsResp.Body.Close()

	body, err := io.ReadAll(jobsResp.Body)
	if err != nil {
		return fmt.Errorf("reading jobs response: %w", err)
	}

	var jobsResult struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(body, &jobsResult); err != nil {
		return fmt.Errorf("parsing jobs: %w", err)
	}

	// Convert to migration format
	var jobs []migrationJob
	byState := make(map[string]int)
	byQueue := make(map[string]int)

	for _, raw := range jobsResult.Jobs {
		var j migrationJob
		if err := json.Unmarshal(raw, &j); err != nil {
			continue
		}

		// Apply queue filter
		if len(flags.Queues) > 0 && !contains(flags.Queues, j.Queue) {
			continue
		}

		jobs = append(jobs, j)
		byState[j.State]++
		byQueue[j.Queue]++
	}

	export := migrationExport{
		Version: "1.0",
		Source: migrationSource{
			Backend: health.Backend,
			URL:     serverURL,
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

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling export: %w", err)
	}

	if flags.OutputFile != "" && flags.OutputFile != "-" {
		if err := os.WriteFile(flags.OutputFile, data, 0644); err != nil {
			return fmt.Errorf("writing file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "✅ Exported %d jobs to %s\n", len(jobs), flags.OutputFile)
	} else {
		fmt.Println(string(data))
	}

	// Print stats summary to stderr
	fmt.Fprintf(os.Stderr, "   Backend: %s\n", health.Backend)
	fmt.Fprintf(os.Stderr, "   Total: %d jobs\n", len(jobs))
	for state, count := range byState {
		fmt.Fprintf(os.Stderr, "   %s: %d\n", state, count)
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
