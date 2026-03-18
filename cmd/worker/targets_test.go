package worker

import (
	"testing"
)

func TestValidateTarget(t *testing.T) {
	tests := []struct {
		target BuildTarget
		ok     bool
	}{
		{TargetRust, true},
		{TargetTinyGo, true},
		{TargetJavaScript, true},
		{"python", false},
		{"", false},
	}
	for _, tt := range tests {
		err := ValidateTarget(tt.target)
		if tt.ok && err != nil {
			t.Errorf("ValidateTarget(%q) unexpected error: %v", tt.target, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ValidateTarget(%q) expected error", tt.target)
		}
	}
}

func TestParseTarget(t *testing.T) {
	tests := []struct {
		input string
		want  BuildTarget
		ok    bool
	}{
		{"rust", TargetRust, true},
		{"TINYGO", TargetTinyGo, true},
		{"JavaScript", TargetJavaScript, true},
		{" rust ", TargetRust, true},
		{"go", "", false},
	}
	for _, tt := range tests {
		got, err := ParseTarget(tt.input)
		if tt.ok {
			if err != nil {
				t.Errorf("ParseTarget(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseTarget(%q) = %q, want %q", tt.input, got, tt.want)
			}
		} else if err == nil {
			t.Errorf("ParseTarget(%q) expected error", tt.input)
		}
	}
}

func TestAllTargets(t *testing.T) {
	targets := AllTargets()
	if len(targets) != 3 {
		t.Errorf("expected 3 targets, got %d", len(targets))
	}
}

func TestToolchainName(t *testing.T) {
	if TargetRust.ToolchainName() == "" {
		t.Error("Rust toolchain name is empty")
	}
	if TargetTinyGo.ToolchainName() == "" {
		t.Error("TinyGo toolchain name is empty")
	}
	if TargetJavaScript.ToolchainName() == "" {
		t.Error("JavaScript toolchain name is empty")
	}
}

func TestWasmTarget(t *testing.T) {
	for _, target := range AllTargets() {
		if target.WasmTarget() != "wasm32-wasip1" {
			t.Errorf("%s.WasmTarget() = %q", target, target.WasmTarget())
		}
	}
}
