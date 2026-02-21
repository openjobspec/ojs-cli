package commands

import (
	"encoding/json"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Retries shows the retry history for a job.
func Retries(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("job ID required\n\nUsage: ojs retries <job-id>")
	}

	jobID := args[0]
	data, _, err := c.Get("/jobs/" + jobID + "/retries")
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		JobID   string `json:"job_id"`
		Retries []struct {
			Attempt   int    `json:"attempt"`
			State     string `json:"state"`
			Error     string `json:"error"`
			StartedAt string `json:"started_at"`
			FailedAt  string `json:"failed_at"`
			NextRetry string `json:"next_retry_at"`
		} `json:"retries"`
		Policy struct {
			MaxAttempts     int    `json:"max_attempts"`
			InitialInterval string `json:"initial_interval"`
			BackoffStrategy string `json:"backoff_strategy"`
		} `json:"policy"`
	}
	json.Unmarshal(data, &resp)

	fmt.Printf("Retry history for job %s\n", jobID)
	if resp.Policy.MaxAttempts > 0 {
		fmt.Printf("Policy: max_attempts=%d, backoff=%s, initial_interval=%s\n",
			resp.Policy.MaxAttempts, resp.Policy.BackoffStrategy, resp.Policy.InitialInterval)
	}
	fmt.Println()

	if len(resp.Retries) == 0 {
		fmt.Println("No retry attempts recorded.")
		return nil
	}

	headers := []string{"ATTEMPT", "STATE", "ERROR", "STARTED", "FAILED", "NEXT RETRY"}
	rows := make([][]string, 0, len(resp.Retries))
	for _, r := range resp.Retries {
		nextRetry := r.NextRetry
		if nextRetry == "" {
			nextRetry = "-"
		}
		failedAt := r.FailedAt
		if failedAt == "" {
			failedAt = "-"
		}
		errMsg := r.Error
		if errMsg == "" {
			errMsg = "-"
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", r.Attempt), r.State, errMsg,
			r.StartedAt, failedAt, nextRetry,
		})
	}
	output.Table(headers, rows)
	return nil
}

