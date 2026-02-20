package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// MigrateGenerate generates OJS-compatible job definitions from source system analysis
func MigrateGenerate(args []string) error {
	fs := flag.NewFlagSet("migrate generate", flag.ExitOnError)
	source := fs.String("source", "", "Source system (sidekiq, bullmq, celery, faktory, river)")
	outputDir := fs.String("output", "./ojs-migration", "Output directory for generated files")
	format := fs.String("format", "json", "Output format (json, yaml)")
	fs.Usage = func() {
		fmt.Print(`Usage: ojs migrate generate [flags]

Generate OJS-compatible job definitions and migration plan from a source system.

This creates:
  - Job type definitions mapped from source system
  - Queue configuration recommendations
  - Retry policy mappings
  - A step-by-step migration plan

Flags:
  --source <system>  Source system: sidekiq, bullmq, celery, faktory, river
  --output <dir>     Output directory (default: ./ojs-migration)
  --format <fmt>     Output format: json, yaml (default: json)

Examples:
  ojs migrate generate --source sidekiq
  ojs migrate generate --source bullmq --output ./migration-plan
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *source == "" {
		fs.Usage()
		return fmt.Errorf("--source is required")
	}

	templates := getMigrationTemplates(*source)
	if templates == nil {
		return fmt.Errorf("unsupported source system: %s (supported: sidekiq, bullmq, celery, faktory, river)", *source)
	}

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Write migration plan
	plan := generateMigrationPlan(*source, templates)
	planPath := filepath.Join(*outputDir, "migration-plan."+*format)
	if err := writeJSON(planPath, plan); err != nil {
		return fmt.Errorf("writing migration plan: %w", err)
	}
	fmt.Printf("✅ Migration plan: %s\n", planPath)

	// Write job type mappings
	mappingPath := filepath.Join(*outputDir, "job-mappings."+*format)
	if err := writeJSON(mappingPath, templates.JobMappings); err != nil {
		return fmt.Errorf("writing job mappings: %w", err)
	}
	fmt.Printf("✅ Job mappings:   %s\n", mappingPath)

	// Write queue config
	queuePath := filepath.Join(*outputDir, "queue-config."+*format)
	if err := writeJSON(queuePath, templates.QueueConfig); err != nil {
		return fmt.Errorf("writing queue config: %w", err)
	}
	fmt.Printf("✅ Queue config:   %s\n", queuePath)

	// Write retry policy mapping
	retryPath := filepath.Join(*outputDir, "retry-policies."+*format)
	if err := writeJSON(retryPath, templates.RetryPolicies); err != nil {
		return fmt.Errorf("writing retry policies: %w", err)
	}
	fmt.Printf("✅ Retry policies: %s\n", retryPath)

	fmt.Printf("\n📋 Migration plan generated for %s → OJS\n", *source)
	fmt.Printf("   Review the files in %s and customize as needed.\n", *outputDir)
	fmt.Printf("   Then run: ojs migrate import --file %s\n", mappingPath)

	return nil
}

type migrationTemplates struct {
	Source        string                `json:"source"`
	JobMappings   []jobMapping          `json:"job_mappings"`
	QueueConfig   []queueConfig         `json:"queue_config"`
	RetryPolicies []retryPolicyMapping  `json:"retry_policies"`
}

type jobMapping struct {
	SourceType  string            `json:"source_type"`
	OJSType     string            `json:"ojs_type"`
	SourceQueue string            `json:"source_queue"`
	OJSQueue    string            `json:"ojs_queue"`
	Notes       string            `json:"notes,omitempty"`
	ArgsMapping map[string]string `json:"args_mapping,omitempty"`
}

type queueConfig struct {
	Name        string `json:"name"`
	Priority    int    `json:"priority"`
	Concurrency int    `json:"concurrency"`
	Notes       string `json:"notes,omitempty"`
}

type retryPolicyMapping struct {
	SourcePolicy string `json:"source_policy"`
	OJSPolicy    string `json:"ojs_policy"`
	MaxRetries   int    `json:"max_retries"`
	Backoff      string `json:"backoff"`
	Notes        string `json:"notes,omitempty"`
}

type migrationPlan struct {
	Source      string          `json:"source"`
	Target      string          `json:"target"`
	GeneratedAt string          `json:"generated_at"`
	Steps       []migrationStep `json:"steps"`
}

type migrationStep struct {
	Order       int    `json:"order"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Duration    string `json:"estimated_duration"`
}

func getMigrationTemplates(source string) *migrationTemplates {
	switch source {
	case "sidekiq":
		return &migrationTemplates{
			Source: "sidekiq",
			JobMappings: []jobMapping{
				{SourceType: "HardWorker", OJSType: "hard_worker", SourceQueue: "default", OJSQueue: "default", Notes: "Map Sidekiq worker class to OJS job type (snake_case)"},
				{SourceType: "EmailWorker", OJSType: "email.send", SourceQueue: "mailers", OJSQueue: "email", Notes: "Namespace with dot notation for OJS"},
				{SourceType: "ReportWorker", OJSType: "report.generate", SourceQueue: "low", OJSQueue: "reports", Notes: "Consider renaming queue for clarity"},
			},
			QueueConfig: []queueConfig{
				{Name: "default", Priority: 5, Concurrency: 10, Notes: "Maps from Sidekiq 'default' queue"},
				{Name: "email", Priority: 7, Concurrency: 5, Notes: "Maps from Sidekiq 'mailers' queue — higher priority"},
				{Name: "reports", Priority: 3, Concurrency: 3, Notes: "Maps from Sidekiq 'low' queue"},
			},
			RetryPolicies: []retryPolicyMapping{
				{SourcePolicy: "Sidekiq default (25 retries, exponential)", OJSPolicy: "exponential", MaxRetries: 25, Backoff: "exponential", Notes: "Sidekiq formula: (retry_count^4) + 15 + (rand(30) * (retry_count+1))"},
				{SourcePolicy: "Custom retry", OJSPolicy: "linear", MaxRetries: 3, Backoff: "linear", Notes: "For jobs with sidekiq_retry_in override"},
			},
		}
	case "bullmq":
		return &migrationTemplates{
			Source: "bullmq",
			JobMappings: []jobMapping{
				{SourceType: "emailQueue.add('send')", OJSType: "email.send", SourceQueue: "emailQueue", OJSQueue: "email", Notes: "BullMQ queue+name → OJS type"},
				{SourceType: "reportQueue.add('generate')", OJSType: "report.generate", SourceQueue: "reportQueue", OJSQueue: "reports", Notes: "Map BullMQ named jobs"},
			},
			QueueConfig: []queueConfig{
				{Name: "default", Priority: 5, Concurrency: 10, Notes: "Default queue"},
				{Name: "email", Priority: 7, Concurrency: 5, Notes: "Maps from BullMQ 'emailQueue'"},
			},
			RetryPolicies: []retryPolicyMapping{
				{SourcePolicy: "BullMQ default (exponential)", OJSPolicy: "exponential", MaxRetries: 3, Backoff: "exponential", Notes: "BullMQ default: 3 attempts, exponential backoff"},
				{SourcePolicy: "BullMQ custom backoff", OJSPolicy: "custom", MaxRetries: 5, Backoff: "custom", Notes: "Map custom backoff strategies"},
			},
		}
	case "celery":
		return &migrationTemplates{
			Source: "celery",
			JobMappings: []jobMapping{
				{SourceType: "myapp.tasks.send_email", OJSType: "email.send", SourceQueue: "celery", OJSQueue: "default", Notes: "Map Celery task path to OJS dot-notation type"},
				{SourceType: "myapp.tasks.generate_report", OJSType: "report.generate", SourceQueue: "celery", OJSQueue: "reports", Notes: "Consider splitting Celery default queue"},
			},
			QueueConfig: []queueConfig{
				{Name: "default", Priority: 5, Concurrency: 10, Notes: "Maps from Celery 'celery' default queue"},
				{Name: "reports", Priority: 3, Concurrency: 3, Notes: "For long-running report tasks"},
			},
			RetryPolicies: []retryPolicyMapping{
				{SourcePolicy: "Celery default (3 retries, exponential)", OJSPolicy: "exponential", MaxRetries: 3, Backoff: "exponential", Notes: "Celery default retry behavior"},
				{SourcePolicy: "autoretry_for exceptions", OJSPolicy: "exponential", MaxRetries: 5, Backoff: "exponential", Notes: "Map Celery autoretry_for to OJS retry config"},
			},
		}
	case "faktory":
		return &migrationTemplates{
			Source: "faktory",
			JobMappings: []jobMapping{
				{SourceType: "SendEmail", OJSType: "email.send", SourceQueue: "default", OJSQueue: "default", Notes: "Map Faktory job type to OJS dot notation"},
			},
			QueueConfig: []queueConfig{
				{Name: "default", Priority: 5, Concurrency: 10, Notes: "Maps from Faktory 'default' queue"},
			},
			RetryPolicies: []retryPolicyMapping{
				{SourcePolicy: "Faktory default (25 retries)", OJSPolicy: "exponential", MaxRetries: 25, Backoff: "exponential", Notes: "Faktory uses same retry semantics as Sidekiq"},
			},
		}
	case "river":
		return &migrationTemplates{
			Source: "river",
			JobMappings: []jobMapping{
				{SourceType: "SortArgs{}", OJSType: "sort.execute", SourceQueue: "default", OJSQueue: "default", Notes: "Map River Go struct to OJS type"},
			},
			QueueConfig: []queueConfig{
				{Name: "default", Priority: 5, Concurrency: 10, Notes: "Maps from River default queue"},
			},
			RetryPolicies: []retryPolicyMapping{
				{SourcePolicy: "River default (exponential)", OJSPolicy: "exponential", MaxRetries: 25, Backoff: "exponential", Notes: "River default retry behavior"},
			},
		}
	}
	return nil
}

func generateMigrationPlan(source string, templates *migrationTemplates) migrationPlan {
	return migrationPlan{
		Source:      source,
		Target:      "ojs",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Steps: []migrationStep{
			{Order: 1, Title: "Set up OJS backend", Description: "Deploy an OJS backend (Redis or PostgreSQL recommended for migration from " + source + ")", Command: "docker compose -f docker-compose.quickstart.yml up -d", Duration: "30 minutes"},
			{Order: 2, Title: "Install OJS SDK", Description: "Add the OJS SDK to your application", Duration: "15 minutes"},
			{Order: 3, Title: "Map job types", Description: "Review job-mappings.json and adjust OJS type names to match your domain", Duration: "1-2 hours"},
			{Order: 4, Title: "Configure queues", Description: "Create OJS queues based on queue-config.json", Duration: "30 minutes"},
			{Order: 5, Title: "Implement workers", Description: "Create OJS worker handlers for each job type, reusing existing business logic", Duration: "1-3 days"},
			{Order: 6, Title: "Dual-write phase", Description: "Enqueue jobs to both " + source + " and OJS simultaneously. Process only from " + source + ".", Command: "# Enable dual-write in your application code", Duration: "1 day"},
			{Order: 7, Title: "Shadow processing", Description: "Start OJS workers alongside " + source + " workers. Compare results without affecting production.", Duration: "1-2 weeks"},
			{Order: 8, Title: "Cutover", Description: "Switch production processing to OJS. Keep " + source + " as fallback.", Duration: "1 day"},
			{Order: 9, Title: "Decommission", Description: "Remove " + source + " dependencies after confirming OJS stability.", Duration: "1 week"},
			{Order: 10, Title: "Validate", Description: "Run ojs doctor --production to verify the deployment", Command: "ojs doctor --production", Duration: "30 minutes"},
		},
	}
}

func writeJSON(path string, v interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
