package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupObservabilityGeneratesValidPrometheusTarget(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "monitoring")
	err := setupObservability([]string{
		"--output-dir", outputDir,
		"--ojs-url", "https://jobs.example:8443",
		"--prometheus-url", "http://prometheus.internal:9090",
	})
	if err != nil {
		t.Fatal(err)
	}

	prometheusPath := filepath.Join(outputDir, "prometheus", "prometheus.yml")
	data, err := os.ReadFile(prometheusPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{
		`scheme: "https"`,
		`- "jobs.example:8443"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("prometheus.yml missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "https://jobs.example:8443") {
		t.Fatalf("Prometheus target incorrectly includes URL scheme:\n%s", content)
	}

	for _, artifact := range observabilityArtifacts(outputDir) {
		if _, err := os.Stat(artifact.path); err != nil {
			t.Errorf("generated artifact %s: %v", artifact.path, err)
		}
	}
}

func TestSetupObservabilityRejectsInvalidOJSURL(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "monitoring")
	err := setupObservability([]string{"--output-dir", outputDir, "--ojs-url", "not-a-url"})
	if err == nil {
		t.Fatal("setupObservability() error = nil, want invalid URL error")
	}
}
