package main

import (
	"fmt"
	"os"

	"github.com/openjobspec/ojs-cli/cmd/ojs/commands"
	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/config"
	"github.com/openjobspec/ojs-cli/internal/output"
)

const version = "0.1.0"

func main() {
	cfg := config.Load()
	c := client.New(cfg)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Global flags
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--url":
			if i+1 < len(args) {
				cfg.ServerURL = args[i+1]
				c = client.New(cfg)
				args = append(args[:i], args[i+2:]...)
				i--
			}
		case "--json":
			output.Format = "json"
			args = append(args[:i], args[i+1:]...)
			i--
		case "--version", "-v":
			fmt.Println("ojs version", version)
			os.Exit(0)
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		}
	}

	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch args[0] {
	case "enqueue":
		err = commands.Enqueue(c, args[1:])
	case "status":
		err = commands.Status(c, args[1:])
	case "cancel":
		err = commands.Cancel(c, args[1:])
	case "health":
		err = commands.Health(c, args[1:])
	case "queues":
		err = commands.Queues(c, args[1:])
	case "workers":
		err = commands.Workers(c, args[1:])
	case "dead-letter":
		err = commands.DeadLetter(c, args[1:])
	case "cron":
		err = commands.Cron(c, args[1:])
	case "monitor":
		err = commands.Monitor(c, args[1:])
	case "workflow":
		err = commands.Workflow(c, args[1:])
	case "migrate":
		err = commands.Migrate(c, args[1:])
	case "completion":
		err = commands.Completion(args[1:])
	case "jobs":
		err = commands.Jobs(c, args[1:])
	case "result":
		err = commands.Result(c, args[1:])
	case "bulk":
		err = commands.Bulk(c, args[1:])
	case "priority":
		err = commands.Priority(c, args[1:])
	case "retries":
		err = commands.Retries(c, args[1:])
	case "metrics":
		err = commands.Metrics(c, args[1:])
	case "rate-limits":
		err = commands.RateLimits(c, args[1:])
	case "events":
		err = commands.Events(cfg, args[1:])
	case "system":
		err = commands.System(c, args[1:])
	case "webhooks":
		err = commands.Webhooks(c, args[1:])
	case "stats":
		err = commands.Stats(c, args[1:])
	case "retry":
		err = commands.Retry(c, args[1:])
	case "doctor":
		err = commands.Doctor(c, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`ojs - Open Job Spec CLI

Usage:
  ojs <command> [flags]

Job Commands:
  enqueue      Enqueue a new job (supports --batch for bulk)
  status       Get job status (includes progress, --detail for full envelope)
  cancel       Cancel a job
  retry        Retry a job
  result       Get job result
  jobs         List and search jobs
  priority     Update job priority
  retries      View job retry history
  bulk         Bulk cancel/retry/delete operations

Queue & Server Commands:
  queues       List, create, delete, purge, configure, pause/resume queues
  workers      List, detail, quiet, deregister workers
  dead-letter  Manage dead letter queue (list, retry, purge, stats)
  cron         Manage cron jobs (register, trigger, pause, history, detail, update)
  workflow     Manage workflows
  webhooks     Manage webhook subscriptions
  rate-limits  Inspect and override rate limits
  events       Stream server-sent events
  metrics      View server metrics
  stats        Aggregate system statistics
  system       System maintenance mode and config
  health       Check server health
  monitor      Live monitoring dashboard

Utility Commands:
  migrate      Migrate jobs from other systems
  doctor       Run health and production readiness checks
  completion   Generate shell completions

Global Flags:
  --url <url>  OJS server URL (default: $OJS_URL or http://localhost:8080)
  --json       Output as JSON
  --version    Show version
  --help       Show help

Environment Variables:
  OJS_URL         Server URL
  OJS_AUTH_TOKEN  Authentication token
  OJS_OUTPUT      Default output format (table|json)
`)
}

