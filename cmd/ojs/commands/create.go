package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/openjobspec/ojs-cli/internal/fileutil"
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
	cfg := defaultProjectConfig(name)
	outputDir, dryRun, err := parseCreateOptions(&cfg, args[1:])
	if err != nil {
		return err
	}
	if outputDir == "" {
		outputDir = name
	}

	files, err := scaffold.Generate(cfg)
	if err != nil {
		return fmt.Errorf("scaffold error: %w", err)
	}
	if dryRun {
		printCreateDryRun(files, outputDir)
		return nil
	}
	if err := writeProjectFiles(files, outputDir); err != nil {
		return err
	}

	fmt.Printf("✅ Created %d files in %s/\n\n", len(files), outputDir)
	printCreateNextSteps(cfg.Language, name, outputDir)
	return nil
}

func defaultProjectConfig(name string) scaffold.ProjectConfig {
	return scaffold.ProjectConfig{
		Name:         name,
		Backend:      scaffold.BackendRedis,
		Language:     scaffold.LangGo,
		Port:         8080,
		EnableDocker: true,
		EnableCI:     true,
		CIProvider:   "github",
	}
}

func parseCreateOptions(cfg *scaffold.ProjectConfig, args []string) (string, bool, error) {
	outputDir := ""
	dryRun := false
	simpleHandlers := map[string]func(string){
		"backend": func(value string) {
			cfg.Backend = scaffold.Backend(value)
		},
		"language": func(value string) {
			cfg.Language = scaffold.Language(value)
		},
		"queue": func(value string) {
			cfg.Queue = value
		},
		"ci-provider": func(value string) {
			cfg.CIProvider = value
		},
		"module": func(value string) {
			cfg.ModulePath = value
		},
		"output-dir": func(value string) {
			outputDir = value
		},
	}
	validatedHandlers := map[string]func(string) error{
		"port": func(value string) error {
			port, err := strconv.Atoi(value)
			if err != nil || port < 1 || port > 65535 {
				return fmt.Errorf("invalid --port %q (expected 1-65535)", value)
			}
			cfg.Port = port
			return nil
		},
		"otel": func(value string) error {
			enabled, err := parseOptionalBool(value)
			cfg.EnableOTel = enabled
			return err
		},
		"docker": func(value string) error {
			enabled, err := parseOptionalBool(value)
			cfg.EnableDocker = enabled
			return err
		},
		"ci": func(value string) error {
			enabled, err := parseOptionalBool(value)
			cfg.EnableCI = enabled
			return err
		},
		"dry-run": func(value string) error {
			enabled, err := parseOptionalBool(value)
			dryRun = enabled
			return err
		},
	}

	for _, arg := range args {
		key, value, _ := strings.Cut(arg, "=")
		key = strings.TrimPrefix(key, "--")
		switch key {
		case "lang":
			key = "language"
		case "output", "dir":
			key = "output-dir"
		}
		if handler, ok := simpleHandlers[key]; ok {
			handler(value)
			continue
		}
		if handler, ok := validatedHandlers[key]; ok {
			if err := handler(value); err != nil {
				return "", false, err
			}
			continue
		}
		return "", false, fmt.Errorf("unknown option: --%s\n\nSupported backends: %s\nSupported languages: %s",
			key,
			strings.Join(backendNames(), ", "),
			strings.Join(languageNames(), ", "))
	}
	return outputDir, dryRun, nil
}

func parseOptionalBool(value string) (bool, error) {
	switch value {
	case "", "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q (expected true or false)", value)
	}
}

func printCreateDryRun(files []scaffold.GeneratedFile, outputDir string) {
	fmt.Printf("Would create %d files in %s/:\n\n", len(files), outputDir)
	for i := range files {
		fmt.Printf("  📄 %s (%d bytes)\n", files[i].Path, len(files[i].Content))
	}
}

func writeProjectFiles(files []scaffold.GeneratedFile, outputDir string) error {
	for i := range files {
		fullPath := filepath.Join(outputDir, files[i].Path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			printPartialProjectWarning(i, len(files), outputDir)
			return fmt.Errorf("creating directory %s: %w\n\nHint: check directory permissions and available disk space", dir, err)
		}
		err := fileutil.WriteAtomic(fullPath, 0o644, func(writer io.Writer) error {
			_, err := io.WriteString(writer, files[i].Content)
			return err
		})
		if err != nil {
			printPartialProjectWarning(i, len(files), outputDir)
			return fmt.Errorf("writing %s: %w\n\nHint: check available disk space and file permissions", fullPath, err)
		}
	}
	return nil
}

func printPartialProjectWarning(created, total int, outputDir string) {
	if created > 0 {
		fmt.Fprintf(os.Stderr, "⚠️  Partial project created (%d/%d files). Clean up with: rm -rf %s\n", created, total, outputDir)
	}
}

func printCreateNextSteps(language scaffold.Language, name, outputDir string) {
	fmt.Printf("Next steps:\n")
	fmt.Printf("  cd %s\n", outputDir)
	fmt.Printf("  docker compose up -d\n")

	switch language {
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
