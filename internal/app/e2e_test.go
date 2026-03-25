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

func TestSelfDedupBasic(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "D")

	mustWriteFile(t, filepath.Join(dir, "a", "photo.jpg"), "image-data")
	mustWriteFile(t, filepath.Join(dir, "b", "copy.jpg"), "image-data") // 重複
	mustWriteFile(t, filepath.Join(dir, "c", "unique.txt"), "unique")

	indexPath := filepath.Join(tmp, "D.checksums.jsonl")
	planPath := filepath.Join(tmp, "D.plan.jsonl")

	indexUC := app.IndexUseCase{}
	if _, err := indexUC.Run(app.IndexParams{Dir: dir, Out: indexPath}); err != nil {
		t.Fatalf("index failed: %v", err)
	}

	planUC := app.PlanUseCase{}
	planSummary, err := planUC.Run(app.PlanParams{
		AIndexPath: indexPath,
		Out:        planPath,
		Self:       true,
	})
	if err != nil {
		t.Fatalf("plan --self failed: %v", err)
	}
	if planSummary.Matches != 1 {
		t.Fatalf("expected 1 match, got %d", planSummary.Matches)
	}

	dupPath := filepath.Join(dir, "b", "copy.jpg")
	keepPath := filepath.Join(dir, "a", "photo.jpg")

	// dry-run: ファイルは削除されない
	var dryOut bytes.Buffer
	applyUC := app.ApplyUseCase{Stdout: &dryOut}
	drySummary, err := applyUC.Run(app.ApplyParams{PlanPath: planPath, Execute: false})
	if err != nil {
		t.Fatalf("apply dry-run failed: %v", err)
	}
	if drySummary.Candidates != 1 || drySummary.Deleted != 0 {
		t.Fatalf("unexpected dry-run summary: %+v", drySummary)
	}
	if _, err := os.Stat(dupPath); err != nil {
		t.Fatalf("duplicate should remain after dry-run: %v", err)
	}

	// execute: 重複ファイルだけ削除される
	execSummary, err := applyUC.Run(app.ApplyParams{PlanPath: planPath, Execute: true})
	if err != nil {
		t.Fatalf("apply execute failed: %v", err)
	}
	if execSummary.Deleted != 1 {
		t.Fatalf("expected 1 deleted, got %+v", execSummary)
	}
	if _, err := os.Stat(dupPath); !os.IsNotExist(err) {
		t.Fatalf("duplicate should be deleted, stat err=%v", err)
	}
	if _, err := os.Stat(keepPath); err != nil {
		t.Fatalf("kept file should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "c", "unique.txt")); err != nil {
		t.Fatalf("unique file should remain: %v", err)
	}
}

func TestSelfDedupThreeWay(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "D")

	mustWriteFile(t, filepath.Join(dir, "a", "x.txt"), "dup-content")
	mustWriteFile(t, filepath.Join(dir, "b", "x.txt"), "dup-content")
	mustWriteFile(t, filepath.Join(dir, "c", "x.txt"), "dup-content")

	indexPath := filepath.Join(tmp, "D.checksums.jsonl")
	planPath := filepath.Join(tmp, "D.plan.jsonl")

	indexUC := app.IndexUseCase{}
	if _, err := indexUC.Run(app.IndexParams{Dir: dir, Out: indexPath}); err != nil {
		t.Fatalf("index failed: %v", err)
	}

	planUC := app.PlanUseCase{}
	planSummary, err := planUC.Run(app.PlanParams{
		AIndexPath: indexPath,
		Out:        planPath,
		Self:       true,
	})
	if err != nil {
		t.Fatalf("plan --self failed: %v", err)
	}
	// 3件のうち1件（辞書順最小のa/x.txt）を残し、残り2件が候補
	if planSummary.Matches != 2 {
		t.Fatalf("expected 2 matches, got %d", planSummary.Matches)
	}

	applyUC := app.ApplyUseCase{Stdout: &bytes.Buffer{}}
	execSummary, err := applyUC.Run(app.ApplyParams{PlanPath: planPath, Execute: true})
	if err != nil {
		t.Fatalf("apply execute failed: %v", err)
	}
	if execSummary.Deleted != 2 {
		t.Fatalf("expected 2 deleted, got %+v", execSummary)
	}
	if _, err := os.Stat(filepath.Join(dir, "a", "x.txt")); err != nil {
		t.Fatalf("kept file a/x.txt should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b", "x.txt")); !os.IsNotExist(err) {
		t.Fatalf("b/x.txt should be deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "c", "x.txt")); !os.IsNotExist(err) {
		t.Fatalf("c/x.txt should be deleted")
	}
}

