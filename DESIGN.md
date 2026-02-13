# same-file-deleter 設計書

## 1. 目的
- ディレクトリAとディレクトリBを比較し、**Aに存在するファイル内容と一致するファイル**をBから削除する。
- Bは `B1, B2, B3...` のように複数パターンで繰り返し比較されるため、毎回ファイル実体を全読込しない運用を可能にする。
- 処理は以下を分離する。
1. チェックサム情報の作成/更新
2. チェックサム同士の比較（削除候補抽出）
3. 実削除

## 2. スコープ
- 対象: ローカルファイルシステム上の通常ファイル
- 非対象（初期版）:
  - リモートストレージ連携（S3等）
  - 同名比較のみ（内容比較が主）
  - ディレクトリ自体の削除

## 3. 用語
- `manifest`: ディレクトリ走査結果（パス/サイズ/mtime/checksum等）を保存したファイル
- `plan`: 比較結果として得られる削除候補一覧ファイル
- `A-manifest`: 削除基準側（残す側）のmanifest
- `B-manifest`: 削除対象側のmanifest

## 4. 要件
### 4.1 機能要件
1. 指定ディレクトリのmanifestを作成/更新できる
2. A-manifest と B-manifest を比較し、B側の削除候補を抽出できる
3. planを使ってB側のファイルを削除できる
4. 削除前にドライランで確認できる
5. Bが複数ある運用を想定し、A-manifestを再利用できる

### 4.2 非機能要件
1. 大量ファイルを扱える（メモリ使用量はファイル数に対して線形以下に抑える）
2. 再実行時に高速（未変更ファイルは再ハッシュしない）
3. 安全性（誤削除防止: dry-run, plan確認, パス検証）
4. 再現性（同じmanifest同士なら同じplanを出力）

## 5. コマンド設計（提案）
CLI名は仮に `sfd` とする。

### 5.1 `sfd index`
ディレクトリを走査してmanifestを作成/更新する。

例:
```bash
sfd index --dir /data/A --manifest .cache/A.manifest.jsonl
sfd index --dir /data/B1 --manifest .cache/B1.manifest.jsonl
```

主なオプション:
- `--dir <path>`: 対象ディレクトリ
- `--manifest <path>`: 出力manifestパス
- `--algo <blake3|sha256>`: ハッシュアルゴリズム（初期値: `blake3`）
- `--update`: 既存manifestを読み、未変更ファイルのchecksum再利用
- `--exclude <glob>`: 除外パターン（複数指定可）
- `--follow-symlink`: シンボリックリンクを辿る（初期値は辿らない）

### 5.2 `sfd plan`
A/Bのmanifestを比較し、削除候補planを作る。

例:
```bash
sfd plan \
  --a .cache/A.manifest.jsonl \
  --b .cache/B1.manifest.jsonl \
  --out .cache/plan_B1.jsonl
```

複数B対応例:
```bash
sfd plan \
  --a .cache/A.manifest.jsonl \
  --b .cache/B1.manifest.jsonl \
  --b .cache/B2.manifest.jsonl \
  --out-dir .cache/plans
```

主なオプション:
- `--a <manifest>`: A-manifest
- `--b <manifest>`: B-manifest（複数指定可）
- `--out <path>`: 単一plan出力
- `--out-dir <path>`: Bごとにplan出力
- `--strict`: size+checksum一致に加えてファイル種別等も厳密確認

### 5.3 `sfd apply`
planに基づき実削除する。

例:
```bash
sfd apply --plan .cache/plan_B1.jsonl --dry-run
sfd apply --plan .cache/plan_B1.jsonl --execute
```

主なオプション:
- `--plan <path>`: planファイル
- `--dry-run`: 削除せず一覧表示（初期動作）
- `--execute`: 実削除を実行
- `--trash-dir <path>`: 直接削除せず退避（任意）
- `--max-delete <n>`: 誤操作防止の上限

## 6. データフォーマット
大量データを想定し、1行1JSONの `JSONL` を採用する。

### 6.1 manifestレコード例
```json
{"path":"sub/x.txt","size":1234,"mtime_ns":1739420000000000000,"algo":"blake3","checksum":"ab12...","type":"file"}
```

フィールド:
- `path`: 走査rootからの相対パス
- `size`: バイト数
- `mtime_ns`: 更新時刻（ナノ秒）
- `algo`: ハッシュ方式
- `checksum`: ファイル内容ハッシュ
- `type`: 現在は `file` のみ

### 6.2 planレコード例
```json
{"b_root":"/data/B1","path":"sub/x.txt","reason":"checksum_match_with_A","checksum":"ab12...","size":1234}
```

## 7. 処理詳細
### 7.1 index処理
1. ディレクトリを再帰走査
2. 各ファイルで `path,size,mtime_ns` を取得
3. `--update` 時は既存manifestを辞書化し、`size+mtime_ns` が同一ならchecksum再利用
4. 変更/新規ファイルのみ再ハッシュ
5. 新manifestを出力（アトミックに置換）

### 7.2 plan処理
1. A-manifestを読み、`(algo, checksum, size)` をキーに集合化
2. B-manifestを走査し、同キーがA集合にあれば削除候補としてplanへ出力
3. 統計情報を出力（対象件数、合計サイズ、スキップ件数）

### 7.3 apply処理
1. planの各パスがBルート配下か検証（パストラバーサル防止）
2. `--dry-run` では削除対象一覧/件数のみ表示
3. `--execute` で削除実行（失敗は集約して最後に報告）
4. 実行ログ（timestamp, path, result）を保存可能にする

## 8. 安全設計
- デフォルトは `dry-run`
- `--execute` 明示時のみ削除
- `plan` を人間確認可能なテキスト/JSONLで保持
- `--max-delete` 超過時は停止
- Bルート外のパスは即エラー

## 9. パフォーマンス設計
- ハッシュ計算はワーカープールで並列化（CPUコア数ベース）
- ファイル読み込みはストリーム処理
- manifest比較はA側をハッシュ集合化してO(1)照合
- 巨大Aに備えて将来はSQLite backendを選択可能にする

## 10. エラー処理方針
- 1ファイルの失敗で全体停止しない（集約して終了時にサマリ）
- ただしmanifest入出力失敗・フォーマット破損は即停止
- 終了コード:
  - `0`: 正常
  - `1`: 実行時エラー
  - `2`: 入力不正（オプション/manifest整合性）

## 11. テスト戦略
1. 単体テスト
  - checksum計算
  - manifest読み書き
  - plan判定ロジック
2. 結合テスト
  - A/B小規模サンプルで `index -> plan -> apply`
  - `dry-run` と `execute` の差分検証
3. 負荷テスト
  - 大量小ファイル
  - 少数巨大ファイル

## 12. 最小実装（MVP）範囲
1. `sfd index`（`--update` 対応）
2. `sfd plan`（単一B対応）
3. `sfd apply`（dry-run/execute）
4. JSONL manifest/plan

## 13. 将来拡張
- 複数B一括処理の最適化
- SQLite保存（より高速な差分更新）
- 削除ではなく隔離（trash）標準化
- TUI/GUIフロントエンド
