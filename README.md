# same-file-deleter

`same-file-deleter` はファイルの内容（チェックサム）を使って重複ファイルを検出・削除するツールです。
運用は `index -> plan -> apply` の3ステップで行います。

- **A/B比較モード**: ディレクトリAとBを比較し、Aにある内容と一致するファイルをBから削除する
- **自己重複検出モード（`--self`）**: ディレクトリA内の重複ファイルを検出し、1ファイルを残して他を削除する

## インストール

配布先からOS/アーキテクチャに合うバイナリを取得してください。

- macOS: `sfd`
- Windows: `sfd.exe`

macOS/Linux では実行権限を付与します。

```bash
chmod +x sfd
./sfd --help
```

## 使い方

### 1. 各ディレクトリのインデックスを作成する

```bash
sfd index --dir /path/A --out A.checksums.jsonl
sfd index --dir /path/B --out B.checksums.jsonl
```

`--update` を付けると既存インデックスを再利用し、変更ファイルだけ再ハッシュします（再実行時の速度向上）。

```bash
sfd index --dir /path/A --out A.checksums.jsonl --update
sfd index --dir /path/B --out B.checksums.jsonl --update
```

### 2. 削除候補を作成する

**A/B比較モード**: AにあるファイルをBから削除する場合

```bash
sfd plan --a A.checksums.jsonl --b B.checksums.jsonl --out delete-plan.jsonl
```

**自己重複検出モード**: A内の重複ファイルを削除する場合

```bash
sfd plan --a A.checksums.jsonl --self --out delete-plan.jsonl
```

`--self` モードでは、同一内容のファイルグループからパス辞書順で最小のファイルを残し、残りを削除候補にします。

### 3. 候補を確認し、実行する

```bash
sfd apply --plan delete-plan.jsonl --dry-run
sfd apply --plan delete-plan.jsonl --execute
```

### 注意事項

- `apply` は既定でdry-run（削除なし）。`--execute` を明示した場合のみ削除します
- `--max-delete <n>` を指定すると、削除候補がn件を超えた場合に停止します（誤操作防止）
- `.git` は除外、シンボリックリンクは対象外
- 一致ファイルが複数ある場合は全件を削除候補化

## 開発者向け情報

開発者向けのビルドや実行手順は [DEVELOPER.md](DEVELOPER.md) を参照してください。

## 関連ドキュメント

背景や仕様を詳しく知りたい場合は、以下の順に読むことを推奨します。

1. [EXISTING_TOOLS.md](EXISTING_TOOLS.md) — なぜこのツールが必要か（既存ツールとの違い）
2. [DESIGN.md](DESIGN.md) — 機能要件・仕様の詳細
3. [ARCHITECTURE.md](ARCHITECTURE.md) — 実装アーキテクチャ（コードを触る人向け）
4. [DEVELOPER.md](DEVELOPER.md) — ビルド・テスト手順
