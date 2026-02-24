// Package importbridge converts Temporal workflow definitions and Airflow DAGs
// into OJS workflow YAML manifests.
//
// It parses Go-style Temporal workflow registrations and Python Airflow DAG
// definitions, mapping their task/activity structure to OJS chain/group/batch
// workflow primitives.
package importbridge

import (
	"fmt"
	"regexp"
	"strings"
)

// WorkflowDef represents a parsed workflow ready for OJS conversion.
type WorkflowDef struct {
	Name        string    `json:"name"`
	Source      string    `json:"source"` // "temporal" or "airflow"
	Steps       []StepDef `json:"steps"`
	Parallel    bool      `json:"parallel,omitempty"`
	Description string    `json:"description,omitempty"`
}

// StepDef represents a single step in the workflow.
type StepDef struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"` // "task", "chain", "group"
	JobType     string    `json:"job_type"`
	Queue       string    `json:"queue"`
	Children    []StepDef `json:"children,omitempty"`
	DependsOn   []string  `json:"depends_on,omitempty"`
	Description string    `json:"description,omitempty"`
}

// ConvertResult holds the OJS YAML output and any warnings.
type ConvertResult struct {
	YAML     string   `json:"yaml"`
	Warnings []string `json:"warnings,omitempty"`
}

// --- Temporal Parsing ---

var (
	temporalWorkflowRe = regexp.MustCompile(`(?m)func\s+(\w+)\(ctx\s+workflow\.Context`)
	temporalActivityRe = regexp.MustCompile(`(?m)workflow\.ExecuteActivity\(\w+,\s*(\w+)`)
	temporalChildRe    = regexp.MustCompile(`(?m)workflow\.ExecuteChildWorkflow\(ctx,\s*(\w+)`)
	temporalGoRe       = regexp.MustCompile(`(?m)workflow\.Go\(ctx,\s*func`)
	temporalSelectorRe = regexp.MustCompile(`(?m)selector\.AddFuture`)
)

// ParseTemporalGo parses Go Temporal workflow source code.
func ParseTemporalGo(source string) ([]WorkflowDef, []string) {
	var workflows []WorkflowDef
	var warnings []string

	// Find workflow functions
	wfMatches := temporalWorkflowRe.FindAllStringSubmatch(source, -1)
	for _, match := range wfMatches {
		wfName := match[1]
		wf := WorkflowDef{
			Name:   wfName,
			Source: "temporal",
		}

		// Extract activities within this workflow
		activities := temporalActivityRe.FindAllStringSubmatch(source, -1)
		for _, act := range activities {
			wf.Steps = append(wf.Steps, StepDef{
				Name:    act[1],
				Type:    "task",
				JobType: toOJSJobType(act[1]),
				Queue:   "default",
			})
		}

		// Check for child workflows
		children := temporalChildRe.FindAllStringSubmatch(source, -1)
		for _, child := range children {
			wf.Steps = append(wf.Steps, StepDef{
				Name:    child[1],
				Type:    "chain",
				JobType: toOJSJobType(child[1]),
				Queue:   "default",
			})
			warnings = append(warnings, fmt.Sprintf("child workflow '%s' — verify nesting structure manually", child[1]))
		}

		// Check for parallel patterns
		if temporalGoRe.MatchString(source) || temporalSelectorRe.MatchString(source) {
			wf.Parallel = true
			warnings = append(warnings, fmt.Sprintf("workflow '%s' uses goroutines/selectors — parallel steps may need manual review", wfName))
		}

		if len(wf.Steps) == 0 {
			warnings = append(warnings, fmt.Sprintf("workflow '%s' has no detected activities", wfName))
		}

		workflows = append(workflows, wf)
	}

	return workflows, warnings
}

// --- Airflow Parsing ---

var (
	airflowDagRe      = regexp.MustCompile(`(?m)DAG\(\s*['"]([^'"]+)['"]`)
	airflowTaskRe     = regexp.MustCompile(`(?m)(\w+)\s*=\s*(?:PythonOperator|BashOperator|KubernetesPodOperator|SimpleHttpOperator|EmailOperator)\(\s*\n?\s*task_id\s*=\s*['"]([^'"]+)['"]`)
	airflowDepRe      = regexp.MustCompile(`(?m)(\w+)\s*>>\s*(\w+)`)
	airflowDepListRe  = regexp.MustCompile(`(?m)\[([^\]]+)\]\s*>>\s*(\w+)`)
)

