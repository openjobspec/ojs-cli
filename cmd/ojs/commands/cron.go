package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Cron manages cron jobs.
func Cron(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("cron", flag.ExitOnError)
	register := fs.Bool("register", false, "Register a new cron job")
	deleteName := fs.String("delete", "", "Delete a cron job by name")
	name := fs.String("name", "", "Cron job name (for register)")
	expression := fs.String("expression", "", "Cron expression (for register)")
	jobType := fs.String("type", "", "Job type (for register)")
	queue := fs.String("queue", "default", "Queue (for register)")
	trigger := fs.String("trigger", "", "Trigger a cron job immediately by name")
	history := fs.String("history", "", "Show execution history for a cron job")
	historyLimit := fs.Int("history-limit", 10, "Max history entries to show")
	pause := fs.String("pause", "", "Pause a cron job by name")
	resume := fs.String("resume", "", "Resume a cron job by name")
	detail := fs.String("detail", "", "Show detailed info for a cron job by name")
	update := fs.String("update", "", "Update a cron job by name")
	enabled := fs.String("enabled", "", "Filter list by enabled status (true/false)")
	fs.Parse(args)

	if *detail != "" {
		return cronDetail(c, *detail)
	}

	if *update != "" {
		return cronUpdate(c, *update, *expression, *jobType, *queue)
	}

	if *trigger != "" {
		return triggerCron(c, *trigger)
	}

	if *history != "" {
		return cronHistory(c, *history, *historyLimit)
	}

	if *pause != "" {
		return setCronState(c, *pause, false)
	}

	if *resume != "" {
		return setCronState(c, *resume, true)
	}

	if *deleteName != "" {
		data, _, err := c.Delete("/cron/" + *deleteName)
		if err != nil {
			return err
		}
		if output.Format == "json" {
			var result any
			json.Unmarshal(data, &result)
			return output.JSON(result)
		}
		output.Success("Cron job %q deleted", *deleteName)
		return nil
	}

	if *register {
		return registerCron(c, *name, *expression, *jobType, *queue)
	}

	return listCron(c, *enabled)
}

func registerCron(c *client.Client, name, expression, jobType, queue string) error {
	if name == "" || expression == "" || jobType == "" {
		return fmt.Errorf("--name, --expression, and --type are required for registration\n\n" +
			"Usage: ojs cron --register --name <name> --expression '<cron>' --type <type>")
	}

	body := map[string]any{
		"name":       name,
		"expression": expression,
		"job_template": map[string]any{
			"type": jobType,
			"options": map[string]any{
				"queue": queue,
			},
		},
	}

	data, _, err := c.Post("/cron", body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	output.Success("Cron job %q registered (expression=%s, type=%s)", name, expression, jobType)
	return nil
}

func listCron(c *client.Client, enabledFilter string) error {
	path := "/cron"
	if enabledFilter != "" {
		path += "?enabled=" + enabledFilter
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
		CronJobs []struct {
			Name       string `json:"name"`
			Expression string `json:"expression"`
			Enabled    bool   `json:"enabled"`
			NextRunAt  string `json:"next_run_at"`
			LastRunAt  string `json:"last_run_at"`
		} `json:"cron_jobs"`
	}
	json.Unmarshal(data, &resp)

	if len(resp.CronJobs) == 0 {
		fmt.Println("No cron jobs registered.")
		return nil
	}

	headers := []string{"NAME", "EXPRESSION", "ENABLED", "NEXT RUN", "LAST RUN"}
	rows := make([][]string, 0, len(resp.CronJobs))
	for _, cj := range resp.CronJobs {
		enabled := "✓"
		if !cj.Enabled {
			enabled = "✗"
		}
		rows = append(rows, []string{
			cj.Name, cj.Expression, enabled, cj.NextRunAt, cj.LastRunAt,
		})
	}
	output.Table(headers, rows)
	return nil
}

func triggerCron(c *client.Client, name string) error {
	data, _, err := c.Post("/cron/"+name+"/trigger", nil)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp map[string]any
	json.Unmarshal(data, &resp)
	output.Success("Cron job %q triggered (job_id=%s)", name, str(resp["job_id"]))
	return nil
}

func cronHistory(c *client.Client, name string, limit int) error {
	data, _, err := c.Get(fmt.Sprintf("/cron/%s/history?limit=%d", name, limit))
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		Executions []struct {
			JobID       string `json:"job_id"`
			State       string `json:"state"`
			ScheduledAt string `json:"scheduled_at"`
			StartedAt   string `json:"started_at"`
			CompletedAt string `json:"completed_at"`
		} `json:"executions"`
	}
	json.Unmarshal(data, &resp)

	if len(resp.Executions) == 0 {
		fmt.Printf("No execution history for cron job %q.\n", name)
		return nil
	}

	fmt.Printf("Execution history for %q:\n\n", name)
	headers := []string{"JOB ID", "STATE", "SCHEDULED", "STARTED", "COMPLETED"}
	rows := make([][]string, 0, len(resp.Executions))
	for _, e := range resp.Executions {
		completed := e.CompletedAt
		if completed == "" {
			completed = "-"
		}
		started := e.StartedAt
		if started == "" {
			started = "-"
		}
		rows = append(rows, []string{e.JobID, e.State, e.ScheduledAt, started, completed})
	}
	output.Table(headers, rows)
	return nil
}

