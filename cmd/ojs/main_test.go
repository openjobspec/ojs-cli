package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openjobspec/ojs-cli/internal/config"
	"github.com/openjobspec/ojs-cli/internal/output"
)

func TestParseGlobalArgsJSONOverridesEnvironment(t *testing.T) {
	cfg := &config.Config{ServerURL: "http://localhost:8080", Output: "table"}
	options, err := parseGlobalArgs([]string{"health", "--json"}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !options.json {
		t.Fatal("--json was not recorded")
	}
	if len(options.args) != 1 || options.args[0] != "health" {
		t.Fatalf("clean args = %v, want [health]", options.args)
	}
}

func TestParseGlobalArgsPreservesJSONEnvironmentWithoutFlag(t *testing.T) {
	cfg := &config.Config{ServerURL: "http://localhost:8080", Output: "json"}
	options, err := parseGlobalArgs([]string{"health"}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if options.json {
		t.Fatal("JSON flag unexpectedly set")
	}
	if cfg.Output != "json" {
		t.Fatalf("cfg.Output = %q, want json", cfg.Output)
	}
}

func TestParseGlobalArgsURLAndJSONPrecedence(t *testing.T) {
	cfg := &config.Config{ServerURL: "http://env.example", Output: "table"}
	options, err := parseGlobalArgs(
		[]string{"--url", "http://flag.example", "--json", "health"},
		cfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerURL != "http://flag.example" {
		t.Fatalf("ServerURL = %q, want flag value", cfg.ServerURL)
	}
	if !options.json {
		t.Fatal("--json should take precedence over table environment")
	}
}

func TestParseGlobalArgsMissingURLValue(t *testing.T) {
	cfg := &config.Config{}
	if _, err := parseGlobalArgs([]string{"health", "--url"}, cfg); err == nil {
		t.Fatal("missing --url value should fail")
	}
}

func TestRunInitializesOutputFromEnvironmentAndJSONFlagWins(t *testing.T) {
	previous := output.Format
	t.Cleanup(func() { output.Format = previous })

	t.Setenv("OJS_OUTPUT", "json")
	if err := run([]string{"--version"}); err != nil {
		t.Fatal(err)
	}
	if output.Format != "json" {
		t.Fatalf("environment output format = %q, want json", output.Format)
	}

	t.Setenv("OJS_OUTPUT", "table")
	if err := run([]string{"--json", "--version"}); err != nil {
		t.Fatal(err)
	}
	if output.Format != "json" {
		t.Fatalf("--json output format = %q, want json", output.Format)
	}
}

func TestRunRejectsUnsupportedEnvironmentOutput(t *testing.T) {
	t.Setenv("OJS_OUTPUT", "yaml")
	if err := run([]string{"--version"}); err == nil {
		t.Fatal("run() error = nil, want unsupported OJS_OUTPUT error")
	}
}

func TestRunHelpUnknownAndHealthCommand(t *testing.T) {
	t.Setenv("OJS_OUTPUT", "table")
	if err := run([]string{"--help"}); err != nil {
		t.Fatalf("help: %v", err)
	}
	if err := run(nil); !errors.Is(err, errAlreadyReported) {
		t.Fatalf("empty args error = %v, want errAlreadyReported", err)
	}
	if err := run([]string{"not-a-command"}); !errors.Is(err, errAlreadyReported) {
		t.Fatalf("unknown command error = %v, want errAlreadyReported", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ojs/v1/health" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "version": "1"})
	}))
	defer server.Close()
	if err := run([]string{"health", "--url", server.URL, "--json"}); err != nil {
		t.Fatalf("health command: %v", err)
	}
}
