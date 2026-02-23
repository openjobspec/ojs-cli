package commands

import (
	"flag"
	"fmt"

	"github.com/openjobspec/ojs-cli/internal/codegen"
)

// Codegen generates type-safe SDK code from job type definitions.
func Codegen(args []string) error {
	fs := flag.NewFlagSet("codegen", flag.ExitOnError)
	manifest := fs.String("manifest", "ojs-jobs.yaml", "Path to job type manifest (YAML or JSON)")
	lang := fs.String("lang", "go", "Target language: go, typescript, python")
	outDir := fs.String("out", "./generated", "Output directory")
	pkg := fs.String("package", "", "Package name override (Go only)")
	fs.Parse(args)

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
	default:
		return fmt.Errorf("unsupported language: %s (supported: go, typescript, python)", *lang)
	}

	gen := codegen.NewGenerator(m, language, *outDir)
	if err := gen.Generate(); err != nil {
		return fmt.Errorf("code generation failed: %w", err)
	}

	fmt.Printf("✓ Generated %s code for %d job types in %s\n", *lang, len(m.JobTypes), *outDir)
	return nil
}
