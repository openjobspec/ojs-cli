package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Debug provides interactive debugging commands for OJS jobs.
func Debug(c *client.Client, args []string) error {
	if len(args) == 0 {
		return debugHelp()
	}

	switch args[0] {
	case "inspect":
		return debugInspect(c, args[1:])
	case "trace":
		return debugTrace(c, args[1:])
	case "replay":
		return debugReplay(c, args[1:])
	case "history":
		return debugHistory(c, args[1:])
	case "bottleneck":
		return debugBottleneck(c, args[1:])
	case "queue":
		return debugQueue(c, args[1:])
	case "failures":
		return debugFailures(c, args[1:])
	case "health":
		return debugHealth(c, args[1:])
	default:
		return fmt.Errorf("unknown debug subcommand: %s (try: inspect, trace, replay, history, bottleneck, queue, failures, health)", args[0])
	}
}

func debugHelp() error {
	fmt.Println(`ojs debug — Interactive job debugging toolkit

Subcommands:
  inspect <job-id>       Show detailed job state, args, errors, and metadata
  trace <job-id>         Show distributed trace spans for a job
  replay <job-id>        Re-enqueue a failed job with optional arg modifications
  history <job-id>       Show state transition timeline for a job
  bottleneck             Identify slowest job types and queues
  queue <name>           Show live queue depth, throughput, and latency
  failures [--limit N]   List recent failures with error details
  health                 Show composite system health score

Examples:
  ojs debug inspect 019411a7-...
  ojs debug replay 019411a7-... --queue priority
  ojs debug bottleneck --limit 10
  ojs debug failures --limit 20`)
	return nil
}

// debugInspect shows detailed job information.
func debugInspect(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ojs debug inspect <job-id>")
	}
	jobID := args[0]

	data, _, err := c.Get("/jobs/" + jobID)
	if err != nil {
		return fmt.Errorf("failed to fetch job: %w", err)
	}

	var job map[string]interface{}
	json.Unmarshal(data, &job)

	if output.Format == "json" {
		return output.JSON(job)
	}

	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║                  JOB INSPECTION                      ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()

	printField("Job ID", getString(job, "id"))
	printField("Type", getString(job, "type"))
	printField("Queue", getString(job, "queue"))
	printField("State", colorState(getString(job, "state")))
	printField("Priority", fmt.Sprintf("%v", job["priority"]))
	printField("Attempt", fmt.Sprintf("%v / %v", job["attempt"], job["max_attempts"]))
	fmt.Println()

	if args, ok := job["args"]; ok {
		printField("Args", fmt.Sprintf("%v", args))
	}
	if meta, ok := job["meta"]; ok && meta != nil {
		printField("Metadata", fmt.Sprintf("%v", meta))
	}
	if errMsg := getString(job, "error"); errMsg != "" {
		printField("Last Error", errMsg)
	}
	if errors, ok := job["errors"].([]interface{}); ok && len(errors) > 0 {
		fmt.Println("\n  Error History:")
		for i, e := range errors {
			fmt.Printf("    %d. %v\n", i+1, e)
		}
	}

	fmt.Println()
	printField("Created", getString(job, "created_at"))
	if v := getString(job, "scheduled_at"); v != "" {
		printField("Scheduled", v)
	}
	if v := getString(job, "completed_at"); v != "" {
		printField("Completed", v)
	}

	return nil
}

// debugTrace shows trace information for a job.
func debugTrace(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ojs debug trace <job-id>")
	}
	jobID := args[0]

	data, _, err := c.Get("/admin/jobs/" + jobID + "/trace")
	if err != nil {
		// Fallback to basic job info
		data, _, err = c.Get("/jobs/" + jobID)
		if err != nil {
			return err
		}
		fmt.Println("ℹ  Trace endpoint not available. Showing job state:")
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var trace struct {
		Snapshots []struct {
			Phase    string `json:"phase"`
			Duration string `json:"duration"`
			State    string `json:"state"`
			Error    string `json:"error"`
		} `json:"snapshots"`
	}
	json.Unmarshal(data, &trace)

	fmt.Printf("Trace for job %s\n\n", jobID)
	for i, snap := range trace.Snapshots {
		marker := "  ├─"
		if i == len(trace.Snapshots)-1 {
			marker = "  └─"
		}
		errStr := ""
		if snap.Error != "" {
			errStr = " ⚠ " + snap.Error
		}
		fmt.Printf("%s [%s] %s (%s)%s\n", marker, snap.State, snap.Phase, snap.Duration, errStr)
	}

	return nil
}

// debugReplay re-enqueues a failed job.
func debugReplay(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("debug replay", flag.ExitOnError)
	queue := fs.String("queue", "", "Override queue for replayed job")
	priority := fs.Int("priority", 0, "Override priority")
	fs.Parse(args)

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("usage: ojs debug replay <job-id> [--queue Q] [--priority N]")
	}
	jobID := remaining[0]

	// Fetch original job
	data, _, err := c.Get("/jobs/" + jobID)
	if err != nil {
		return fmt.Errorf("failed to fetch job: %w", err)
	}

	var job map[string]interface{}
	json.Unmarshal(data, &job)

	// Build replay request
	reqBody := map[string]interface{}{
		"type": job["type"],
		"args": job["args"],
	}
	if *queue != "" {
		reqBody["queue"] = *queue
	} else if q, ok := job["queue"]; ok {
		reqBody["queue"] = q
	}
	if *priority > 0 {
		reqBody["priority"] = *priority
	}
	if meta, ok := job["meta"]; ok {
		reqBody["meta"] = meta
	}

	body, _ := json.Marshal(reqBody)
	respData, _, err := c.Post("/jobs", body)
	if err != nil {
		return fmt.Errorf("failed to replay job: %w", err)
	}

	var resp map[string]interface{}
	json.Unmarshal(respData, &resp)

	fmt.Printf("✓ Job replayed successfully\n")
	fmt.Printf("  Original: %s\n", jobID)
	fmt.Printf("  New ID:   %s\n", getString(resp, "id"))
	fmt.Printf("  Queue:    %s\n", getString(resp, "queue"))

	return nil
}