// ParseAirflowDAG parses a Python Airflow DAG file.
func ParseAirflowDAG(source string) ([]WorkflowDef, []string) {
	var workflows []WorkflowDef
	var warnings []string

	// Find DAG name
	dagMatches := airflowDagRe.FindAllStringSubmatch(source, -1)
	for _, match := range dagMatches {
		dagName := match[1]
		wf := WorkflowDef{
			Name:   dagName,
			Source: "airflow",
		}

		// Find tasks
		taskMap := make(map[string]StepDef)
		taskMatches := airflowTaskRe.FindAllStringSubmatch(source, -1)
		for _, tm := range taskMatches {
			varName := tm[1]
			taskID := tm[2]
			step := StepDef{
				Name:    varName,
				Type:    "task",
				JobType: toOJSJobType(taskID),
				Queue:   "default",
			}
			taskMap[varName] = step
			wf.Steps = append(wf.Steps, step)
		}

		// Find dependencies (task_a >> task_b)
		depMatches := airflowDepRe.FindAllStringSubmatch(source, -1)
		for _, dm := range depMatches {
			upstream := dm[1]
			downstream := dm[2]
			if step, ok := taskMap[downstream]; ok {
				step.DependsOn = append(step.DependsOn, upstream)
				taskMap[downstream] = step
			}
		}

		if len(wf.Steps) == 0 {
			warnings = append(warnings, fmt.Sprintf("DAG '%s' has no detected tasks", dagName))
		}

		workflows = append(workflows, wf)
	}

	return workflows, warnings
}

// --- OJS YAML Generation ---

// ToOJSYAML converts parsed workflows to OJS YAML manifest format.
func ToOJSYAML(workflows []WorkflowDef) string {
	var b strings.Builder
	b.WriteString("# Generated by ojs migrate — review and adjust before use\n")
	b.WriteString("version: \"1.0\"\n")
	b.WriteString("package: migrated\n")
	b.WriteString("job_types:\n")

	seen := make(map[string]bool)
	for _, wf := range workflows {
		for _, step := range wf.Steps {
			if seen[step.JobType] {
				continue
			}
			seen[step.JobType] = true

			b.WriteString(fmt.Sprintf("  - type: %s\n", step.JobType))
			if step.Description != "" {
				b.WriteString(fmt.Sprintf("    description: %s\n", step.Description))
			} else {
				b.WriteString(fmt.Sprintf("    description: Migrated from %s (%s)\n", wf.Source, step.Name))
			}
			b.WriteString(fmt.Sprintf("    queue: %s\n", step.Queue))
			b.WriteString("    args:\n")
			b.WriteString("      - name: input\n")
			b.WriteString("        type: object\n")
			b.WriteString("        required: true\n")
			b.WriteString("        description: \"TODO: define typed args\"\n")
			b.WriteString("    retry:\n")
			b.WriteString("      max_attempts: 3\n")
			b.WriteString("      backoff: exponential\n")
			b.WriteString("      initial_ms: 1000\n")
		}
	}

	// Generate workflow definitions
	for _, wf := range workflows {
		if len(wf.Steps) < 2 {
			continue
		}
		b.WriteString(fmt.Sprintf("\n# Workflow: %s (from %s)\n", wf.Name, wf.Source))
		b.WriteString(fmt.Sprintf("# ojs workflow create --name %s \\\n", toOJSJobType(wf.Name)))

		if wf.Parallel {
			b.WriteString("#   --type group \\\n")
		} else {
			b.WriteString("#   --type chain \\\n")
		}

		for _, step := range wf.Steps {
			b.WriteString(fmt.Sprintf("#   --step %s \\\n", step.JobType))
		}
	}

	return b.String()
}

// toOJSJobType converts a function/task name to an OJS dotted job type.
func toOJSJobType(name string) string {
	// Convert CamelCase to dot.separated
	name = camelToSnake(name)
	name = strings.ReplaceAll(name, "_", ".")
	name = strings.ReplaceAll(name, "-", ".")
	// Collapse consecutive dots
	for strings.Contains(name, "..") {
		name = strings.ReplaceAll(name, "..", ".")
	}
	return strings.Trim(name, ".")
}

func camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('.')
			}
			result.WriteRune(r + 32) // lowercase
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
