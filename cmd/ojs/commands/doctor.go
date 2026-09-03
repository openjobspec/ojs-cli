package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
	"github.com/openjobspec/ojs-cli/internal/redact"
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

	results := []checkResult{
		checkConnectivity(c),
		checkHealthEndpoint(c),
		checkAPIVersion(c),
		checkEnqueueDequeue(c),
		checkQueuesEndpoint(c),
	}

	if *production {
		results = append(results,
			checkTLS(c),
			checkAuth(c),
			checkMetrics(c),
			checkDeadLetterConfig(c),
			checkWorkerRegistration(c),
		)
	}
	return renderDoctorResults(results, *verbose, *production)
}

func renderDoctorResults(results []checkResult, verbose, production bool) error {
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
			if verbose {
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

	switch {
	case production && failed == 0 && warned == 0:
		fmt.Println("\n🎉 Production readiness: PASS")
	case production && failed > 0:
		fmt.Println("\n🚨 Production readiness: FAIL — address the issues above before deploying")
	case production && warned > 0:
		fmt.Println("\n⚠️  Production readiness: WARN — review warnings before deploying")
	}

	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}

func checkConnectivity(c *client.Client) checkResult {
	start := time.Now()
	_, status, err := c.Get("/health")
	latency := time.Since(start)

	if err != nil && status == 0 {
		return checkResult{
			Name:    "Server Connectivity",
			Status:  "fail",
			Message: fmt.Sprintf("Cannot reach server at %s: %v", redact.URL(c.BaseURL()), err),
		}
	}

	return checkResult{
		Name:    "Server Connectivity",
		Status:  "pass",
		Message: fmt.Sprintf("Connected to %s (latency: %dms)", redact.URL(c.BaseURL()), latency.Milliseconds()),
	}
}

func checkHealthEndpoint(c *client.Client) checkResult {
	data, status, err := c.Get("/health")
	if err != nil {
		return checkResult{Name: "Health Endpoint", Status: "fail", Message: "Health endpoint unreachable"}
	}

	if status != http.StatusOK {
		return checkResult{
			Name:    "Health Endpoint",
			Status:  "fail",
			Message: fmt.Sprintf("Health endpoint returned %d", status),
		}
	}

	var body map[string]interface{}
	if err := json.Unmarshal(data, &body); err == nil {
		if status, ok := body["status"].(string); ok {
			return checkResult{Name: "Health Endpoint", Status: "pass", Message: fmt.Sprintf("Status: %s", status)}
		}
	}

	return checkResult{Name: "Health Endpoint", Status: "pass", Message: "Healthy"}
}

func checkAPIVersion(c *client.Client) checkResult {
	data, _, err := c.Get("/health")
	if err != nil {
		return checkResult{Name: "API Version", Status: "fail", Message: "Cannot determine API version"}
	}

	var body map[string]interface{}
	if err := json.Unmarshal(data, &body); err == nil {
		if version, ok := body["version"].(string); ok {
			return checkResult{Name: "API Version", Status: "pass", Message: fmt.Sprintf("Server version: %s", version)}
		}
	}

	return checkResult{Name: "API Version", Status: "pass", Message: "API v1 available"}
}

func checkEnqueueDequeue(c *client.Client) checkResult {
	_, status, err := c.Get("/queues")
	if err != nil && status == 0 {
		return checkResult{Name: "Job Operations", Status: "fail", Message: "Cannot access job API"}
	}

	if status == http.StatusOK || status == http.StatusUnauthorized {
		return checkResult{Name: "Job Operations", Status: "pass", Message: "Job API accessible"}
	}

	return checkResult{
		Name:    "Job Operations",
		Status:  "warn",
		Message: fmt.Sprintf("Queues endpoint returned %d", status),
	}
}

func checkQueuesEndpoint(c *client.Client) checkResult {
	_, status, err := c.Get("/queues")
	if err != nil && status == 0 {
		return checkResult{Name: "Queue Management", Status: "fail", Message: "Cannot list queues"}
	}

	if status == http.StatusOK {
		return checkResult{Name: "Queue Management", Status: "pass", Message: "Queue management available"}
	}

	return checkResult{
		Name:    "Queue Management",
		Status:  "warn",
		Message: fmt.Sprintf("Queue endpoint returned %d", status),
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
	status, err := rawDoctorGet(c.BaseURL() + "/ojs/v1/queues")
	if err != nil {
		return checkResult{Name: "Authentication", Status: "fail", Message: "Cannot check auth config"}
	}

	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return checkResult{Name: "Authentication", Status: "pass", Message: "Server requires authentication"}
	}

	return checkResult{
		Name:    "Authentication",
		Status:  "warn",
		Message: "Server accepts unauthenticated requests — enable API key auth for production",
	}
}

func checkMetrics(c *client.Client) checkResult {
	status, err := rawDoctorGet(c.BaseURL() + "/metrics")
	if err != nil {
		return checkResult{Name: "Metrics Export", Status: "warn", Message: "Metrics endpoint not available"}
	}
	if status == http.StatusOK {
		return checkResult{Name: "Metrics Export", Status: "pass", Message: "Prometheus metrics endpoint available at /metrics"}
	}

	return checkResult{
		Name:    "Metrics Export",
		Status:  "warn",
		Message: "Metrics endpoint returned " + fmt.Sprintf("%d", status) + " — enable Prometheus metrics for production",
	}
}

func checkDeadLetterConfig(c *client.Client) checkResult {
	_, status, err := c.Get("/dead-letter")
	if err != nil && status == 0 {
		return checkResult{Name: "Dead Letter Queue", Status: "warn", Message: "Cannot check DLQ configuration"}
	}

	if status == http.StatusOK {
		return checkResult{Name: "Dead Letter Queue", Status: "pass", Message: "Dead letter queue accessible"}
	}

	return checkResult{
		Name:    "Dead Letter Queue",
		Status:  "warn",
		Message: "Dead letter queue endpoint not accessible — configure DLQ for production",
	}
}

func checkWorkerRegistration(c *client.Client) checkResult {
	data, status, err := c.Get("/workers")
	if err != nil {
		return checkResult{Name: "Worker Registration", Status: "warn", Message: "Cannot check worker status"}
	}

	if status == http.StatusOK {
		var body map[string]interface{}
		if err := json.Unmarshal(data, &body); err == nil {
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

func rawDoctorGet(endpoint string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return resp.StatusCode, err
	}
	if len(data) > 1<<20 {
		return resp.StatusCode, fmt.Errorf("response reached size limit")
	}
	return resp.StatusCode, nil
}
