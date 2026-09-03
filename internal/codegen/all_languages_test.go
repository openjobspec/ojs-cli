package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratorAllLanguages(t *testing.T) {
	manifest := &Manifest{
		Version: "1.0",
		Package: "jobs",
		JobTypes: []JobTypeDef{{
			Type:        "example.run",
			Description: "Run an example",
			Queue:       "default",
			Args: []ArgDef{
				{Name: "text_value", Type: "string", Required: true},
				{Name: "int_value", Type: "int"},
				{Name: "float_value", Type: "float"},
				{Name: "bool_value", Type: "bool"},
				{Name: "object_value", Type: "object"},
				{Name: "array_value", Type: "array"},
				{Name: "unknown_value", Type: "custom"},
			},
		}},
	}
	languages := []Language{LangGo, LangTypeScript, LangPython, LangJava, LangRust, LangRuby, LangDotNet}
	for _, language := range languages {
		t.Run(string(language), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), string(language))
			if err := NewGenerator(manifest, language, dir).Generate(); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) == 0 {
				t.Fatal("generator produced no files")
			}
		})
	}
	if err := NewGenerator(manifest, Language("unknown"), t.TempDir()).Generate(); err == nil {
		t.Fatal("unsupported language did not fail")
	}
}

func TestTypeMappingsAndNamingBranches(t *testing.T) {
	types := []string{"string", "int", "float", "bool", "object", "array", "custom"}
	for _, value := range types {
		if toGoType(value) == "" || toTSType(value) == "" || toPythonType(value) == "" ||
			toJavaType(value) == "" || toRustType(value) == "" || toCSharpType(value) == "" {
			t.Fatalf("empty mapping for %q", value)
		}
	}
	if got := toCamelCase("alreadyCamel"); got != "alreadycamel" {
		t.Fatalf("toCamelCase = %q", got)
	}
}

func TestLoadManifestErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadManifest(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Fatal("missing manifest did not fail")
	}
	unsupported := filepath.Join(dir, "jobs.toml")
	if err := os.WriteFile(unsupported, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(unsupported); err == nil {
		t.Fatal("unsupported manifest extension did not fail")
	}
	invalid := filepath.Join(dir, "jobs.json")
	if err := os.WriteFile(invalid, []byte(`{"job_types":[{"type":"","args":[]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(invalid); err == nil {
		t.Fatal("invalid manifest did not fail")
	}
}
