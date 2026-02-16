package commands

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

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
	uniqueKey := fs.String("unique-key", "", "Unique job key for deduplication")
	uniqueWithin := fs.String("unique-within", "", "Uniqueness window (e.g. 1h, 30m)")
	batchFile := fs.String("batch", "", "NDJSON file for bulk enqueue")
	fs.Parse(args)

	if *batchFile != "" {
		return batchEnqueue(c, *batchFile)
	}

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
	if *uniqueKey != "" {
		unique := map[string]any{
			"key": *uniqueKey,
		}
		if *uniqueWithin != "" {
			unique["within"] = *uniqueWithin
		}
		opts["unique"] = unique
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

func batchEnqueue(c *client.Client, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open batch file: %w", err)
	}
	defer f.Close()

	var jobs []json.RawMessage
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		jobs = append(jobs, json.RawMessage(append([]byte{}, line...)))
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read batch file: %w", err)
	}

	if len(jobs) == 0 {
		return fmt.Errorf("batch file is empty")
	}

	body := map[string]any{
		"jobs": jobs,
	}

	data, _, err := c.Post("/jobs/batch", body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		Enqueued int `json:"enqueued"`
		Failed   int `json:"failed"`
	}
	json.Unmarshal(data, &resp)
	output.Success("Batch enqueue: %d enqueued, %d failed (from %d jobs)", resp.Enqueued, resp.Failed, len(jobs))
	return nil
}
