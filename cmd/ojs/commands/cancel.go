package commands

import (
	"encoding/json"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Cancel cancels a job by ID.
func Cancel(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("job ID required\n\nUsage: ojs cancel <job-id>")
	}

	jobID := args[0]
	data, _, err := c.Delete("/jobs/" + jobID)
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

	var job map[string]any
	if err := json.Unmarshal(data, &job); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	output.Success("Job %s cancelled (state=%s)", jobID, job["state"])
	return nil
}
