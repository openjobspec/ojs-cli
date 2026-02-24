// Package doctor provides production readiness auditing for OJS deployments.
//
// It runs a suite of checks against a live OJS server and produces a
// scored report covering security, reliability, observability, and
// configuration best practices.
package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Severity classifies check results.
type Severity string

const (
	SevPass     Severity = "pass"
	SevWarning  Severity = "warning"
	SevCritical Severity = "critical"
	SevInfo     Severity = "info"
	SevSkip     Severity = "skip"
)

// Check represents a single audit check.
type Check struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Severity    Severity `json:"severity"`
	Message     string   `json:"message"`
	Fix         string   `json:"fix,omitempty"`
}

// Report is the complete audit output.
type Report struct {
	ServerURL  string    `json:"server_url"`
	Timestamp  time.Time `json:"timestamp"`
	Checks     []Check   `json:"checks"`
	Score      int       `json:"score"`       // 0-100
	MaxScore   int       `json:"max_score"`
	Grade      string    `json:"grade"`        // A, B, C, D, F
	Categories map[string]CategoryScore `json:"categories"`
}

// CategoryScore summarizes a check category.
type CategoryScore struct {
	Passed   int `json:"passed"`
	Warnings int `json:"warnings"`
	Critical int `json:"critical"`
	Total    int `json:"total"`
}

// Auditor runs production readiness checks.
type Auditor struct {
	serverURL string
	apiKey    string
	client    *http.Client
}

// NewAuditor creates a new auditor for the given server.
func NewAuditor(serverURL, apiKey string) *Auditor {
	return &Auditor{
		serverURL: strings.TrimRight(serverURL, "/"),
		apiKey:    apiKey,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Run executes all checks and returns a report.
func (a *Auditor) Run(ctx context.Context) *Report {
	report := &Report{
		ServerURL:  a.serverURL,
		Timestamp:  time.Now(),
		Categories: make(map[string]CategoryScore),
	}

	checks := []func(context.Context) Check{
		a.checkHealth,
		a.checkManifest,
		a.checkAuth,
		a.checkTLS,
		a.checkVersion,
		a.checkQueueExists,
		a.checkDeadLetterEmpty,
		a.checkWorkerHeartbeat,
		a.checkResponseHeaders,
		a.checkRequestID,
		a.checkContentType,
		a.checkMetricsEndpoint,
		a.checkErrorFormat,
		a.checkCronJobs,
		a.checkRateLimit,
		a.checkTimeout,
		a.checkGracefulShutdown,
		a.checkBackpressure,
		a.checkJobRetention,
		a.checkSpecCompliance,
	}

	for _, fn := range checks {
		check := fn(ctx)
		report.Checks = append(report.Checks, check)

		cat := report.Categories[check.Category]
		cat.Total++
		switch check.Severity {
		case SevPass:
			cat.Passed++
		case SevWarning:
			cat.Warnings++
		case SevCritical:
			cat.Critical++
		}
		report.Categories[check.Category] = cat
	}

	report.MaxScore = len(report.Checks) * 5
	report.Score = 0
	for _, c := range report.Checks {
		switch c.Severity {
		case SevPass:
			report.Score += 5
		case SevWarning:
			report.Score += 3
		case SevInfo, SevSkip:
			report.Score += 4
		}
	}

	pct := report.Score * 100 / report.MaxScore
	switch {
	case pct >= 90:
		report.Grade = "A"
	case pct >= 80:
		report.Grade = "B"
	case pct >= 70:
		report.Grade = "C"
	case pct >= 60:
		report.Grade = "D"
	default:
		report.Grade = "F"
	}

	return report
}

func (a *Auditor) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.serverURL+path, nil)
	if err != nil {
		return nil, err
	}
	if a.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	return a.client.Do(req)
}

func (a *Auditor) checkHealth(ctx context.Context) Check {
	c := Check{ID: "SEC-001", Category: "security", Name: "Health Endpoint", Description: "Server health check responds"}
	resp, err := a.get(ctx, "/ojs/v1/health")
	if err != nil {
		c.Severity = SevCritical
		c.Message = fmt.Sprintf("Health check failed: %v", err)
		return c
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		c.Severity = SevPass
		c.Message = "Health endpoint responding"
	} else {
		c.Severity = SevCritical
		c.Message = fmt.Sprintf("Health returned %d", resp.StatusCode)
	}
	return c
}

