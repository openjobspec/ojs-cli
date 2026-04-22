package commands

import (
	"fmt"
	"sort"
	"strings"
)

// Completion generates shell completion scripts.
func Completion(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("shell type required\n\nUsage: ojs completion <bash|zsh|fish>")
	}

	switch args[0] {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		return fmt.Errorf("unsupported shell: %s\n\nSupported: bash, zsh, fish", args[0])
	}
	return nil
}

var commands = map[string][]string{
	"bulk":        {},
	"cancel":      {},
	"codegen":     {"--manifest", "--lang", "--out", "--package"},
	"completion":  {},
	"contract":    {},
	"create":      {"--backend", "--language", "--queue", "--port", "--otel", "--docker", "--ci", "--ci-provider", "--module", "--output-dir", "--dry-run"},
	"cron":        {"--register", "--delete", "--name", "--expression", "--type", "--queue", "--trigger", "--history", "--history-limit", "--pause", "--resume", "--detail", "--update", "--enabled"},
	"dead-letter": {"--retry", "--delete", "--limit", "--purge", "--stats", "--older-than"},
	"debug":       {},
	"dev":         {"--port", "--grpc", "--verbose"},
	"doctor":      {"--production", "--verbose"},
	"enqueue":     {"--type", "--queue", "--priority", "--args", "--meta", "--max-attempts", "--unique-key", "--unique-within", "--batch"},
	"events":      {"--follow", "--types", "--queue"},
	"health":      {},
	"jobs":        {"--state", "--queue", "--type", "--limit"},
	"metrics":     {"--format"},
	"migrate":     {},
	"monitor":     {"--interval"},
	"priority":    {"--set"},
	"queues":      {"--stats", "--pause", "--resume", "--create", "--delete", "--purge", "--config", "--concurrency", "--max-size", "--states", "--retention"},
	"rate-limits": {"--inspect", "--override", "--concurrency", "--clear"},
	"result":      {"--wait", "--timeout"},
	"retries":     {},
	"retry":       {},
	"setup":       {},
	"stats":       {"--history", "--period", "--since", "--queue"},
	"status":      {"--detail"},
	"system":      {},
	"webhooks":    {},
	"workers":     {"--quiet", "--resume", "--detail", "--quiet-worker", "--deregister"},
	"workflow":    {},
}

var subcommandFlags = map[string]map[string][]string{
	"bulk": {
		"cancel": {"--ids", "--state", "--queue"},
		"delete": {"--ids", "--state", "--queue", "--older-than"},
		"retry":  {"--ids", "--state", "--queue"},
	},
	"contract": {
		"init":     {"--service", "--role"},
		"test":     {"--contracts", "--registry"},
		"validate": {"--contracts"},
	},
	"debug": {
		"bottleneck": {"--limit"},
		"failures":   {"--limit"},
		"health":     {},
		"history":    {},
		"inspect":    {},
		"queue":      {},
		"replay":     {"--queue", "--priority"},
		"trace":      {},
	},
	"migrate": {
		"analyze":         {"--redis"},
		"bullmq":          {"--output", "--dry-run"},
		"celery":          {"--output", "--dry-run"},
		"detect":          {},
		"export":          {"--redis", "--output", "--allow-partial"},
		"generate":        {"--source", "--output"},
		"import":          {"--file", "--dry-run"},
		"shadow":          {},
		"sidekiq":         {"--output", "--dry-run"},
		"validate":        {"--file"},
		"validate-config": {},
	},
	"setup": {
		"observability": {"--output-dir", "--ojs-url", "--prometheus-url"},
	},
	"system": {
		"config":      {},
		"maintenance": {"--enable", "--disable", "--reason"},
	},
	"webhooks": {
		"create":        {"--url", "--events", "--secret"},
		"delete":        {},
		"get":           {},
		"list":          {"--limit"},
		"rotate-secret": {},
		"test":          {},
	},
	"workflow": {
		"cancel": {},
		"create": {"--name", "--steps"},
		"list":   {"--limit", "--state"},
		"status": {},
	},
}

var commandDescriptions = map[string]string{
	"bulk":        "Bulk cancel, retry, or delete jobs",
	"cancel":      "Cancel a job",
	"codegen":     "Generate type-safe SDK code",
	"completion":  "Generate shell completions",
	"contract":    "Validate schema contracts",
	"create":      "Create an OJS project",
	"cron":        "Manage cron jobs",
	"dead-letter": "Manage dead letter queue",
	"debug":       "Debug jobs and queues",
	"dev":         "Run a local development server",
	"doctor":      "Run health and readiness checks",
	"enqueue":     "Enqueue a new job",
	"events":      "Stream server-sent events",
	"health":      "Check server health",
	"jobs":        "List and search jobs",
	"metrics":     "View server metrics",
	"migrate":     "Migrate jobs from other systems",
	"monitor":     "Live monitoring dashboard",
	"priority":    "Update job priority",
	"queues":      "List and manage queues",
	"rate-limits": "Inspect and override rate limits",
	"result":      "Get job result",
	"retries":     "View job retry history",
	"retry":       "Retry a job",
	"setup":       "Generate operational configuration",
	"stats":       "Aggregate system statistics",
	"status":      "Get job status",
	"system":      "System maintenance and config",
	"webhooks":    "Manage webhook subscriptions",
	"workers":     "List and manage workers",
	"workflow":    "Manage workflows",
}

var globalFlags = []string{"--url", "--json", "--version", "--help"}

var (
	bashCompletion = buildBashCompletion()
	zshCompletion  = buildZshCompletion()
	fishCompletion = buildFishCompletion()
)

