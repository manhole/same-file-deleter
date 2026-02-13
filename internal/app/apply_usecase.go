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

type ApplyParams struct {
	PlanPath  string
	Execute   bool
	MaxDelete int
}

type ApplySummary struct {
	Candidates   int
	Deleted      int
	Failed       int
	DeletedBytes int64
}

type ApplyUseCase struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (uc ApplyUseCase) Run(params ApplyParams) (ApplySummary, error) {
	summary := ApplySummary{}

	if strings.TrimSpace(params.PlanPath) == "" {
		return summary, NewInputErrorf("--plan is required")
	}
	if params.MaxDelete < 0 {
		return summary, NewInputErrorf("--max-delete must be >= 0")
	}

	count, err := uc.countCandidates(params.PlanPath)
	if err != nil {
		return summary, classifyPlanReadError(err)
	}
	if params.MaxDelete > 0 && count > params.MaxDelete {
		return summary, NewInputErrorf("plan candidates exceed --max-delete: candidates=%d max=%d", count, params.MaxDelete)
	}

	dryRun := !params.Execute
	err = infra.ReadPlanJSONL(params.PlanPath, func(rec domain.PlanRecord) error {
		summary.Candidates++

		target := filepath.Join(rec.BRoot, filepath.FromSlash(rec.Path))
		if err := infra.EnsureWithinRoot(rec.BRoot, target); err != nil {
			summary.Failed++
			uc.logErrf("path guard error: %v\n", err)
			return nil
		}

		if dryRun {
			uc.logOutf("%s\n", target)
			return nil
		}

		if err := os.Remove(target); err != nil {
			summary.Failed++
			uc.logErrf("delete error: %s: %v\n", target, err)
			return nil
		}

		summary.Deleted++
		summary.DeletedBytes += rec.Size
		return nil
	})
	if err != nil {
		return summary, classifyPlanReadError(err)
	}

	return summary, nil
}

func (uc ApplyUseCase) countCandidates(planPath string) (int, error) {
	count := 0
	err := infra.ReadPlanJSONL(planPath, func(rec domain.PlanRecord) error {
		count++
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func classifyPlanReadError(err error) error {
	if err == nil {
		return nil
	}
	if IsInputError(err) {
		return err
	}
	if os.IsNotExist(err) {
		return NewInputErrorf("plan file does not exist: %v", err)
	}
	var parseErr *infra.JSONLParseError
	if errors.As(err, &parseErr) {
		return NewInputErrorf("plan file is invalid: %v", err)
	}
	return err
}

func (uc ApplyUseCase) logOutf(format string, args ...any) {
	if uc.Stdout == nil {
		return
	}
	fmt.Fprintf(uc.Stdout, format, args...)
}

func (uc ApplyUseCase) logErrf(format string, args ...any) {
	if uc.Stderr == nil {
		return
	}
	fmt.Fprintf(uc.Stderr, format, args...)
}