func TestSelfDedupNoDuplicates(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "D")

	mustWriteFile(t, filepath.Join(dir, "a.txt"), "content-a")
	mustWriteFile(t, filepath.Join(dir, "b.txt"), "content-b")

	indexPath := filepath.Join(tmp, "D.checksums.jsonl")
	planPath := filepath.Join(tmp, "D.plan.jsonl")

	indexUC := app.IndexUseCase{}
	if _, err := indexUC.Run(app.IndexParams{Dir: dir, Out: indexPath}); err != nil {
		t.Fatalf("index failed: %v", err)
	}

	planUC := app.PlanUseCase{}
	planSummary, err := planUC.Run(app.PlanParams{
		AIndexPath: indexPath,
		Out:        planPath,
		Self:       true,
	})
	if err != nil {
		t.Fatalf("plan --self failed: %v", err)
	}
	if planSummary.Matches != 0 {
		t.Fatalf("expected 0 matches, got %d", planSummary.Matches)
	}
}

func TestABPlanSkipsRecycleBinInB(t *testing.T) {
	// B側の #recycle 内ファイルは A と一致しても削除候補にしない
	tmp := t.TempDir()
	aDir := filepath.Join(tmp, "A")
	bDir := filepath.Join(tmp, "B")

	mustWriteFile(t, filepath.Join(aDir, "photo.jpg"), "image-data")
	mustWriteFile(t, filepath.Join(bDir, "#recycle", "photo.jpg"), "image-data") // 削除対象外
	mustWriteFile(t, filepath.Join(bDir, "photos", "photo.jpg"), "image-data")   // 削除対象

	aIndex := filepath.Join(tmp, "A.checksums.jsonl")
	bIndex := filepath.Join(tmp, "B.checksums.jsonl")
	planPath := filepath.Join(tmp, "plan.jsonl")

	indexUC := app.IndexUseCase{}
	if _, err := indexUC.Run(app.IndexParams{Dir: aDir, Out: aIndex}); err != nil {
		t.Fatalf("index A failed: %v", err)
	}
	if _, err := indexUC.Run(app.IndexParams{Dir: bDir, Out: bIndex}); err != nil {
		t.Fatalf("index B failed: %v", err)
	}

	planUC := app.PlanUseCase{}
	planSummary, err := planUC.Run(app.PlanParams{AIndexPath: aIndex, BIndexPath: bIndex, Out: planPath})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	// #recycle/photo.jpg は対象外、photos/photo.jpg だけが候補
	if planSummary.Matches != 1 {
		t.Fatalf("expected 1 match (recycle excluded), got %d", planSummary.Matches)
	}

	applyUC := app.ApplyUseCase{Stdout: &bytes.Buffer{}}
	execSummary, err := applyUC.Run(app.ApplyParams{PlanPath: planPath, Execute: true})
	if err != nil {
		t.Fatalf("apply execute failed: %v", err)
	}
	if execSummary.Deleted != 1 {
		t.Fatalf("expected 1 deleted, got %+v", execSummary)
	}
	if _, err := os.Stat(filepath.Join(bDir, "#recycle", "photo.jpg")); err != nil {
		t.Fatalf("#recycle/photo.jpg should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bDir, "photos", "photo.jpg")); !os.IsNotExist(err) {
		t.Fatalf("photos/photo.jpg should be deleted")
	}
}

func TestSelfDedupSkipsRecycleBinWhenSingleOutside(t *testing.T) {
	// #recycle 内 + outside 1件 → どちらも削除対象にしない
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "D")

	mustWriteFile(t, filepath.Join(dir, "#recycle", "photo.jpg"), "image-data")
	mustWriteFile(t, filepath.Join(dir, "photos", "photo.jpg"), "image-data")

	indexPath := filepath.Join(tmp, "D.checksums.jsonl")
	planPath := filepath.Join(tmp, "D.plan.jsonl")

	indexUC := app.IndexUseCase{}
	if _, err := indexUC.Run(app.IndexParams{Dir: dir, Out: indexPath}); err != nil {
		t.Fatalf("index failed: %v", err)
	}

	planUC := app.PlanUseCase{}
	planSummary, err := planUC.Run(app.PlanParams{AIndexPath: indexPath, Out: planPath, Self: true})
	if err != nil {
		t.Fatalf("plan --self failed: %v", err)
	}
	if planSummary.Matches != 0 {
		t.Fatalf("expected 0 matches (recycle+1 outside), got %d", planSummary.Matches)
	}
}

