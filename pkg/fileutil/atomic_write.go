package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWrite writes file contents using crash-safe replacement semantics for
// repo-managed state files:
//  1. create temp file in the same directory as target
//  2. write full contents
//  3. fsync temp file contents
//  4. close temp file
//  5. rename temp file over target
//  6. fsync parent directory where supported
//
// The parent directory must already exist; this helper does not create
// directories implicitly. Callers should create required directories before
// calling AtomicWrite.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	tmpFile, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	removeTmp := true
	defer func() {
		if removeTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to set temporary file permissions: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write temporary file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to fsync temporary file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// #nosec G703 -- tmpPath is created in dir and path is an explicit caller-selected repo-managed file path.
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to replace destination file: %w", err)
	}
	removeTmp = false

	if err := syncDir(dir); err != nil {
		return fmt.Errorf("failed to fsync parent directory: %w", err)
	}

	return nil
}