func setCronState(c *client.Client, name string, enabled bool) error {
	action := "pause"
	if enabled {
		action = "resume"
	}

	_, _, err := c.Post("/cron/"+name+"/"+action, nil)
	if err != nil {
		return err
	}

	if enabled {
		output.Success("Cron job %q resumed", name)
	} else {
		output.Success("Cron job %q paused", name)
	}
	return nil
}

func cronDetail(c *client.Client, name string) error {
	data, _, err := c.Get("/cron/" + name)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var cj struct {
		Name        string `json:"name"`
		Expression  string `json:"expression"`
		Enabled     bool   `json:"enabled"`
		NextRunAt   string `json:"next_run_at"`
		LastRunAt   string `json:"last_run_at"`
		CreatedAt   string `json:"created_at"`
		JobTemplate struct {
			Type    string         `json:"type"`
			Options map[string]any `json:"options"`
		} `json:"job_template"`
		RunCount int `json:"run_count"`
	}
	json.Unmarshal(data, &cj)

	enabled := "true"
	if !cj.Enabled {
		enabled = "false"
	}
	lastRun := cj.LastRunAt
	if lastRun == "" {
		lastRun = "-"
	}
	optionsJSON, _ := json.Marshal(cj.JobTemplate.Options)

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"Name", cj.Name},
		{"Expression", cj.Expression},
		{"Enabled", enabled},
		{"Job Type", cj.JobTemplate.Type},
		{"Options", string(optionsJSON)},
		{"Next Run", cj.NextRunAt},
		{"Last Run", lastRun},
		{"Run Count", fmt.Sprintf("%d", cj.RunCount)},
		{"Created", cj.CreatedAt},
	}
	output.Table(headers, rows)
	return nil
}

func cronUpdate(c *client.Client, name, expression, jobType, queue string) error {
	body := map[string]any{}

	if expression != "" {
		body["expression"] = expression
	}
	if jobType != "" || queue != "default" {
		jt := map[string]any{}
		if jobType != "" {
			jt["type"] = jobType
		}
		if queue != "default" {
			jt["options"] = map[string]any{"queue": queue}
		}
		body["job_template"] = jt
	}

	if len(body) == 0 {
		return fmt.Errorf("at least one field must be specified for update\n\n" +
			"Usage: ojs cron --update <name> [--expression '<cron>'] [--type <type>] [--queue <queue>]")
	}

	data, _, err := c.Patch("/cron/"+name, body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	output.Success("Cron job %q updated", name)
	return nil
}
