package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Workers lists active workers and manages worker state.
func Workers(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("workers", flag.ExitOnError)
	quiet := fs.Bool("quiet", false, "Signal all workers to stop fetching new jobs")
	resume := fs.Bool("resume", false, "Signal all workers to resume fetching jobs")
	fs.Parse(args)

	if *quiet {
		return setWorkerDirective(c, "quiet")
	}
	if *resume {
		return setWorkerDirective(c, "resume")
	}

	data, _, err := c.Get("/admin/workers")
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		Items []struct {
			ID            string `json:"id"`
			State         string `json:"state"`
			Directive     string `json:"directive"`
			ActiveJobs    int    `json:"active_jobs"`
			LastHeartbeat string `json:"last_heartbeat"`
		} `json:"items"`
		Summary struct {
			Total   int `json:"total"`
			Running int `json:"running"`
			Quiet   int `json:"quiet"`
			Stale   int `json:"stale"`
		} `json:"summary"`
	}
	json.Unmarshal(data, &resp)

	fmt.Printf("Workers: %d total, %d running, %d quiet, %d stale\n\n",
		resp.Summary.Total, resp.Summary.Running, resp.Summary.Quiet, resp.Summary.Stale)

	if len(resp.Items) == 0 {
		fmt.Println("No workers found.")
		return nil
	}

	headers := []string{"ID", "STATE", "DIRECTIVE", "ACTIVE JOBS", "LAST HEARTBEAT"}
	rows := make([][]string, 0, len(resp.Items))
	for _, w := range resp.Items {
		rows = append(rows, []string{
			w.ID, w.State, w.Directive,
			fmt.Sprintf("%d", w.ActiveJobs), w.LastHeartbeat,
		})
	}
	output.Table(headers, rows)
	return nil
}

func setWorkerDirective(c *client.Client, directive string) error {
	body := map[string]any{
		"directive": directive,
	}

	data, _, err := c.Post("/admin/workers/directive", body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	switch directive {
	case "quiet":
		output.Success("Workers signaled to stop fetching new jobs")
	case "resume":
		output.Success("Workers signaled to resume fetching jobs")
	}
	return nil
}
