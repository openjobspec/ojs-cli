package commands

import (
	"encoding/json"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Status retrieves the status of a job.
func Status(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("job ID required\n\nUsage: ojs status <job-id>")
	}

	jobID := args[0]
	data, _, err := c.Get("/jobs/" + jobID)
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

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", str(job["id"])},
		{"Type", str(job["type"])},
		{"State", str(job["state"])},
		{"Queue", str(job["queue"])},
		{"Attempt", str(job["attempt"])},
		{"Created", str(job["created_at"])},
	}
	if job["completed_at"] != nil {
		rows = append(rows, []string{"Completed", str(job["completed_at"])})
	}
	if job["error"] != nil {
		rows = append(rows, []string{"Error", str(job["error"])})
	}
	output.Table(headers, rows)
	return nil
}

func str(v any) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%v", v)
}
