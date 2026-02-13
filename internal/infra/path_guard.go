package infra

import (
	"fmt"
	"path/filepath"
	"strings"
)

func EnsureWithinRoot(root string, target string) error {
	if root == "" {
		return fmt.Errorf("root is empty")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve root path: %w", err)
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}

	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return fmt.Errorf("compute relative path: %w", err)
	}
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes root: root=%q target=%q", absRoot, absTarget)
	}
	return nil
}