func TestSelfDedupDeletesExtraOutsideWithRecycleBin(t *testing.T) {
	// #recycle 内 + outside 2件 → outside の1件を削除候補にする（#recycle は触らない）
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "D")

	mustWriteFile(t, filepath.Join(dir, "#recycle", "photo.jpg"), "image-data")
	mustWriteFile(t, filepath.Join(dir, "a", "photo.jpg"), "image-data")
	mustWriteFile(t, filepath.Join(dir, "b", "photo.jpg"), "image-data")

	indexPath := filepath.Join(tmp, "D.checksums.jsonl")
	planPath := filepath.Join(tmp, "D.plan.jsonl")

	indexUC := app.IndexUseCase{}
	if _, err := indexUC.Run(app.IndexParams{Dir: dir, Out: indexPath}); err != nil {
		t.Fatalf("index failed: %v", err)
	}

	planUC := app.PlanUseCase{}
	planSummary, err := planUC.Run(app.PlanParams{AIndexPath: indexPath, Out: planPath, Self: true})
	if err != nil {
		t.Fatalf("plan --self failed: %v", err)
	}
	if planSummary.Matches != 1 {
		t.Fatalf("expected 1 match (recycle+2 outside), got %d", planSummary.Matches)
	}

	applyUC := app.ApplyUseCase{Stdout: &bytes.Buffer{}}
	execSummary, err := applyUC.Run(app.ApplyParams{PlanPath: planPath, Execute: true})
	if err != nil {
		t.Fatalf("apply execute failed: %v", err)
	}
	if execSummary.Deleted != 1 {
		t.Fatalf("expected 1 deleted, got %+v", execSummary)
	}
	// a/photo.jpg（辞書順最小）が残り、b/photo.jpg が削除される
	if _, err := os.Stat(filepath.Join(dir, "a", "photo.jpg")); err != nil {
		t.Fatalf("a/photo.jpg should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b", "photo.jpg")); !os.IsNotExist(err) {
		t.Fatalf("b/photo.jpg should be deleted")
	}
	// #recycle は触らない
	if _, err := os.Stat(filepath.Join(dir, "#recycle", "photo.jpg")); err != nil {
		t.Fatalf("#recycle/photo.jpg should remain: %v", err)
	}
}

func TestSelfDedupWithBIsError(t *testing.T) {
	planUC := app.PlanUseCase{}
	_, err := planUC.Run(app.PlanParams{
		AIndexPath: "a.jsonl",
		BIndexPath: "b.jsonl",
		Out:        "plan.jsonl",
		Self:       true,
	})
	if err == nil || !app.IsInputError(err) {
		t.Fatalf("expected InputError when --self and --b are both set, got: %v", err)
	}
}

func TestSelfDedupWithoutSelfAndNoBIsError(t *testing.T) {
	planUC := app.PlanUseCase{}
	_, err := planUC.Run(app.PlanParams{
		AIndexPath: "a.jsonl",
		Out:        "plan.jsonl",
		Self:       false,
	})
	if err == nil || !app.IsInputError(err) {
		t.Fatalf("expected InputError when --b is missing and --self is not set, got: %v", err)
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
