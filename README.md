# ojs-cli

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

# Shell completions
ojs completion bash   # Add to ~/.bashrc: eval "$(ojs completion bash)"
ojs completion zsh    # Add to ~/.zshrc: eval "$(ojs completion zsh)"
ojs completion fish   # Save to ~/.config/fish/completions/ojs.fish
```

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
