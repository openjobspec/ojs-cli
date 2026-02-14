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

// Load reads configuration from environment variables and flags.
func Load() *Config {
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

	return cfg
}

// BaseURL returns the API base URL.
func (c *Config) BaseURL() string {
	return fmt.Sprintf("%s/ojs/v1", c.ServerURL)
}
