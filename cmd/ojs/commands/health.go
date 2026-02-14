package commands

import (
	"encoding/json"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/client"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Health checks the server health.
func Health(c *client.Client, args []string) error {
	data, _, err := c.Get("/health")
	if err != nil {
		return fmt.Errorf("server health check failed: %w", err)
	}

	if output.Format == "json" {
		var result any
		json.Unmarshal(data, &result)
		return output.JSON(result)
	}

	var health map[string]any
	json.Unmarshal(data, &health)

	status := str(health["status"])
	version := str(health["version"])
	uptime := str(health["uptime_seconds"])

	backend, _ := health["backend"].(map[string]any)
	backendType := "-"
	backendStatus := "-"
	if backend != nil {
		backendType = str(backend["type"])
		backendStatus = str(backend["status"])
	}

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"Status", status},
		{"Version", version},
		{"Uptime", uptime + "s"},
		{"Backend", backendType},
		{"Backend Status", backendStatus},
	}
	output.Table(headers, rows)
	return nil
}
