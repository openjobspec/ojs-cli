package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifestYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobs.yaml")
	os.WriteFile(path, []byte(`
version: "1.0"
package: myjobs
job_types:
  - type: email.send
    description: Send an email
    queue: email
    args:
      - name: to
        type: string
        required: true
      - name: subject
        type: string
        required: true
      - name: body
        type: string
        required: true
    retry:
      max_attempts: 3
      backoff: exponential
      initial_ms: 1000
  - type: image.resize
    description: Resize an image
    queue: media
    args:
      - name: url
        type: string
        required: true
      - name: width
        type: int
        required: true
      - name: height
        type: int
        required: true
`), 0o644)

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(m.JobTypes) != 2 {
		t.Fatalf("expected 2 job types, got %d", len(m.JobTypes))
	}
	if m.JobTypes[0].Type != "email.send" {
		t.Errorf("expected email.send, got %s", m.JobTypes[0].Type)
	}
	if len(m.JobTypes[0].Args) != 3 {
		t.Errorf("expected 3 args, got %d", len(m.JobTypes[0].Args))
	}
}

func TestLoadManifestValidation(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "empty job types",
			content: "version: '1.0'\njob_types: []\n",
			wantErr: "no job types",
		},
		{
			name:    "missing type",
			content: "version: '1.0'\njob_types:\n  - queue: default\n",
			wantErr: "type is required",
		},
		{
			name:    "duplicate type",
			content: "version: '1.0'\njob_types:\n  - type: foo\n  - type: foo\n",
			wantErr: "duplicate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.name+".yaml")
			os.WriteFile(path, []byte(tt.content), 0o644)
			_, err := LoadManifest(path)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestGenerateGo(t *testing.T) {
	m := &Manifest{
		Package: "myjobs",
		JobTypes: []JobTypeDef{
			{
				Type:        "email.send",
				Description: "Send an email",
				Queue:       "email",
				Args: []ArgDef{
					{Name: "to", Type: "string", Required: true},
					{Name: "subject", Type: "string", Required: true},
				},
			},
		},
	}

	dir := t.TempDir()
	gen := NewGenerator(m, LangGo, dir)
	if err := gen.Generate(); err != nil {
		t.Fatalf("Generate Go: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "ojs_generated.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	code := string(content)
	if !strings.Contains(code, "EmailSendArgs") {
		t.Error("expected EmailSendArgs struct")
	}
	if !strings.Contains(code, "EnqueueEmailSend") {
		t.Error("expected EnqueueEmailSend function")
	}
	if !strings.Contains(code, "package myjobs") {
		t.Error("expected package myjobs")
	}
}

func TestGenerateTypeScript(t *testing.T) {
	m := &Manifest{
		Package: "ojs",
		JobTypes: []JobTypeDef{
			{
				Type:  "image.resize",
				Queue: "media",
				Args: []ArgDef{
					{Name: "url", Type: "string", Required: true},
					{Name: "width", Type: "int", Required: true},
				},
			},
		},
	}

	dir := t.TempDir()
	gen := NewGenerator(m, LangTypeScript, dir)
	if err := gen.Generate(); err != nil {
		t.Fatalf("Generate TS: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "ojs-generated.ts"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	code := string(content)
	if !strings.Contains(code, "ImageResizeArgs") {
		t.Error("expected ImageResizeArgs interface")
	}
	if !strings.Contains(code, "enqueueImageResize") {
		t.Error("expected enqueueImageResize function")
	}
}

func TestGeneratePython(t *testing.T) {
	m := &Manifest{
		Package: "ojs",
		JobTypes: []JobTypeDef{
			{
				Type:  "data.process",
				Queue: "default",
				Args: []ArgDef{
					{Name: "file_path", Type: "string", Required: true},
					{Name: "batch_size", Type: "int", Required: true},
				},
			},
		},
	}

	dir := t.TempDir()
	gen := NewGenerator(m, LangPython, dir)
	if err := gen.Generate(); err != nil {
		t.Fatalf("Generate Python: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "ojs_generated.py"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	code := string(content)
	if !strings.Contains(code, "DataProcessArgs") {
		t.Error("expected DataProcessArgs dataclass")
	}
	if !strings.Contains(code, "enqueue_data_process") {
		t.Error("expected enqueue_data_process function")
	}
}

func TestNamingHelpers(t *testing.T) {
	tests := []struct {
		input  string
		pascal string
		camel  string
		snake  string
	}{
		{"email.send", "EmailSend", "emailSend", "email_send"},
		{"image-resize", "ImageResize", "imageResize", "image_resize"},
		{"data_process", "DataProcess", "dataProcess", "data_process"},
	}

	for _, tt := range tests {
		if got := toPascalCase(tt.input); got != tt.pascal {
			t.Errorf("pascalCase(%q) = %q, want %q", tt.input, got, tt.pascal)
		}
		if got := toCamelCase(tt.input); got != tt.camel {
			t.Errorf("camelCase(%q) = %q, want %q", tt.input, got, tt.camel)
		}
		if got := toSnakeCase(tt.input); got != tt.snake {
			t.Errorf("snakeCase(%q) = %q, want %q", tt.input, got, tt.snake)
		}
	}
}
