package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/openjobspec/ojs-cli/internal/config"
	"github.com/openjobspec/ojs-cli/internal/output"
)

const maxSSEEventLineBytes = 1 << 20

// Events streams server-sent events from the OJS server.
func Events(cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("events", flag.ContinueOnError)
	follow := fs.Bool("follow", true, "Stream events continuously")
	types := fs.String("types", "", "Filter by event types (comma-separated)")
	queue := fs.String("queue", "", "Filter by queue name")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	query := make(url.Values)
	if *types != "" {
		query.Set("types", *types)
	}
	if *queue != "" {
		query.Set("queue", *queue)
	}
	endpoint := cfg.ServerURL + "/ojs/v1/events/stream"
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	if cfg.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.AuthToken)
	}

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil
		}
		return fmt.Errorf("connect to event stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("event stream returned HTTP %d", resp.StatusCode)
	}

	if !*follow {
		fmt.Println("Streaming events (press Ctrl+C to stop)...")
	} else {
		fmt.Println("Following events (press Ctrl+C to stop)...")
	}
	fmt.Println()

	err = scanSSE(ctx, resp.Body, renderSSEData)
	switch {
	case errors.Is(err, context.Canceled):
		fmt.Println("\nEvent stream stopped.")
		return nil
	case err != nil:
		return fmt.Errorf("read event stream: %w", err)
	default:
		fmt.Println("\nEvent stream closed.")
		return nil
	}
}

func scanSSE(ctx context.Context, reader io.Reader, handle func(string) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), maxSSEEventLineBytes)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if err := handle(data); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func renderSSEData(data string) error {
	if output.Format == "json" {
		fmt.Println(data)
		return nil
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		fmt.Println(data)
		return nil
	}
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("[%s] %s: %s (job=%s, queue=%s)\n",
		timestamp, str(event["type"]), str(event["event"]),
		str(event["job_id"]), str(event["queue"]))
	return nil
}
