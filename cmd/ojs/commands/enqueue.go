package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Enqueue creates a new job.
func Enqueue(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("enqueue", flag.ExitOnError)
	jobType := fs.String("type", "", "Job type (required)")
	queue := fs.String("queue", "default", "Target queue")
	priority := fs.Int("priority", 0, "Job priority (0-10)")
	argsJSON := fs.String("args", "[]", "Job args as JSON array")
	metaJSON := fs.String("meta", "", "Job metadata as JSON object")
	maxAttempts := fs.Int("max-attempts", 0, "Max retry attempts")
	fs.Parse(args)

	if *jobType == "" {
		return fmt.Errorf("--type is required\n\nUsage: ojs enqueue --type <type> [--queue <queue>] [--args '<json>']")
	}

	body := map[string]any{
		"type": *jobType,
	}

	var jobArgs json.RawMessage
	if err := json.Unmarshal([]byte(*argsJSON), &jobArgs); err != nil {
		return fmt.Errorf("invalid --args JSON: %w", err)
	}
	body["args"] = jobArgs

	opts := map[string]any{
		"queue": *queue,
	}
	if *priority > 0 {
		opts["priority"] = *priority
	}
	if *maxAttempts > 0 {
		opts["max_attempts"] = *maxAttempts
	}
	body["options"] = opts

	if *metaJSON != "" {
		var meta json.RawMessage
		if err := json.Unmarshal([]byte(*metaJSON), &meta); err != nil {
			return fmt.Errorf("invalid --meta JSON: %w", err)
		}
		body["meta"] = meta
	}

	data, _, err := c.Post("/jobs", body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var job map[string]any
	json.Unmarshal(data, &job)
	output.Success("Job enqueued: %s (type=%s, queue=%s, state=%s)",
		job["id"], job["type"], job["queue"], job["state"])
	return nil
}
