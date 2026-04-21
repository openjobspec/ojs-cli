package commands

import (
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Retry retries an individual job by ID.
func Retry(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("job ID required\n\nUsage: ojs retry <job-id>")
	}

	jobID := args[0]
	data, _, err := c.Post("/admin/jobs/"+jobID+"/retry", nil)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		return printJSONResponse(data)
	}

	var resp map[string]any
	if err := decodeResponse(data, &resp); err != nil {
		return err
	}
	output.Success("Job %s retried (state=%s)", jobID, str(resp["state"]))
	return nil
}
