package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/migrate"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Migrate implements the migration wizard with subcommands: analyze, export, import.
func Migrate(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing subcommand\n\nUsage:\n  ojs migrate analyze <source> --redis <url>\n  ojs migrate export <source> --redis <url> --output <file>\n  ojs migrate import --file <file>")
	}

	switch args[0] {
	case "analyze":
		return migrateAnalyze(args[1:])
	case "export":
		return migrateExport(args[1:])
	case "import":
		return migrateImport(c, args[1:])
	default:
		return fmt.Errorf("unknown migrate subcommand: %s\n\nSubcommands: analyze, export, import", args[0])
	}
}

func migrateAnalyze(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing source\n\nSupported sources: sidekiq, bullmq, celery")
	}

	sourceName := args[0]
	fs := flag.NewFlagSet("migrate analyze", flag.ExitOnError)
	redisURL := fs.String("redis", "redis://localhost:6379", "Redis connection URL")
	fs.Parse(args[1:])

	src, err := newSource(sourceName, *redisURL)
	if err != nil {
		return err
	}

	result, err := src.Analyze()
	if err != nil {
		return fmt.Errorf("analyze failed: %w", err)
	}

	if output.Format == "json" {
		return output.JSON(result)
	}

	fmt.Printf("Migration Analysis: %s\n", result.Source)
	fmt.Printf("Connection: %s\n", result.Connection)
	fmt.Printf("Total Jobs: %d\n\n", result.TotalJobs)

	headers := []string{"QUEUE", "PENDING", "JOB TYPES"}
	var rows [][]string
	for _, q := range result.Queues {
		types := ""
		for t, count := range q.JobTypes {
			if types != "" {
				types += ", "
			}
			types += fmt.Sprintf("%s(%d)", t, count)
		}
		rows = append(rows, []string{q.Name, fmt.Sprintf("%d", q.PendingJobs), types})
	}
	output.Table(headers, rows)

	fmt.Printf("\n%s\n", result.Summary)
	return nil
}

func migrateExport(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing source\n\nSupported sources: sidekiq, bullmq, celery")
	}

	sourceName := args[0]
	fs := flag.NewFlagSet("migrate export", flag.ExitOnError)
	redisURL := fs.String("redis", "redis://localhost:6379", "Redis connection URL")
	outputFile := fs.String("output", "jobs.ndjson", "Output NDJSON file")
	fs.Parse(args[1:])

	src, err := newSource(sourceName, *redisURL)
	if err != nil {
		return err
	}

	jobs, err := src.Export()
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	f, err := os.Create(*outputFile)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, job := range jobs {
		if err := enc.Encode(job); err != nil {
			return fmt.Errorf("write job: %w", err)
		}
	}

	output.Success("Exported %d jobs to %s", len(jobs), *outputFile)
	return nil
}

func migrateImport(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("migrate import", flag.ExitOnError)
	file := fs.String("file", "", "NDJSON file to import (required)")
	fs.Parse(args)

	if *file == "" {
		return fmt.Errorf("--file is required\n\nUsage: ojs migrate import --file <file>")
	}

	result, err := migrate.ImportFile(c, *file, func(imported, total int) {
		fmt.Fprintf(os.Stderr, "\r  Imported %d/%d jobs...", imported, total)
	})
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	fmt.Fprintln(os.Stderr)

	if output.Format == "json" {
		return output.JSON(result)
	}

	output.Success("Import complete: %d succeeded, %d failed (%d batches)",
		result.Success, result.Failed, result.Batches)
	return nil
}

func newSource(name, redisURL string) (migrate.Source, error) {
	switch name {
	case "sidekiq":
		return migrate.NewSidekiqSource(redisURL)
	case "bullmq":
		return migrate.NewBullMQSource(redisURL)
	case "celery":
		return migrate.NewCelerySource(redisURL)
	default:
		return nil, fmt.Errorf("unsupported source: %s\n\nSupported sources: sidekiq, bullmq, celery", name)
	}
}
