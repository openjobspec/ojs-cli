package codegen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

// Language represents a target language for code generation.
type Language string

const (
	LangGo         Language = "go"
	LangTypeScript Language = "typescript"
	LangPython     Language = "python"
)

// Generator produces SDK code from job type definitions.
type Generator struct {
	manifest *Manifest
	language Language
	outDir   string
}

// NewGenerator creates a code generator for the given manifest and language.
func NewGenerator(manifest *Manifest, language Language, outDir string) *Generator {
	return &Generator{manifest: manifest, language: language, outDir: outDir}
}

// Generate writes generated code to the output directory.
func (g *Generator) Generate() error {
	if err := os.MkdirAll(g.outDir, 0o755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	switch g.language {
	case LangGo:
		return g.generateGo()
	case LangTypeScript:
		return g.generateTypeScript()
	case LangPython:
		return g.generatePython()
	default:
		return fmt.Errorf("unsupported language: %s", g.language)
	}
}

// --- Go Code Generation ---

func (g *Generator) generateGo() error {
	tmpl, err := template.New("go").Funcs(template.FuncMap{
		"pascalCase":  toPascalCase,
		"camelCase":   toCamelCase,
		"goType":      toGoType,
		"snakeCase":   toSnakeCase,
		"packageName": func() string { return g.manifest.Package },
	}).Parse(goTemplate)
	if err != nil {
		return fmt.Errorf("parsing Go template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, g.manifest); err != nil {
		return fmt.Errorf("executing Go template: %w", err)
	}

	outPath := filepath.Join(g.outDir, "ojs_generated.go")
	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}

// --- TypeScript Code Generation ---

func (g *Generator) generateTypeScript() error {
	tmpl, err := template.New("ts").Funcs(template.FuncMap{
		"pascalCase": toPascalCase,
		"camelCase":  toCamelCase,
		"tsType":     toTSType,
	}).Parse(tsTemplate)
	if err != nil {
		return fmt.Errorf("parsing TypeScript template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, g.manifest); err != nil {
		return fmt.Errorf("executing TypeScript template: %w", err)
	}

	outPath := filepath.Join(g.outDir, "ojs-generated.ts")
	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}

// --- Python Code Generation ---

func (g *Generator) generatePython() error {
	tmpl, err := template.New("py").Funcs(template.FuncMap{
		"pascalCase": toPascalCase,
		"snakeCase":  toSnakeCase,
		"pyType":     toPythonType,
	}).Parse(pyTemplate)
	if err != nil {
		return fmt.Errorf("parsing Python template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, g.manifest); err != nil {
		return fmt.Errorf("executing Python template: %w", err)
	}

	outPath := filepath.Join(g.outDir, "ojs_generated.py")
	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}

// --- Naming Helpers ---

func toPascalCase(s string) string {
	parts := splitIdentifier(s)
	var result strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			result.WriteRune(unicode.ToUpper(rune(p[0])))
			result.WriteString(p[1:])
		}
	}
	return result.String()
}

func toCamelCase(s string) string {
	parts := splitIdentifier(s)
	var result strings.Builder
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		if i == 0 {
			result.WriteString(strings.ToLower(p))
		} else {
			result.WriteRune(unicode.ToUpper(rune(p[0])))
			result.WriteString(p[1:])
		}
	}
	return result.String()
}

func toSnakeCase(s string) string {
	parts := splitIdentifier(s)
	return strings.Join(parts, "_")
}

func splitIdentifier(s string) []string {
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	parts := strings.Split(s, "_")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, strings.ToLower(p))
		}
	}
	return result
}

func toGoType(t string) string {
	switch strings.ToLower(t) {
	case "string":
		return "string"
	case "int", "integer":
		return "int"
	case "float", "number":
		return "float64"
	case "bool", "boolean":
		return "bool"
	case "object":
		return "map[string]interface{}"
	case "array":
		return "[]interface{}"
	default:
		return "interface{}"
	}
}

func toTSType(t string) string {
	switch strings.ToLower(t) {
	case "string":
		return "string"
	case "int", "integer", "float", "number":
		return "number"
	case "bool", "boolean":
		return "boolean"
	case "object":
		return "Record<string, unknown>"
	case "array":
		return "unknown[]"
	default:
		return "unknown"
	}
}

