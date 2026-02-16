package commands

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// DeadLetter manages the dead letter queue.
func DeadLetter(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("dead-letter", flag.ExitOnError)
	retryID := fs.String("retry", "", "Retry a dead letter job by ID")
	deleteID := fs.String("delete", "", "Delete a dead letter job by ID")
	limit := fs.Int("limit", 25, "Max results to return")
	purge := fs.Bool("purge", false, "Purge all dead letter jobs")
	stats := fs.Bool("stats", false, "Show dead letter queue statistics")
	olderThan := fs.String("older-than", "", "Purge jobs older than duration (e.g. 7d, 24h)")
	fs.Parse(args)

	if *stats {
		return deadLetterStats(c)
	}

	if *purge {
		return deadLetterPurge(c, *olderThan)
	}

	if *retryID != "" {
		data, _, err := c.Post("/dead-letter/"+*retryID+"/retry", nil)
		if err != nil {
			return err
		}
		if output.Format == "json" {
			var result any
			json.Unmarshal(data, &result)
			return output.JSON(result)
		}
		output.Success("Dead letter job %s retried", *retryID)
		return nil
	}

	if *deleteID != "" {
		_, _, err := c.Delete("/dead-letter/" + *deleteID)
		if err != nil {
			return err
		}
		output.Success("Dead letter job %s deleted", *deleteID)
		return nil
	}

	return listDeadLetter(c, *limit)
}

func listDeadLetter(c *client.Client, limit int) error {
	data, _, err := c.Get(fmt.Sprintf("/dead-letter?limit=%d", limit))
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		Jobs  []map[string]any `json:"jobs"`
		Total int              `json:"total"`
	}
	json.Unmarshal(data, &resp)

	fmt.Printf("Dead letter jobs: %d total\n\n", resp.Total)

	if len(resp.Jobs) == 0 {
		fmt.Println("No dead letter jobs.")
		return nil
	}

	headers := []string{"ID", "TYPE", "QUEUE", "ATTEMPTS", "DISCARDED AT"}
	rows := make([][]string, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		rows = append(rows, []string{
			str(j["id"]), str(j["type"]), str(j["queue"]),
			str(j["attempt"]), str(j["discarded_at"]),
		})
	}
	output.Table(headers, rows)
	return nil
}

func deadLetterStats(c *client.Client) error {
	data, _, err := c.Get("/dead-letter/stats")
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		Total    int `json:"total"`
		ByQueue  map[string]int `json:"by_queue"`
		ByType   map[string]int `json:"by_type"`
	}
	json.Unmarshal(data, &resp)

	fmt.Printf("Dead letter statistics: %d total\n\n", resp.Total)

	if len(resp.ByQueue) > 0 {
		fmt.Println("By Queue:")
		headers := []string{"QUEUE", "COUNT"}
		rows := make([][]string, 0, len(resp.ByQueue))
		for q, count := range resp.ByQueue {
			rows = append(rows, []string{q, fmt.Sprintf("%d", count)})
		}
		output.Table(headers, rows)
		fmt.Println()
	}

	if len(resp.ByType) > 0 {
		fmt.Println("By Job Type:")
		headers := []string{"TYPE", "COUNT"}
		rows := make([][]string, 0, len(resp.ByType))
		for t, count := range resp.ByType {
			rows = append(rows, []string{t, fmt.Sprintf("%d", count)})
		}
		output.Table(headers, rows)
	}

	return nil
}

func deadLetterPurge(c *client.Client, olderThan string) error {
	path := "/dead-letter/purge"
	if olderThan != "" {
		path += "?older_than=" + olderThan
	}

	data, _, err := c.Post(path, nil)
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
	output.Success("Purged %d dead letter jobs", resp.Deleted)
	return nil
}
