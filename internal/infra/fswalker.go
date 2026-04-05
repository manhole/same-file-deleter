package infra

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type WalkFile struct {
	AbsPath string
	RelPath string
	Size    int64
	MTimeNS int64
}

// defaultExcludeDirs はデフォルトで除外するディレクトリ名。
// WalkFiles の skipDefaults=true のときに適用される。
var defaultExcludeDirs = []string{".git"}

func WalkFiles(root string, excludes []string, skipDefaults bool, onFile func(WalkFile) error, onError func(string, error)) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve root path: %w", err)
	}

	return filepath.WalkDir(absRoot, func(current string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if onError != nil {
				onError(current, walkErr)
			}
			return nil
		}

		if current == absRoot {
			return nil
		}

		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(absRoot, current)
		if err != nil {
			if onError != nil {
				onError(current, fmt.Errorf("resolve relative path: %w", err))
			}
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		if d.IsDir() {
			if skipDefaults {
				for _, name := range defaultExcludeDirs {
					if d.Name() == name {
						return filepath.SkipDir
					}
				}
			}
			if matchesExclude(relPath, excludes) {
				return filepath.SkipDir
			}
			return nil
		}

		if matchesExclude(relPath, excludes) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			if onError != nil {
				onError(current, fmt.Errorf("read file info: %w", err))
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		if onFile == nil {
			return nil
		}
		return onFile(WalkFile{
			AbsPath: current,
			RelPath: relPath,
			Size:    info.Size(),
			MTimeNS: info.ModTime().UnixNano(),
		})
	})
}

func matchesExclude(relPath string, patterns []string) bool {
	relPath = strings.TrimPrefix(relPath, "./")
	base := path.Base(relPath)

	for _, p := range patterns {
		pattern := filepath.ToSlash(strings.TrimSpace(p))
		if pattern == "" {
			continue
		}
		if matched, _ := path.Match(pattern, relPath); matched {
			return true
		}
		if matched, _ := path.Match(pattern, base); matched {
			return true
		}
	}
	return false
}
