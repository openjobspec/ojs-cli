package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Status retrieves the status of a job.
func Status(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	detail := fs.Bool("detail", false, "Show full job envelope with args, meta, and errors")
	fs.Parse(args)

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
		progressJSON, _ := json.Marshal(job["progress_data"])
		rows = append(rows, []string{"Progress Data", string(progressJSON)})
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
		{"Priority", str(job["priority"])},
		{"Created", str(job["created_at"])},
	}
	if job["args"] != nil {
		argsJSON, _ := json.Marshal(job["args"])
		rows = append(rows, []string{"Args", string(argsJSON)})
	}
	if job["meta"] != nil {
		metaJSON, _ := json.Marshal(job["meta"])
		rows = append(rows, []string{"Meta", string(metaJSON)})
	}
	if job["options"] != nil {
		optsJSON, _ := json.Marshal(job["options"])
		rows = append(rows, []string{"Options", string(optsJSON)})
	}
	if job["scheduled_at"] != nil {
		rows = append(rows, []string{"Scheduled", str(job["scheduled_at"])})
	}
	if job["started_at"] != nil {
		rows = append(rows, []string{"Started", str(job["started_at"])})
	}
	if job["completed_at"] != nil {
		rows = append(rows, []string{"Completed", str(job["completed_at"])})
	}
	if job["progress"] != nil {
		rows = append(rows, []string{"Progress", fmt.Sprintf("%.0f%%", toFloat(job["progress"])*100)})
	}
	if job["result"] != nil {
		resultJSON, _ := json.Marshal(job["result"])
		rows = append(rows, []string{"Result", string(resultJSON)})
	}
	if job["error"] != nil {
		rows = append(rows, []string{"Error", str(job["error"])})
	}
	if job["errors"] != nil {
		errorsJSON, _ := json.Marshal(job["errors"])
		rows = append(rows, []string{"Error History", string(errorsJSON)})
	}
	output.Table(headers, rows)
	return nil
}
