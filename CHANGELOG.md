# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Core CLI commands: `enqueue`, `cancel`, `status`, `health`, `monitor`
- Queue management: `queues` command with pause/resume support
- Worker inspection: `workers` command with detailed worker listing
- Dead letter queue management: `deadletter` command with retry/discard operations
- Cron job management: `cron` command for listing and managing periodic jobs
- Workflow inspection: `workflow` command for viewing workflow state
- Framework migration tooling: `migrate` command supporting Sidekiq, Celery, BullMQ
- Shell completion: `completion` command for bash/zsh/fish/powershell
- Configurable output formats: JSON and table output via `--format` flag
- Configurable server URL via `--url` flag and `OJS_URL` environment variable
- Makefile with build, test, lint, and run targets
- GitHub Actions CI workflow
- README with installation, usage, and command reference
- **`jobs` command**: List and search jobs with `--state`, `--queue`, `--type`, `--limit` filters
- **`result` command**: Retrieve job results with `--wait` and `--timeout` for synchronous polling
- **`bulk` command**: Bulk `cancel` and `retry` operations by IDs or state/queue filters
- **`priority` command**: Update job priority via PATCH
- **`retries` command**: View retry history and policy for a job
- **`metrics` command**: View server metrics in table, JSON, or Prometheus format
- **`rate-limits` command**: List, inspect, and override rate limits
- **`events` command**: Stream server-sent events with `--follow`, `--types`, `--queue` filters
- **`system` command**: Manage maintenance mode (`--enable`/`--disable`) and view system config
- **Enqueue enhancements**: `--unique-key`/`--unique-within` for unique job deduplication, `--batch` for bulk enqueue from NDJSON files
- **Status enhancements**: Progress tracking display (percentage and progress data)
- **Dead letter enhancements**: `--purge` (with `--older-than`), `--stats` (by queue and type)
- **Cron enhancements**: `--trigger` (manual trigger), `--history` (execution history), `--pause`/`--resume`
- **Queue enhancements**: `--create` (with `--concurrency`/`--max-size`), `--delete`, `--purge` (with `--states` filter)
- HTTP client `Patch()` and `Put()` methods for update operations
- **`webhooks` command**: Full webhook subscription CRUD — `create`, `list`, `get`, `delete`, `test`, `rotate-secret`
- **`stats` command**: Aggregate system statistics with `--history`, `--period`, `--since`, `--queue` for time-series
- **`retry` command**: Retry an individual job by ID (admin endpoint)
- **Bulk delete**: `ojs bulk delete` subcommand with `--ids`, `--state`, `--older-than` filters
- **Worker management**: `--detail <id>`, `--quiet-worker <id>`, `--deregister <id>` for per-worker operations
- **Cron detail/update**: `--detail <name>` for full cron info, `--update <name>` for PATCH updates, `--enabled` filter for list
- **Queue config**: `--config <name>` with `--concurrency`, `--max-size`, `--retention` for updating queue configuration
- **Status detail**: `--detail` flag for full admin job envelope (args, meta, options, error history)
