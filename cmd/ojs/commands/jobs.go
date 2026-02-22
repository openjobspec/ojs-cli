package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Jobs lists jobs with optional filtering.
func Jobs(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("jobs", flag.ExitOnError)
	state := fs.String("state", "", "Filter by state (available, active, completed, retryable, discarded, cancelled)")
	queue := fs.String("queue", "", "Filter by queue name")
	jobType := fs.String("type", "", "Filter by job type")
	limit := fs.Int("limit", 25, "Max results to return")
	fs.Parse(args)

	path := fmt.Sprintf("/jobs?limit=%d", *limit)
	if *state != "" {
		path += "&state=" + *state
	}
	if *queue != "" {
		path += "&queue=" + *queue
	}
	if *jobType != "" {
		path += "&type=" + *jobType
	}

	data, _, err := c.Get(path)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		Jobs  []map[string]any `json:"jobs"`
		Total int              `json:"total"`
	}
	json.Unmarshal(data, &resp)

	fmt.Printf("Jobs: %d total\n\n", resp.Total)

	if len(resp.Jobs) == 0 {
		fmt.Println("No jobs found.")
		return nil
	}

	headers := []string{"ID", "TYPE", "STATE", "QUEUE", "ATTEMPT", "CREATED"}
	rows := make([][]string, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		rows = append(rows, []string{
			str(j["id"]), str(j["type"]), str(j["state"]),
			str(j["queue"]), str(j["attempt"]), str(j["created_at"]),
		})
	}
	output.Table(headers, rows)
	return nil
}

