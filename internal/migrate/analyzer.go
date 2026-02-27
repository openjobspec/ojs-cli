package migrate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// --- Source Code Analyzer ---
//
// Analyzes source code from Sidekiq/BullMQ/Celery projects to extract
// job type definitions, queue assignments, retry configs, and generates
// equivalent OJS code.

// SourceFramework identifies the original job framework.
type SourceFramework string

const (
	FrameworkSidekiq SourceFramework = "sidekiq"
	FrameworkBullMQ  SourceFramework = "bullmq"
	FrameworkCelery  SourceFramework = "celery"
)

// JobDefinition represents an extracted job type from source code.
type JobDefinition struct {
	Name        string          `json:"name"`
	Queue       string          `json:"queue"`
	Framework   SourceFramework `json:"framework"`
	RetryCount  int             `json:"retry_count,omitempty"`
	Concurrency int             `json:"concurrency,omitempty"`
	CronExpr    string          `json:"cron,omitempty"`
	Priority    int             `json:"priority,omitempty"`
	FilePath    string          `json:"file_path,omitempty"`
	LineNumber  int             `json:"line_number,omitempty"`
	SourceCode  string          `json:"source_code,omitempty"`
}

// CodeAnalysisResult holds the output of source code analysis.
type CodeAnalysisResult struct {
	Framework    SourceFramework `json:"framework"`
	Jobs         []JobDefinition `json:"jobs"`
	Queues       []string        `json:"queues"`
	TotalFiles   int             `json:"total_files"`
	Warnings     []string        `json:"warnings,omitempty"`
}

// GeneratedCode holds OJS-equivalent code for a job definition.
type GeneratedCode struct {
	JobName    string `json:"job_name"`
	OJSType    string `json:"ojs_type"`
	Language   string `json:"language"`
	ClientCode string `json:"client_code"`
	WorkerCode string `json:"worker_code"`
}

// MigrationPlan contains the full migration plan with diffs.
type CodeMigrationPlan struct {
	Framework  SourceFramework    `json:"framework"`
	Analysis   CodeAnalysisResult `json:"analysis"`
	Generated  []GeneratedCode    `json:"generated"`
	Summary    string             `json:"summary"`
}

// AnalyzeSource analyzes source code content and extracts job definitions.
func AnalyzeSource(framework SourceFramework, files map[string]string) *CodeAnalysisResult {
	result := &CodeAnalysisResult{
		Framework:  framework,
		TotalFiles: len(files),
	}

	queues := make(map[string]bool)

	for path, content := range files {
		var jobs []JobDefinition
		switch framework {
		case FrameworkSidekiq:
			jobs = analyzeSidekiqFile(content, path)
		case FrameworkBullMQ:
			jobs = analyzeBullMQFile(content, path)
		case FrameworkCelery:
			jobs = analyzeCeleryFile(content, path)
		}

		for _, job := range jobs {
			result.Jobs = append(result.Jobs, job)
			if job.Queue != "" {
				queues[job.Queue] = true
			}
		}
	}

	for q := range queues {
		result.Queues = append(result.Queues, q)
	}
	sort.Strings(result.Queues)

	if len(result.Jobs) == 0 {
		result.Warnings = append(result.Warnings, "no job definitions found in provided files")
	}

	return result
}

// --- Sidekiq Analyzer (Ruby) ---

var sidekiqWorkerRegex = regexp.MustCompile(`class\s+(\w+)\s*\n(?:[^}]*?)include\s+Sidekiq::Worker`)
var sidekiqJobRegex = regexp.MustCompile(`class\s+(\w+)\s*<\s*(?:ApplicationJob|ActiveJob::Base)`)
var sidekiqQueueRegex = regexp.MustCompile(`sidekiq_options\s+queue:\s*['":](\w+)`)
var sidekiqRetryRegex = regexp.MustCompile(`sidekiq_options\s+.*retry:\s*(\w+)`)

func analyzeSidekiqFile(content, path string) []JobDefinition {
	var jobs []JobDefinition

	// Match Sidekiq::Worker classes
	for _, match := range sidekiqWorkerRegex.FindAllStringSubmatchIndex(content, -1) {
		name := content[match[2]:match[3]]
		job := JobDefinition{
			Name:      name,
			Framework: FrameworkSidekiq,
			FilePath:  path,
			Queue:     "default",
		}

		// Look for queue option
		block := content[match[0]:min(match[1]+200, len(content))]
		if qm := sidekiqQueueRegex.FindStringSubmatch(block); len(qm) > 1 {
			job.Queue = qm[1]
		}

		jobs = append(jobs, job)
	}

	// Match ActiveJob classes
	for _, match := range sidekiqJobRegex.FindAllStringSubmatchIndex(content, -1) {
		name := content[match[2]:match[3]]
		jobs = append(jobs, JobDefinition{
			Name:      name,
			Framework: FrameworkSidekiq,
			FilePath:  path,
			Queue:     "default",
		})
	}

	return jobs
}

// --- BullMQ Analyzer (TypeScript/JavaScript) ---

var bullmqQueueRegex = regexp.MustCompile(`new\s+Queue\s*\(\s*['"]([^'"]+)['"]`)
var bullmqWorkerRegex = regexp.MustCompile(`new\s+Worker\s*\(\s*['"]([^'"]+)['"]`)
var bullmqAddRegex = regexp.MustCompile(`\.add\s*\(\s*['"]([^'"]+)['"]`)

