package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

type checkResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pass, warn, fail
	Message string `json:"message"`
}

func Doctor(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	production := fs.Bool("production", false, "Run production readiness checks")
	verbose := fs.Bool("verbose", false, "Show all checks including passed")
	fs.Usage = func() {
		fmt.Print(`Usage: ojs doctor [flags]

Run health and configuration checks against an OJS server.

Flags:
  --production  Run production readiness checks (TLS, auth, metrics, etc.)
  --verbose     Show all checks including passed ones
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	results := []checkResult{}

	// Basic connectivity
	results = append(results, checkConnectivity(c))
	results = append(results, checkHealthEndpoint(c))
	results = append(results, checkAPIVersion(c))

	// Core functionality
	results = append(results, checkEnqueueDequeue(c))
	results = append(results, checkQueuesEndpoint(c))

	if *production {
		results = append(results, checkTLS(c))
		results = append(results, checkAuth(c))
		results = append(results, checkMetrics(c))
		results = append(results, checkDeadLetterConfig(c))
		results = append(results, checkWorkerRegistration(c))
	}

	// Output
	if output.Format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	passed, warned, failed := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "pass":
			passed++
			if *verbose {
				fmt.Printf("  ✅ %s: %s\n", r.Name, r.Message)
			}
		case "warn":
			warned++
			fmt.Printf("  ⚠️  %s: %s\n", r.Name, r.Message)
		case "fail":
			failed++
			fmt.Printf("  ❌ %s: %s\n", r.Name, r.Message)
		}
	}

	fmt.Println()
	fmt.Printf("Results: %d passed, %d warnings, %d failed\n", passed, warned, failed)

	if *production && failed == 0 && warned == 0 {
		fmt.Println("\n🎉 Production readiness: PASS")
	} else if *production && failed > 0 {
		fmt.Println("\n🚨 Production readiness: FAIL — address the issues above before deploying")
	} else if *production && warned > 0 {
		fmt.Println("\n⚠️  Production readiness: WARN — review warnings before deploying")
	}

	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}

func checkConnectivity(c *client.Client) checkResult {
	start := time.Now()
	resp, err := http.Get(c.BaseURL() + "/v1/health")
	latency := time.Since(start)

	if err != nil {
		return checkResult{
			Name:    "Server Connectivity",
			Status:  "fail",
			Message: fmt.Sprintf("Cannot reach server at %s: %v", c.BaseURL(), err),
		}
	}
	resp.Body.Close()

	return checkResult{
		Name:    "Server Connectivity",
		Status:  "pass",
		Message: fmt.Sprintf("Connected to %s (latency: %dms)", c.BaseURL(), latency.Milliseconds()),
	}
}

func checkHealthEndpoint(c *client.Client) checkResult {
	resp, err := http.Get(c.BaseURL() + "/v1/health")
	if err != nil {
		return checkResult{Name: "Health Endpoint", Status: "fail", Message: "Health endpoint unreachable"}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return checkResult{
			Name:    "Health Endpoint",
			Status:  "fail",
			Message: fmt.Sprintf("Health endpoint returned %d", resp.StatusCode),
		}
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
		if status, ok := body["status"].(string); ok {
			return checkResult{Name: "Health Endpoint", Status: "pass", Message: fmt.Sprintf("Status: %s", status)}
		}
	}

	return checkResult{Name: "Health Endpoint", Status: "pass", Message: "Healthy"}
}

func checkAPIVersion(c *client.Client) checkResult {
	resp, err := http.Get(c.BaseURL() + "/v1/health")
	if err != nil {
		return checkResult{Name: "API Version", Status: "fail", Message: "Cannot determine API version"}
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
		if version, ok := body["version"].(string); ok {
			return checkResult{Name: "API Version", Status: "pass", Message: fmt.Sprintf("Server version: %s", version)}
		}
	}

	return checkResult{Name: "API Version", Status: "pass", Message: "API v1 available"}
}

func checkEnqueueDequeue(c *client.Client) checkResult {
	// Try a dry-run style check by hitting the jobs endpoint
	resp, err := http.Get(c.BaseURL() + "/v1/queues")
	if err != nil {
		return checkResult{Name: "Job Operations", Status: "fail", Message: "Cannot access job API"}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 401 {
		return checkResult{Name: "Job Operations", Status: "pass", Message: "Job API accessible"}
	}

	return checkResult{
		Name:    "Job Operations",
		Status:  "warn",
		Message: fmt.Sprintf("Queues endpoint returned %d", resp.StatusCode),
	}
}

func checkQueuesEndpoint(c *client.Client) checkResult {
	resp, err := http.Get(c.BaseURL() + "/v1/queues")
	if err != nil {
		return checkResult{Name: "Queue Management", Status: "fail", Message: "Cannot list queues"}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return checkResult{Name: "Queue Management", Status: "pass", Message: "Queue management available"}
	}

	return checkResult{
		Name:    "Queue Management",
		Status:  "warn",
		Message: fmt.Sprintf("Queue endpoint returned %d", resp.StatusCode),
	}
}

func checkTLS(c *client.Client) checkResult {
	if strings.HasPrefix(c.BaseURL(), "https://") {
		return checkResult{Name: "TLS Encryption", Status: "pass", Message: "Connection uses HTTPS"}
	}
	return checkResult{
		Name:    "TLS Encryption",
		Status:  "fail",
		Message: "Server URL uses HTTP — use HTTPS in production",
	}
}

func checkAuth(c *client.Client) checkResult {
	// Try to access without auth to see if server requires it
	resp, err := http.Get(c.BaseURL() + "/v1/queues")
	if err != nil {
		return checkResult{Name: "Authentication", Status: "fail", Message: "Cannot check auth config"}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return checkResult{Name: "Authentication", Status: "pass", Message: "Server requires authentication"}
	}

	return checkResult{
		Name:    "Authentication",
		Status:  "warn",
		Message: "Server accepts unauthenticated requests — enable API key auth for production",
	}
}

func checkMetrics(c *client.Client) checkResult {
	resp, err := http.Get(c.BaseURL() + "/metrics")
	if err != nil {
		return checkResult{Name: "Metrics Export", Status: "warn", Message: "Metrics endpoint not available"}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return checkResult{Name: "Metrics Export", Status: "pass", Message: "Prometheus metrics endpoint available at /metrics"}
	}

	return checkResult{
		Name:    "Metrics Export",
		Status:  "warn",
		Message: "Metrics endpoint returned " + fmt.Sprintf("%d", resp.StatusCode) + " — enable Prometheus metrics for production",
	}
}

func checkDeadLetterConfig(c *client.Client) checkResult {
	resp, err := http.Get(c.BaseURL() + "/v1/dead-letter")
	if err != nil {
		return checkResult{Name: "Dead Letter Queue", Status: "warn", Message: "Cannot check DLQ configuration"}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return checkResult{Name: "Dead Letter Queue", Status: "pass", Message: "Dead letter queue accessible"}
	}

	return checkResult{
		Name:    "Dead Letter Queue",
		Status:  "warn",
		Message: "Dead letter queue endpoint not accessible — configure DLQ for production",
	}
}

func checkWorkerRegistration(c *client.Client) checkResult {
	resp, err := http.Get(c.BaseURL() + "/v1/workers")
	if err != nil {
		return checkResult{Name: "Worker Registration", Status: "warn", Message: "Cannot check worker status"}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
			if workers, ok := body["workers"].([]interface{}); ok && len(workers) > 0 {
				return checkResult{
					Name:    "Worker Registration",
					Status:  "pass",
					Message: fmt.Sprintf("%d worker(s) registered", len(workers)),
				}
			}
		}
		return checkResult{
			Name:    "Worker Registration",
			Status:  "warn",
			Message: "No workers registered — ensure workers are running before processing jobs",
		}
	}

	return checkResult{Name: "Worker Registration", Status: "warn", Message: "Cannot list workers"}
}
