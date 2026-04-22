package commands

import (
	"slices"
	"strings"
	"testing"
)

func TestCompletionIncludesEveryTopLevelCommand(t *testing.T) {
	want := []string{
		"bulk", "cancel", "codegen", "completion", "contract", "create", "cron",
		"dead-letter", "debug", "dev", "doctor", "enqueue", "events", "health",
		"jobs", "metrics", "migrate", "monitor", "priority", "queues",
		"rate-limits", "result", "retries", "retry", "setup", "stats", "status",
		"system", "webhooks", "workers", "workflow",
	}
	got := commandNames()
	if !slices.Equal(got, want) {
		t.Fatalf("commandNames() = %v, want %v", got, want)
	}
	for _, command := range want {
		if !strings.Contains(bashCompletion, command) {
			t.Errorf("bash completion missing %q", command)
		}
	}
}

func TestCompletionGenerationIsDeterministicAndDoesNotMutateFlags(t *testing.T) {
	before := append([]string(nil), subcommandFlags["bulk"]["delete"]...)
	first := buildBashCompletion()
	second := buildBashCompletion()
	if first != second {
		t.Fatal("bash completion output changed between identical builds")
	}
	if !slices.Equal(before, subcommandFlags["bulk"]["delete"]) {
		t.Fatalf("completion generation mutated flags: before=%v after=%v",
			before, subcommandFlags["bulk"]["delete"])
	}
}

func TestCompletionIncludesMigrationAndBulkDeleteSubcommands(t *testing.T) {
	for _, script := range []string{bashCompletion, zshCompletion, fishCompletion} {
		for _, subcommand := range []string{"validate-config", "delete"} {
			if !strings.Contains(script, subcommand) {
				t.Errorf("completion script missing %q", subcommand)
			}
		}
	}
	if !strings.Contains(bashCompletion, "allow-partial") {
		t.Error("bash completion missing migrate export --allow-partial")
	}
}
