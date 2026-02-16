package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Stats shows aggregate system statistics.
func Stats(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	history := fs.Bool("history", false, "Show historical time-series statistics")
	period := fs.String("period", "1h", "Aggregation period for history (5m, 1h, 1d)")
	since := fs.String("since", "", "Start time for history (e.g. 2024-01-01T00:00:00Z or 24h)")
	queue := fs.String("queue", "", "Filter stats by queue name")
	fs.Parse(args)

	if *history {
		return statsHistory(c, *period, *since, *queue)
	}

	return statsOverview(c, *queue)
}

func statsOverview(c *client.Client, queue string) error {
	path := "/admin/stats"
	if queue != "" {
		path += "?queue=" + queue
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
		Queues struct {
			Total  int `json:"total"`
			Active int `json:"active"`
			Paused int `json:"paused"`
		} `json:"queues"`
		Workers struct {
			Total   int `json:"total"`
			Running int `json:"running"`
			Quiet   int `json:"quiet"`
			Stale   int `json:"stale"`
		} `json:"workers"`
		Jobs struct {
			Available int `json:"available"`
			Active    int `json:"active"`
			Completed int `json:"completed"`
			Retryable int `json:"retryable"`
			Scheduled int `json:"scheduled"`
			Discarded int `json:"discarded"`
			Cancelled int `json:"cancelled"`
		} `json:"jobs"`
		Throughput struct {
			EnqueuedPerMin int     `json:"enqueued_per_min"`
			CompletedPerMin int    `json:"completed_per_min"`
			FailedPerMin   int     `json:"failed_per_min"`
			AvgLatencyMs   float64 `json:"avg_latency_ms"`
		} `json:"throughput"`
	}
	json.Unmarshal(data, &resp)

	fmt.Println("System Statistics")
	fmt.Println()

	fmt.Println("Queues:")
	headers := []string{"METRIC", "VALUE"}
	rows := [][]string{
		{"Total", fmt.Sprintf("%d", resp.Queues.Total)},
		{"Active", fmt.Sprintf("%d", resp.Queues.Active)},
		{"Paused", fmt.Sprintf("%d", resp.Queues.Paused)},
	}
	output.Table(headers, rows)
	fmt.Println()

	fmt.Println("Workers:")
	rows = [][]string{
		{"Total", fmt.Sprintf("%d", resp.Workers.Total)},
		{"Running", fmt.Sprintf("%d", resp.Workers.Running)},
		{"Quiet", fmt.Sprintf("%d", resp.Workers.Quiet)},
		{"Stale", fmt.Sprintf("%d", resp.Workers.Stale)},
	}
	output.Table(headers, rows)
	fmt.Println()

	fmt.Println("Jobs:")
	rows = [][]string{
		{"Available", fmt.Sprintf("%d", resp.Jobs.Available)},
		{"Active", fmt.Sprintf("%d", resp.Jobs.Active)},
		{"Completed", fmt.Sprintf("%d", resp.Jobs.Completed)},
		{"Retryable", fmt.Sprintf("%d", resp.Jobs.Retryable)},
		{"Scheduled", fmt.Sprintf("%d", resp.Jobs.Scheduled)},
		{"Discarded", fmt.Sprintf("%d", resp.Jobs.Discarded)},
		{"Cancelled", fmt.Sprintf("%d", resp.Jobs.Cancelled)},
	}
	output.Table(headers, rows)
	fmt.Println()

	fmt.Println("Throughput:")
	rows = [][]string{
		{"Enqueued/min", fmt.Sprintf("%d", resp.Throughput.EnqueuedPerMin)},
		{"Completed/min", fmt.Sprintf("%d", resp.Throughput.CompletedPerMin)},
		{"Failed/min", fmt.Sprintf("%d", resp.Throughput.FailedPerMin)},
		{"Avg Latency", fmt.Sprintf("%.2fms", resp.Throughput.AvgLatencyMs)},
	}
	output.Table(headers, rows)
	return nil
}

func statsHistory(c *client.Client, period, since, queue string) error {
	path := fmt.Sprintf("/admin/stats/history?period=%s", period)
	if since != "" {
		path += "&since=" + since
	}
	if queue != "" {
		path += "&queue=" + queue
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
		Period     string `json:"period"`
		DataPoints []struct {
			Timestamp   string `json:"timestamp"`
			Enqueued    int    `json:"enqueued"`
			Completed   int    `json:"completed"`
			Failed      int    `json:"failed"`
			AvgLatencyMs float64 `json:"avg_latency_ms"`
		} `json:"data_points"`
	}
	json.Unmarshal(data, &resp)

	fmt.Printf("Statistics history (period=%s)\n\n", resp.Period)

	if len(resp.DataPoints) == 0 {
		fmt.Println("No data points available.")
		return nil
	}

	headers := []string{"TIMESTAMP", "ENQUEUED", "COMPLETED", "FAILED", "AVG LATENCY"}
	rows := make([][]string, 0, len(resp.DataPoints))
	for _, dp := range resp.DataPoints {
		rows = append(rows, []string{
			dp.Timestamp,
			fmt.Sprintf("%d", dp.Enqueued),
			fmt.Sprintf("%d", dp.Completed),
			fmt.Sprintf("%d", dp.Failed),
			fmt.Sprintf("%.2fms", dp.AvgLatencyMs),
		})
	}
	output.Table(headers, rows)
	return nil
}
