package fileutil

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicReplacesOnlyOnSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jobs.ndjson")
	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeErr := errors.New("encode failed")
	err := WriteAtomic(path, 0o644, func(w io.Writer) error {
		if _, err := io.WriteString(w, "partial"); err != nil {
			t.Fatal(err)
		}
		return writeErr
	})
	if !errors.Is(err, writeErr) {
		t.Fatalf("WriteAtomic error = %v, want %v", err, writeErr)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatalf("failed write replaced destination with %q", data)
	}

	if err := WriteAtomic(path, 0o644, func(w io.Writer) error {
		_, err := io.WriteString(w, "complete")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "complete" {
		t.Fatalf("successful write = %q, want complete", data)
	}
}