func commandNames() []string {
	return sortedMapKeys(commands)
}

func sortedMapKeys[T any](values map[string]T) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func flagsWithGlobals(flags []string) []string {
	result := append([]string(nil), flags...)
	return append(result, globalFlags...)
}

func generateSubcommandCompletion(subcommands map[string][]string) string {
	var builder strings.Builder
	names := sortedMapKeys(subcommands)
	builder.WriteString(fmt.Sprintf(`            if [ ${COMP_CWORD} -eq 2 ]; then
                COMPREPLY=($(compgen -W %q -- "${cur}"))
            else
                case "${COMP_WORDS[2]}" in
`, strings.Join(names, " ")))
	for _, name := range names {
		flags := flagsWithGlobals(subcommands[name])
		builder.WriteString(fmt.Sprintf(
			"                    %s) COMPREPLY=($(compgen -W %q -- \"${cur}\")) ;;\n",
			name,
			strings.Join(flags, " "),
		))
	}
	builder.WriteString("                esac\n            fi\n")
	return builder.String()
}

func buildBashCompletion() string {
	var builder strings.Builder
	builder.WriteString(`_ojs() {
    local cur prev commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    commands="` + strings.Join(commandNames(), " ") + `"

    if [ ${COMP_CWORD} -eq 1 ]; then
        COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
        return 0
    fi

    case "${COMP_WORDS[1]}" in
`)
	for _, command := range commandNames() {
		builder.WriteString(fmt.Sprintf("        %s)\n", command))
		switch {
		case command == "completion":
			builder.WriteString(`            COMPREPLY=($(compgen -W "bash zsh fish" -- "${cur}"))
`)
		case subcommandFlags[command] != nil:
			builder.WriteString(generateSubcommandCompletion(subcommandFlags[command]))
		default:
			flags := flagsWithGlobals(commands[command])
			builder.WriteString(fmt.Sprintf(
				"            COMPREPLY=($(compgen -W %q -- \"${cur}\"))\n",
				strings.Join(flags, " "),
			))
		}
		builder.WriteString("            ;;\n")
	}
	builder.WriteString(`    esac
    return 0
}
complete -F _ojs ojs
`)
	return builder.String()
}

func buildZshCompletion() string {
	var builder strings.Builder
	builder.WriteString(`#compdef ojs

_ojs() {
    local -a commands
    commands=(
`)
	for _, command := range commandNames() {
		builder.WriteString(fmt.Sprintf("        '%s:%s'\n", command, commandDescriptions[command]))
	}
	builder.WriteString(`    )

    _arguments -C \
        '--url[OJS server URL]:url' \
        '--json[Output as JSON]' \
        '--version[Show version]' \
        '--help[Show help]' \
        '1:command:->command' \
        '*::arg:->args'

    case $state in
    command)
        _describe 'command' commands
        ;;
    args)
        case $words[1] in
`)
	for _, command := range commandNames() {
		builder.WriteString(buildZshCommandCompletion(command))
	}
	builder.WriteString(`        esac
        ;;
    esac
}

_ojs "$@"
`)
	return builder.String()
}

func buildZshCommandCompletion(command string) string {
	if command == "completion" {
		return `        completion)
            _values 'shell' bash zsh fish
            ;;
`
	}
	if subcommands := subcommandFlags[command]; subcommands != nil {
		return fmt.Sprintf(
			"        %s)\n            _values 'subcommand' %s\n            ;;\n",
			command,
			strings.Join(sortedMapKeys(subcommands), " "),
		)
	}
	if len(commands[command]) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("        %s)\n            _arguments", command))
	for _, flag := range commands[command] {
		builder.WriteString(fmt.Sprintf(" \\\n                '%s'", flag))
	}
	builder.WriteString("\n            ;;\n")
	return builder.String()
}

func buildFishCompletion() string {
	var builder strings.Builder
	builder.WriteString("# Fish completion for ojs\n\ncomplete -c ojs -f\n\n")
	for _, flag := range globalFlags {
		builder.WriteString(fmt.Sprintf("complete -c ojs -l %s\n", strings.TrimPrefix(flag, "--")))
	}
	builder.WriteString("\n")
	for _, command := range commandNames() {
		builder.WriteString(fmt.Sprintf(
			"complete -c ojs -n '__fish_use_subcommand' -a %s -d '%s'\n",
			command,
			commandDescriptions[command],
		))
	}
	builder.WriteString("\n")
	for _, command := range commandNames() {
		writeFishCommandCompletion(&builder, command)
	}
	return builder.String()
}

func writeFishCommandCompletion(builder *strings.Builder, command string) {
	if command == "completion" {
		for _, shell := range []string{"bash", "zsh", "fish"} {
			fmt.Fprintf(
				builder,
				"complete -c ojs -n '__fish_seen_subcommand_from completion' -a %s -d '%s completion'\n",
				shell,
				shell,
			)
		}
		return
	}
	if subcommands := subcommandFlags[command]; subcommands != nil {
		for _, subcommand := range sortedMapKeys(subcommands) {
			fmt.Fprintf(
				builder,
				"complete -c ojs -n '__fish_seen_subcommand_from %s' -a %s\n",
				command,
				subcommand,
			)
		}
		return
	}
	for _, flag := range commands[command] {
		fmt.Fprintf(
			builder,
			"complete -c ojs -n '__fish_seen_subcommand_from %s' -l %s\n",
			command,
			strings.TrimPrefix(flag, "--"),
		)
	}
}
