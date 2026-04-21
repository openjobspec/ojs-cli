package commands

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

var (
	typePattern  = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
	queuePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9\-\.]*$`)
)

type enqueueOptions struct {
	jobType      string
	queue        string
	priority     int
	argsJSON     string
	metaJSON     string
	maxAttempts  int
	uniqueKey    string
	uniqueWithin string
	batchFile    string
}

// Enqueue creates a new job.
func Enqueue(c *client.Client, args []string) error {
	options, err := parseEnqueueOptions(args)
	if err != nil {
		return err
	}
	if options.batchFile != "" {
		return batchEnqueue(c, options.batchFile)
	}
	if err := validateEnqueueOptions(options); err != nil {
		return err
	}

	body, err := buildEnqueueBody(options)
	if err != nil {
		return err
	}
	data, _, err := c.Post("/jobs", body)
	if err != nil {
		return err
	}
	return renderEnqueueResponse(data)
}

func parseEnqueueOptions(args []string) (*enqueueOptions, error) {
	options := &enqueueOptions{}
	fs := flag.NewFlagSet("enqueue", flag.ContinueOnError)
	fs.StringVar(&options.jobType, "type", "", "Job type (required)")
	fs.StringVar(&options.queue, "queue", "default", "Target queue")
	fs.IntVar(&options.priority, "priority", 0, "Job priority (0-10)")
	fs.StringVar(&options.argsJSON, "args", "[]", "Job args as JSON array")
	fs.StringVar(&options.metaJSON, "meta", "", "Job metadata as JSON object")
	fs.IntVar(&options.maxAttempts, "max-attempts", 0, "Max retry attempts")
	fs.StringVar(&options.uniqueKey, "unique-key", "", "Unique job key for deduplication")
	fs.StringVar(&options.uniqueWithin, "unique-within", "", "Uniqueness window (e.g. 1h, 30m)")
	fs.StringVar(&options.batchFile, "batch", "", "NDJSON file for bulk enqueue")
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}
	return options, nil
}

func validateEnqueueOptions(options *enqueueOptions) error {
	switch {
	case options.jobType == "":
		return fmt.Errorf("--type is required\n\nUsage: ojs enqueue --type <type> [--queue <queue>] [--args '<json>']")
	case len(options.jobType) > 255:
		return fmt.Errorf("--type must not exceed 255 characters")
	case !typePattern.MatchString(options.jobType):
		return fmt.Errorf("invalid --type %q: must match ^[a-z][a-z0-9_]*(\\.[a-z][a-z0-9_]*)*$", options.jobType)
	case len(options.queue) > 128:
		return fmt.Errorf("--queue must not exceed 128 characters")
	case !queuePattern.MatchString(options.queue):
		return fmt.Errorf("invalid --queue %q: must match ^[a-z0-9][a-z0-9\\-.]*$", options.queue)
	default:
		return nil
	}
}

func buildEnqueueBody(options *enqueueOptions) (map[string]any, error) {
	body := map[string]any{"type": options.jobType}
	var jobArgs json.RawMessage
	if err := json.Unmarshal([]byte(options.argsJSON), &jobArgs); err != nil {
		return nil, fmt.Errorf("invalid --args JSON: %w", err)
	}
	body["args"] = jobArgs

	opts := map[string]any{"queue": options.queue}
	if options.priority > 0 {
		opts["priority"] = options.priority
	}
	if options.maxAttempts > 0 {
		opts["max_attempts"] = options.maxAttempts
	}
	if options.uniqueKey != "" {
		unique := map[string]any{"key": options.uniqueKey}
		if options.uniqueWithin != "" {
			unique["within"] = options.uniqueWithin
		}
		opts["unique"] = unique
	}
	body["options"] = opts

	if options.metaJSON != "" {
		var meta json.RawMessage
		if err := json.Unmarshal([]byte(options.metaJSON), &meta); err != nil {
			return nil, fmt.Errorf("invalid --meta JSON: %w", err)
		}
		body["meta"] = meta
	}
	return body, nil
}

func renderEnqueueResponse(data []byte) error {
	if output.Format == "json" {
		var result any
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		return output.JSON(result)
	}

	var job map[string]any
	if err := json.Unmarshal(data, &job); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
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
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		return output.JSON(result)
	}

	var resp struct {
		Jobs  []any `json:"jobs"`
		Count int   `json:"count"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	output.Success("Batch enqueue: %d jobs enqueued (from %d submitted)", len(resp.Jobs), len(jobs))
	return nil
}
