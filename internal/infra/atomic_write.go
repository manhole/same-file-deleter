package infra

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func openAtomicFile(target string) (file *os.File, tmpPath string, absTarget string, err error) {
	absTarget, err = filepath.Abs(target)
	if err != nil {
		return nil, "", "", fmt.Errorf("resolve output path: %w", err)
	}

	dir := filepath.Dir(absTarget)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", "", fmt.Errorf("create output directory: %w", err)
	}

	f, err := os.CreateTemp(dir, ".sfd-*.tmp")
	if err != nil {
		return nil, "", "", fmt.Errorf("create temp output file: %w", err)
	}

	return f, f.Name(), absTarget, nil
}

func commitAtomicFile(file *os.File, tmpPath string, target string) error {
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync temp output file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp output file: %w", err)
	}

	err := os.Rename(tmpPath, target)
	if err == nil {
		return nil
	}

	// On Windows, os.Rename may fail if target exists. Replace manually in that case.
	if runtime.GOOS == "windows" {
		_ = os.Remove(target)
		err = os.Rename(tmpPath, target)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("replace output file: %w", err)
}

func abortAtomicFile(file *os.File, tmpPath string) {
	if file != nil {
		_ = file.Close()
	}
	if tmpPath != "" {
		_ = os.Remove(tmpPath)
	}
}
