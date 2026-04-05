package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	MatchPath  bool
}

type PlanSummary struct {
	ARecords   int
	BRecords   int
	Matches    int
	MatchBytes int64
}

type PlanUseCase struct {
	Stderr io.Writer
	Stdout io.Writer
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
		if params.MatchPath {
			return summary, NewInputErrorf("--self and --match-path are mutually exclusive")
		}
		return uc.runSelf(params)
	}

	if strings.TrimSpace(params.BIndexPath) == "" {
		return summary, NewInputErrorf("--b is required")
	}

	if sameFile(params.AIndexPath, params.BIndexPath) {
		return summary, NewInputErrorf("--a and --b must not be the same file")
	}

	if params.MatchPath {
		return uc.runMatchPath(params)
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
		if isInRecycleBin(rec.Path) {
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

// runMatchPath は --match-path モードの処理。
// A と B でパスが同じ かつ checksum+size が同じファイルを B の削除候補にする。
func (uc PlanUseCase) runMatchPath(params PlanParams) (PlanSummary, error) {
	summary := PlanSummary{}

	// A のインデックスを「パス → レコード」のマップとして読み込む
	aByPath := make(map[string]domain.IndexRecord)
	if err := infra.ReadIndexJSONL(params.AIndexPath, func(rec domain.IndexRecord) error {
		summary.ARecords++
		aByPath[rec.Path] = rec
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

		// ① B のパスが A に存在するか
		aRec, exists := aByPath[rec.Path]
		if !exists {
			return nil
		}
		// ② checksum+size+algo が一致するか
		if domain.MatchKeyFromIndex(aRec) != domain.MatchKeyFromIndex(rec) {
			return nil
		}
		if isInRecycleBin(rec.Path) {
			return nil
		}
		if strings.TrimSpace(rec.Root) == "" {
			return NewInputErrorf("B index record missing root for path: %s", rec.Path)
		}

		plan := domain.PlanRecord{
			BRoot:    rec.Root,
			Path:     rec.Path,
			Reason:   domain.PlanReasonPathAndChecksumMatch,
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

	// #recycle 内のファイルは常に削除候補から除外する。
	// #recycle 外のファイルが 2 件以上あるときのみ削除候補を抽出する。
	// 出力を決定論的にするため、keep パスで昇順ソートしてから処理する。
	type dupGroup struct {
		keep    domain.IndexRecord
		deletes []domain.IndexRecord
	}
	var orderedGroups []dupGroup
	for _, recs := range groups {
		if len(recs) < 2 {
			continue
		}
		var outside []domain.IndexRecord
		for _, rec := range recs {
			if !isInRecycleBin(rec.Path) {
				outside = append(outside, rec)
			}
		}
		if len(outside) < 2 {
			continue
		}
		sort.Slice(outside, func(i, j int) bool { return outside[i].Path < outside[j].Path })
		orderedGroups = append(orderedGroups, dupGroup{keep: outside[0], deletes: outside[1:]})
	}
	sort.Slice(orderedGroups, func(i, j int) bool {
		return orderedGroups[i].keep.Path < orderedGroups[j].keep.Path
	})

	for _, g := range orderedGroups {
		if uc.Stdout != nil {
			fmt.Fprintf(uc.Stdout, "group: %s [keep]\n", g.keep.Path)
			for _, rec := range g.deletes {
				fmt.Fprintf(uc.Stdout, "  delete: %s\n", rec.Path)
			}
			fmt.Fprintln(uc.Stdout)
		}
		for _, rec := range g.deletes {
			plan := domain.PlanRecord{
				BRoot:    rec.Root,
				Path:     rec.Path,
				Reason:   domain.PlanReasonSelfDuplicate,
				Checksum: rec.Checksum,
				Size:     rec.Size,
				KeptPath: g.keep.Path,
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

func isInRecycleBin(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == "#recycle" {
			return true
		}
	}
	return false
}

// sameFile は2つのパスが同じファイルを指すか判定する。
// filepath.Abs で正規化した文字列を比較する。
func sameFile(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	return absA == absB
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
