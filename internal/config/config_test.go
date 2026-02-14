package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear env vars
	os.Unsetenv("OJS_URL")
	os.Unsetenv("OJS_AUTH_TOKEN")
	os.Unsetenv("OJS_OUTPUT")

	cfg := Load()
	if cfg.ServerURL != "http://localhost:8080" {
		t.Errorf("ServerURL = %q, want default", cfg.ServerURL)
	}
	if cfg.AuthToken != "" {
		t.Errorf("AuthToken = %q, want empty", cfg.AuthToken)
	}
	if cfg.Output != "table" {
		t.Errorf("Output = %q, want table", cfg.Output)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	os.Setenv("OJS_URL", "http://prod:9090")
	os.Setenv("OJS_AUTH_TOKEN", "secret")
	os.Setenv("OJS_OUTPUT", "json")
	defer func() {
		os.Unsetenv("OJS_URL")
		os.Unsetenv("OJS_AUTH_TOKEN")
		os.Unsetenv("OJS_OUTPUT")
	}()

	cfg := Load()
	if cfg.ServerURL != "http://prod:9090" {
		t.Errorf("ServerURL = %q, want http://prod:9090", cfg.ServerURL)
	}
	if cfg.AuthToken != "secret" {
		t.Errorf("AuthToken = %q, want secret", cfg.AuthToken)
	}
	if cfg.Output != "json" {
		t.Errorf("Output = %q, want json", cfg.Output)
	}
}

func TestBaseURL(t *testing.T) {
	cfg := &Config{ServerURL: "http://localhost:8080"}
	want := "http://localhost:8080/ojs/v1"
	if got := cfg.BaseURL(); got != want {
		t.Errorf("BaseURL() = %q, want %q", got, want)
	}
}