func toPythonType(t string) string {
	switch strings.ToLower(t) {
	case "string":
		return "str"
	case "int", "integer":
		return "int"
	case "float", "number":
		return "float"
	case "bool", "boolean":
		return "bool"
	case "object":
		return "dict[str, Any]"
	case "array":
		return "list[Any]"
	default:
		return "Any"
	}
}

// --- Templates ---

var goTemplate = `// Code generated by ojs codegen. DO NOT EDIT.
package {{ packageName }}

import (
	"context"
	"encoding/json"
)

// OJSClient is the interface for enqueuing jobs.
type OJSClient interface {
	Enqueue(ctx context.Context, jobType string, args []interface{}, opts ...EnqueueOption) (string, error)
}

// EnqueueOption configures an enqueue call.
type EnqueueOption func(*enqueueOpts)

type enqueueOpts struct {
	Queue    string
	Priority *int
	Tags     []string
}

func WithQueue(q string) EnqueueOption { return func(o *enqueueOpts) { o.Queue = q } }
func WithPriority(p int) EnqueueOption { return func(o *enqueueOpts) { o.Priority = &p } }
func WithTags(t ...string) EnqueueOption { return func(o *enqueueOpts) { o.Tags = t } }
{{ range .JobTypes }}
// --- {{ pascalCase .Type }} ---
{{ if .Description }}
// {{ pascalCase .Type }}Args represents the arguments for {{ .Type }} jobs.
// {{ .Description }}{{ end }}
type {{ pascalCase .Type }}Args struct {
{{- range .Args }}
	{{ pascalCase .Name }} {{ goType .Type }} ` + "`" + `json:"{{ snakeCase .Name }}"` + "`" + `{{ if .Description }} // {{ .Description }}{{ end }}
{{- end }}
}

// Enqueue{{ pascalCase .Type }} enqueues a {{ .Type }} job with type-safe arguments.
func Enqueue{{ pascalCase .Type }}(ctx context.Context, client OJSClient, args {{ pascalCase .Type }}Args) (string, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	var argsSlice []interface{}
	if err := json.Unmarshal(raw, &argsSlice); err != nil {
		argsSlice = []interface{}{json.RawMessage(raw)}
	}
	return client.Enqueue(ctx, "{{ .Type }}", argsSlice, WithQueue("{{ .Queue }}"))
}
{{ end }}`

var tsTemplate = `// Code generated by ojs codegen. DO NOT EDIT.

import type { OJSClient } from '@openjobspec/sdk';
{{ range .JobTypes }}
/** {{ .Description }} */
export interface {{ pascalCase .Type }}Args {
{{- range .Args }}
  {{ camelCase .Name }}{{ if not .Required }}?{{ end }}: {{ tsType .Type }};{{ if .Description }} // {{ .Description }}{{ end }}
{{- end }}
}

/** Enqueue a {{ .Type }} job with type-safe arguments. */
export async function enqueue{{ pascalCase .Type }}(client: OJSClient, args: {{ pascalCase .Type }}Args): Promise<string> {
  return client.enqueue('{{ .Type }}', Object.values(args), { queue: '{{ .Queue }}' });
}
{{ end }}`

var pyTemplate = `# Code generated by ojs codegen. DO NOT EDIT.

from __future__ import annotations
from dataclasses import dataclass
from typing import Any, TYPE_CHECKING

if TYPE_CHECKING:
    from openjobspec import OJSClient
{{ range .JobTypes }}

@dataclass
class {{ pascalCase .Type }}Args:
    """{{ .Description }}"""
{{- range .Args }}
    {{ snakeCase .Name }}: {{ pyType .Type }}{{ if .Description }}  # {{ .Description }}{{ end }}
{{- end }}

async def enqueue_{{ snakeCase .Type }}(client: "OJSClient", args: {{ pascalCase .Type }}Args) -> str:
    """Enqueue a {{ .Type }} job with type-safe arguments."""
    return await client.enqueue("{{ .Type }}", vars(args), queue="{{ .Queue }}")
{{ end }}`
