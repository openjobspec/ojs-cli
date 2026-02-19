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

// Migrate implements the migration wizard with subcommands: analyze, export, import, validate.
func Migrate(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing subcommand\n\nUsage:\n  ojs migrate analyze <source> --redis <url>\n  ojs migrate export <source> --redis <url> --output <file>\n  ojs migrate import --file <file> [--dry-run]\n  ojs migrate validate --file <file>\n\nSupported sources: sidekiq, bullmq, celery, faktory")
	}

	switch args[0] {
	case "analyze":
		return migrateAnalyze(args[1:])
	case "export":
		return migrateExport(args[1:])
	case "import":
		return migrateImport(c, args[1:])
	case "validate":
		return migrateValidate(args[1:])
	default:
		return fmt.Errorf("unknown migrate subcommand: %s\n\nSubcommands: analyze, export, import, validate", args[0])
	}
}

func migrateAnalyze(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing source\n\nSupported sources: sidekiq, bullmq, celery, faktory")
	}

	sourceName := args[0]
	fs := flag.NewFlagSet("migrate analyze", flag.ContinueOnError)
	redisURL := fs.String("redis", "redis://localhost:6379", "Redis connection URL")
	if err := fs.Parse(args[1:]); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	src, err := newSource(sourceName, *redisURL)
	if err != nil {
		return err
	}
	defer src.Close()

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
		return fmt.Errorf("missing source\n\nSupported sources: sidekiq, bullmq, celery, faktory")
	}

	sourceName := args[0]
	fs := flag.NewFlagSet("migrate export", flag.ContinueOnError)
	redisURL := fs.String("redis", "redis://localhost:6379", "Redis connection URL")
	outputFile := fs.String("output", "jobs.ndjson", "Output NDJSON file")
	if err := fs.Parse(args[1:]); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	src, err := newSource(sourceName, *redisURL)
	if err != nil {
		return err
	}
	defer src.Close()

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
	fs := flag.NewFlagSet("migrate import", flag.ContinueOnError)
	file := fs.String("file", "", "NDJSON file to import (required)")
	dryRun := fs.Bool("dry-run", false, "Validate and count jobs without actually importing")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	if *file == "" {
		return fmt.Errorf("--file is required\n\nUsage: ojs migrate import --file <file> [--dry-run]")
	}

	if *dryRun {
		vr, err := migrate.ValidateFile(*file)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		if output.Format == "json" {
			return output.JSON(vr)
		}

		output.Success("Dry run: %d valid, %d invalid out of %d total jobs",
			vr.Valid, vr.Invalid, vr.Total)
		if vr.Invalid > 0 {
			fmt.Fprintf(os.Stderr, "\nFirst errors:\n")
			limit := vr.Invalid
			if limit > 5 {
				limit = 5
			}
			for i := 0; i < limit; i++ {
				fmt.Fprintf(os.Stderr, "  Line %d: %s\n", vr.Errors[i].Line, vr.Errors[i].Message)
			}
		}
		return nil
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
	case "faktory":
		return migrate.NewFaktorySource(redisURL, "")
	default:
		return nil, fmt.Errorf("unsupported source: %s\n\nSupported sources: sidekiq, bullmq, celery", name)
	}
}

func migrateValidate(args []string) error {
	fs := flag.NewFlagSet("migrate validate", flag.ContinueOnError)
	file := fs.String("file", "", "NDJSON file to validate (required)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	if *file == "" {
		return fmt.Errorf("--file is required\n\nUsage: ojs migrate validate --file <file>")
	}

	result, err := migrate.ValidateFile(*file)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if output.Format == "json" {
		return output.JSON(result)
	}

	if result.Invalid == 0 {
		output.Success("All %d jobs are valid", result.Total)
	} else {
		fmt.Printf("Validation: %d valid, %d invalid out of %d total\n\n",
			result.Valid, result.Invalid, result.Total)

		headers := []string{"LINE", "ERROR"}
		var rows [][]string
		limit := len(result.Errors)
		if limit > 20 {
			limit = 20
		}
		for i := 0; i < limit; i++ {
			rows = append(rows, []string{
				fmt.Sprintf("%d", result.Errors[i].Line),
				result.Errors[i].Message,
			})
		}
		output.Table(headers, rows)

		if len(result.Errors) > 20 {
			fmt.Printf("\n... and %d more errors\n", len(result.Errors)-20)
		}
	}

	return nil
}
