package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"same-file-deleter/internal/domain"
	"same-file-deleter/internal/infra"
)

type PlanParams struct {
	AIndexPath string
	BIndexPath string
	Out        string
	Self       bool
}

type PlanSummary struct {
	ARecords   int
	BRecords   int
	Matches    int
	MatchBytes int64
}

type PlanUseCase struct {
	Stderr io.Writer
}

func (uc PlanUseCase) Run(params PlanParams) (PlanSummary, error) {
	summary := PlanSummary{}

	if strings.TrimSpace(params.AIndexPath) == "" {
		return summary, NewInputErrorf("--a is required")
	}
	if strings.TrimSpace(params.Out) == "" {
		return summary, NewInputErrorf("--out is required")
	}

	if params.Self {
		if strings.TrimSpace(params.BIndexPath) != "" {
			return summary, NewInputErrorf("--self and --b are mutually exclusive")
		}
		return uc.runSelf(params)
	}

	if strings.TrimSpace(params.BIndexPath) == "" {
		return summary, NewInputErrorf("--b is required")
	}

	aKeys := make(map[domain.MatchKey]struct{})
	if err := infra.ReadIndexJSONL(params.AIndexPath, func(rec domain.IndexRecord) error {
		summary.ARecords++
		aKeys[domain.MatchKeyFromIndex(rec)] = struct{}{}
		return nil
	}); err != nil {
		return summary, classifyIndexReadError("--a", err)
	}

	writer, err := infra.NewJSONLAtomicWriter(params.Out)
	if err != nil {
		return summary, err
	}
	defer writer.Abort()

	err = infra.ReadIndexJSONL(params.BIndexPath, func(rec domain.IndexRecord) error {
		summary.BRecords++
		key := domain.MatchKeyFromIndex(rec)
		if _, ok := aKeys[key]; !ok {
			return nil
		}

		if strings.TrimSpace(rec.Root) == "" {
			return NewInputErrorf("B index record missing root for path: %s", rec.Path)
		}

		plan := domain.PlanRecord{
			BRoot:    rec.Root,
			Path:     rec.Path,
			Reason:   domain.PlanReasonChecksumMatchA,
			Checksum: rec.Checksum,
			Size:     rec.Size,
		}
		if err := writer.Write(plan); err != nil {
			return fmt.Errorf("write plan record: %w", err)
		}
		summary.Matches++
		summary.MatchBytes += rec.Size
		return nil
	})
	if err != nil {
		if IsInputError(err) {
			return summary, err
		}
		return summary, classifyIndexReadError("--b", err)
	}

	if err := writer.Commit(); err != nil {
		return summary, err
	}
	return summary, nil
}

func (uc PlanUseCase) runSelf(params PlanParams) (PlanSummary, error) {
	summary := PlanSummary{}

	groups := make(map[domain.MatchKey][]domain.IndexRecord)
	if err := infra.ReadIndexJSONL(params.AIndexPath, func(rec domain.IndexRecord) error {
		summary.ARecords++
		key := domain.MatchKeyFromIndex(rec)
		groups[key] = append(groups[key], rec)
		return nil
	}); err != nil {
		return summary, classifyIndexReadError("--a", err)
	}
	summary.BRecords = summary.ARecords

	writer, err := infra.NewJSONLAtomicWriter(params.Out)
	if err != nil {
		return summary, err
	}
	defer writer.Abort()

	for _, recs := range groups {
		if len(recs) < 2 {
			continue
		}
		sort.Slice(recs, func(i, j int) bool { return recs[i].Path < recs[j].Path })
		for _, rec := range recs[1:] { // recs[0] は keep
			plan := domain.PlanRecord{
				BRoot:    rec.Root,
				Path:     rec.Path,
				Reason:   domain.PlanReasonSelfDuplicate,
				Checksum: rec.Checksum,
				Size:     rec.Size,
			}
			if err := writer.Write(plan); err != nil {
				return summary, fmt.Errorf("write plan record: %w", err)
			}
			summary.Matches++
			summary.MatchBytes += rec.Size
		}
	}

	if err := writer.Commit(); err != nil {
		return summary, err
	}
	return summary, nil
}

func classifyIndexReadError(flagName string, err error) error {
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return NewInputErrorf("%s file does not exist: %v", flagName, err)
	}

	var parseErr *infra.JSONLParseError
	if errors.As(err, &parseErr) {
		return NewInputErrorf("%s file is invalid: %v", flagName, err)
	}
	return err
}
