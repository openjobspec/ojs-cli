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
	fs.Parse(args)

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
