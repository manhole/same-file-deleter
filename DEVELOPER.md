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

git タグがバージョンの唯一の情報源です。ビルド時に自動で埋め込まれます。

```bash
cd /path/to/same-file-deleter
go build -ldflags="-X main.version=$(git describe --tags --always)" ./cmd/sfd
./sfd version
```

タグがない場合やタグ後にコミットがある場合は、コミットハッシュを含む文字列になります:

```
v1.2.3          ← タグと一致するコミット
v1.2.3-3-gabc1234  ← タグ後に3コミット
abc1234         ← タグなし
```

`-ldflags` なしでビルドすると `dev` と表示されます。

```bash
go build ./cmd/sfd
./sfd version   # → sfd version dev
```

## リリース手順

```bash
git tag v1.2.3
git push origin v1.2.3
# あとはビルド時に git describe が v1.2.3 を返す
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
