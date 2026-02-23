package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateProjectDryRun(t *testing.T) {
	err := CreateProject([]string{"testapp", "--backend=redis", "--language=go", "--dry-run"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestCreateProjectAllLanguages(t *testing.T) {
	for _, lang := range []string{"go", "typescript", "python", "java", "rust", "ruby", "dotnet"} {
		err := CreateProject([]string{"test-" + lang, "--language=" + lang, "--dry-run"})
		if err != nil {
			t.Errorf("language %s: %v", lang, err)
		}
	}
}

func TestCreateProjectAllBackends(t *testing.T) {
	for _, backend := range []string{"redis", "postgres", "nats", "kafka", "sqs", "lite"} {
		err := CreateProject([]string{"test-" + backend, "--backend=" + backend, "--dry-run"})
		if err != nil {
			t.Errorf("backend %s: %v", backend, err)
		}
	}
}

func TestCreateProjectWriteFiles(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "myapp")

	err := CreateProject([]string{"myapp", "--backend=redis", "--language=go", "--output-dir=" + outputDir})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Check key files exist
	for _, path := range []string{"docker-compose.yml", ".env", "README.md", "go.mod", "cmd/worker/main.go"} {
		full := filepath.Join(outputDir, path)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			t.Errorf("missing file: %s", path)
		}
	}
}

func TestCreateProjectWithOptions(t *testing.T) {
	err := CreateProject([]string{
		"fullapp",
		"--backend=postgres",
		"--language=typescript",
		"--port=9090",
		"--queue=high",
		"--otel",
		"--ci-provider=gitlab",
		"--dry-run",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestCreateProjectNoName(t *testing.T) {
	err := CreateProject(nil)
	if err == nil {
		t.Error("expected error for no name")
	}
}

func TestCreateProjectInvalidBackend(t *testing.T) {
	err := CreateProject([]string{"app", "--backend=mongo", "--dry-run"})
	if err == nil {
		t.Error("expected error for invalid backend")
	}
}

func TestCreateProjectUnknownOption(t *testing.T) {
	err := CreateProject([]string{"app", "--unknown-option=x"})
	if err == nil {
		t.Error("expected error for unknown option")
	}
}
