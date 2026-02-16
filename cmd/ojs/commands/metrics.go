package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Metrics retrieves server metrics.
func Metrics(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("metrics", flag.ExitOnError)
	format := fs.String("format", "", "Output format: prometheus or json (default: auto)")
	fs.Parse(args)

	if *format == "prometheus" {
		data, _, err := c.Get("/metrics?format=prometheus")
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	}

	data, _, err := c.Get("/metrics")
	if err != nil {
		return err
	}

	if output.Format == "json" || *format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		Uptime          float64 `json:"uptime_seconds"`
		JobsEnqueued    int     `json:"jobs_enqueued_total"`
		JobsCompleted   int     `json:"jobs_completed_total"`
		JobsFailed      int     `json:"jobs_failed_total"`
		JobsActive      int     `json:"jobs_active"`
		QueuesActive    int     `json:"queues_active"`
		WorkersActive   int     `json:"workers_active"`
		AvgLatencyMs    float64 `json:"avg_latency_ms"`
		ThroughputPerSec float64 `json:"throughput_per_second"`
	}
	json.Unmarshal(data, &resp)

	headers := []string{"METRIC", "VALUE"}
	rows := [][]string{
		{"Uptime", fmt.Sprintf("%.0fs", resp.Uptime)},
		{"Jobs Enqueued", fmt.Sprintf("%d", resp.JobsEnqueued)},
		{"Jobs Completed", fmt.Sprintf("%d", resp.JobsCompleted)},
		{"Jobs Failed", fmt.Sprintf("%d", resp.JobsFailed)},
		{"Jobs Active", fmt.Sprintf("%d", resp.JobsActive)},
		{"Queues Active", fmt.Sprintf("%d", resp.QueuesActive)},
		{"Workers Active", fmt.Sprintf("%d", resp.WorkersActive)},
		{"Avg Latency", fmt.Sprintf("%.2fms", resp.AvgLatencyMs)},
		{"Throughput", fmt.Sprintf("%.2f/s", resp.ThroughputPerSec)},
	}
	output.Table(headers, rows)
	return nil
}
