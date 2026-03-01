package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
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
	fs := flag.NewFlagSet("setup observability", flag.ExitOnError)
	outputDir := fs.String("output-dir", "./monitoring", "Directory to write monitoring configs")
	ojsURL := fs.String("ojs-url", "http://localhost:8080", "OJS server URL for Prometheus scraping")
	promURL := fs.String("prometheus-url", "http://prometheus:9090", "Prometheus URL for Grafana datasource")
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
		return err
	}

	dirs := []string{
		filepath.Join(*outputDir, "grafana", "dashboards"),
		filepath.Join(*outputDir, "grafana", "provisioning", "dashboards"),
		filepath.Join(*outputDir, "grafana", "provisioning", "datasources"),
		filepath.Join(*outputDir, "prometheus", "rules"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	files := map[string]string{
		filepath.Join(*outputDir, "docker-compose.yml"):                                          dockerComposeTmpl,
		filepath.Join(*outputDir, "prometheus", "prometheus.yml"):                                prometheusTmpl,
		filepath.Join(*outputDir, "prometheus", "rules", "ojs-recording.yml"):                    recordingRulesTmpl,
		filepath.Join(*outputDir, "prometheus", "rules", "ojs-alerting.yml"):                     alertingRulesTmpl,
		filepath.Join(*outputDir, "grafana", "provisioning", "datasources", "prometheus.yml"):    grafanaDatasourceTmpl,
		filepath.Join(*outputDir, "grafana", "provisioning", "dashboards", "ojs-dashboards.yml"): grafanaDashboardProvTmpl,
		filepath.Join(*outputDir, "grafana", "dashboards", "ojs-overview.json"):                  overviewDashboard,
	}

	data := map[string]string{
		"OJSURL":        *ojsURL,
		"PrometheusURL": *promURL,
	}

	for path, tmplStr := range files {
		tmpl, err := template.New(filepath.Base(path)).Parse(tmplStr)
		if err != nil {
			return fmt.Errorf("parse template for %s: %w", path, err)
		}
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("create %s: %w", path, err)
		}
		if err := tmpl.Execute(f, data); err != nil {
			f.Close()
			return fmt.Errorf("write %s: %w", path, err)
		}
		f.Close()
		fmt.Printf("  ✓ %s\n", path)
	}

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
`, *outputDir, *ojsURL)

	return nil
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
    metrics_path: /metrics
    static_configs:
      - targets:
          - "{{.OJSURL}}"
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
          description: "Current p99: {{`+"`"+`{{ $value | humanizeDuration }}`+"`"+`}}"

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
