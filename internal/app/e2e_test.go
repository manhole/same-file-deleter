package app_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"same-file-deleter/internal/app"
)

func TestIndexPlanApplyEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	aDir := filepath.Join(tmp, "A")
	bDir := filepath.Join(tmp, "B")
	mustMkdirAll(t, aDir)
	mustMkdirAll(t, bDir)

	mustWriteFile(t, filepath.Join(aDir, "keep", "x.txt"), "same-content")
	mustWriteFile(t, filepath.Join(aDir, "only_a.txt"), "A-only")
	mustWriteFile(t, filepath.Join(bDir, "dup", "x-copy.txt"), "same-content")
	mustWriteFile(t, filepath.Join(bDir, "keep_b.txt"), "B-only")

	aIndex := filepath.Join(tmp, "A.checksums.jsonl")
	bIndex := filepath.Join(tmp, "B.checksums.jsonl")
	plan := filepath.Join(tmp, "A_to_B.delete-plan.jsonl")

	var indexErr bytes.Buffer
	indexUC := app.IndexUseCase{Stderr: &indexErr}

	aSummary, err := indexUC.Run(app.IndexParams{Dir: aDir, Out: aIndex})
	if err != nil {
		t.Fatalf("index A failed: %v", err)
	}
	if aSummary.Errors != 0 {
		t.Fatalf("index A should not have file errors: %+v", aSummary)
	}

	bSummary, err := indexUC.Run(app.IndexParams{Dir: bDir, Out: bIndex})
	if err != nil {
		t.Fatalf("index B failed: %v", err)
	}
	if bSummary.Errors != 0 {
		t.Fatalf("index B should not have file errors: %+v", bSummary)
	}

	planUC := app.PlanUseCase{}
	planSummary, err := planUC.Run(app.PlanParams{
		AIndexPath: aIndex,
		BIndexPath: bIndex,
		Out:        plan,
	})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if planSummary.Matches != 1 {
		t.Fatalf("expected 1 match, got %d", planSummary.Matches)
	}

	dupPath := filepath.Join(bDir, "dup", "x-copy.txt")
	if _, err := os.Stat(dupPath); err != nil {
		t.Fatalf("duplicate file should exist before apply: %v", err)
	}

	var dryOut bytes.Buffer
	applyUC := app.ApplyUseCase{Stdout: &dryOut}
	drySummary, err := applyUC.Run(app.ApplyParams{
		PlanPath: plan,
		Execute:  false,
	})
	if err != nil {
		t.Fatalf("apply dry-run failed: %v", err)
	}
	if drySummary.Candidates != 1 || drySummary.Deleted != 0 {
		t.Fatalf("unexpected dry-run summary: %+v", drySummary)
	}
	if _, err := os.Stat(dupPath); err != nil {
		t.Fatalf("duplicate file should remain after dry-run: %v", err)
	}

	execSummary, err := applyUC.Run(app.ApplyParams{
		PlanPath: plan,
		Execute:  true,
	})
	if err != nil {
		t.Fatalf("apply execute failed: %v", err)
	}
	if execSummary.Deleted != 1 {
		t.Fatalf("expected one deleted file, got %+v", execSummary)
	}
	if _, err := os.Stat(dupPath); !os.IsNotExist(err) {
		t.Fatalf("duplicate file should be deleted, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(bDir, "keep_b.txt")); err != nil {
		t.Fatalf("non-duplicate file should remain: %v", err)
	}
}

func TestIndexUpdateReusesChecksum(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "D")
	mustMkdirAll(t, dir)
	mustWriteFile(t, filepath.Join(dir, "one.txt"), "hello")

	indexPath := filepath.Join(tmp, "D.checksums.jsonl")
	uc := app.IndexUseCase{}

	first, err := uc.Run(app.IndexParams{
		Dir: dir,
		Out: indexPath,
	})
	if err != nil {
		t.Fatalf("first index failed: %v", err)
	}
	if first.Rehashed != 1 {
		t.Fatalf("expected first rehash count=1, got %+v", first)
	}

	second, err := uc.Run(app.IndexParams{
		Dir:    dir,
		Out:    indexPath,
		Update: true,
	})
	if err != nil {
		t.Fatalf("second index failed: %v", err)
	}
	if second.Scanned != 1 || second.Reused != 1 || second.Rehashed != 0 {
		t.Fatalf("expected reuse on second index, got %+v", second)
	}
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
