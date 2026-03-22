# same-file-deleter アーキテクチャ設計

## 1. 目的

`DESIGN.md` の仕様を、実装可能なモジュール構成・データフロー・実行方式に落とし込む。

## 2. 技術選定

- 実装言語: Go（1.22+）
- 理由:
  - 単一バイナリ配布しやすい
  - macOS/Windowsの両対応が低コスト
  - 並列処理（goroutine）でハッシュ計算の並列化が容易
- ハッシュアルゴリズム: `blake3` 固定（MVP）

## 3. システム境界

- CLI単体アプリケーション（ローカル実行）
- 外部サービス・DBは使わない（MVP）
- 入力:
  - ファイルシステム上のA/Bディレクトリ
  - 既存の `checksums.jsonl` / `plan.jsonl`
- 出力:
  - `checksums.jsonl`
  - `delete-plan.jsonl`
  - 標準出力サマリと標準エラーのエラー詳細

## 4. レイヤ構成

- `cmd`: エントリーポイントと引数パース
- `internal/app`: ユースケース実行（index/plan/apply）
- `internal/domain`: エンティティ・判定ルール・ポリシー
- `internal/infra`: ファイルシステム、JSONL I/O、ハッシュ計算

依存方向:
- `cmd -> app -> domain`
- `app -> infra`
- `domain` は `infra` を参照しない

## 5. ディレクトリ構成

```text
cmd/
  sfd/
    main.go
internal/
  app/
    index_usecase.go
    plan_usecase.go
    apply_usecase.go
    errors.go
    e2e_test.go
  domain/
    model.go
    matcher.go
    matcher_test.go
    policy.go
  infra/
    fswalker.go
    jsonl_reader.go
    jsonl_writer.go
    blake3_hasher.go
    path_guard.go
    path_guard_test.go
    atomic_write.go
```

## 6. コマンド別フロー

### 6.1 `sfd index`

1. 引数検証（`--dir`, `--out` 必須）
2. 対象ディレクトリ再帰走査（`.git` 除外、symlink無視）
3. `--update` 指定時は既存indexを読み、`path -> (size, mtime_ns, checksum)` を構築
4. `size+mtime_ns` が一致するファイルはchecksum再利用、未一致のみ再ハッシュ
5. JSONLをテンポラリへ出力し、最後にアトミックリネーム
6. サマリ出力（走査件数、再利用件数、再ハッシュ件数、エラー件数）

### 6.2 `sfd plan`

1. 引数検証（`--a`, `--b`, `--out` 必須）
2. Aのindexを読み、`MatchKey` の集合を構築
3. Bのindexをストリーム読み込みし、キー一致行をplanへ書き出し
4. 一致したBレコードはすべて出力（重複ファイルを含む）
5. サマリ出力（一致件数、一致合計サイズ）

### 6.3 `sfd apply`

1. 引数検証（`--plan` 必須）
2. `--dry-run` をデフォルトにし、`--execute` 明示時のみ削除
3. planを1行ずつ読み、`b_root/path` を正規化してBルート配下か検証
4. `--dry-run` は一覧と件数のみ表示
5. `--execute` は通常削除（再checksum検証なし）
6. サマリ出力（成功件数、失敗件数、削除合計サイズ）

## 7. 並列処理と性能

- `index` のみハッシュ計算を並列化する（**現在は未実装 — 単一スレッドで動作**）。
- 予定方式:
  - 走査スレッド1本
  - ハッシュワーカーN本（`N = runtime.NumCPU()`）
- `plan`/`apply` はストリーム処理中心で、I/O優位のため単純実装を優先。
- メモリ使用:
  - `index --update`: 既存indexを `path` キーで保持（O(files_in_dir)）
  - `plan`: A側キー集合のみ保持（O(files_in_A)）

## 8. パス安全性

- `apply` で `filepath.Clean` + `filepath.Rel` を使用し、Bルート外アクセスを拒否
- 絶対パス・`..` 混入を防止
- Windowsでも同一ロジックで検証可能にする

## 9. JSONL I/O 方針

- 1行1JSONでストリーム読み書き
- 行単位の読み込み失敗時は行番号付きで報告
- 互換性:
  - 追加フィールドは無視可能に実装
  - 必須フィールド欠落は不正行として扱う

## 10. クロスプラットフォーム方針

- MVP対象: macOS
- 追加コスト小でWindows対応:
  - パス操作は `filepath` 統一
  - 区切り文字差異を吸収（内部処理は正規化）
  - OS依存APIは使わない

## 11. テストアーキテクチャ

- 単体テスト:
  - `matcher`（一致判定）
  - `path_guard`（Bルート逸脱防止）
  - `jsonl_reader/writer`
- 結合テスト:
  - `index -> plan -> apply` の一連フロー
  - `--dry-run` と `--execute` の差分
  - 読み取り不可ファイルが混在した場合の継続動作と終了コード
- 性能テスト:
  - 大量小ファイルで `--update` 再利用率を確認
