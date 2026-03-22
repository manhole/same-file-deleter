# same-file-deleter 設計書

## 1. 目的

ディレクトリAとディレクトリBを比較し、**Aに存在するファイル内容と一致するファイル**をBから削除する。

- 同じA/Bを繰り返し比較する運用を想定し、チェックサム情報をファイルとして保存・再利用することでファイル実体の再読込を減らす
- 削除候補抽出と実削除を分離し、`plan -> dry-run -> execute` の手順で確認しながら実行できるようにする
- 既存ツールとの位置づけの違いは `EXISTING_TOOLS.md` を参照する

## 2. 用語

- `checksum index file`: ディレクトリ走査結果（path/size/mtime/checksum等）を保存したファイル。
- `plan file`: 比較結果として得られる削除候補一覧ファイル。

補足:
- 一般にはこの種の索引ファイルを `manifest` と呼ぶことが多い。
- 本設計では分かりやすさ優先で `checksum index file` を主名称とし、必要な箇所でのみ `manifest` を併記する。

## 3. 代表ユースケース

1. **初回運用**:
   - A/Bそれぞれに対して `sfd index --out ...` を実行し、`sfd plan` で削除候補を作成する。
   - `sfd apply --dry-run` で候補件数と対象パスを確認し、問題なければ `sfd apply --execute` を実行する。
2. **同じA/Bでの再実行**:
   - AまたはBに変更が入った後、対象ディレクトリに対して `sfd index --update` を再実行する。
   - 未変更ファイルは既存indexのchecksumを再利用し、新規/変更ファイルだけを再ハッシュして `sfd plan` と `sfd apply` を回す。
3. **安全重視の削除運用**:
   - 削除対象の確定は常にplan file経由で行い、いきなり比較結果を削除へ直結しない。
   - 実削除前にdry-runを標準手順とし、誤削除時の影響を減らす。

## 4. スコープ

- 対象: ローカルファイルシステム上の通常ファイル
- 非対象（初期版）:
  - リモートストレージ連携（S3等）
  - ディレクトリ自体の削除

## 5. 要件

### 5.1 機能要件

1. 指定ディレクトリのchecksum index fileを作成/更新できる
2. A用とB用のchecksum index fileを比較し、B側の削除候補を抽出できる
3. B側で一致ファイルが複数ある場合は、該当ファイルをすべて削除対象にできる
4. plan fileを使ってB側のファイルを削除できる
5. 削除前にドライランで確認できる

### 5.2 非機能要件

1. 大量ファイルを扱える（メモリ使用量はファイル数に対して線形以下に抑える）
2. 再実行時に高速（未変更ファイルは再ハッシュしない）
3. 安全性（誤削除防止: dry-run, plan確認, パス検証）
4. 再現性（同じindex同士なら同じplanを出力）
5. 低コストでのクロスプラットフォーム対応（macOS優先、可能ならWindows対応）

## 6. コマンド設計

CLIコマンド名は `sfd`。

### 6.1 `sfd index`

ディレクトリを走査してchecksum index fileを作成/更新する。

例:
```bash
sfd index --dir /data/A --out .cache/A.checksums.jsonl
sfd index --dir /data/B --out .cache/B.checksums.jsonl
```

主なオプション:
- `--dir <path>`: 対象ディレクトリ
- `--out <path>`: 出力ファイルパス（必須）
- `--update`: 既存indexを読み、未変更ファイルのchecksum再利用
- `--exclude <glob>`: 除外パターン（複数指定可）
- `.git` は常に除外
- シンボリックリンクは常に無視する

### 6.2 `sfd plan`

A/Bのchecksum index fileを比較し、削除候補planを作る。

例:
```bash
sfd plan \
  --a .cache/A.checksums.jsonl \
  --b .cache/B.checksums.jsonl \
  --out .cache/A_to_B.delete-plan.jsonl
```

主なオプション:
- `--a <file>`: A側checksum index file
- `--b <file>`: B側checksum index file
- `--out <path>`: plan出力

### 6.3 `sfd apply`

planに基づき実削除する。

