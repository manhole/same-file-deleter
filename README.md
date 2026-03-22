# same-file-deleter

`same-file-deleter` はディレクトリAとBを比較し、内容が一致するファイルをBから削除するためのツールです。
運用は `index -> plan -> apply` の3ステップで行います。

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

```bash
sfd plan --a A.checksums.jsonl --b B.checksums.jsonl --out delete-plan.jsonl
```

### 3. 候補を確認し、実行する

```bash
sfd apply --plan delete-plan.jsonl --dry-run
sfd apply --plan delete-plan.jsonl --execute
```

### 注意事項

- `apply` は既定でdry-run（削除なし）。`--execute` を明示した場合のみ削除します
- `--max-delete <n>` を指定すると、削除候補がn件を超えた場合に停止します（誤操作防止）
- `.git` は除外、シンボリックリンクは対象外
- B側で一致が複数ある場合は全件を削除候補化

## 開発者向け情報

開発者向けのビルドや実行手順は [DEVELOPER.md](DEVELOPER.md) を参照してください。

## 関連ドキュメント

- [DESIGN.md](DESIGN.md) — 機能要件・設計書
- [ARCHITECTURE.md](ARCHITECTURE.md) — 実装アーキテクチャ
- [EXISTING_TOOLS.md](EXISTING_TOOLS.md) — 既存ツール（rmlint/fclones）との違い
