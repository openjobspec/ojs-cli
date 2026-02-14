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
	fs.Parse(args)

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

	return listCron(c)
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

func listCron(c *client.Client) error {
	data, _, err := c.Get("/cron")
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
