package app_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"same-file-deleter/internal/app"
	"same-file-deleter/internal/domain"
	"same-file-deleter/internal/infra"
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

	var planStdout bytes.Buffer
	planUC := app.PlanUseCase{Stdout: &planStdout}
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

	// plan JSONL の kept_path が "a/photo.jpg" であることを確認
	if err := infra.ReadPlanJSONL(planPath, func(rec domain.PlanRecord) error {
		if rec.KeptPath != "a/photo.jpg" {
			t.Errorf("expected KeptPath=a/photo.jpg, got %q", rec.KeptPath)
		}
		return nil
	}); err != nil {
		t.Fatalf("read plan jsonl failed: %v", err)
	}

	// stdout にグループ情報が含まれることを確認
	out := planStdout.String()
	if !strings.Contains(out, "a/photo.jpg [keep]") {
		t.Errorf("stdout should contain keep line, got:\n%s", out)
	}
	if !strings.Contains(out, "delete: b/copy.jpg") {
		t.Errorf("stdout should contain delete line, got:\n%s", out)
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

	planUC := app.PlanUseCase{Stdout: &bytes.Buffer{}}
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

	// 削除レコード2件すべての KeptPath が "a/x.txt" であることを確認
	if err := infra.ReadPlanJSONL(planPath, func(rec domain.PlanRecord) error {
		if rec.KeptPath != "a/x.txt" {
			t.Errorf("expected KeptPath=a/x.txt, got %q", rec.KeptPath)
		}
		return nil
	}); err != nil {
		t.Fatalf("read plan jsonl failed: %v", err)
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

func TestSelfDedupPlanOutput(t *testing.T) {
	// plan --self の stdout にグループ情報（[keep] / delete:）が出力されることを確認
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "D")

	mustWriteFile(t, filepath.Join(dir, "a", "x.txt"), "dup")
	mustWriteFile(t, filepath.Join(dir, "b", "x.txt"), "dup")
	mustWriteFile(t, filepath.Join(dir, "c", "x.txt"), "dup")

	indexPath := filepath.Join(tmp, "D.checksums.jsonl")
	planPath := filepath.Join(tmp, "D.plan.jsonl")

	if _, err := (app.IndexUseCase{}).Run(app.IndexParams{Dir: dir, Out: indexPath}); err != nil {
		t.Fatalf("index failed: %v", err)
	}

	var stdout bytes.Buffer
	planUC := app.PlanUseCase{Stdout: &stdout}
	if _, err := planUC.Run(app.PlanParams{AIndexPath: indexPath, Out: planPath, Self: true}); err != nil {
		t.Fatalf("plan failed: %v", err)
	}

	out := stdout.String()
	// keep ファイル（辞書順最小）と delete ファイル2件がグループ出力される
	if !strings.Contains(out, "a/x.txt [keep]") {
		t.Errorf("stdout should contain keep line, got:\n%s", out)
	}
	if !strings.Contains(out, "delete: b/x.txt") {
		t.Errorf("stdout should contain delete b/x.txt, got:\n%s", out)
	}
	if !strings.Contains(out, "delete: c/x.txt") {
		t.Errorf("stdout should contain delete c/x.txt, got:\n%s", out)
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

// TestMatchPathBasic は --match-path の基本動作を確認する。
// A と B でパスが同じかつ内容が同じファイルだけが削除候補になる。
func TestMatchPathBasic(t *testing.T) {
	tmp := t.TempDir()
	aDir := filepath.Join(tmp, "A")
	bDir := filepath.Join(tmp, "B")

	// パス・内容ともに一致 → 削除候補
	mustWriteFile(t, filepath.Join(aDir, "sub", "same.txt"), "same-content")
	mustWriteFile(t, filepath.Join(bDir, "sub", "same.txt"), "same-content")

	// パスが同じだが内容が違う → 削除候補にならない
	mustWriteFile(t, filepath.Join(aDir, "modified.txt"), "version-A")
	mustWriteFile(t, filepath.Join(bDir, "modified.txt"), "version-B")

	// 内容が同じだがパスが違う → --match-path では削除候補にならない
	mustWriteFile(t, filepath.Join(aDir, "original.txt"), "shared-content")
	mustWriteFile(t, filepath.Join(bDir, "renamed.txt"), "shared-content")

	// B にしか存在しないファイル → 削除候補にならない
	mustWriteFile(t, filepath.Join(bDir, "b-only.txt"), "b-only")

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
	planSummary, err := planUC.Run(app.PlanParams{
		AIndexPath: aIndex,
		BIndexPath: bIndex,
		Out:        planPath,
		MatchPath:  true,
	})
	if err != nil {
		t.Fatalf("plan --match-path failed: %v", err)
	}
	// sub/same.txt だけが候補
	if planSummary.Matches != 1 {
		t.Fatalf("expected 1 match, got %d", planSummary.Matches)
	}

	// dry-run でファイルが残ることを確認
	applyUC := app.ApplyUseCase{Stdout: &bytes.Buffer{}}
	drySummary, err := applyUC.Run(app.ApplyParams{PlanPath: planPath, Execute: false})
	if err != nil {
		t.Fatalf("apply dry-run failed: %v", err)
	}
	if drySummary.Candidates != 1 || drySummary.Deleted != 0 {
		t.Fatalf("unexpected dry-run summary: %+v", drySummary)
	}

	// execute で sub/same.txt だけ削除される
	execSummary, err := applyUC.Run(app.ApplyParams{PlanPath: planPath, Execute: true})
	if err != nil {
		t.Fatalf("apply execute failed: %v", err)
	}
	if execSummary.Deleted != 1 {
		t.Fatalf("expected 1 deleted, got %+v", execSummary)
	}
	if _, err := os.Stat(filepath.Join(bDir, "sub", "same.txt")); !os.IsNotExist(err) {
		t.Fatal("sub/same.txt should be deleted")
	}
	// パスが同じでも内容が違うファイルは残る
	if _, err := os.Stat(filepath.Join(bDir, "modified.txt")); err != nil {
		t.Fatalf("modified.txt should remain: %v", err)
	}
	// 内容が同じでもパスが違うファイルは残る
	if _, err := os.Stat(filepath.Join(bDir, "renamed.txt")); err != nil {
		t.Fatalf("renamed.txt should remain: %v", err)
	}
}

// TestMatchPathVsABMode は --match-path と通常 A/B モードの違いを確認する。
// 通常モードはパスが違っても内容一致で削除候補にするが、--match-path はしない。
func TestMatchPathVsABMode(t *testing.T) {
	tmp := t.TempDir()
	aDir := filepath.Join(tmp, "A")
	bDir := filepath.Join(tmp, "B")

	// A に only_a.txt、B に different_path.txt として同じ内容を置く
	mustWriteFile(t, filepath.Join(aDir, "only_a.txt"), "shared-content")
	mustWriteFile(t, filepath.Join(bDir, "different_path.txt"), "shared-content")

	aIndex := filepath.Join(tmp, "A.checksums.jsonl")
	bIndex := filepath.Join(tmp, "B.checksums.jsonl")

	indexUC := app.IndexUseCase{}
	if _, err := indexUC.Run(app.IndexParams{Dir: aDir, Out: aIndex}); err != nil {
		t.Fatalf("index A failed: %v", err)
	}
	if _, err := indexUC.Run(app.IndexParams{Dir: bDir, Out: bIndex}); err != nil {
		t.Fatalf("index B failed: %v", err)
	}

	planUC := app.PlanUseCase{}

	// 通常 A/B モード: パスが違っても内容一致なので1件候補になる
	normalPlan := filepath.Join(tmp, "normal-plan.jsonl")
	normalSummary, err := planUC.Run(app.PlanParams{
		AIndexPath: aIndex,
		BIndexPath: bIndex,
		Out:        normalPlan,
	})
	if err != nil {
		t.Fatalf("normal plan failed: %v", err)
	}
	if normalSummary.Matches != 1 {
		t.Fatalf("normal mode: expected 1 match, got %d", normalSummary.Matches)
	}

	// --match-path モード: パスが違うので候補なし
	matchPlan := filepath.Join(tmp, "match-plan.jsonl")
	matchSummary, err := planUC.Run(app.PlanParams{
		AIndexPath: aIndex,
		BIndexPath: bIndex,
		Out:        matchPlan,
		MatchPath:  true,
	})
	if err != nil {
		t.Fatalf("match-path plan failed: %v", err)
	}
	if matchSummary.Matches != 0 {
		t.Fatalf("match-path mode: expected 0 matches, got %d", matchSummary.Matches)
	}
}

// TestMatchPathSkipsRecycleBin は --match-path でも #recycle 内は除外されることを確認する。
func TestMatchPathSkipsRecycleBin(t *testing.T) {
	tmp := t.TempDir()
	aDir := filepath.Join(tmp, "A")
	bDir := filepath.Join(tmp, "B")

	mustWriteFile(t, filepath.Join(aDir, "#recycle", "photo.jpg"), "image-data")
	mustWriteFile(t, filepath.Join(bDir, "#recycle", "photo.jpg"), "image-data")

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
	planSummary, err := planUC.Run(app.PlanParams{
		AIndexPath: aIndex,
		BIndexPath: bIndex,
		Out:        planPath,
		MatchPath:  true,
	})
	if err != nil {
		t.Fatalf("plan --match-path failed: %v", err)
	}
	if planSummary.Matches != 0 {
		t.Fatalf("expected 0 matches (#recycle excluded), got %d", planSummary.Matches)
	}
}

// TestMatchPathWithSelfIsError は --match-path と --self の併用がエラーになることを確認する。
func TestMatchPathWithSelfIsError(t *testing.T) {
	planUC := app.PlanUseCase{}
	_, err := planUC.Run(app.PlanParams{
		AIndexPath: "a.jsonl",
		Out:        "plan.jsonl",
		Self:       true,
		MatchPath:  true,
	})
	if err == nil || !app.IsInputError(err) {
		t.Fatalf("expected InputError when --self and --match-path are both set, got: %v", err)
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
