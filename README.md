# ojs-cli

[![CI](https://github.com/openjobspec/ojs-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/openjobspec/ojs-cli/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/openjobspec/ojs-cli)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Command-line interface for [Open Job Spec](https://openjobspec.org) servers.

## Installation

```bash
go install github.com/openjobspec/ojs-cli/cmd/ojs@latest
```

Or build from source:

```bash
make build
```

## Quick Start

```bash
# Check server health
ojs health

# Enqueue a job
ojs enqueue --type email.send --args '["user@example.com", "Welcome!"]'

# Check job status
ojs status <job-id>

# Cancel a job
ojs cancel <job-id>

# List queues with stats
ojs queues
ojs queues --stats default

# Manage queues
ojs queues --pause critical
ojs queues --resume critical

# List workers
ojs workers

# Signal workers to stop fetching (graceful quiet)
ojs workers --quiet

# Resume worker fetching
ojs workers --resume

# Dead letter management
ojs dead-letter
ojs dead-letter --retry <job-id>
ojs dead-letter --delete <job-id>

# Cron jobs
ojs cron
ojs cron --register --name daily-report --expression '0 9 * * *' --type report.generate
ojs cron --delete daily-report

# Workflow management
ojs workflow create --name order-pipeline --steps '[{"id":"validate","type":"order.validate","args":["order-123"]},{"id":"charge","type":"payment.charge","args":["order-123"],"depends_on":["validate"]}]'
ojs workflow status <workflow-id>
ojs workflow cancel <workflow-id>
ojs workflow list
ojs workflow list --state running

# Live monitoring dashboard
ojs monitor
ojs monitor --interval 5s

# --- New Commands ---

# List and search jobs
ojs jobs --state active --queue billing --type email.send --limit 50

# Get job result (with optional wait)
ojs result <job-id>
ojs result <job-id> --wait --timeout 30

# View retry history
ojs retries <job-id>

# Update job priority
ojs priority <job-id> --set 5

# Bulk operations
ojs bulk cancel --ids job-1,job-2,job-3
ojs bulk retry --ids job-1,job-2
ojs bulk cancel --state available --queue old-queue

# Enqueue with unique constraint
ojs enqueue --type email.send --args '["user@example.com"]' --unique-key user-123 --unique-within 1h

# Bulk enqueue from NDJSON file
ojs enqueue --batch jobs.ndjson

# Dead letter purge and stats
ojs dead-letter --stats
ojs dead-letter --purge
ojs dead-letter --purge --older-than 7d

# Cron trigger, history, pause/resume
ojs cron --trigger daily-report
ojs cron --history daily-report --history-limit 20
ojs cron --pause daily-report
ojs cron --resume daily-report

# Queue create, delete, purge
ojs queues --create billing --concurrency 5 --max-size 1000
ojs queues --delete old-queue
ojs queues --purge default --states completed,discarded

# Rate limits
ojs rate-limits
ojs rate-limits --inspect email
ojs rate-limits --override email --concurrency 20
ojs rate-limits --override email --clear

# Server metrics
ojs metrics
ojs metrics --format prometheus
ojs metrics --format json

# Event streaming (SSE)
ojs events --types job.completed,job.failed --queue billing

# System maintenance
ojs system maintenance
ojs system maintenance --enable --reason "scheduled upgrade"
ojs system maintenance --disable
ojs system config

# --- Round 2 Commands ---

# Retry a job
ojs retry <job-id>

# Bulk delete terminal jobs
ojs bulk delete --ids job-1,job-2
ojs bulk delete --state completed --older-than 7d

# Webhook subscriptions
ojs webhooks list
ojs webhooks create --url https://example.com/hooks --events job.completed,job.failed --secret mysecret
ojs webhooks get <subscription-id>
ojs webhooks delete <subscription-id>
ojs webhooks test <subscription-id>
ojs webhooks rotate-secret <subscription-id>

# System statistics
ojs stats
ojs stats --queue billing
ojs stats --history --period 5m --since 24h

# Worker management (per-worker)
ojs workers --detail <worker-id>
ojs workers --quiet-worker <worker-id>
ojs workers --deregister <worker-id>

# Cron detail and update
ojs cron --detail daily-report
ojs cron --update daily-report --expression '0 10 * * *'
ojs cron --enabled true

# Queue configuration
ojs queues --config default --concurrency 10 --retention 7d

# Job detail view (full envelope)
ojs status <job-id> --detail

# Shell completions
ojs completion bash   # Add to ~/.bashrc: eval "$(ojs completion bash)"
ojs completion zsh    # Add to ~/.zshrc: eval "$(ojs completion zsh)"
ojs completion fish   # Save to ~/.config/fish/completions/ojs.fish
```

## Migration

Migrate jobs from existing job systems to OJS:

```bash
# Analyze a Sidekiq installation
ojs migrate analyze sidekiq --redis redis://localhost:6379

# Analyze a BullMQ installation
ojs migrate analyze bullmq --redis redis://localhost:6379

# Analyze a Celery installation
ojs migrate analyze celery --redis redis://localhost:6379

# Export jobs to NDJSON format
ojs migrate export sidekiq --redis redis://localhost:6379 --output jobs.ndjson

# Import into an OJS server
ojs migrate import --file jobs.ndjson
```

Supported sources: `sidekiq`, `bullmq`, `celery`. The `analyze` subcommand provides a
non-destructive report of queue names, job counts, and types. The `export` subcommand
extracts jobs to a portable NDJSON file. The `import` subcommand batch-enqueues jobs
into the target OJS server.

## Configuration

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `OJS_URL` | Server URL | `http://localhost:8080` |
| `OJS_AUTH_TOKEN` | Authentication token | (none) |
| `OJS_OUTPUT` | Output format (`table`/`json`) | `table` |

### Global Flags

```
--url <url>   Override server URL
--json        Output as JSON
--version     Show version
--help        Show help
```

## Output Formats

Table format (default):
```
NAME      STATUS
────────────────────────────────
default   active
priority  paused
```

JSON format (`--json` or `OJS_OUTPUT=json`):
```json
{
  "queues": [
    {"name": "default", "status": "active"},
    {"name": "priority", "status": "paused"}
  ]
}
```

## Development

```bash
make build    # Build binary to bin/ojs
make test     # Run tests
make lint     # Run go vet
```
