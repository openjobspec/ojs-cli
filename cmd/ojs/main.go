package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/openjobspec/ojs-cli/cmd/ojs/commands"
	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/config"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// version is overridden at release time via -ldflags "-X main.version=...".
var version = "dev"

var errAlreadyReported = errors.New("CLI error already reported")

type globalAction int

const (
	actionRun globalAction = iota
	actionHelp
	actionVersion
)

type globalOptions struct {
	args   []string
	action globalAction
	json   bool
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if !errors.Is(err, errAlreadyReported) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	options, err := parseGlobalArgs(args, cfg)
	if err != nil {
		return err
	}
	if options.json {
		cfg.Output = "json"
	}
	output.Format = cfg.Output

	switch options.action {
	case actionHelp:
		printUsage()
		return nil
	case actionVersion:
		fmt.Println("ojs version", version)
		return nil
	}
	if len(options.args) == 0 {
		printUsage()
		return errAlreadyReported
	}

	apiClient := client.New(cfg)
	handler, ok := commandHandlers(apiClient, cfg)[options.args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", options.args[0])
		printUsage()
		return errAlreadyReported
	}
	return handler(options.args[1:])
}

func parseGlobalArgs(args []string, cfg *config.Config) (globalOptions, error) {
	options := globalOptions{args: make([]string, 0, len(args))}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--url":
			if i+1 >= len(args) {
				return globalOptions{}, fmt.Errorf("--url requires a value")
			}
			cfg.ServerURL = args[i+1]
			i++
		case "--json":
			options.json = true
		case "--version", "-v":
			options.action = actionVersion
		case "--help", "-h":
			options.action = actionHelp
		default:
			options.args = append(options.args, args[i])
		}
	}
	return options, nil
}

func commandHandlers(apiClient *client.Client, cfg *config.Config) map[string]func([]string) error {
	return map[string]func([]string) error{
		"dev":         commands.Dev,
		"enqueue":     func(args []string) error { return commands.Enqueue(apiClient, args) },
		"status":      func(args []string) error { return commands.Status(apiClient, args) },
		"cancel":      func(args []string) error { return commands.Cancel(apiClient, args) },
		"health":      func(args []string) error { return commands.Health(apiClient, args) },
		"queues":      func(args []string) error { return commands.Queues(apiClient, args) },
		"workers":     func(args []string) error { return commands.Workers(apiClient, args) },
		"dead-letter": func(args []string) error { return commands.DeadLetter(apiClient, args) },
		"cron":        func(args []string) error { return commands.Cron(apiClient, args) },
		"monitor":     func(args []string) error { return commands.Monitor(apiClient, args) },
		"workflow":    func(args []string) error { return commands.Workflow(apiClient, args) },
		"migrate":     func(args []string) error { return commands.Migrate(apiClient, args) },
		"completion":  commands.Completion,
		"jobs":        func(args []string) error { return commands.Jobs(apiClient, args) },
		"result":      func(args []string) error { return commands.Result(apiClient, args) },
		"bulk":        func(args []string) error { return commands.Bulk(apiClient, args) },
		"priority":    func(args []string) error { return commands.Priority(apiClient, args) },
		"retries":     func(args []string) error { return commands.Retries(apiClient, args) },
		"metrics":     func(args []string) error { return commands.Metrics(apiClient, args) },
		"rate-limits": func(args []string) error { return commands.RateLimits(apiClient, args) },
		"events":      func(args []string) error { return commands.Events(cfg, args) },
		"system":      func(args []string) error { return commands.System(apiClient, args) },
		"webhooks":    func(args []string) error { return commands.Webhooks(apiClient, args) },
		"stats":       func(args []string) error { return commands.Stats(apiClient, args) },
		"retry":       func(args []string) error { return commands.Retry(apiClient, args) },
		"doctor":      func(args []string) error { return commands.Doctor(apiClient, args) },
		"debug":       func(args []string) error { return commands.Debug(apiClient, args) },
		"codegen":     commands.Codegen,
		"contract":    commands.RunContractCommand,
		"setup":       commands.RunSetupCommand,
		"create":      commands.CreateProject,
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
  contract     Validate producer/consumer schema contracts
  doctor       Run health and production readiness checks
  debug        Interactive job debugging (inspect, trace, replay, history, bottleneck)
  codegen      Generate type-safe SDK code from job definitions
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