例:
```bash
sfd apply --plan .cache/A_to_B.delete-plan.jsonl --dry-run
sfd apply --plan .cache/A_to_B.delete-plan.jsonl --execute
```

主なオプション:
- `--plan <path>`: plan file
- `--dry-run`: 削除せず一覧表示（既定）
- `--execute`: 実削除を実行
- `--max-delete <n>`: 削除候補がn件を超えた場合に停止（0は無制限）

## 7. データフォーマット

大量データを想定し、1行1JSONの `JSONL` を採用する。

### 7.1 checksum indexレコード例

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

### 7.2 planレコード例

```json
{"b_root":"/data/B","path":"sub/x.txt","reason":"checksum_match_with_A","checksum":"ab12...","size":1234}
```

### 7.3 ファイル名ルール

- 拡張子は `checksums.jsonl` を推奨（JSONLのみ）
- A/Bを同一作業ディレクトリで扱う場合は `A.checksums.jsonl` / `B.checksums.jsonl` でもよい
- 出力先は `--out` で明示指定する（必須）

## 8. 処理詳細

### 8.1 index処理

1. ディレクトリを再帰走査
2. 各ファイルで `path,size,mtime_ns` を取得
3. `.git` を除外し、シンボリックリンクは無視
4. `--update` 時は既存indexを辞書化し、`size+mtime_ns` が同一ならchecksum再利用
5. 対象ディレクトリ内にチェックサムファイルが存在しても自動除外しない
6. 変更/新規ファイルのみ再ハッシュ
7. ハッシュ方式は `blake3` 固定（MVP）
8. 新indexを出力（アトミックに置換）

### 8.2 plan処理

1. A-indexを読み、`(algo, checksum, size)` をキーに集合化
2. B-indexを走査し、同キーがA集合にあれば削除候補としてplanへ出力
3. 同一キーに該当するB側ファイルはすべてplanへ出力
4. 統計情報を出力（対象件数、合計サイズ、スキップ件数）

### 8.3 apply処理

1. planの各パスがBルート配下か検証（パストラバーサル防止）
2. `--dry-run` では削除対象一覧/件数のみ表示
3. `--execute` で削除実行（失敗は集約して最後に報告）
4. plan作成後のファイル再checksum検証は行わない

## 9. 安全設計

- デフォルトは `dry-run`
- `--execute` 明示時のみ削除
- `plan` を人間確認可能なテキスト/JSONLで保持
- `--max-delete` 超過時は停止
- Bルート外のパスは即エラー
- 削除はゴミ箱退避ではなく、通常のファイル削除を行う

## 10. パフォーマンス設計

- ハッシュ計算はワーカープールで並列化（CPUコア数ベース）
- ファイル読み込みはストリーム処理
- index比較はA側をハッシュ集合化してO(1)照合
- 一致判定は `checksum+size`（サイズ比較は低コストの追加確認）

## 11. エラー処理方針

- 1ファイルの失敗で全体停止しない（集約して終了時にサマリ）
- 読み取り不可ファイルはエラー記録して継続し、最後に非0で終了する
- ただしindex入出力失敗・フォーマット破損は即停止
- 終了コード:
  - `0`: 正常
  - `1`: 実行時エラー（個別ファイルエラーを含む）
  - `2`: 入力不正（オプション/フォーマット整合性）

## 12. テスト戦略

1. 単体テスト
  - checksum計算
  - index読み書き
  - plan判定ロジック
2. 結合テスト
  - A/B小規模サンプルで `index -> plan -> apply`
  - `dry-run` と `execute` の差分検証
3. 負荷テスト
  - 大量小ファイル
  - 少数巨大ファイル

## 13. 最小実装（MVP）範囲

1. `sfd index`（`--update` 対応）
2. `sfd plan`（A/B固定）
3. `sfd apply`（dry-run/execute）
4. JSONL index/plan

## 14. 将来拡張

- sha256等の追加アルゴリズム対応
- SQLite保存（より高速な差分更新）
- 削除ではなく隔離（trash）標準化
- TUI/GUIフロントエンド
