package commands

import (
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Status retrieves the status of a job.
func Status(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	detail := fs.Bool("detail", false, "Show full job envelope with args, meta, and errors")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("job ID required\n\nUsage: ojs status <job-id> [--detail]")
	}

	jobID := remaining[0]

	if *detail {
		return jobDetail(c, jobID)
	}

	data, _, err := c.Get("/jobs/" + jobID)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		return printJSONResponse(data)
	}

	var job map[string]any
	if err := decodeResponse(data, &job); err != nil {
		return err
	}

	rows, err := statusRows(job)
	if err != nil {
		return err
	}
	output.Table([]string{"FIELD", "VALUE"}, rows)
	return nil
}

func statusRows(job map[string]any) ([][]string, error) {
	rows := [][]string{
		{"ID", str(job["id"])},
		{"Type", str(job["type"])},
		{"State", str(job["state"])},
		{"Queue", str(job["queue"])},
		{"Attempt", str(job["attempt"])},
		{"Priority", str(job["priority"])},
		{"Created", str(job["created_at"])},
	}
	if job["scheduled_at"] != nil {
		rows = append(rows, []string{"Scheduled", str(job["scheduled_at"])})
	}
	if job["completed_at"] != nil {
		rows = append(rows, []string{"Completed", str(job["completed_at"])})
	}
	if job["progress"] != nil {
		rows = append(rows, []string{"Progress", fmt.Sprintf("%.0f%%", toFloat(job["progress"])*100)})
	}
	if job["progress_data"] != nil {
		progressJSON, err := formatJSONValue(job["progress_data"])
		if err != nil {
			return nil, err
		}
		rows = append(rows, []string{"Progress Data", progressJSON})
	}
	if job["error"] != nil {
		rows = append(rows, []string{"Error", str(job["error"])})
	}
	return rows, nil
}

func str(v any) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%v", v)
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return 0
	}
}

func jobDetail(c *client.Client, jobID string) error {
	data, _, err := c.Get("/admin/jobs/" + jobID)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		return printJSONResponse(data)
	}

	var job map[string]any
	if err := decodeResponse(data, &job); err != nil {
		return err
	}

	rows, err := detailedJobRows(job)
	if err != nil {
		return err
	}
	output.Table([]string{"FIELD", "VALUE"}, rows)
	return nil
}

func detailedJobRows(job map[string]any) ([][]string, error) {
	rows := [][]string{
		{"ID", str(job["id"])},
		{"Type", str(job["type"])},
		{"State", str(job["state"])},
		{"Queue", str(job["queue"])},
		{"Attempt", str(job["attempt"])},
		{"Priority", str(job["priority"])},
		{"Created", str(job["created_at"])},
	}
	for _, field := range []struct {
		key    string
		label  string
		format string
	}{
		{key: "args", label: "Args", format: "json"},
		{key: "meta", label: "Meta", format: "json"},
		{key: "options", label: "Options", format: "json"},
		{key: "scheduled_at", label: "Scheduled"},
		{key: "started_at", label: "Started"},
		{key: "completed_at", label: "Completed"},
		{key: "progress", label: "Progress", format: "progress"},
		{key: "result", label: "Result", format: "json"},
		{key: "error", label: "Error"},
		{key: "errors", label: "Error History", format: "json"},
	} {
		if job[field.key] == nil {
			continue
		}
		value := str(job[field.key])
		switch field.format {
		case "json":
			formatted, err := formatJSONValue(job[field.key])
			if err != nil {
				return nil, err
			}
			value = formatted
		case "progress":
			value = fmt.Sprintf("%.0f%%", toFloat(job[field.key])*100)
		}
		rows = append(rows, []string{field.label, value})
	}
	return rows, nil
}
