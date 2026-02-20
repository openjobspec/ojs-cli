package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// sidekiqConfig represents a Sidekiq YAML configuration file.
type sidekiqConfig struct {
	Concurrency int              `yaml:":concurrency"`
	Queues      []string         `yaml:":queues"`
	Workers     []sidekiqWorker  `yaml:"workers"`
}

type sidekiqWorker struct {
	Class string `yaml:"class"`
	Queue string `yaml:"queue"`
	Retry any    `yaml:"retry"`
	Cron  string `yaml:"cron"`
}

// ojsJobDefinition is the output format for converted job definitions.
type ojsJobDefinition struct {
	Type    string             `json:"type"`
	Queue   string             `json:"queue"`
	Options ojsJobOptions      `json:"options,omitempty"`
	Cron    string             `json:"cron,omitempty"`
}

type ojsJobOptions struct {
	Queue       string          `json:"queue"`
	MaxAttempts int             `json:"max_attempts,omitempty"`
	Retry       *ojsRetryPolicy `json:"retry,omitempty"`
}

type ojsRetryPolicy struct {
	MaxAttempts int    `json:"max_attempts"`
	Backoff     string `json:"backoff,omitempty"`
	Delay       int    `json:"delay_ms,omitempty"`
}

// ojsMigrateOutput is the top-level output for all migrate converters.
type ojsMigrateOutput struct {
	Source   string             `json:"source"`
	Jobs     []ojsJobDefinition `json:"jobs"`
	Warnings []string           `json:"warnings,omitempty"`
}

func migrateSidekiq(args []string, dryRun bool, outputFile string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing config file\n\nUsage: ojs migrate sidekiq <config-file> [--output <file>] [--dry-run]")
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	result, err := convertSidekiqConfig(data)
	if err != nil {
		return fmt.Errorf("convert sidekiq config: %w", err)
	}

	if dryRun {
		fmt.Fprintf(os.Stderr, "Dry run: would convert %d Sidekiq workers to OJS job definitions\n", len(result.Jobs))
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "  ⚠ %s\n", w)
		}
		return nil
	}

	return writeOutput(result, outputFile)
}

func convertSidekiqConfig(data []byte) (*ojsMigrateOutput, error) {
	var cfg sidekiqConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	result := &ojsMigrateOutput{Source: "sidekiq"}

	for _, w := range cfg.Workers {
		jobType := sidekiqClassToOJSType(w.Class)
		queue := w.Queue
		if queue == "" {
			queue = "default"
		}

		job := ojsJobDefinition{
			Type:  jobType,
			Queue: queue,
			Options: ojsJobOptions{
				Queue: queue,
			},
		}

		switch v := w.Retry.(type) {
		case int:
			job.Options.Retry = &ojsRetryPolicy{
				MaxAttempts: v,
				Backoff:     "exponential",
			}
			job.Options.MaxAttempts = v
		case bool:
			if v {
				// Sidekiq default: 25 retries
				job.Options.Retry = &ojsRetryPolicy{
					MaxAttempts: 25,
					Backoff:     "exponential",
				}
				job.Options.MaxAttempts = 25
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s: using Sidekiq default 25 retries; consider reducing for OJS", w.Class))
			}
		}

		if w.Cron != "" {
			job.Cron = w.Cron
		}

		result.Jobs = append(result.Jobs, job)
	}

	return result, nil
}

// sidekiqClassToOJSType converts a Ruby class name to an OJS type.
func sidekiqClassToOJSType(class string) string {
	s := strings.ReplaceAll(class, "::", ".")
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := rune(s[i-1])
			if prev != '.' && prev >= 'a' && prev <= 'z' {
				result = append(result, '.')
			}
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

func writeOutput(result *ojsMigrateOutput, outputFile string) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	writer := os.Stdout

	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()
		writer = f
		enc = json.NewEncoder(f)
		enc.SetIndent("", "  ")
	}
	_ = writer

	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	if outputFile != "" {
		fmt.Fprintf(os.Stderr, "✓ Wrote %d job definitions to %s\n", len(result.Jobs), outputFile)
	}

	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "⚠ %s\n", w)
	}

	return nil
}
