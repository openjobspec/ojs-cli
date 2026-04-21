package commands

import (
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Workers lists active workers and manages worker state.
func Workers(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("workers", flag.ContinueOnError)
	quiet := fs.Bool("quiet", false, "Signal all workers to stop fetching new jobs")
	resume := fs.Bool("resume", false, "Signal all workers to resume fetching jobs")
	detail := fs.String("detail", "", "Show detailed info for a specific worker ID")
	quietWorker := fs.String("quiet-worker", "", "Signal a specific worker to stop fetching")
	deregister := fs.String("deregister", "", "Deregister a stale worker by ID")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	if *detail != "" {
		return workerDetail(c, *detail)
	}
	if *quietWorker != "" {
		return quietSpecificWorker(c, *quietWorker)
	}
	if *deregister != "" {
		return deregisterWorker(c, *deregister)
	}
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
		return printJSONResponse(data)
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
	if err := decodeResponse(data, &resp); err != nil {
		return err
	}

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
		return printJSONResponse(data)
	}

	switch directive {
	case "quiet":
		output.Success("Workers signaled to stop fetching new jobs")
	case "resume":
		output.Success("Workers signaled to resume fetching jobs")
	}
	return nil
}

func workerDetail(c *client.Client, workerID string) error {
	data, _, err := c.Get("/admin/workers/" + workerID)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		return printJSONResponse(data)
	}

	var w struct {
		ID            string   `json:"id"`
		State         string   `json:"state"`
		Directive     string   `json:"directive"`
		ActiveJobs    int      `json:"active_jobs"`
		Queues        []string `json:"queues"`
		LastHeartbeat string   `json:"last_heartbeat"`
		StartedAt     string   `json:"started_at"`
		Hostname      string   `json:"hostname"`
		PID           int      `json:"pid"`
	}
	if err := decodeResponse(data, &w); err != nil {
		return err
	}

	queuesStr := "-"
	if len(w.Queues) > 0 {
		queuesStr = ""
		for i, q := range w.Queues {
			if i > 0 {
				queuesStr += ", "
			}
			queuesStr += q
		}
	}

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", w.ID},
		{"State", w.State},
		{"Directive", w.Directive},
		{"Active Jobs", fmt.Sprintf("%d", w.ActiveJobs)},
		{"Queues", queuesStr},
		{"Hostname", w.Hostname},
		{"PID", fmt.Sprintf("%d", w.PID)},
		{"Started", w.StartedAt},
		{"Last Heartbeat", w.LastHeartbeat},
	}
	output.Table(headers, rows)
	return nil
}

func quietSpecificWorker(c *client.Client, workerID string) error {
	_, _, err := c.Post("/admin/workers/"+workerID+"/quiet", nil)
	if err != nil {
		return err
	}
	output.Success("Worker %s signaled to stop fetching", workerID)
	return nil
}

func deregisterWorker(c *client.Client, workerID string) error {
	_, _, err := c.Delete("/admin/workers/" + workerID)
	if err != nil {
		return err
	}
	output.Success("Worker %s deregistered", workerID)
	return nil
}
