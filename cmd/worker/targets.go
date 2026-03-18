// Package worker provides the `ojs worker` subcommands for building, signing,
// and pushing WASI worker components.
//
// targets.go defines the supported build targets and validation logic.
package worker

import (
	"fmt"
	"strings"
)

// BuildTarget identifies a compilation target for WASI worker builds.
type BuildTarget string

const (
	// TargetRust compiles a Rust handler to wasm32-wasip1.
	TargetRust BuildTarget = "rust"
	// TargetTinyGo compiles a TinyGo handler to wasm32-wasip1.
	TargetTinyGo BuildTarget = "tinygo"
	// TargetJavaScript bundles a JS handler into a WASI-compatible module.
	TargetJavaScript BuildTarget = "javascript"
)

// AllTargets returns all supported build targets.
func AllTargets() []BuildTarget {
	return []BuildTarget{TargetRust, TargetTinyGo, TargetJavaScript}
}

// ValidateTarget checks if a BuildTarget is supported.
func ValidateTarget(t BuildTarget) error {
	switch t {
	case TargetRust, TargetTinyGo, TargetJavaScript:
		return nil
	default:
		return fmt.Errorf("unsupported build target %q: must be one of %s",
			t, strings.Join(targetNames(), ", "))
	}
}

// ParseTarget parses a string into a BuildTarget.
func ParseTarget(s string) (BuildTarget, error) {
	t := BuildTarget(strings.ToLower(strings.TrimSpace(s)))
	if err := ValidateTarget(t); err != nil {
		return "", err
	}
	return t, nil
}

// ToolchainName returns the human-readable toolchain name for a target.
func (t BuildTarget) ToolchainName() string {
	switch t {
	case TargetRust:
		return "rustc + wasm32-wasip1 target"
	case TargetTinyGo:
		return "TinyGo (wasm32-wasip1)"
	case TargetJavaScript:
		return "ComponentizeJS (wasm32-wasip1)"
	default:
		return "unknown"
	}
}

// WasmTarget returns the WASI compilation target string.
func (t BuildTarget) WasmTarget() string {
	return "wasm32-wasip1"
}

func targetNames() []string {
	targets := AllTargets()
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = string(t)
	}
	return names
}
