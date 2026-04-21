package commands

import (
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/codegen"
)

// Codegen generates type-safe SDK code from job type definitions.
func Codegen(args []string) error {
	fs := flag.NewFlagSet("codegen", flag.ContinueOnError)
	manifest := fs.String("manifest", "ojs-jobs.yaml", "Path to job type manifest (YAML or JSON)")
	lang := fs.String("lang", "go", "Target language: go, typescript, python, java, rust, ruby, dotnet")
	outDir := fs.String("out", "./generated", "Output directory")
	pkg := fs.String("package", "", "Package name override (Go only)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	m, err := codegen.LoadManifest(*manifest)
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	if *pkg != "" {
		m.Package = *pkg
	}
	if m.Package == "" {
		m.Package = "ojsjobs"
	}

	var language codegen.Language
	switch *lang {
	case "go":
		language = codegen.LangGo
	case "typescript", "ts":
		language = codegen.LangTypeScript
	case "python", "py":
		language = codegen.LangPython
	case "java":
		language = codegen.LangJava
	case "rust":
		language = codegen.LangRust
	case "ruby":
		language = codegen.LangRuby
	case "dotnet", "csharp":
		language = codegen.LangDotNet
	default:
		return fmt.Errorf("unsupported language: %s (supported: go, typescript, python, java, rust, ruby, dotnet)", *lang)
	}

	gen := codegen.NewGenerator(m, language, *outDir)
	if err := gen.Generate(); err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}

	fmt.Printf("✓ Generated %s code for %d job types in %s\n", *lang, len(m.JobTypes), *outDir)
	return nil
}
