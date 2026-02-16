package commands

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/openjobspec/ojs-cli/internal/config"
	"github.com/openjobspec/ojs-cli/internal/output"
)

// Events streams server-sent events from the OJS server.
func Events(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("events", flag.ExitOnError)
	follow := fs.Bool("follow", true, "Stream events continuously")
	types := fs.String("types", "", "Filter by event types (comma-separated)")
	queue := fs.String("queue", "", "Filter by queue name")
	fs.Parse(args)

	path := "/ojs/v1/events/stream"
	params := []string{}
	if *types != "" {
		params = append(params, "types="+*types)
	}
	if *queue != "" {
		params = append(params, "queue="+*queue)
	}
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}

	url := cfg.ServerURL + path

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	if cfg.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.AuthToken)
	}

	httpClient := &http.Client{
		Timeout: 0, // no timeout for SSE
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connect to event stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("event stream returned HTTP %d", resp.StatusCode)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	if !*follow {
		fmt.Println("Streaming events (press Ctrl+C to stop)...")
	} else {
		fmt.Println("Following events (press Ctrl+C to stop)...")
	}
	fmt.Println()

	scanner := bufio.NewScanner(resp.Body)
	eventCh := make(chan string, 1)

	go func() {
		for scanner.Scan() {
			eventCh <- scanner.Text()
		}
		close(eventCh)
	}()

	for {
		select {
		case line, ok := <-eventCh:
			if !ok {
				fmt.Println("\nEvent stream closed.")
				return nil
			}
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimPrefix(line, "data:")
				data = strings.TrimSpace(data)
				if output.Format == "json" {
					fmt.Println(data)
				} else {
					var event map[string]any
					if json.Unmarshal([]byte(data), &event) == nil {
						ts := time.Now().Format("15:04:05")
						fmt.Printf("[%s] %s: %s (job=%s, queue=%s)\n",
							ts, str(event["type"]), str(event["event"]),
							str(event["job_id"]), str(event["queue"]))
					} else {
						fmt.Println(data)
					}
				}
			}
		case <-sigCh:
			fmt.Println("\nEvent stream stopped.")
			return nil
		}
	}
}
