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
	create := fs.String("create", "", "Create a new queue")
	deleteQueue := fs.String("delete", "", "Delete a queue")
	purge := fs.String("purge", "", "Purge completed jobs from a queue")
	concurrency := fs.Int("concurrency", 0, "Concurrency limit (for create/config)")
	maxSize := fs.Int("max-size", 0, "Max queue size (for create/config)")
	purgeStates := fs.String("states", "completed", "States to purge (comma-separated)")
	configQueue := fs.String("config", "", "Update configuration for a queue")
	retention := fs.String("retention", "", "Retention duration (for config, e.g. 24h, 7d)")
	fs.Parse(args)

	if *configQueue != "" {
		return updateQueueConfig(c, *configQueue, *concurrency, *maxSize, *retention)
	}

	if *create != "" {
		return createQueue(c, *create, *concurrency, *maxSize)
	}

	if *deleteQueue != "" {
		return deleteQueueCmd(c, *deleteQueue)
	}

	if *purge != "" {
		return purgeQueue(c, *purge, *purgeStates)
	}

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

func createQueue(c *client.Client, name string, concurrency, maxSize int) error {
	body := map[string]any{
		"name": name,
	}
	if concurrency > 0 {
		body["concurrency"] = concurrency
	}
	if maxSize > 0 {
		body["max_size"] = maxSize
	}

	data, _, err := c.Post("/queues", body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	output.Success("Queue %q created", name)
	return nil
}

func deleteQueueCmd(c *client.Client, name string) error {
	_, _, err := c.Delete("/queues/" + name)
	if err != nil {
		return err
	}
	output.Success("Queue %q deleted", name)
	return nil
}

func purgeQueue(c *client.Client, name, states string) error {
	body := map[string]any{
		"states": splitCommaStr(states),
	}

	data, _, err := c.Post("/queues/"+name+"/purge", body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		Deleted int `json:"deleted"`
	}
	json.Unmarshal(data, &resp)
	output.Success("Purged %d jobs from queue %q", resp.Deleted, name)
	return nil
}

func splitCommaStr(s string) []string {
	result := []string{}
	current := ""
	for _, ch := range s {
		if ch == ',' {
			if current != "" {
				result = append(result, current)
			}
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func updateQueueConfig(c *client.Client, name string, concurrency, maxSize int, retention string) error {
	body := map[string]any{}
	if concurrency > 0 {
		body["concurrency"] = concurrency
	}
	if maxSize > 0 {
		body["max_size"] = maxSize
	}
	if retention != "" {
		body["retention"] = retention
	}

	if len(body) == 0 {
		return fmt.Errorf("at least one config option is required\n\n" +
			"Usage: ojs queues --config <name> [--concurrency <n>] [--max-size <n>] [--retention <duration>]")
	}

	data, _, err := c.Put("/admin/queues/"+name+"/config", body)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	output.Success("Queue %q configuration updated", name)
	return nil
}
