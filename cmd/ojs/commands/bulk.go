package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Bulk manages bulk operations on jobs.
func Bulk(c *client.Client, args []string) error {
	if len(args) == 0 {
		return printBulkUsage()
	}

	switch args[0] {
	case "cancel":
		return bulkCancel(c, args[1:])
	case "retry":
		return bulkRetry(c, args[1:])
	case "delete":
		return bulkDelete(c, args[1:])
	default:
		return printBulkUsage()
	}
}

func bulkCancel(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("bulk cancel", flag.ContinueOnError)
	ids := fs.String("ids", "", "Comma-separated job IDs (required)")
	state := fs.String("state", "", "Cancel all jobs in this state")
	queue := fs.String("queue", "", "Filter by queue (used with --state)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	body, err := buildBulkBody(*ids, *state, *queue, "")
	if err != nil {
		return fmt.Errorf("%w\n\nUsage: ojs bulk cancel --ids <id1,id2,...>\n       ojs bulk cancel --state <state> [--queue <queue>]", err)
	}

	data, _, err := c.Post("/jobs/bulk/cancel", body)
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
		Cancelled int `json:"cancelled"`
		Failed    int `json:"failed"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	output.Success("Bulk cancel: %d cancelled, %d failed", resp.Cancelled, resp.Failed)
	return nil
}

func bulkRetry(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("bulk retry", flag.ContinueOnError)
	ids := fs.String("ids", "", "Comma-separated job IDs (required)")
	state := fs.String("state", "", "Retry all jobs in this state")
	queue := fs.String("queue", "", "Filter by queue (used with --state)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	body, err := buildBulkBody(*ids, *state, *queue, "")
	if err != nil {
		return fmt.Errorf("%w\n\nUsage: ojs bulk retry --ids <id1,id2,...>\n       ojs bulk retry --state <state> [--queue <queue>]", err)
	}

	data, _, err := c.Post("/jobs/bulk/retry", body)
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
		Retried int `json:"retried"`
		Failed  int `json:"failed"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	output.Success("Bulk retry: %d retried, %d failed", resp.Retried, resp.Failed)
	return nil
}

func splitIDs(s string) []string {
	var ids []string
	for _, id := range splitComma(s) {
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func splitComma(s string) []string {
	result := []string{}
	current := ""
	for _, ch := range s {
		if ch == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	result = append(result, current)
	return result
}

func printBulkUsage() error {
	return fmt.Errorf("subcommand required\n\nUsage: ojs bulk <subcommand>\n\n" +
		"Subcommands:\n" +
		"  cancel   Bulk cancel jobs by IDs or filter\n" +
		"  retry    Bulk retry jobs by IDs or filter\n" +
		"  delete   Bulk delete terminal jobs by IDs or filter")
}

func bulkDelete(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("bulk delete", flag.ContinueOnError)
	ids := fs.String("ids", "", "Comma-separated job IDs")
	state := fs.String("state", "", "Delete all jobs in this terminal state (completed, discarded, cancelled)")
	queue := fs.String("queue", "", "Filter by queue (used with --state)")
	olderThan := fs.String("older-than", "", "Delete jobs older than duration (e.g. 7d, 24h)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	body, err := buildBulkBody(*ids, *state, *queue, *olderThan)
	if err != nil {
		return fmt.Errorf("%w\n\nUsage: ojs bulk delete --ids <id1,id2,...>\n       ojs bulk delete --state <state> [--queue <queue>] [--older-than <duration>]", err)
	}

	data, _, err := c.Post("/jobs/bulk/delete", body)
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
		Deleted int `json:"deleted"`
		Failed  int `json:"failed"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	output.Success("Bulk delete: %d deleted, %d failed", resp.Deleted, resp.Failed)
	return nil
}

func buildBulkBody(ids, state, queue, olderThan string) (map[string]any, error) {
	body := make(map[string]any)
	switch {
	case ids != "":
		body["job_ids"] = splitIDs(ids)
	case state != "":
		filter := map[string]any{"state": state}
		if queue != "" {
			filter["queue"] = queue
		}
		if olderThan != "" {
			filter["older_than"] = olderThan
		}
		body["filter"] = filter
	default:
		return nil, fmt.Errorf("--ids or --state is required")
	}
	return body, nil
}