// debugHistory shows state transition timeline.
func debugHistory(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ojs debug history <job-id>")
	}
	jobID := args[0]

	// Try event sourcing endpoint first
	data, _, err := c.Get("/events/" + jobID + "/history")
	if err != nil {
		// Fallback to job info
		data, _, err = c.Get("/jobs/" + jobID)
		if err != nil {
			return err
		}
		fmt.Println("ℹ  Event history not available. Showing current state.")
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		Events []struct {
			Type      string    `json:"type"`
			State     string    `json:"state"`
			Timestamp time.Time `json:"timestamp"`
		} `json:"events"`
	}
	json.Unmarshal(data, &resp)

	fmt.Printf("State history for %s:\n\n", jobID)
	for i, e := range resp.Events {
		marker := "│"
		if i == len(resp.Events)-1 {
			marker = "└"
		}
		fmt.Printf("  %s %s  %s → %s\n", marker, e.Timestamp.Format("15:04:05.000"), e.Type, colorState(e.State))
	}

	return nil
}

// debugBottleneck identifies slowest job types and queues.
func debugBottleneck(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("debug bottleneck", flag.ExitOnError)
	limit := fs.Int("limit", 10, "Number of results")
	fs.Parse(args)

	data, _, err := c.Get(fmt.Sprintf("/admin/stats?detail=true&limit=%d", *limit))
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var stats struct {
		Queues []struct {
			Name         string  `json:"name"`
			Depth        int     `json:"depth"`
			AvgDurationMs float64 `json:"avg_duration_ms"`
			ErrorRate    float64 `json:"error_rate"`
		} `json:"queues"`
	}
	json.Unmarshal(data, &stats)

	fmt.Println("Queue Bottleneck Analysis")
	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("%-20s %8s %12s %10s\n", "Queue", "Depth", "Avg Duration", "Error Rate")
	fmt.Println("─────────────────────────────────────────")

	for _, q := range stats.Queues {
		fmt.Printf("%-20s %8d %10.1fms %9.1f%%\n",
			q.Name, q.Depth, q.AvgDurationMs, q.ErrorRate*100)
	}

	return nil
}

// debugQueue shows live queue information.
func debugQueue(c *client.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ojs debug queue <queue-name>")
	}
	queueName := args[0]

	data, _, err := c.Get("/queues/" + queueName)
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var q map[string]interface{}
	json.Unmarshal(data, &q)

	fmt.Printf("Queue: %s\n", queueName)
	fmt.Println("───────────────────────────")
	printField("Depth", fmt.Sprintf("%v", q["depth"]))
	printField("Running", fmt.Sprintf("%v", q["running"]))
	printField("Paused", fmt.Sprintf("%v", q["paused"]))

	return nil
}

// debugFailures lists recent failures.
func debugFailures(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("debug failures", flag.ExitOnError)
	limit := fs.Int("limit", 20, "Number of failures to show")
	fs.Parse(args)

	data, _, err := c.Get(fmt.Sprintf("/jobs?state=discarded&limit=%d", *limit))
	if err != nil {
		return err
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var resp struct {
		Jobs []struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Error string `json:"error"`
			Queue string `json:"queue"`
		} `json:"jobs"`
	}
	json.Unmarshal(data, &resp)

	fmt.Printf("Recent Failures (%d)\n", len(resp.Jobs))
	fmt.Println("─────────────────────────────────────────────────────")

	for _, j := range resp.Jobs {
		errStr := j.Error
		if len(errStr) > 60 {
			errStr = errStr[:57] + "..."
		}
		fmt.Printf("  %s  %-20s  %s\n", j.ID[:12], j.Type, errStr)
	}

	return nil
}

// debugHealth shows composite system health.
func debugHealth(c *client.Client, _ []string) error {
	data, _, err := c.Get("/admin/observability/health")
	if err != nil {
		// Fallback to basic health check
		data, _, err = c.Get("/health")
		if err != nil {
			return err
		}
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var health struct {
		Score         float64 `json:"score"`
		Status        string  `json:"status"`
		SLOViolations int     `json:"slo_violations"`
		AnomalyCount  int     `json:"anomaly_count"`
	}
	json.Unmarshal(data, &health)

	icon := "✓"
	if health.Status == "degraded" {
		icon = "⚠"
	} else if health.Status == "critical" {
		icon = "✗"
	}

	fmt.Printf("%s System Health: %.0f/100 (%s)\n", icon, health.Score, strings.ToUpper(health.Status))
	if health.SLOViolations > 0 {
		fmt.Printf("  ⚠ %d SLO violation(s)\n", health.SLOViolations)
	}
	if health.AnomalyCount > 0 {
		fmt.Printf("  ⚠ %d anomaly(s) detected\n", health.AnomalyCount)
	}

	return nil
}

// --- helpers ---

func printField(label, value string) {
	fmt.Printf("  %-14s %s\n", label+":", value)
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func colorState(state string) string {
	switch state {
	case "completed":
		return "✓ " + state
	case "active":
		return "⟳ " + state
	case "available", "pending":
		return "○ " + state
	case "retryable":
		return "↻ " + state
	case "discarded", "failed":
		return "✗ " + state
	case "cancelled":
		return "⊘ " + state
	default:
		return state
	}
}
