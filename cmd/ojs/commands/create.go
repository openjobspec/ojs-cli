package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openjobspec/ojs-go-backend-common/scaffold"
)

// CreateProject scaffolds a new OJS project.
//
// Usage:
//
//	ojs create <name> --backend=redis --language=go [options]
//
// Options:
//
//	--backend    Backend type: redis, postgres, nats, kafka, sqs, lite (default: redis)
//	--language   SDK language: go, typescript, python, java, rust, ruby, dotnet (default: go)
//	--queue      Default queue name (default: "default")
//	--port       Server port (default: 8080)
//	--otel       Enable OpenTelemetry (default: false)
//	--docker     Generate Dockerfile (default: true)
//	--ci         Generate CI pipeline (default: true)
//	--ci-provider  CI provider: github, gitlab (default: github)
//	--module     Go module path (Go only, default: github.com/example/<name>)
//	--output-dir Output directory (default: ./<name>)
//	--dry-run    Print files without writing (default: false)
func CreateProject(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ojs create <project-name> [options]\n\nExample:\n  ojs create myapp --backend=redis --language=typescript")
	}

	name := args[0]
	cfg := scaffold.ProjectConfig{
		Name:         name,
		Backend:      scaffold.BackendRedis,
		Language:     scaffold.LangGo,
		Port:         8080,
		EnableDocker: true,
		EnableCI:     true,
		CIProvider:   "github",
	}

	var outputDir string
	var dryRun bool

	for _, arg := range args[1:] {
		key, value, _ := strings.Cut(arg, "=")
		key = strings.TrimPrefix(key, "--")

		switch key {
		case "backend":
			cfg.Backend = scaffold.Backend(value)
		case "language", "lang":
			cfg.Language = scaffold.Language(value)
		case "queue":
			cfg.Queue = value
		case "port":
			fmt.Sscanf(value, "%d", &cfg.Port)
		case "otel":
			cfg.EnableOTel = value == "" || value == "true"
		case "docker":
			cfg.EnableDocker = value != "false"
		case "ci":
			cfg.EnableCI = value != "false"
		case "ci-provider":
			cfg.CIProvider = value
		case "module":
			cfg.ModulePath = value
		case "output-dir", "output", "dir":
			outputDir = value
		case "dry-run":
			dryRun = value == "" || value == "true"
		default:
			return fmt.Errorf("unknown option: --%s\n\nSupported backends: %s\nSupported languages: %s",
				key,
				strings.Join(backendNames(), ", "),
				strings.Join(languageNames(), ", "))
		}
	}

	if outputDir == "" {
		outputDir = name
	}

	files, err := scaffold.Generate(cfg)
	if err != nil {
		return fmt.Errorf("scaffold error: %w", err)
	}

	if dryRun {
		fmt.Printf("Would create %d files in %s/:\n\n", len(files), outputDir)
		for _, f := range files {
			fmt.Printf("  📄 %s (%d bytes)\n", f.Path, len(f.Content))
		}
		return nil
	}

	// Write files
	created := 0
	for _, f := range files {
		fullPath := filepath.Join(outputDir, f.Path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(f.Content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", fullPath, err)
		}
		created++
	}

	fmt.Printf("✅ Created %d files in %s/\n\n", created, outputDir)
	fmt.Printf("Next steps:\n")
	fmt.Printf("  cd %s\n", outputDir)
	fmt.Printf("  docker compose up -d\n")

	switch cfg.Language {
	case scaffold.LangGo:
		fmt.Printf("  go run ./cmd/worker\n")
	case scaffold.LangTypeScript:
		fmt.Printf("  npm install && npm run worker\n")
	case scaffold.LangPython:
		fmt.Printf("  pip install -e . && python -m %s.worker\n", name)
	case scaffold.LangJava:
		fmt.Printf("  mvn package && java -jar target/%s.jar\n", name)
	case scaffold.LangRust:
		fmt.Printf("  cargo run --bin worker\n")
	case scaffold.LangRuby:
		fmt.Printf("  bundle install && ruby worker.rb\n")
	case scaffold.LangDotNet:
		fmt.Printf("  dotnet run --project Worker\n")
	}

	return nil
}

// CreateProjectJSON generates scaffold output as JSON (for programmatic use).
func CreateProjectJSON(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ojs create --json <project-name> [options]")
	}

	name := args[0]
	cfg := scaffold.ProjectConfig{
		Name:         name,
		Backend:      scaffold.BackendRedis,
		Language:     scaffold.LangGo,
		Port:         8080,
		EnableDocker: true,
		EnableCI:     true,
	}

	for _, arg := range args[1:] {
		key, value, _ := strings.Cut(arg, "=")
		key = strings.TrimPrefix(key, "--")
		switch key {
		case "backend":
			cfg.Backend = scaffold.Backend(value)
		case "language", "lang":
			cfg.Language = scaffold.Language(value)
		}
	}

	files, err := scaffold.Generate(cfg)
	if err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(files)
}

func backendNames() []string {
	var names []string
	for _, b := range scaffold.SupportedBackends() {
		names = append(names, string(b))
	}
	return names
}

func languageNames() []string {
	var names []string
	for _, l := range scaffold.SupportedLanguages() {
		names = append(names, string(l))
	}
	return names
}
