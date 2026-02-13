package infra

import (
	"path/filepath"
	"testing"
)

func TestEnsureWithinRootAcceptsChildPath(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "sub", "file.txt")
	if err := EnsureWithinRoot(root, target); err != nil {
		t.Fatalf("expected child path to pass, got error: %v", err)
	}
}

func TestEnsureWithinRootRejectsOutsidePath(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Clean(filepath.Join(root, "..", "outside.txt"))
	if err := EnsureWithinRoot(root, outside); err == nil {
		t.Fatalf("expected outside path to be rejected")
	}
}
