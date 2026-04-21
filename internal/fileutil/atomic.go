// Package fileutil contains filesystem helpers shared by CLI commands.
package fileutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// WriteAtomic writes a file in its destination directory and renames it into
// place only after the complete payload has been flushed successfully.
func WriteAtomic(path string, perm os.FileMode, write func(io.Writer) error) (err error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	f, err := os.CreateTemp(dir, "."+base+".partial-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	tempPath := f.Name()
	closed := false
	defer func() {
		if !closed {
			if closeErr := f.Close(); err == nil && closeErr != nil {
				err = fmt.Errorf("close temporary output: %w", closeErr)
			}
		}
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if err = f.Chmod(perm); err != nil {
		return fmt.Errorf("set output permissions: %w", err)
	}
	if writeErr := write(f); writeErr != nil {
		return writeErr
	}
	if err = f.Sync(); err != nil {
		return fmt.Errorf("sync output: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	closed = true
	if err = os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace output: %w", err)
	}
	return nil
}
