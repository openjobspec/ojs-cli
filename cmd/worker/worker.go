// Package worker provides the `ojs worker` subcommands for building, signing,
// and pushing WASI worker components.
package worker

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// BuildOptions configures a worker build.
type BuildOptions struct {
	Target      string // Build target (e.g. "wasm32-wasip1")
	OutputDir   string // Directory for the output artifact
	SBOMGenerate bool  // Whether to generate an SBOM
}

// BuildResult describes the output of a successful build.
type BuildResult struct {
	OutputPath string // Path to the produced .wasm file
	Size       int64  // File size in bytes
	SHA256     string // Hex-encoded SHA-256 of the artifact
	SBOM       string // Path to the SBOM file, empty if not generated
}

// SignResult describes the output of a signing operation.
type SignResult struct {
	SHA256    string // Hex-encoded SHA-256 of the signed artifact
	Signature string // Hex-encoded signature (P1: same as SHA256)
	KeyID     string // Identifier for the signing key used
}

var supportedLangs = map[string]bool{
	"rust":   true,
	"tinygo": true,
	"js":     true,
}

// Build validates the source directory, determines the build toolchain,
// and returns the output path. This is a P1 prototype that validates
// inputs and returns a placeholder result.
func Build(lang, dir string, opts BuildOptions) (*BuildResult, error) {
	if lang == "" {
		return nil, fmt.Errorf("lang is required")
	}
	if !supportedLangs[lang] {
		return nil, fmt.Errorf("unsupported language %q: must be one of rust, tinygo, js", lang)
	}

	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("source directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path %q is not a directory", dir)
	}

	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = dir
	}

	outputPath := fmt.Sprintf("%s/worker.wasm", outputDir)

	// Compute a meaningful SHA-256 of all source files in the directory.
	dirHash, dirSize, err := hashSourceDir(dir)
	if err != nil {
		return nil, fmt.Errorf("hashing source directory: %w", err)
	}

	result := &BuildResult{
		OutputPath: outputPath,
		Size:       dirSize,
		SHA256:     dirHash,
	}

	if opts.SBOMGenerate {
		result.SBOM = fmt.Sprintf("%s/worker.sbom.json", outputDir)
	}

	return result, nil
}

// Sign signs a .wasm artifact with an Ed25519 key. The key file must
// contain a 32-byte Ed25519 seed (same format as CTN).
func Sign(wasmPath, keyPath string) (*SignResult, error) {
	if wasmPath == "" {
		return nil, fmt.Errorf("wasmPath is required")
	}
	if keyPath == "" {
		return nil, fmt.Errorf("keyPath is required")
	}

	// Read and validate the Ed25519 key seed.
	seed, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading key file: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("key file %q must be exactly %d bytes (Ed25519 seed), got %d", keyPath, ed25519.SeedSize, len(seed))
	}
	privKey := ed25519.NewKeyFromSeed(seed)

	// Compute SHA-256 of the WASM file.
	f, err := os.Open(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("opening wasm file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("computing SHA-256: %w", err)
	}

	digestBytes := h.Sum(nil)
	digest := hex.EncodeToString(digestBytes)

	// Sign the SHA-256 digest with Ed25519.
	sig := ed25519.Sign(privKey, digestBytes)

	return &SignResult{
		SHA256:    digest,
		Signature: base64.StdEncoding.EncodeToString(sig),
		KeyID:     keyPath,
	}, nil
}

// Push pushes a .wasm artifact to an OCI registry.
// In the P1 prototype, it validates inputs without performing the push.
func Push(wasmPath, registry, name, tag string) error {
	if wasmPath == "" {
		return fmt.Errorf("wasmPath is required")
	}
	if registry == "" {
		return fmt.Errorf("registry is required")
	}
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if tag == "" {
		return fmt.Errorf("tag is required")
	}

	if _, err := os.Stat(wasmPath); err != nil {
		return fmt.Errorf("wasm file %q: %w", wasmPath, err)
	}

	return nil
}

// hashSourceDir computes a deterministic SHA-256 over all regular files
// in dir (sorted by path) and returns the hex digest and total size.
func hashSourceDir(dir string) (string, int64, error) {
	h := sha256.New()
	var totalSize int64

	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return "", 0, err
	}

	sort.Strings(paths)

	for _, rel := range paths {
		full := filepath.Join(dir, rel)
		fi, err := os.Stat(full)
		if err != nil {
			return "", 0, err
		}
		totalSize += fi.Size()
		// Include the relative path in the hash for determinism.
		h.Write([]byte(rel))
		f, err := os.Open(full)
		if err != nil {
			return "", 0, err
		}
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return "", 0, err
		}
		f.Close()
	}

	return hex.EncodeToString(h.Sum(nil)), totalSize, nil
}
