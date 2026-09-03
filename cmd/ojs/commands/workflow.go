package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Workflow manages workflows.
func Workflow(c *client.Client, args []string) error {
	if len(args) == 0 {
		return printWorkflowUsage()
	}

	switch args[0] {
	case "create":
		return workflowCreate(c, args[1:])
	case "status":
		return workflowStatus(c, args[1:])
	case "cancel":
		return workflowCancel(c, args[1:])
	case "list":
		return workflowList(c, args[1:])
	default:
		return printWorkflowUsage()
	}
}

func workflowCreate(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("workflow create", flag.ContinueOnError)
	name := fs.String("name", "", "Workflow name (required)")
	stepsJSON := fs.String("steps", "", "Steps as JSON array (required)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	if *name == "" || *stepsJSON == "" {
		return fmt.Errorf("--name and --steps are required\n\n" +
			"Usage: ojs workflow create --name <name> --steps '<json>'\n\n" +
			"Example:\n" +
			`  ojs workflow create --name order-pipeline --steps '[{"id":"validate","type":"order.validate","args":["order-123"]},{"id":"charge","type":"payment.charge","args":["order-123"],"depends_on":["validate"]}]'`)
	}

	var steps json.RawMessage
	if err := json.Unmarshal([]byte(*stepsJSON), &steps); err != nil {
		return fmt.Errorf("invalid --steps JSON: %w", err)
	}

	body := map[string]any{
		"name":  *name,
		"steps": steps,
	}

	data, _, err := c.Post("/workflows", body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		return printJSONResponse(data)
	}

	var wf map[string]any
	if err := decodeResponse(data, &wf); err != nil {
		return err
	}
	output.Success("Workflow created: %s (id=%s, state=%s)", *name, str(wf["id"]), str(wf["state"]))
	return nil
}

func workflowStatus(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID required\n\nUsage: ojs workflow status <workflow-id>")
	}

	wfID := args[0]
	data, _, err := c.Get("/workflows/" + wfID)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		return printJSONResponse(data)
	}

	var wf struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		State string `json:"state"`
		Steps []struct {
			ID          string `json:"id"`
			Type        string `json:"type"`
			State       string `json:"state"`
			JobID       string `json:"job_id"`
			StartedAt   string `json:"started_at"`
			CompletedAt string `json:"completed_at"`
		} `json:"steps"`
		CreatedAt   string `json:"created_at"`
		CompletedAt string `json:"completed_at"`
	}
	if err := decodeResponse(data, &wf); err != nil {
		return err
	}

	fmt.Printf("Workflow: %s (%s)\n", wf.Name, wf.ID)
	fmt.Printf("State:    %s\n", wf.State)
	fmt.Printf("Created:  %s\n", wf.CreatedAt)
	if wf.CompletedAt != "" {
		fmt.Printf("Completed: %s\n", wf.CompletedAt)
	}
	fmt.Println()

	if len(wf.Steps) > 0 {
		headers := []string{"STEP", "TYPE", "STATE", "JOB ID", "STARTED", "COMPLETED"}
		rows := make([][]string, 0, len(wf.Steps))
		for _, s := range wf.Steps {
			started := s.StartedAt
			if started == "" {
				started = "-"
			}
			completed := s.CompletedAt
			if completed == "" {
				completed = "-"
			}
			jobID := s.JobID
			if jobID == "" {
				jobID = "-"
			}
			rows = append(rows, []string{s.ID, s.Type, s.State, jobID, started, completed})
		}
		output.Table(headers, rows)
	}

	return nil
}

func workflowCancel(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow ID required\n\nUsage: ojs workflow cancel <workflow-id>")
	}

	wfID := args[0]
	data, _, err := c.Delete("/workflows/" + wfID)
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
	output.Success("Workflow %s cancelled (cancelled_steps=%s)", wfID, str(resp["cancelled_steps"]))
	return nil
}

func workflowList(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("workflow list", flag.ContinueOnError)
	limit := fs.Int("limit", 25, "Max results to return")
	state := fs.String("state", "", "Filter by state (running, completed, failed, cancelled)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	path := fmt.Sprintf("/workflows?limit=%d", *limit)
	if *state != "" {
		path += "&state=" + *state
	}

	data, _, err := c.Get(path)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		return printJSONResponse(data)
	}

	var resp struct {
		Workflows []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			State       string `json:"state"`
			StepCount   int    `json:"step_count"`
			CreatedAt   string `json:"created_at"`
			CompletedAt string `json:"completed_at"`
		} `json:"workflows"`
		Total int `json:"total"`
	}
	if err := decodeResponse(data, &resp); err != nil {
		return err
	}

	fmt.Printf("Workflows: %d total\n\n", resp.Total)

	if len(resp.Workflows) == 0 {
		fmt.Println("No workflows found.")
		return nil
	}

	headers := []string{"ID", "NAME", "STATE", "STEPS", "CREATED", "COMPLETED"}
	rows := make([][]string, 0, len(resp.Workflows))
	for _, wf := range resp.Workflows {
		completed := wf.CompletedAt
		if completed == "" {
			completed = "-"
		}
		rows = append(rows, []string{
			wf.ID, wf.Name, wf.State,
			fmt.Sprintf("%d", wf.StepCount),
			wf.CreatedAt, completed,
		})
	}
	output.Table(headers, rows)
	return nil
}

func printWorkflowUsage() error {
	return fmt.Errorf("subcommand required\n\nUsage: ojs workflow <subcommand>\n\n" +
		"Subcommands:\n" +
		"  create   Create a new workflow\n" +
		"  status   Get workflow status\n" +
		"  cancel   Cancel a workflow\n" +
		"  list     List workflows")
}
