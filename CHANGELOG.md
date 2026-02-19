# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 1.0.0 (2026-02-19)


### Features

* **cli:** add main entry point and command dispatcher ([a7c7921](https://github.com/openjobspec/ojs-cli/commit/a7c79211e0ed88543036ddbd91fb1c19c6548ba3))
* **client:** add HTTP client with auth and error handling ([d0aa078](https://github.com/openjobspec/ojs-cli/commit/d0aa078e44aa53f349c94ccc873996cf2bc1b0ef))
* **client:** add Patch and Put HTTP methods ([34e6257](https://github.com/openjobspec/ojs-cli/commit/34e625786e9e56ec927ed9eefc0156da98752b62))
* **commands:** add cron job scheduling commands ([b2ac46b](https://github.com/openjobspec/ojs-cli/commit/b2ac46b8d3d53e53b73f245b26a12771d0fa06c9))
* **commands:** add dead letter queue management ([1f9298c](https://github.com/openjobspec/ojs-cli/commit/1f9298c43f010ee8e9e43218d3213f898fd6bd16))
* **commands:** add job enqueue command ([59aae9f](https://github.com/openjobspec/ojs-cli/commit/59aae9f27942e4f966dcd366b9d2777a190c7294))
* **commands:** add job status and cancel commands ([0220971](https://github.com/openjobspec/ojs-cli/commit/0220971bd81b77af6fa0e7d593e055c81085d773))
* **commands:** add jobs, result, bulk, priority, retries, and retry commands ([208b526](https://github.com/openjobspec/ojs-cli/commit/208b5265bfd9b62e8c7a4ec6ceaaea2f958f091f))
* **commands:** add live monitoring dashboard ([9311534](https://github.com/openjobspec/ojs-cli/commit/9311534661400509e401d8f12a23bbc1086a06e5))
* **commands:** add metrics, rate-limits, events, and system commands ([1a9a299](https://github.com/openjobspec/ojs-cli/commit/1a9a2990270057af4f3e0b6d4b5459e4d1671ef5))
* **commands:** add migrate command with analyze, export, and import subcommands ([c98e67e](https://github.com/openjobspec/ojs-cli/commit/c98e67eeca275931f4b8ae7011318abf668df940))
* **commands:** add queue management commands ([ced578a](https://github.com/openjobspec/ojs-cli/commit/ced578a500a8cac67dc4c5575beae95e3eb589d2))
* **commands:** add server health check command ([31f02ff](https://github.com/openjobspec/ojs-cli/commit/31f02ffcfe0328a23166e34f70270b3fe6b45aab))
* **commands:** add shell completion generation ([8ec26f6](https://github.com/openjobspec/ojs-cli/commit/8ec26f646f0233db5e8c29c9b8b82156014f97f6))
* **commands:** add webhooks and stats commands ([c9bf2bc](https://github.com/openjobspec/ojs-cli/commit/c9bf2bce88bfe85fcf76e085fcf1e14dc2a3154a))
* **commands:** add worker management commands ([77a1fd6](https://github.com/openjobspec/ojs-cli/commit/77a1fd61062cf878810a2652ae335bd3de6eafcd))
* **commands:** add workflow management commands ([8312c1c](https://github.com/openjobspec/ojs-cli/commit/8312c1c7cfe0120047af4e5b9b14a0586225001c))
* **commands:** enhance enqueue, status, dead-letter, cron, queues, and workers ([9390be4](https://github.com/openjobspec/ojs-cli/commit/9390be482d7f22f19f0f5d1b48aa2eb877384b33))
* **commands:** wire new commands into CLI and update completions ([ec8627e](https://github.com/openjobspec/ojs-cli/commit/ec8627ecf57761941e0cecf1e890bdd5a99535d8))
* **config:** add environment-based configuration management ([5c65bdb](https://github.com/openjobspec/ojs-cli/commit/5c65bdb41562817d5910d086aee5926d7f779941))
* **migrate:** add Faktory migration source ([db6bb17](https://github.com/openjobspec/ojs-cli/commit/db6bb1703e1b310800652340faf606a76f185589))
* **migrate:** add migration source types and interfaces ([b60e3f2](https://github.com/openjobspec/ojs-cli/commit/b60e3f2142e70f7fd81862888fa94f2b7280afcc))
* **migrate:** add River migration source ([5ce4c20](https://github.com/openjobspec/ojs-cli/commit/5ce4c203bed8d4ac4ed7db25a6142597f327cc72))
* **migrate:** add Sidekiq, BullMQ, and Celery source adapters ([1bc3114](https://github.com/openjobspec/ojs-cli/commit/1bc31143c97a1501373ee9d3db103373dc0ce570))
* **migrate:** add validate subcommand and dry-run import flag ([59ac349](https://github.com/openjobspec/ojs-cli/commit/59ac349f954c39b0744abb01ceb2819321b03d69))
* **output:** add table and JSON output formatting ([e94630b](https://github.com/openjobspec/ojs-cli/commit/e94630ba34fe9786fed0e6542e11676fa234ac64))


### Bug Fixes

* **migrate:** add context timeouts, Close method, and better error handling ([7d3c96c](https://github.com/openjobspec/ojs-cli/commit/7d3c96c0f12a1f9ceb4c02d9040bc13866ddc4af))

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
