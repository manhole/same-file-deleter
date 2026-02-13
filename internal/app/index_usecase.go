package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"same-file-deleter/internal/domain"
	"same-file-deleter/internal/infra"
)

type IndexParams struct {
	Dir      string
	Out      string
	Update   bool
	Excludes []string
}

type IndexSummary struct {
	Scanned  int
	Reused   int
	Rehashed int
	Errors   int
}

type IndexUseCase struct {
	Hasher infra.FileHasher
	Stderr io.Writer
}

func (uc IndexUseCase) Run(params IndexParams) (IndexSummary, error) {
	summary := IndexSummary{}

	if strings.TrimSpace(params.Dir) == "" {
		return summary, NewInputErrorf("--dir is required")
	}
	if strings.TrimSpace(params.Out) == "" {
		return summary, NewInputErrorf("--out is required")
	}

	absDir, err := filepath.Abs(params.Dir)
	if err != nil {
		return summary, NewInputErrorf("invalid --dir: %v", err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return summary, NewInputErrorf("--dir does not exist: %s", absDir)
		}
		return summary, fmt.Errorf("stat --dir: %w", err)
	}
	if !info.IsDir() {
		return summary, NewInputErrorf("--dir is not a directory: %s", absDir)
	}

	hasher := uc.Hasher
	if hasher == nil {
		hasher = infra.Blake3Hasher{}
	}

	previousByPath := map[string]domain.IndexRecord{}
	if params.Update {
		if err := loadPreviousIndex(params.Out, previousByPath); err != nil {
			return summary, err
		}
	}

	writer, err := infra.NewJSONLAtomicWriter(params.Out)
	if err != nil {
		return summary, err
	}
	defer writer.Abort()

	onError := func(path string, err error) {
		summary.Errors++
		uc.logf("index error: %s: %v\n", path, err)
	}

	err = infra.WalkFiles(absDir, params.Excludes, func(file infra.WalkFile) error {
		summary.Scanned++
		checksum := ""

		prev, ok := previousByPath[file.RelPath]
		if ok &&
			prev.Size == file.Size &&
			prev.MTimeNS == file.MTimeNS &&
			prev.Algo == domain.AlgoBLAKE3 &&
			prev.Checksum != "" {
			checksum = prev.Checksum
			summary.Reused++
		} else {
			var hashErr error
			checksum, hashErr = hasher.HashFile(file.AbsPath)
			if hashErr != nil {
				summary.Errors++
				uc.logf("hash error: %s: %v\n", file.AbsPath, hashErr)
				return nil
			}
			summary.Rehashed++
		}

		rec := domain.IndexRecord{
			Root:     absDir,
			Path:     file.RelPath,
			Size:     file.Size,
			MTimeNS:  file.MTimeNS,
			Algo:     domain.AlgoBLAKE3,
			Checksum: checksum,
			Type:     domain.FileTypeRegular,
		}
		if err := writer.Write(rec); err != nil {
			return fmt.Errorf("write index record: %w", err)
		}
		return nil
	}, onError)
	if err != nil {
		return summary, err
	}

	if err := writer.Commit(); err != nil {
		return summary, err
	}

	return summary, nil
}

func loadPreviousIndex(path string, dest map[string]domain.IndexRecord) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat previous index: %w", err)
	}

	err := infra.ReadIndexJSONL(path, func(rec domain.IndexRecord) error {
		dest[rec.Path] = rec
		return nil
	})
	if err == nil {
		return nil
	}

	var parseErr *infra.JSONLParseError
	if errors.As(err, &parseErr) {
		return NewInputErrorf("invalid existing index file: %v", err)
	}
	return err
}

func (uc IndexUseCase) logf(format string, args ...any) {
	if uc.Stderr == nil {
		return
	}
	fmt.Fprintf(uc.Stderr, format, args...)
}
