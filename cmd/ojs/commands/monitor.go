package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/openjobspec/ojs-cli/internal/client"
)

// Monitor provides a live monitoring dashboard.
func Monitor(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	interval := fs.Duration("interval", 2*time.Second, "Refresh interval")
	fs.Parse(args)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	// Initial render
	if err := renderDashboard(c); err != nil {
		return err
	}

	for {
		select {
		case <-ticker.C:
			if err := renderDashboard(c); err != nil {
				fmt.Fprintf(os.Stderr, "⚠ refresh error: %v\n", err)
			}
		case <-sigCh:
			fmt.Println("\n\nMonitor stopped.")
			return nil
		}
	}
}

func renderDashboard(c *client.Client) error {
	// Clear screen
	fmt.Print("\033[2J\033[H")

	// Header
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║           OJS Monitor — Live Dashboard           ║")
	fmt.Printf("║  %s  ║\n", time.Now().Format("2006-01-02 15:04:05          "))
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()

	// Health
	healthData, _, err := c.Get("/health")
	if err != nil {
		fmt.Printf("  Server: ❌ unreachable (%v)\n\n", err)
		return nil
	}

	var health map[string]any
	json.Unmarshal(healthData, &health)
	status := "✅"
	if str(health["status"]) != "ok" {
		status = "⚠️"
	}
	fmt.Printf("  Server: %s %s (v%s, uptime: %ss)\n\n",
		status, str(health["status"]), str(health["version"]), str(health["uptime_seconds"]))

	// Queues
	queuesData, _, err := c.Get("/queues")
	if err != nil {
		fmt.Printf("  Queues: error loading (%v)\n\n", err)
	} else {
		var qResp struct {
			Queues []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"queues"`
		}
		json.Unmarshal(queuesData, &qResp)

		fmt.Printf("  Queues (%d):\n", len(qResp.Queues))
		fmt.Printf("  %-20s %-10s %8s %8s %8s %8s %8s\n",
			"NAME", "STATUS", "AVAIL", "ACTIVE", "SCHED", "RETRY", "DEAD")
		fmt.Printf("  %s\n", strings.Repeat("─", 78))

		for _, q := range qResp.Queues {
			statsData, _, err := c.Get("/queues/" + q.Name + "/stats")
			if err != nil {
				fmt.Printf("  %-20s %-10s %8s\n", q.Name, q.Status, "err")
				continue
			}
			var stats struct {
				Stats struct {
					Available int `json:"available"`
					Active    int `json:"active"`
					Scheduled int `json:"scheduled"`
					Retryable int `json:"retryable"`
					Dead      int `json:"dead"`
				} `json:"stats"`
			}
			json.Unmarshal(statsData, &stats)
			fmt.Printf("  %-20s %-10s %8d %8d %8d %8d %8d\n",
				q.Name, q.Status,
				stats.Stats.Available, stats.Stats.Active,
				stats.Stats.Scheduled, stats.Stats.Retryable, stats.Stats.Dead)
		}
		fmt.Println()
	}

	// Dead letter summary
	dlData, _, err := c.Get("/dead-letter?limit=1")
	if err == nil {
		var dlResp struct {
			Total int `json:"total"`
		}
		json.Unmarshal(dlData, &dlResp)
		if dlResp.Total > 0 {
			fmt.Printf("  ⚠ Dead letter jobs: %d\n\n", dlResp.Total)
		}
	}

	fmt.Println("  Press Ctrl+C to exit")
	return nil
}
