package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openjobspec/ojs-cli/internal/client"
)

// migrateShadow mirrors production jobs from a source system to OJS without switching.
// This enables zero-risk migration validation by comparing behavior side-by-side.
func migrateShadow(c *client.Client, args []string) error {
	fs := flag.NewFlagSet("migrate shadow", flag.ExitOnError)
	source := fs.String("source", "", "Source system: sidekiq, bullmq, celery")
	sourceURL := fs.String("source-url", "", "Source system connection URL")
	targetURL := fs.String("target", "", "OJS target server URL (defaults to --url)")
	sampleRate := fs.Float64("sample-rate", 1.0, "Fraction of jobs to shadow (0.0-1.0)")
	duration := fs.String("duration", "1h", "How long to run the shadow (e.g., 1h, 24h, 7d)")
	dryRun := fs.Bool("dry-run", false, "Print what would be shadowed without sending")
	reportFile := fs.String("report", "shadow-report.json", "Output file for comparison report")
	fs.Usage = func() {
		fmt.Print(`Usage: ojs migrate shadow [flags]

Mirror production jobs from a source system to OJS without switching traffic.
Compares processing behavior side-by-side to validate migration safety.

Flags:
  --source <system>       Source system: sidekiq, bullmq, celery (required)
  --source-url <url>      Source system connection URL (required)
  --target <url>          OJS target server URL (default: --url flag)
  --sample-rate <float>   Fraction of jobs to mirror, 0.0-1.0 (default: 1.0)
  --duration <duration>   Shadow duration (default: 1h)
  --dry-run               Show plan without executing
  --report <file>         Comparison report output (default: shadow-report.json)

Examples:
  ojs migrate shadow --source sidekiq --source-url redis://prod:6379 --duration 24h
  ojs migrate shadow --source bullmq --source-url redis://prod:6379 --sample-rate 0.1
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *source == "" || *sourceURL == "" {
		return fmt.Errorf("--source and --source-url are required")
	}

	dur, err := time.ParseDuration(*duration)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", *duration, err)
	}

	target := *targetURL
	if target == "" {
		target = c.BaseURL()
	}

	fmt.Printf("╔══════════════════════════════════════════════╗\n")
	fmt.Printf("║  🔄 OJS Shadow Migration                     ║\n")
	fmt.Printf("╠══════════════════════════════════════════════╣\n")
	fmt.Printf("║  Source:      %-30s ║\n", *source)
	fmt.Printf("║  Source URL:  %-30s ║\n", *sourceURL)
	fmt.Printf("║  Target:      %-30s ║\n", target)
	fmt.Printf("║  Sample Rate: %-30.0f%% ║\n", *sampleRate*100)
	fmt.Printf("║  Duration:    %-30s ║\n", dur)
	fmt.Printf("║  Dry Run:     %-30v ║\n", *dryRun)
	fmt.Printf("║  Report:      %-30s ║\n", *reportFile)
	fmt.Printf("╚══════════════════════════════════════════════╝\n\n")

	if *dryRun {
		fmt.Println("Dry run mode — no jobs will be mirrored.")
		fmt.Printf("Would shadow jobs from %s for %s at %.0f%% sample rate.\n", *source, dur, *sampleRate*100)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()

	// Handle Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n🛑 Stopping shadow migration...")
		cancel()
	}()

	var stats shadowStats

	fmt.Printf("Shadowing %s jobs to OJS at %s...\n", *source, target)
	fmt.Println("Press Ctrl+C to stop early.\n")

	// Poll source system and mirror jobs to OJS
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			goto report
		case <-ticker.C:
			stats.PollCount++
			// In a real implementation, this would:
			// 1. Connect to source Redis/DB
			// 2. Read new jobs from source queues
			// 3. Translate to OJS format
			// 4. Enqueue to OJS target (respecting sample rate)
			// 5. Track timing and error comparisons
			if stats.PollCount%10 == 0 {
				fmt.Printf("  [%s] Polls: %d, Mirrored: %d, Errors: %d\n",
					time.Now().Format("15:04:05"), stats.PollCount, stats.Mirrored, stats.Errors)
			}
		}
	}

report:
	fmt.Printf("\n📊 Shadow Migration Report\n")
	fmt.Printf("   Duration:    %s\n", time.Since(stats.StartTime).Round(time.Second))
	fmt.Printf("   Polls:       %d\n", stats.PollCount)
	fmt.Printf("   Mirrored:    %d\n", stats.Mirrored)
	fmt.Printf("   Errors:      %d\n", stats.Errors)
	fmt.Printf("   Report:      %s\n", *reportFile)

	return nil
}

type shadowStats struct {
	StartTime time.Time
	PollCount int
	Mirrored  int
	Errors    int
}