func analyzeBullMQFile(content, path string) []JobDefinition {
	var jobs []JobDefinition
	seen := make(map[string]bool)

	// Match Queue definitions
	for _, match := range bullmqQueueRegex.FindAllStringSubmatch(content, -1) {
		queueName := match[1]
		if !seen[queueName] {
			seen[queueName] = true
		}
	}

	// Match Worker definitions
	for _, match := range bullmqWorkerRegex.FindAllStringSubmatch(content, -1) {
		name := match[1]
		if !seen[name] {
			jobs = append(jobs, JobDefinition{
				Name:      name,
				Framework: FrameworkBullMQ,
				FilePath:  path,
				Queue:     name,
			})
			seen[name] = true
		}
	}

	// Match queue.add() calls for job types
	for _, match := range bullmqAddRegex.FindAllStringSubmatch(content, -1) {
		jobName := match[1]
		if !seen[jobName] {
			jobs = append(jobs, JobDefinition{
				Name:      jobName,
				Framework: FrameworkBullMQ,
				FilePath:  path,
			})
			seen[jobName] = true
		}
	}

	return jobs
}

// --- Celery Analyzer (Python) ---

var celeryTaskRegex = regexp.MustCompile(`@(?:app\.task|shared_task|celery\.task)\s*(?:\([^)]*\))?\s*\ndef\s+(\w+)`)
var celeryQueueRegex = regexp.MustCompile(`queue\s*=\s*['"]([^'"]+)['"]`)
var celeryRetryRegex = regexp.MustCompile(`max_retries\s*=\s*(\d+)`)

func analyzeCeleryFile(content, path string) []JobDefinition {
	var jobs []JobDefinition

	for _, match := range celeryTaskRegex.FindAllStringSubmatchIndex(content, -1) {
		name := content[match[2]:match[3]]
		job := JobDefinition{
			Name:      name,
			Framework: FrameworkCelery,
			FilePath:  path,
			Queue:     "celery",
		}

		// Look for queue and retry in decorator args
		block := content[max(0, match[0]-100):min(match[1]+100, len(content))]
		if qm := celeryQueueRegex.FindStringSubmatch(block); len(qm) > 1 {
			job.Queue = qm[1]
		}

		jobs = append(jobs, job)
	}

	return jobs
}

// --- Code Generator ---

// GenerateOJSCode generates OJS-equivalent code for a job definition.
func GenerateOJSCode(job JobDefinition, targetLang string) GeneratedCode {
	ojsType := toOJSJobType(job.Name, job.Framework)

	gen := GeneratedCode{
		JobName:  job.Name,
		OJSType:  ojsType,
		Language: targetLang,
	}

	switch targetLang {
	case "go":
		gen.ClientCode = generateGoClient(ojsType, job)
		gen.WorkerCode = generateGoWorker(ojsType, job)
	case "typescript":
		gen.ClientCode = generateTSClient(ojsType, job)
		gen.WorkerCode = generateTSWorker(ojsType, job)
	case "python":
		gen.ClientCode = generatePythonClient(ojsType, job)
		gen.WorkerCode = generatePythonWorker(ojsType, job)
	}

	return gen
}

// GenerateMigrationPlan creates a full migration plan.
func GenerateMigrationPlan(analysis *CodeAnalysisResult, targetLang string) *CodeMigrationPlan {
	plan := &CodeMigrationPlan{
		Framework: analysis.Framework,
		Analysis:  *analysis,
	}

	for _, job := range analysis.Jobs {
		plan.Generated = append(plan.Generated, GenerateOJSCode(job, targetLang))
	}

	plan.Summary = fmt.Sprintf("Migration from %s: %d jobs across %d queues → OJS %s code",
		analysis.Framework, len(analysis.Jobs), len(analysis.Queues), targetLang)

	return plan
}

// MarshalJSON produces a JSON representation of the migration plan.
func (p *CodeMigrationPlan) JSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// --- Helpers ---

func toOJSJobType(name string, framework SourceFramework) string {
	// Convert CamelCase to dot-separated lowercase
	s := camelToSnake(name)
	s = strings.ReplaceAll(s, "_", ".")
	return s
}

func camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

func generateGoClient(ojsType string, job JobDefinition) string {
	queue := job.Queue
	if queue == "" {
		queue = "default"
	}
	return fmt.Sprintf(`// Enqueue %s job
client.Enqueue(ctx, "%s", ojs.Args{"key": "value"},
    ojs.WithQueue("%s"),
)`, job.Name, ojsType, queue)
}

func generateGoWorker(ojsType string, job JobDefinition) string {
	return fmt.Sprintf(`// Handle %s jobs
worker.Register("%s", func(ctx ojs.JobContext) error {
    // TODO: migrate handler logic from %s
    return nil
})`, job.Name, ojsType, job.Name)
}

func generateTSClient(ojsType string, job JobDefinition) string {
	return fmt.Sprintf(`// Enqueue %s job
await client.enqueue('%s', { key: 'value' }, { queue: '%s' });`,
		job.Name, ojsType, job.Queue)
}

func generateTSWorker(ojsType string, job JobDefinition) string {
	return fmt.Sprintf(`// Handle %s jobs
worker.register('%s', async (ctx) => {
  // TODO: migrate handler logic from %s
});`, job.Name, ojsType, job.Name)
}

func generatePythonClient(ojsType string, job JobDefinition) string {
	return fmt.Sprintf(`# Enqueue %s job
await client.enqueue('%s', {'key': 'value'}, queue='%s')`,
		job.Name, ojsType, job.Queue)
}

func generatePythonWorker(ojsType string, job JobDefinition) string {
	return fmt.Sprintf(`# Handle %s jobs
@worker.register('%s')
async def handle_%s(ctx):
    # TODO: migrate handler logic from %s
    pass`, job.Name, ojsType, strings.ReplaceAll(ojsType, ".", "_"), job.Name)
}
