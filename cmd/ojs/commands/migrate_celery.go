package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// celeryConfig represents a Celery configuration file.
type celeryConfig struct {
	TaskRoutes      map[string]celeryRoute      `json:"task_routes"`
	TaskAnnotations map[string]celeryAnnotation  `json:"task_annotations"`
	BeatSchedule    map[string]celeryBeat        `json:"beat_schedule"`
}

type celeryRoute struct {
	Queue string `json:"queue"`
}

type celeryAnnotation struct {
	MaxRetries int    `json:"max_retries"`
	RateLimit  string `json:"rate_limit"`
}

type celeryBeat struct {
	Task     string             `json:"task"`
	Schedule celeryBeatSchedule `json:"schedule"`
}

type celeryBeatSchedule struct {
	Crontab string `json:"crontab"`
}

func migrateCelery(args []string, dryRun bool, outputFile string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing config file\n\nUsage: ojs migrate celery <config-file> [--output <file>] [--dry-run]")
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	result, err := convertCeleryConfig(data)
	if err != nil {
		return fmt.Errorf("convert celery config: %w", err)
	}

	if dryRun {
		fmt.Fprintf(os.Stderr, "Dry run: would convert %d Celery tasks to OJS job definitions\n", len(result.Jobs))
		for _, w := range result.Warnings {
			fmt.Fprintf(os.Stderr, "  ⚠ %s\n", w)
		}
		return nil
	}

	return writeOutput(result, outputFile)
}

func convertCeleryConfig(data []byte) (*ojsMigrateOutput, error) {
	var cfg celeryConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	result := &ojsMigrateOutput{Source: "celery"}

	// Build cron lookup from beat_schedule
	cronMap := make(map[string]string)
	for _, beat := range cfg.BeatSchedule {
		if beat.Schedule.Crontab != "" {
			cronMap[beat.Task] = beat.Schedule.Crontab
		}
	}

	for task, route := range cfg.TaskRoutes {
		queue := route.Queue
		if queue == "" {
			queue = "celery"
		}

		job := ojsJobDefinition{
			Type:  task,
			Queue: queue,
			Options: ojsJobOptions{
				Queue: queue,
			},
		}

		if ann, ok := cfg.TaskAnnotations[task]; ok {
			if ann.MaxRetries > 0 {
				job.Options.MaxAttempts = ann.MaxRetries
				job.Options.Retry = &ojsRetryPolicy{
					MaxAttempts: ann.MaxRetries,
					Backoff:     "exponential",
				}
			}

			if ann.RateLimit != "" {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("%s: rate_limit %q requires OJS rate limiting extension configuration", task, ann.RateLimit))
			}
		}

		if cron, ok := cronMap[task]; ok {
			job.Cron = cron
		}

		result.Jobs = append(result.Jobs, job)
	}

	// Warn about beat tasks not in task_routes
	for name, beat := range cfg.BeatSchedule {
		if _, ok := cfg.TaskRoutes[beat.Task]; !ok {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("beat schedule %q references task %q not in task_routes", name, beat.Task))
		}
	}

	return result, nil
}

// migrateDetect auto-detects the framework in a directory and shows a migration plan.
func migrateDetect(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing directory\n\nUsage: ojs migrate detect <directory>")
	}

	dir := args[0]
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("access directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	detections := detectFrameworks(dir)
	if len(detections) == 0 {
		return fmt.Errorf("no supported job framework detected in %s", dir)
	}

	type detectResult struct {
		Directory  string            `json:"directory"`
		Frameworks []frameworkDetect `json:"frameworks"`
	}
	result := detectResult{Directory: dir, Frameworks: detections}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

type frameworkDetect struct {
	Name       string `json:"name"`
	Confidence string `json:"confidence"`
	ConfigFile string `json:"config_file"`
	Command    string `json:"migrate_command"`
}

func detectFrameworks(dir string) []frameworkDetect {
	var results []frameworkDetect

	// Check for Sidekiq
	sidekiqFiles := []string{"sidekiq.yml", "config/sidekiq.yml"}
	for _, f := range sidekiqFiles {
		path := dir + "/" + f
		if _, err := os.Stat(path); err == nil {
			results = append(results, frameworkDetect{
				Name:       "sidekiq",
				Confidence: "high",
				ConfigFile: path,
				Command:    fmt.Sprintf("ojs migrate sidekiq %s", path),
			})
			break
		}
	}

	// Check for Gemfile referencing sidekiq
	if gemfileContent, err := os.ReadFile(dir + "/Gemfile"); err == nil {
		if strings.Contains(string(gemfileContent), "sidekiq") {
			found := false
			for _, r := range results {
				if r.Name == "sidekiq" {
					found = true
					break
				}
			}
			if !found {
				results = append(results, frameworkDetect{
					Name:       "sidekiq",
					Confidence: "medium",
					ConfigFile: "",
					Command:    "ojs migrate sidekiq <config-file>",
				})
			}
		}
	}

	// Check for BullMQ
	if pkgContent, err := os.ReadFile(dir + "/package.json"); err == nil {
		if strings.Contains(string(pkgContent), "bullmq") {
			results = append(results, frameworkDetect{
				Name:       "bullmq",
				Confidence: "high",
				ConfigFile: "",
				Command:    "ojs migrate bullmq <config-file>",
			})
		}
	}

	// Check for Celery
	celeryFiles := []string{"celeryconfig.py", "celery.json", "celeryconfig.json"}
	for _, f := range celeryFiles {
		path := dir + "/" + f
		if _, err := os.Stat(path); err == nil {
			results = append(results, frameworkDetect{
				Name:       "celery",
				Confidence: "high",
				ConfigFile: path,
				Command:    fmt.Sprintf("ojs migrate celery %s", path),
			})
			break
		}
	}

	// Check for requirements.txt referencing celery
	if reqContent, err := os.ReadFile(dir + "/requirements.txt"); err == nil {
		if strings.Contains(string(reqContent), "celery") {
			found := false
			for _, r := range results {
				if r.Name == "celery" {
					found = true
					break
				}
			}
			if !found {
				results = append(results, frameworkDetect{
					Name:       "celery",
					Confidence: "medium",
					ConfigFile: "",
					Command:    "ojs migrate celery <config-file>",
				})
			}
		}
	}

	return results
}

// migrateValidateConfig validates an OJS config file generated by migrate commands.
func migrateValidateConfig(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing OJS config file\n\nUsage: ojs migrate validate-config <ojs-config.json>")
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var cfg ojsMigrateOutput
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid OJS config JSON: %w", err)
	}

	var errors []string
	for i, job := range cfg.Jobs {
		if job.Type == "" {
			errors = append(errors, fmt.Sprintf("job[%d]: missing type", i))
		}
		if job.Queue == "" {
			errors = append(errors, fmt.Sprintf("job[%d]: missing queue", i))
		}
		if job.Options.Retry != nil && job.Options.Retry.MaxAttempts < 0 {
			errors = append(errors, fmt.Sprintf("job[%d]: invalid max_attempts", i))
		}
	}

	type validateResult struct {
		Valid    bool     `json:"valid"`
		File     string   `json:"file"`
		Jobs     int      `json:"jobs"`
		Errors   []string `json:"errors,omitempty"`
	}

	result := validateResult{
		Valid:  len(errors) == 0,
		File:   args[0],
		Jobs:   len(cfg.Jobs),
		Errors: errors,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