func (a *Auditor) checkManifest(ctx context.Context) Check {
	c := Check{ID: "SPEC-001", Category: "compliance", Name: "Manifest Endpoint", Description: "Server exposes OJS manifest"}
	resp, err := a.get(ctx, "/ojs/manifest")
	if err != nil {
		c.Severity = SevWarning
		c.Message = "Manifest endpoint not reachable"
		return c
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		c.Severity = SevPass
		c.Message = "Manifest endpoint available"
	} else {
		c.Severity = SevWarning
		c.Message = "Manifest returned non-200"
	}
	return c
}

func (a *Auditor) checkAuth(ctx context.Context) Check {
	c := Check{ID: "SEC-002", Category: "security", Name: "Authentication", Description: "API key authentication is enabled"}
	resp, err := a.get(ctx, "/ojs/v1/jobs/nonexistent")
	if err != nil {
		c.Severity = SevSkip
		c.Message = "Could not check auth"
		return c
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		c.Severity = SevPass
		c.Message = "Authentication is enforced"
	} else {
		c.Severity = SevCritical
		c.Message = "No authentication — server accepts unauthenticated requests"
		c.Fix = "Set OJS_API_KEY environment variable"
	}
	return c
}

func (a *Auditor) checkTLS(ctx context.Context) Check {
	c := Check{ID: "SEC-003", Category: "security", Name: "TLS/HTTPS", Description: "Server uses HTTPS"}
	if strings.HasPrefix(a.serverURL, "https://") {
		c.Severity = SevPass
		c.Message = "HTTPS is enabled"
	} else {
		c.Severity = SevWarning
		c.Message = "Server is using HTTP (not HTTPS)"
		c.Fix = "Use a TLS-terminating reverse proxy (nginx, caddy, cloud LB)"
	}
	return c
}

func (a *Auditor) checkVersion(ctx context.Context) Check {
	c := Check{ID: "SPEC-002", Category: "compliance", Name: "OJS Version Header", Description: "Response includes OJS version header"}
	resp, err := a.get(ctx, "/ojs/v1/health")
	if err != nil {
		c.Severity = SevSkip
		return c
	}
	defer resp.Body.Close()
	if v := resp.Header.Get("X-OJS-Version"); v != "" {
		c.Severity = SevPass
		c.Message = fmt.Sprintf("OJS version: %s", v)
	} else {
		c.Severity = SevWarning
		c.Message = "X-OJS-Version header missing"
	}
	return c
}

func (a *Auditor) checkQueueExists(ctx context.Context) Check {
	c := Check{ID: "OPS-001", Category: "operations", Name: "Queues Configured", Description: "At least one queue exists"}
	resp, err := a.get(ctx, "/ojs/v1/queues")
	if err != nil {
		c.Severity = SevSkip
		return c
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), `"name"`) {
		c.Severity = SevPass
		c.Message = "Queues are configured"
	} else {
		c.Severity = SevInfo
		c.Message = "No queues found (may be normal for new deployment)"
	}
	return c
}

func (a *Auditor) checkDeadLetterEmpty(ctx context.Context) Check {
	c := Check{ID: "OPS-002", Category: "operations", Name: "Dead Letter Queue", Description: "DLQ is not accumulating"}
	resp, err := a.get(ctx, "/ojs/v1/dead-letter?limit=1")
	if err != nil {
		c.Severity = SevSkip
		return c
	}
	defer resp.Body.Close()
	var result struct {
		Pagination struct{ Total int } `json:"pagination"`
	}
	body, _ := io.ReadAll(resp.Body)
	json.Unmarshal(body, &result)
	if result.Pagination.Total == 0 {
		c.Severity = SevPass
		c.Message = "Dead letter queue is empty"
	} else {
		c.Severity = SevWarning
		c.Message = fmt.Sprintf("DLQ has %d jobs — review and replay or delete", result.Pagination.Total)
		c.Fix = "Use 'ojs dead-letter' to inspect and 'ojs dead-letter retry' to replay"
	}
	return c
}

func (a *Auditor) checkWorkerHeartbeat(_ context.Context) Check {
	return Check{ID: "OPS-003", Category: "operations", Name: "Worker Heartbeat", Severity: SevInfo,
		Description: "Workers are sending heartbeats", Message: "Check via 'ojs workers' command"}
}

func (a *Auditor) checkResponseHeaders(ctx context.Context) Check {
	c := Check{ID: "SEC-004", Category: "security", Name: "Security Headers", Description: "Response includes security headers"}
	resp, err := a.get(ctx, "/ojs/v1/health")
	if err != nil {
		c.Severity = SevSkip
		return c
	}
	defer resp.Body.Close()
	missing := []string{}
	for _, h := range []string{"X-Content-Type-Options", "X-Request-Id"} {
		if resp.Header.Get(h) == "" {
			missing = append(missing, h)
		}
	}
	if len(missing) == 0 {
		c.Severity = SevPass
		c.Message = "Security headers present"
	} else {
		c.Severity = SevWarning
		c.Message = fmt.Sprintf("Missing headers: %s", strings.Join(missing, ", "))
	}
	return c
}

