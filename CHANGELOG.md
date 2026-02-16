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
