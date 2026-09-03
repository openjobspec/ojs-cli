package config

import "testing"

func TestLoad_Defaults(t *testing.T) {
	// Clear env vars
	t.Setenv("OJS_URL", "")
	t.Setenv("OJS_AUTH_TOKEN", "")
	t.Setenv("OJS_OUTPUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
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
	t.Setenv("OJS_URL", "http://prod:9090")
	t.Setenv("OJS_AUTH_TOKEN", "secret")
	t.Setenv("OJS_OUTPUT", "json")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
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

func TestLoadRejectsUnsupportedOutput(t *testing.T) {
	t.Setenv("OJS_OUTPUT", "yaml")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want unsupported output error")
	}
}

func TestBaseURL(t *testing.T) {
	cfg := &Config{ServerURL: "http://localhost:8080"}
	want := "http://localhost:8080/ojs/v1"
	if got := cfg.BaseURL(); got != want {
		t.Errorf("BaseURL() = %q, want %q", got, want)
	}
}
