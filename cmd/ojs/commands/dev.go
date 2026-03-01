package commands

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// Dev starts a local development environment using the Lite backend.
func Dev(args []string) error {
	fs := flag.NewFlagSet("dev", flag.ExitOnError)
	port := fs.Int("port", 8080, "HTTP port")
	grpcPort := fs.Int("grpc", 9090, "gRPC port")
	verbose := fs.Bool("verbose", false, "Show server logs")
	fs.Usage = func() {
		fmt.Print(`Usage: ojs dev [flags]

Start a local OJS development server using the Lite (in-memory) backend.
No external dependencies required.

Flags:
  --port <port>    HTTP port (default: 8080)
  --grpc <port>    gRPC port (default: 9090)
  --verbose        Show server logs
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	binary := findLiteBackend()
	if binary == "" {
		return fmt.Errorf("could not find ojs-backend-lite binary.\nBuild it first: cd ojs-backend-lite && make build")
	}

	env := append(os.Environ(),
		fmt.Sprintf("PORT=%d", *port),
		fmt.Sprintf("GRPC_PORT=%d", *grpcPort),
		"OJS_ALLOW_INSECURE_NO_AUTH=true",
		"OJS_LOG_LEVEL=info",
	)

	cmd := exec.Command(binary)
	cmd.Env = env
	if *verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start lite backend: %w", err)
	}

	// Wait for server to be ready
	ready := waitForReady(fmt.Sprintf("http://localhost:%d/healthz", *port), 5*time.Second)
	if !ready {
		fmt.Fprintf(os.Stderr, "⚠ Server may still be starting...\n")
	}

	fmt.Printf(`
╔══════════════════════════════════════════════╗
║  🚀 OJS Development Server                  ║
║                                              ║
║  API:       http://localhost:%d/ojs/v1/     ║
║  Admin UI:  http://localhost:%d/ojs/admin/  ║
║  Health:    http://localhost:%d/healthz     ║
║  gRPC:      localhost:%d                    ║
║                                              ║
║  Backend:   Lite (in-memory, no persistence) ║
║  Auth:      disabled (dev mode)              ║
║                                              ║
║  Press Ctrl+C to stop                        ║
╚══════════════════════════════════════════════╝
`, *port, *port, *port, *grpcPort)

	// Wait for interrupt
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("\n🛑 Shutting down...")
	if cmd.Process != nil {
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()
	}
	return nil
}

func findLiteBackend() string {
	candidates := []string{
		"ojs-backend-lite/bin/ojs-server",
		"../ojs-backend-lite/bin/ojs-server",
		filepath.Join(os.Getenv("GOPATH"), "bin", "ojs-server"),
	}

	cwd, _ := os.Getwd()
	for _, c := range candidates {
		abs := c
		if !filepath.IsAbs(c) {
			abs = filepath.Join(cwd, c)
		}
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}

	// Try PATH
	if p, err := exec.LookPath("ojs-server"); err == nil {
		return p
	}

	return ""
}

func waitForReady(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}
