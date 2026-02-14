package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Queues lists queues and their stats.
func Queues(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("queues", flag.ExitOnError)
	statsName := fs.String("stats", "", "Show detailed stats for a specific queue")
	pause := fs.String("pause", "", "Pause a queue")
	resume := fs.String("resume", "", "Resume a queue")
	fs.Parse(args)

	if *pause != "" {
		_, _, err := c.Post("/queues/"+*pause+"/pause", nil)
		if err != nil {
			return err
		}
		output.Success("Queue %q paused", *pause)
		return nil
	}

	if *resume != "" {
		_, _, err := c.Post("/queues/"+*resume+"/resume", nil)
		if err != nil {
			return err
		}
		output.Success("Queue %q resumed", *resume)
		return nil
	}

	if *statsName != "" {
		return queueStats(c, *statsName)
	}

	return listQueues(c)
}

func listQueues(c *client.Client) error {
	data, _, err := c.Get("/queues")
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		Queues []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"queues"`
	}
	json.Unmarshal(data, &resp)

	headers := []string{"NAME", "STATUS"}
	rows := make([][]string, 0, len(resp.Queues))
	for _, q := range resp.Queues {
		rows = append(rows, []string{q.Name, q.Status})
	}

	if len(rows) == 0 {
		fmt.Println("No queues found.")
		return nil
	}
	output.Table(headers, rows)
	return nil
}

func queueStats(c *client.Client, name string) error {
	data, _, err := c.Get("/queues/" + name + "/stats")
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var stats struct {
		Queue  string `json:"queue"`
		Status string `json:"status"`
		Stats  struct {
			Available int `json:"available"`
			Active    int `json:"active"`
			Completed int `json:"completed"`
			Scheduled int `json:"scheduled"`
			Retryable int `json:"retryable"`
			Dead      int `json:"dead"`
		} `json:"stats"`
	}
	json.Unmarshal(data, &stats)

	headers := []string{"METRIC", "COUNT"}
	rows := [][]string{
		{"Queue", stats.Queue},
		{"Status", stats.Status},
		{"Available", fmt.Sprintf("%d", stats.Stats.Available)},
		{"Active", fmt.Sprintf("%d", stats.Stats.Active)},
		{"Completed", fmt.Sprintf("%d", stats.Stats.Completed)},
		{"Scheduled", fmt.Sprintf("%d", stats.Stats.Scheduled)},
		{"Retryable", fmt.Sprintf("%d", stats.Stats.Retryable)},
		{"Dead", fmt.Sprintf("%d", stats.Stats.Dead)},
	}
	output.Table(headers, rows)
	return nil
}
