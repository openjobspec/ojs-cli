package commands

import (
	"encoding/json"
	"fmt"
	"os"
)

// bullmqConfig represents a BullMQ configuration file.
type bullmqConfig struct {
	Queues []bullmqQueue `json:"queues"`
}

type bullmqQueue struct {
	Name string       `json:"name"`
	Jobs []bullmqJob  `json:"jobs"`
}

type bullmqJob struct {
	Name string         `json:"name"`
	Opts bullmqJobOpts  `json:"opts"`
}

type bullmqJobOpts struct {
	Attempts int             `json:"attempts"`
	Delay    int64           `json:"delay"`
	Backoff  *bullmqBackoff  `json:"backoff,omitempty"`
}

type bullmqBackoff struct {
	Type  string `json:"type"`
	Delay int64  `json:"delay"`
}

func migrateBullMQ(args []string, dryRun bool, outputFile string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing config file\n\nUsage: ojs migrate bullmq <config-file> [--output <file>] [--dry-run]")
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	result, err := convertBullMQConfig(data)
	if err != nil {
		return fmt.Errorf("convert bullmq config: %w", err)
	}

	if dryRun {
		fmt.Fprintf(os.Stderr, "Dry run: would convert %d BullMQ jobs to OJS job definitions\n", len(result.Jobs))
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "  ⚠ %s\n", w)
		}
		return nil
	}

	return writeOutput(result, outputFile)
}

func convertBullMQConfig(data []byte) (*ojsMigrateOutput, error) {
	var cfg bullmqConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	result := &ojsMigrateOutput{Source: "bullmq"}

	for _, q := range cfg.Queues {
		for _, j := range q.Jobs {
			job := ojsJobDefinition{
				Type:  j.Name,
				Queue: q.Name,
				Options: ojsJobOptions{
					Queue: q.Name,
				},
			}

			if j.Opts.Attempts > 0 {
				backoff := "exponential"
				var delayMs int
				if j.Opts.Backoff != nil {
					switch j.Opts.Backoff.Type {
					case "fixed":
						backoff = "fixed"
					case "exponential":
						backoff = "exponential"
					default:
						backoff = "exponential"
						result.Warnings = append(result.Warnings,
							fmt.Sprintf("%s: unsupported backoff type %q, defaulting to exponential", j.Name, j.Opts.Backoff.Type))
					}
					delayMs = int(j.Opts.Backoff.Delay)
				}

				job.Options.MaxAttempts = j.Opts.Attempts
				job.Options.Retry = &ojsRetryPolicy{
					MaxAttempts: j.Opts.Attempts,
					Backoff:     backoff,
					Delay:       delayMs,
				}
			}

			if j.Opts.Delay > 0 {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s: static delay (%dms) converted to scheduled_at at enqueue time", j.Name, j.Opts.Delay))
			}

			result.Jobs = append(result.Jobs, job)
		}
	}

	return result, nil
}
