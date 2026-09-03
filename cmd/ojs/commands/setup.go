package commands

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"text/template"

	"github.com/openjobspec/ojs-cli/internal/fileutil"
	"github.com/openjobspec/ojs-cli/internal/redact"
)

// RunSetupCommand dispatches to setup subcommands.
func RunSetupCommand(args []string) error {
	if len(args) == 0 {
		printSetupUsage()
		return nil
	}
	switch args[0] {
	case "observability":
		return setupObservability(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown setup subcommand: %s\n", args[0])
		printSetupUsage()
		return fmt.Errorf("unknown setup subcommand: %s", args[0])
	}
}

func printSetupUsage() {
	fmt.Print(`Usage: ojs setup <subcommand> [flags]

Subcommands:
  observability   Generate Grafana dashboards, Prometheus rules, and Docker Compose for monitoring

Run 'ojs setup <subcommand> --help' for details.
`)
}

func setupObservability(args []string) error {
	options, err := parseObservabilityOptions(args)
	if err != nil {
		return err
	}
	data, err := observabilityTemplateData(options)
	if err != nil {
		return err
	}
	if err := createObservabilityDirectories(options.outputDir); err != nil {
		return err
	}
	for _, artifact := range observabilityArtifacts(options.outputDir) {
		if err := writeObservabilityArtifact(artifact, data); err != nil {
			return err
		}
		fmt.Printf("  ✓ %s\n", artifact.path)
	}

	printObservabilitySummary(options)
	return nil
}

type observabilityOptions struct {
	outputDir     string
	ojsURL        string
	prometheusURL string
}

type observabilityArtifact struct {
	path     string
	template string
}

func parseObservabilityOptions(args []string) (observabilityOptions, error) {
	options := observabilityOptions{}
	fs := flag.NewFlagSet("setup observability", flag.ContinueOnError)
	fs.StringVar(&options.outputDir, "output-dir", "./monitoring", "Directory to write monitoring configs")
	fs.StringVar(&options.ojsURL, "ojs-url", "http://localhost:8080", "OJS server URL for Prometheus scraping")
	fs.StringVar(&options.prometheusURL, "prometheus-url", "http://prometheus:9090", "Prometheus URL for Grafana datasource")
	fs.Usage = func() {
		fmt.Print(`Usage: ojs setup observability [flags]

Generate a complete monitoring stack for your OJS deployment:
  - Grafana dashboards (overview, queues, workers, dead letter)
  - Prometheus recording rules and alerting rules
  - Docker Compose file to run the monitoring stack
  - Default SLO definitions

Flags:
  --output-dir <dir>       Output directory (default: ./monitoring)
  --ojs-url <url>          OJS server URL for scraping (default: http://localhost:8080)
  --prometheus-url <url>   Prometheus URL for Grafana (default: http://prometheus:9090)
`)
	}
	if err := fs.Parse(args); err != nil {
		return observabilityOptions{}, fmt.Errorf("parse flags: %w", err)
	}
	return options, nil
}

func createObservabilityDirectories(outputDir string) error {
	dirs := []string{
		filepath.Join(outputDir, "grafana", "dashboards"),
		filepath.Join(outputDir, "grafana", "provisioning", "dashboards"),
		filepath.Join(outputDir, "grafana", "provisioning", "datasources"),
		filepath.Join(outputDir, "prometheus", "rules"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}
	return nil
}

func observabilityArtifacts(outputDir string) []observabilityArtifact {
	return []observabilityArtifact{
		{path: filepath.Join(outputDir, "docker-compose.yml"), template: dockerComposeTmpl},
		{path: filepath.Join(outputDir, "prometheus", "prometheus.yml"), template: prometheusTmpl},
		{path: filepath.Join(outputDir, "prometheus", "rules", "ojs-recording.yml"), template: recordingRulesTmpl},
		{path: filepath.Join(outputDir, "prometheus", "rules", "ojs-alerting.yml"), template: alertingRulesTmpl},
		{path: filepath.Join(outputDir, "grafana", "provisioning", "datasources", "prometheus.yml"), template: grafanaDatasourceTmpl},
		{path: filepath.Join(outputDir, "grafana", "provisioning", "dashboards", "ojs-dashboards.yml"), template: grafanaDashboardProvTmpl},
		{path: filepath.Join(outputDir, "grafana", "dashboards", "ojs-overview.json"), template: overviewDashboard},
	}
}

func observabilityTemplateData(options observabilityOptions) (map[string]string, error) {
	ojsURL, err := url.Parse(options.ojsURL)
	if err != nil || ojsURL.Scheme == "" || ojsURL.Host == "" {
		return nil, fmt.Errorf("invalid --ojs-url %q", redact.URL(options.ojsURL))
	}
	if ojsURL.Scheme != "http" && ojsURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported --ojs-url scheme %q", ojsURL.Scheme)
	}
	return map[string]string{
		"OJSScheme":     ojsURL.Scheme,
		"OJSTarget":     ojsURL.Host,
		"PrometheusURL": options.prometheusURL,
	}, nil
}

func writeObservabilityArtifact(artifact observabilityArtifact, data map[string]string) error {
	tmpl, err := template.New(filepath.Base(artifact.path)).Parse(artifact.template)
	if err != nil {
		return fmt.Errorf("parse template for %s: %w", artifact.path, err)
	}
	err = fileutil.WriteAtomic(artifact.path, 0o644, func(writer io.Writer) error {
		if err := tmpl.Execute(writer, data); err != nil {
			return fmt.Errorf("render template: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("write %s: %w", artifact.path, err)
	}
	return nil
}

func printObservabilitySummary(options observabilityOptions) {
	fmt.Printf(`
╔══════════════════════════════════════════════╗
║  📊 OJS Observability Stack Generated        ║
╠══════════════════════════════════════════════╣
║                                              ║
║  Start the monitoring stack:                 ║
║    cd %s
║    docker compose up -d                      ║
║                                              ║
║  Access:                                     ║
║    Grafana:    http://localhost:3000          ║
║                (admin/admin)                 ║
║    Prometheus: http://localhost:9090          ║
║                                              ║
║  OJS server must be running at:              ║
║    %s
║                                              ║
╚══════════════════════════════════════════════╝
`, options.outputDir, redact.URL(options.ojsURL))
}

var dockerComposeTmpl = `services:
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - ./prometheus/rules:/etc/prometheus/rules:ro
    restart: unless-stopped

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    volumes:
      - ./grafana/dashboards:/var/lib/grafana/dashboards:ro
      - ./grafana/provisioning:/etc/grafana/provisioning:ro
    environment:
      GF_SECURITY_ADMIN_USER: admin
      GF_SECURITY_ADMIN_PASSWORD: admin
      GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH: /var/lib/grafana/dashboards/ojs-overview.json
    depends_on:
      - prometheus
    restart: unless-stopped
`

var prometheusTmpl = `global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - /etc/prometheus/rules/*.yml

scrape_configs:
  - job_name: ojs
    scheme: "{{.OJSScheme}}"
    metrics_path: /metrics
    static_configs:
      - targets:
          - "{{.OJSTarget}}"
        labels:
          service: ojs
`

var recordingRulesTmpl = `groups:
  - name: ojs-recording
    interval: 30s
    rules:
      - record: ojs:enqueue:rate5m
        expr: rate(ojs_jobs_enqueued_total[5m])

      - record: ojs:process:rate5m
        expr: rate(ojs_jobs_processed_total[5m])

      - record: ojs:errors:rate5m
        expr: rate(ojs_jobs_failed_total[5m])

      - record: ojs:enqueue:latency_p99
        expr: histogram_quantile(0.99, rate(ojs_enqueue_duration_seconds_bucket[5m]))

      - record: ojs:process:latency_p99
        expr: histogram_quantile(0.99, rate(ojs_job_duration_seconds_bucket[5m]))
`

var alertingRulesTmpl = `groups:
  - name: ojs-slos
    rules:
      - alert: OJSEnqueueLatencyHigh
        expr: ojs:enqueue:latency_p99 > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "OJS enqueue p99 latency exceeds 50ms SLO"
          description: "Current p99: {{` + "`" + `{{ $value | humanizeDuration }}` + "`" + `}}"

      - alert: OJSProcessingLatencyHigh
        expr: ojs:process:latency_p99 > 5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "OJS job processing p99 latency exceeds 5s SLO"

      - alert: OJSDeadLetterRateHigh
        expr: rate(ojs_dead_letter_total[5m]) / (rate(ojs_jobs_processed_total[5m]) + 0.001) > 0.01
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: "DLQ rate exceeds 1% of processed jobs"

      - alert: OJSBackendUnhealthy
        expr: up{job="ojs"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "OJS backend is unreachable"
`

var grafanaDatasourceTmpl = `apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: {{.PrometheusURL}}
    isDefault: true
    editable: false
`

var grafanaDashboardProvTmpl = `apiVersion: 1
providers:
  - name: OJS
    orgId: 1
    folder: OJS
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    options:
      path: /var/lib/grafana/dashboards
      foldersFromFilesStructure: false
`

var overviewDashboard = `{
  "dashboard": {
    "title": "OJS Overview",
    "uid": "ojs-overview",
    "panels": [
      {
        "title": "Enqueue Rate",
        "type": "timeseries",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
        "targets": [{"expr": "ojs:enqueue:rate5m", "legendFormat": "enqueue/s"}]
      },
      {
        "title": "Processing Rate",
        "type": "timeseries",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
        "targets": [{"expr": "ojs:process:rate5m", "legendFormat": "process/s"}]
      },
      {
        "title": "Enqueue Latency (p99)",
        "type": "stat",
        "gridPos": {"h": 4, "w": 6, "x": 0, "y": 8},
        "targets": [{"expr": "ojs:enqueue:latency_p99"}],
        "fieldConfig": {"defaults": {"unit": "s"}}
      },
      {
        "title": "Processing Latency (p99)",
        "type": "stat",
        "gridPos": {"h": 4, "w": 6, "x": 6, "y": 8},
        "targets": [{"expr": "ojs:process:latency_p99"}],
        "fieldConfig": {"defaults": {"unit": "s"}}
      },
      {
        "title": "Error Rate",
        "type": "stat",
        "gridPos": {"h": 4, "w": 6, "x": 12, "y": 8},
        "targets": [{"expr": "ojs:errors:rate5m"}],
        "fieldConfig": {"defaults": {"unit": "ops"}}
      },
      {
        "title": "Dead Letter Queue Size",
        "type": "stat",
        "gridPos": {"h": 4, "w": 6, "x": 18, "y": 8},
        "targets": [{"expr": "ojs_dead_letter_total"}]
      }
    ],
    "time": {"from": "now-1h", "to": "now"},
    "refresh": "10s"
  }
}
`
