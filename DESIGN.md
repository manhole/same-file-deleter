# same-file-deleter 設計書

## 1. 目的
- ディレクトリAとディレクトリBを比較し、**Aに存在するファイル内容と一致するファイル**をBから削除する。
- A/Bで同じ比較を繰り返す運用を想定し、ファイル実体の再読込を減らすために、各ディレクトリごとのチェックサム情報を保存・再利用する。
- 処理は以下を分離する。
1. チェックサム情報の作成/更新
2. チェックサム情報同士の比較（削除候補抽出）
3. 実削除

## 2. スコープ
- 対象: ローカルファイルシステム上の通常ファイル
- 非対象（初期版）:
  - リモートストレージ連携（S3等）
  - ディレクトリ自体の削除

## 3. 用語
- `checksum index file`: ディレクトリ走査結果（path/size/mtime/checksum等）を保存したファイル。
- `plan file`: 比較結果として得られる削除候補一覧ファイル。

補足:
- 一般にはこの種の索引ファイルを `manifest` と呼ぶことが多い。
- 本設計では分かりやすさ優先で `checksum index file` を主名称とし、必要な箇所でのみ `manifest` を併記する。

## 4. 要件
### 4.1 機能要件
1. 指定ディレクトリのchecksum index fileを作成/更新できる
2. A用とB用のchecksum index fileを比較し、B側の削除候補を抽出できる
3. B側で一致ファイルが複数ある場合は、該当ファイルをすべて削除対象にできる
4. plan fileを使ってB側のファイルを削除できる
5. 削除前にドライランで確認できる

### 4.2 非機能要件
1. 大量ファイルを扱える（メモリ使用量はファイル数に対して線形以下に抑える）
2. 再実行時に高速（未変更ファイルは再ハッシュしない）
3. 安全性（誤削除防止: dry-run, plan確認, パス検証）
4. 再現性（同じindex同士なら同じplanを出力）
5. 低コストでのクロスプラットフォーム対応（macOS優先、可能ならWindows対応）

## 5. コマンド設計（提案）
CLI名は仮に `sfd` とする。

### 5.1 `sfd index`
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

### 5.2 `sfd plan`
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

### 5.3 `sfd apply`
planに基づき実削除する。

例:
```bash
sfd apply --plan .cache/A_to_B.delete-plan.jsonl --dry-run
sfd apply --plan .cache/A_to_B.delete-plan.jsonl --execute
```

主なオプション:
- `--plan <path>`: plan file
- `--dry-run`: 削除せず一覧表示（初期動作）
- `--execute`: 実削除を実行
- `--max-delete <n>`: 誤操作防止の上限

## 6. データフォーマット
大量データを想定し、1行1JSONの `JSONL` を採用する。

### 6.1 checksum indexレコード例
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
{"b_root":"/data/B","path":"sub/x.txt","reason":"checksum_match_with_A","checksum":"ab12...","size":1234}
```

### 6.3 ファイル名ルール（提案）
- 拡張子は `checksums.jsonl` を推奨（JSONLのみ）
- A/Bを同一作業ディレクトリで扱う場合は `A.checksums.jsonl` / `B.checksums.jsonl` でもよい
- 出力先は `--out` で明示指定する（必須）
- CSVはMVPではサポートしない

## 7. 処理詳細
### 7.1 index処理
1. ディレクトリを再帰走査
2. 各ファイルで `path,size,mtime_ns` を取得
3. `.git` を除外し、シンボリックリンクは無視
4. `--update` 時は既存indexを辞書化し、`size+mtime_ns` が同一ならchecksum再利用
5. 対象ディレクトリ内にチェックサムファイルが存在しても自動除外しない
6. 変更/新規ファイルのみ再ハッシュ
7. ハッシュ方式は `blake3` 固定（MVP）
8. 新indexを出力（アトミックに置換）

### 7.2 plan処理
1. A-indexを読み、`(algo, checksum, size)` をキーに集合化
2. B-indexを走査し、同キーがA集合にあれば削除候補としてplanへ出力
3. 同一キーに該当するB側ファイルはすべてplanへ出力
4. 統計情報を出力（対象件数、合計サイズ、スキップ件数）

### 7.3 apply処理
1. planの各パスがBルート配下か検証（パストラバーサル防止）
2. `--dry-run` では削除対象一覧/件数のみ表示
3. `--execute` で削除実行（失敗は集約して最後に報告）
4. plan作成後のファイル再検証（再checksum）は行わない
5. 実行ログ（timestamp, path, result）を保存可能にする

## 8. 安全設計
- デフォルトは `dry-run`
- `--execute` 明示時のみ削除
- `plan` を人間確認可能なテキスト/JSONLで保持
- `--max-delete` 超過時は停止
- Bルート外のパスは即エラー
- 削除はゴミ箱退避ではなく、通常のファイル削除を行う

## 9. パフォーマンス設計
- ハッシュ計算はワーカープールで並列化（CPUコア数ベース）
- ファイル読み込みはストリーム処理
- index比較はA側をハッシュ集合化してO(1)照合
- 一致判定は `checksum+size`（サイズ比較は低コストの追加確認）
- 巨大Aに備えて将来はSQLite backendを選択可能にする

## 10. エラー処理方針
- 1ファイルの失敗で全体停止しない（集約して終了時にサマリ）
- 読み取り不可ファイルはエラー記録して継続し、最後に非0で終了する
- ただしindex入出力失敗・フォーマット破損は即停止
- 終了コード:
  - `0`: 正常
  - `1`: 実行時エラー（個別ファイルエラーを含む）
  - `2`: 入力不正（オプション/フォーマット整合性）

## 11. テスト戦略
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

## 12. 最小実装（MVP）範囲
1. `sfd index`（`--update` 対応）
2. `sfd plan`（A/B固定）
3. `sfd apply`（dry-run/execute）
4. JSONL index/plan

## 13. 将来拡張
- sha256等の追加アルゴリズム対応
- SQLite保存（より高速な差分更新）
- 削除ではなく隔離（trash）標準化
- TUI/GUIフロントエンド
