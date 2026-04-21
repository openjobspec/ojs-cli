package commands

import (
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Result retrieves the result of a completed job.
func Result(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("result", flag.ContinueOnError)
	wait := fs.Bool("wait", false, "Wait for job to complete before returning result")
	timeout := fs.Int("timeout", 30, "Timeout in seconds when using --wait")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("job ID required\n\nUsage: ojs result <job-id> [--wait] [--timeout <seconds>]")
	}

	jobID := remaining[0]
	path := fmt.Sprintf("/jobs/%s/result", jobID)
	if *wait {
		path += fmt.Sprintf("?wait=true&timeout=%d", *timeout)
	}

	data, _, err := c.Get(path)
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

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"Job ID", jobID},
		{"State", str(resp["state"])},
	}
	if resp["result"] != nil {
		resultJSON, err := formatJSONValue(resp["result"])
		if err != nil {
			return err
		}
		rows = append(rows, []string{"Result", resultJSON})
	}
	if resp["error"] != nil {
		rows = append(rows, []string{"Error", str(resp["error"])})
	}
	if resp["completed_at"] != nil {
		rows = append(rows, []string{"Completed At", str(resp["completed_at"])})
	}
	output.Table(headers, rows)
	return nil
}
