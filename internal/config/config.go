package config

import (
	"fmt"
	"os"
)

// Config holds CLI configuration.
type Config struct {
	ServerURL string
	AuthToken string
	Output    string // "table", "json"
}

// Load reads and validates configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		ServerURL: "http://localhost:8080",
		Output:    "table",
	}

	if url := os.Getenv("OJS_URL"); url != "" {
		cfg.ServerURL = url
	}
	if token := os.Getenv("OJS_AUTH_TOKEN"); token != "" {
		cfg.AuthToken = token
	}
	if output := os.Getenv("OJS_OUTPUT"); output != "" {
		cfg.Output = output
	}

	if err := ValidateOutput(cfg.Output); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ValidateOutput rejects output modes the renderer does not support.
func ValidateOutput(format string) error {
	switch format {
	case "table", "json":
		return nil
	default:
		return fmt.Errorf("unsupported output format %q (expected table or json)", format)
	}
}

// BaseURL returns the API base URL.
func (c *Config) BaseURL() string {
	return fmt.Sprintf("%s/ojs/v1", c.ServerURL)
}
