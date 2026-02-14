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
	"enqueue":     {"--type", "--queue", "--priority", "--args", "--meta", "--max-attempts"},
	"status":      {},
	"cancel":      {},
	"health":      {},
	"queues":      {"--stats", "--pause", "--resume"},
	"workers":     {"--quiet", "--resume"},
	"dead-letter": {"--retry", "--delete", "--limit"},
	"cron":        {"--register", "--delete", "--name", "--expression", "--type", "--queue"},
	"monitor":     {"--interval"},
	"workflow":    {},
	"completion":  {},
}

var workflowSubcommands = map[string][]string{
	"create": {"--name", "--steps"},
	"status": {},
	"cancel": {},
	"list":   {"--limit", "--state"},
}

var globalFlags = []string{"--url", "--json", "--version", "--help"}

func commandNames() []string {
	names := make([]string, 0, len(commands))
	for k := range commands {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

var bashCompletion = func() string {
	var b strings.Builder
	b.WriteString(`_ojs() {
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
	for _, cmd := range commandNames() {
		flags := append(commands[cmd], globalFlags...)
		b.WriteString(fmt.Sprintf("        %s)\n", cmd))
		if cmd == "workflow" {
			b.WriteString(`            if [ ${COMP_CWORD} -eq 2 ]; then
                COMPREPLY=($(compgen -W "create status cancel list" -- "${cur}"))
            else
                case "${COMP_WORDS[2]}" in
`)
			for sub, subFlags := range workflowSubcommands {
				allFlags := append(subFlags, globalFlags...)
				b.WriteString(fmt.Sprintf("                    %s) COMPREPLY=($(compgen -W \"%s\" -- \"${cur}\")) ;;\n",
					sub, strings.Join(allFlags, " ")))
			}
			b.WriteString("                esac\n            fi\n")
		} else if cmd == "completion" {
			b.WriteString(`            COMPREPLY=($(compgen -W "bash zsh fish" -- "${cur}"))
`)
		} else {
			b.WriteString(fmt.Sprintf("            COMPREPLY=($(compgen -W \"%s\" -- \"${cur}\"))\n",
				strings.Join(flags, " ")))
		}
		b.WriteString("            ;;\n")
	}
	b.WriteString(`    esac
    return 0
}
complete -F _ojs ojs
`)
	return b.String()
}()

var zshCompletion = func() string {
	var b strings.Builder
	b.WriteString(`#compdef ojs

_ojs() {
    local -a commands
    commands=(
`)
	for _, cmd := range commandNames() {
		desc := commandDescriptions[cmd]
		b.WriteString(fmt.Sprintf("        '%s:%s'\n", cmd, desc))
	}
	b.WriteString(`    )

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
	for _, cmd := range commandNames() {
		flags := commands[cmd]
		if cmd == "workflow" {
			b.WriteString(`        workflow)
            local -a subcommands
            subcommands=(
                'create:Create a new workflow'
                'status:Get workflow status'
                'cancel:Cancel a workflow'
                'list:List workflows'
            )
            _describe 'subcommand' subcommands
            ;;
`)
		} else if cmd == "completion" {
			b.WriteString(`        completion)
            _values 'shell' bash zsh fish
            ;;
`)
		} else if len(flags) > 0 {
			b.WriteString(fmt.Sprintf("        %s)\n            _arguments", cmd))
			for _, f := range flags {
				b.WriteString(fmt.Sprintf(" \\\n                '%s'", f))
			}
			b.WriteString("\n            ;;\n")
		}
	}
	b.WriteString(`        esac
        ;;
    esac
}

_ojs "$@"
`)
	return b.String()
}()

var fishCompletion = func() string {
	var b strings.Builder
	b.WriteString("# Fish completion for ojs\n\n")

	// Disable file completions
	b.WriteString("complete -c ojs -f\n\n")

	// Global flags
	for _, f := range globalFlags {
		name := strings.TrimPrefix(f, "--")
		b.WriteString(fmt.Sprintf("complete -c ojs -l %s\n", name))
	}
	b.WriteString("\n")

	// Commands
	for _, cmd := range commandNames() {
		desc := commandDescriptions[cmd]
		b.WriteString(fmt.Sprintf("complete -c ojs -n '__fish_use_subcommand' -a %s -d '%s'\n", cmd, desc))
	}
	b.WriteString("\n")

	// Command-specific flags
	for _, cmd := range commandNames() {
		flags := commands[cmd]
		if cmd == "workflow" {
			for sub, desc := range map[string]string{
				"create": "Create a new workflow",
				"status": "Get workflow status",
				"cancel": "Cancel a workflow",
				"list":   "List workflows",
			} {
				b.WriteString(fmt.Sprintf("complete -c ojs -n '__fish_seen_subcommand_from workflow' -a %s -d '%s'\n", sub, desc))
			}
		} else if cmd == "completion" {
			for _, shell := range []string{"bash", "zsh", "fish"} {
				b.WriteString(fmt.Sprintf("complete -c ojs -n '__fish_seen_subcommand_from completion' -a %s -d '%s completion'\n", shell, shell))
			}
		} else {
			for _, f := range flags {
				name := strings.TrimPrefix(f, "--")
				b.WriteString(fmt.Sprintf("complete -c ojs -n '__fish_seen_subcommand_from %s' -l %s\n", cmd, name))
			}
		}
	}

	return b.String()
}()

var commandDescriptions = map[string]string{
	"enqueue":     "Enqueue a new job",
	"status":      "Get job status",
	"cancel":      "Cancel a job",
	"health":      "Check server health",
	"queues":      "List and manage queues",
	"workers":     "List active workers",
	"dead-letter": "Manage dead letter queue",
	"cron":        "Manage cron jobs",
	"monitor":     "Live monitoring dashboard",
	"workflow":    "Manage workflows",
	"completion":  "Generate shell completions",
}
