package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/openjobspec/ojs-cli/internal/client"
)

// Monitor provides a live monitoring dashboard.
func Monitor(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("monitor", flag.ContinueOnError)
	interval := fs.Duration("interval", 2*time.Second, "Refresh interval")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	if *interval <= 0 {
		return fmt.Errorf("--interval must be greater than zero")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

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
	return renderDashboardTo(c, os.Stdout, isTerminal(os.Stdout))
}

func renderDashboardTo(c *client.Client, writer io.Writer, clear bool) error {
	if clear {
		fmt.Fprint(writer, "\033[2J\033[H")
	}

	fmt.Fprintln(writer, "╔══════════════════════════════════════════════════╗")
	fmt.Fprintln(writer, "║           OJS Monitor — Live Dashboard           ║")
	fmt.Fprintf(writer, "║  %s  ║\n", time.Now().Format("2006-01-02 15:04:05          "))
	fmt.Fprintln(writer, "╚══════════════════════════════════════════════════╝")
	fmt.Fprintln(writer)

	healthData, _, err := c.Get("/health")
	if err != nil {
		fmt.Fprintf(writer, "  Server: ❌ unreachable (%v)\n\n", err)
		return nil
	}

	var health map[string]any
	if err := json.Unmarshal(healthData, &health); err != nil {
		return fmt.Errorf("decode health response: %w", err)
	}
	status := "✅"
	if str(health["status"]) != "ok" {
		status = "⚠️"
	}
	fmt.Fprintf(writer, "  Server: %s %s (v%s, uptime: %ss)\n\n",
		status, str(health["status"]), str(health["version"]), str(health["uptime_seconds"]))

	if err := renderQueueDashboard(c, writer); err != nil {
		return err
	}
	if err := renderDeadLetterSummary(c, writer); err != nil {
		return err
	}

	fmt.Fprintln(writer, "  Press Ctrl+C to exit")
	return nil
}

func renderQueueDashboard(c *client.Client, writer io.Writer) error {
	queuesData, _, err := c.Get("/queues")
	if err != nil {
		fmt.Fprintf(writer, "  Queues: error loading (%v)\n\n", err)
		return nil
	}

	var response struct {
		Queues []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"queues"`
	}
	if err := json.Unmarshal(queuesData, &response); err != nil {
		return fmt.Errorf("decode queues response: %w", err)
	}

	fmt.Fprintf(writer, "  Queues (%d):\n", len(response.Queues))
	fmt.Fprintf(writer, "  %-20s %-10s %8s %8s %8s %8s %8s\n",
		"NAME", "STATUS", "AVAIL", "ACTIVE", "SCHED", "RETRY", "DEAD")
	fmt.Fprintf(writer, "  %s\n", strings.Repeat("─", 78))

	for _, queue := range response.Queues {
		statsData, _, err := c.Get("/queues/" + queue.Name + "/stats")
		if err != nil {
			fmt.Fprintf(writer, "  %-20s %-10s %8s\n", queue.Name, queue.Status, "err")
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
		if err := json.Unmarshal(statsData, &stats); err != nil {
			return fmt.Errorf("decode stats for queue %s: %w", queue.Name, err)
		}
		fmt.Fprintf(writer, "  %-20s %-10s %8d %8d %8d %8d %8d\n",
			queue.Name, queue.Status,
			stats.Stats.Available, stats.Stats.Active,
			stats.Stats.Scheduled, stats.Stats.Retryable, stats.Stats.Dead)
	}
	fmt.Fprintln(writer)
	return nil
}

func renderDeadLetterSummary(c *client.Client, writer io.Writer) error {
	data, _, err := c.Get("/dead-letter?limit=1")
	if err != nil {
		return nil
	}
	var response struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("decode dead-letter response: %w", err)
	}
	if response.Total > 0 {
		fmt.Fprintf(writer, "  ⚠ Dead letter jobs: %d\n\n", response.Total)
	}
	return nil
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
