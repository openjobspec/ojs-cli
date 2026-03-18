package worker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuild_InvalidLang(t *testing.T) {
	dir := t.TempDir()

	_, err := Build("python", dir, BuildOptions{})
	if err == nil {
		t.Fatal("expected error for unsupported language, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported language") {
		t.Fatalf("unexpected error message: %s", err)
	}
}

func TestBuild_EmptyLang(t *testing.T) {
	dir := t.TempDir()

	_, err := Build("", dir, BuildOptions{})
	if err == nil {
		t.Fatal("expected error for empty language, got nil")
	}
	if !strings.Contains(err.Error(), "lang is required") {
		t.Fatalf("unexpected error message: %s", err)
	}
}

func TestBuild_ValidRust(t *testing.T) {
	dir := t.TempDir()

	// Write a source file so hashSourceDir has something to hash.
	if err := os.WriteFile(filepath.Join(dir, "main.rs"), []byte("fn main() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Build("rust", dir, BuildOptions{
		OutputDir:    dir,
		SBOMGenerate: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.OutputPath != dir+"/worker.wasm" {
		t.Fatalf("unexpected output path: %s", result.OutputPath)
	}
	if result.SBOM == "" {
		t.Fatal("expected SBOM path when SBOMGenerate is true")
	}
	if result.SHA256 == "" {
		t.Fatal("expected non-empty SHA256")
	}
	if result.Size <= 0 {
		t.Fatal("expected positive Size")
	}
}

func TestBuild_InvalidDir(t *testing.T) {
	_, err := Build("rust", "/nonexistent/path", BuildOptions{})
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
}

func TestSign_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.seed")
	seed := make([]byte, 32)
	os.WriteFile(keyPath, seed, 0600)

	_, err := Sign("/nonexistent/worker.wasm", keyPath)
	if err == nil {
		t.Fatal("expected error for missing wasm file, got nil")
	}
	if !strings.Contains(err.Error(), "opening wasm file") {
		t.Fatalf("unexpected error message: %s", err)
	}
}

func TestSign_ValidFile(t *testing.T) {
	dir := t.TempDir()
	wasmPath := filepath.Join(dir, "test.wasm")
	if err := os.WriteFile(wasmPath, []byte("fake wasm content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a 32-byte Ed25519 seed file.
	keyPath := filepath.Join(dir, "key.seed")
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i)
	}
	if err := os.WriteFile(keyPath, seed, 0600); err != nil {
		t.Fatal(err)
	}

	result, err := Sign(wasmPath, keyPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SHA256 == "" {
		t.Fatal("expected non-empty SHA256")
	}
	if result.Signature == "" {
		t.Fatal("expected non-empty Signature")
	}
	// Signature should be base64, not equal to SHA256.
	if result.Signature == result.SHA256 {
		t.Fatal("real Ed25519 signature should differ from SHA256 hex")
	}
	if result.KeyID != keyPath {
		t.Fatalf("unexpected KeyID: %s", result.KeyID)
	}
}

func TestSign_InvalidKeySize(t *testing.T) {
	dir := t.TempDir()
	wasmPath := filepath.Join(dir, "test.wasm")
	os.WriteFile(wasmPath, []byte("wasm"), 0644)

	keyPath := filepath.Join(dir, "bad.key")
	os.WriteFile(keyPath, []byte("too short"), 0600)

	_, err := Sign(wasmPath, keyPath)
	if err == nil {
		t.Fatal("expected error for wrong key size")
	}
	if !strings.Contains(err.Error(), "exactly") {
		t.Fatalf("expected key size error, got: %s", err)
	}
}

func TestSign_MissingKeyFile(t *testing.T) {
	dir := t.TempDir()
	wasmPath := filepath.Join(dir, "test.wasm")
	os.WriteFile(wasmPath, []byte("wasm"), 0644)

	_, err := Sign(wasmPath, filepath.Join(dir, "nonexistent.key"))
	if err == nil {
		t.Fatal("expected error for missing key file")
	}
	if !strings.Contains(err.Error(), "reading key file") {
		t.Fatalf("expected key read error, got: %s", err)
	}
}

func TestPush_ValidatesInputs(t *testing.T) {
	tests := []struct {
		name     string
		wasm     string
		registry string
		pushName string
		tag      string
		wantErr  string
	}{
		{"empty wasmPath", "", "reg.io", "worker", "v1", "wasmPath is required"},
		{"empty registry", "f.wasm", "", "worker", "v1", "registry is required"},
		{"empty name", "f.wasm", "reg.io", "", "v1", "name is required"},
		{"empty tag", "f.wasm", "reg.io", "worker", "", "tag is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Push(tc.wasm, tc.registry, tc.pushName, tc.tag)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got: %s", tc.wantErr, err)
			}
		})
	}
}

func TestPush_FileNotFound(t *testing.T) {
	err := Push("/nonexistent/worker.wasm", "reg.io", "worker", "v1")
	if err == nil {
		t.Fatal("expected error for missing wasm file, got nil")
	}
}
