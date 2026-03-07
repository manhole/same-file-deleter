# same-file-deleter

`same-file-deleter` はディレクトリAとBを比較し、内容が一致するファイルをBから削除するためのツールです。  
運用は `index -> plan -> apply` の3ステップで行います。

## 使い方

`checksums.jsonl` を使って事前に比較情報を作る。

```bash
sfd index --dir /path/A --out A.checksums.jsonl --update
sfd index --dir /path/B --out B.checksums.jsonl --update
```

A/Bのインデックスから削除候補を作成する。

```bash
sfd plan --a A.checksums.jsonl --b B.checksums.jsonl --out delete-plan.jsonl
```

候補を確認し、実行する。

```bash
sfd apply --plan delete-plan.jsonl --dry-run
sfd apply --plan delete-plan.jsonl --execute
```

- `apply` は既定でdry-run（削除なし）
- `.git` は除外、シンボリックリンクは対象外
- B側で一致が複数ある場合は全件を削除候補化

## インストール済みコマンドの利用

1. 配布先からOS/アーキテクチャに合うバイナリを取得する
   - macOS: `sfd`
   - Windows: `sfd.exe`
2. 実行権限を付与（macOS/Linux のみ）

```bash
chmod +x sfd
./sfd --help
```

## 開発者向け情報

開発者向けのビルドや実行手順は [DEVELOPER.md](/Users/manhole/project/same-file-deleter/DEVELOPER.md) を参照してください。
