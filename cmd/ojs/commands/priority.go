package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Priority updates job priority.
func Priority(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("priority", flag.ExitOnError)
	set := fs.Int("set", -1, "New priority value (0-255)")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("job ID required\n\nUsage: ojs priority <job-id> --set <priority>")
	}

	jobID := remaining[0]

	if *set < 0 {
		return fmt.Errorf("--set is required\n\nUsage: ojs priority <job-id> --set <priority>")
	}

	body := map[string]any{
		"priority": *set,
	}

	data, _, err := c.Patch("/jobs/"+jobID, body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	output.Success("Job %s priority updated to %d", jobID, *set)
	return nil
}
