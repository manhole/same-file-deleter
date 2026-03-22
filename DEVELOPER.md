# 開発者向け: same-file-deleter

このドキュメントは開発者向けに、ビルド・実行・テスト手順を説明します。

## go run で実行する

Goがある環境で、ビルドせずにソースを直接実行できます。

```bash
cd /path/to/same-file-deleter
go run ./cmd/sfd --help
```

## 開発時の利用フロー

```bash
go run ./cmd/sfd index \
  --dir /path/A \
  --out /tmp/A.checksums.jsonl \
  --update

go run ./cmd/sfd index \
  --dir /path/B \
  --out /tmp/B.checksums.jsonl \
  --update

go run ./cmd/sfd plan \
  --a /tmp/A.checksums.jsonl \
  --b /tmp/B.checksums.jsonl \
  --out /tmp/delete-plan.jsonl

go run ./cmd/sfd apply \
  --plan /tmp/delete-plan.jsonl
```

`apply` は既定でdry-run。実削除する場合は `--execute` を追加します。

## テストを実行する

```bash
go test ./...
```

## コードをフォーマットする

```bash
gofmt -w $(go list -f '{{.Dir}}' ./...)
```

## ビルドする

```bash
cd /path/to/same-file-deleter
go build ./cmd/sfd
./sfd --help
```

生成された `sfd` バイナリを `~/bin` などに置けば、通常のコマンドとして利用できます。

```bash
mkdir -p ~/bin
mv sfd ~/bin/sfd
~/bin/sfd --help
```

## 補足

- Goバージョン: 1.22+ を推奨
- ハッシュは blake3 固定
- `.git` 除外、symlink 無視、dry-run既定
- `--out` は必須