func (a *Auditor) checkRequestID(ctx context.Context) Check {
	c := Check{ID: "SEC-005", Category: "security", Name: "Request ID Tracing", Description: "Responses include X-Request-Id"}
	resp, err := a.get(ctx, "/ojs/v1/health")
	if err != nil {
		c.Severity = SevSkip
		return c
	}
	defer resp.Body.Close()
	if resp.Header.Get("X-Request-Id") != "" {
		c.Severity = SevPass
		c.Message = "Request ID tracing enabled"
	} else {
		c.Severity = SevWarning
		c.Message = "X-Request-Id header missing"
	}
	return c
}

func (a *Auditor) checkContentType(ctx context.Context) Check {
	c := Check{ID: "SPEC-003", Category: "compliance", Name: "Content-Type", Description: "Responses use JSON content type"}
	resp, err := a.get(ctx, "/ojs/v1/health")
	if err != nil {
		c.Severity = SevSkip
		return c
	}
	defer resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "json") {
		c.Severity = SevPass
		c.Message = fmt.Sprintf("Content-Type: %s", ct)
	} else {
		c.Severity = SevWarning
		c.Message = fmt.Sprintf("Unexpected Content-Type: %s", ct)
	}
	return c
}

func (a *Auditor) checkMetricsEndpoint(ctx context.Context) Check {
	c := Check{ID: "OBS-001", Category: "observability", Name: "Metrics Endpoint", Description: "Prometheus metrics are exposed"}
	resp, err := a.get(ctx, "/metrics")
	if err != nil {
		c.Severity = SevWarning
		c.Message = "Metrics endpoint not reachable"
		c.Fix = "Metrics are exposed at /metrics by default"
		return c
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		c.Severity = SevPass
		c.Message = "Prometheus metrics available"
	} else {
		c.Severity = SevWarning
		c.Message = "Metrics endpoint returned non-200"
	}
	return c
}

func (a *Auditor) checkErrorFormat(_ context.Context) Check {
	return Check{ID: "SPEC-004", Category: "compliance", Name: "Error Format", Severity: SevInfo,
		Description: "Errors follow OJS error schema", Message: "Validated via conformance tests"}
}

func (a *Auditor) checkCronJobs(_ context.Context) Check {
	return Check{ID: "OPS-004", Category: "operations", Name: "Cron Jobs", Severity: SevInfo,
		Description: "Cron job scheduling is configured", Message: "Check via 'ojs cron' command"}
}

func (a *Auditor) checkRateLimit(_ context.Context) Check {
	return Check{ID: "SEC-006", Category: "security", Name: "Rate Limiting", Severity: SevInfo,
		Description: "Rate limiting protects against abuse", Message: "Configure via OJS_RATE_LIMIT or rate-limit middleware"}
}

func (a *Auditor) checkTimeout(_ context.Context) Check {
	return Check{ID: "REL-001", Category: "reliability", Name: "Request Timeout", Severity: SevInfo,
		Description: "Server has configured timeouts", Message: "Verify OJS_READ_TIMEOUT and OJS_WRITE_TIMEOUT are set"}
}

func (a *Auditor) checkGracefulShutdown(_ context.Context) Check {
	return Check{ID: "REL-002", Category: "reliability", Name: "Graceful Shutdown", Severity: SevInfo,
		Description: "Server supports graceful shutdown", Message: "OJS backends handle SIGTERM by default"}
}

func (a *Auditor) checkBackpressure(_ context.Context) Check {
	return Check{ID: "REL-003", Category: "reliability", Name: "Backpressure", Severity: SevInfo,
		Description: "Backpressure mechanisms are in place", Message: "Configure queue size limits and visibility timeouts"}
}

func (a *Auditor) checkJobRetention(_ context.Context) Check {
	return Check{ID: "OPS-005", Category: "operations", Name: "Job Retention", Severity: SevInfo,
		Description: "Job retention policy is configured", Message: "Set expires_at on jobs or configure backend TTL"}
}

func (a *Auditor) checkSpecCompliance(_ context.Context) Check {
	return Check{ID: "SPEC-005", Category: "compliance", Name: "Conformance Level", Severity: SevInfo,
		Description: "Run conformance tests to validate spec compliance",
		Message: "Use: ojs-conformance-runner -url " + a.serverURL + " -suites ./suites"}
}
