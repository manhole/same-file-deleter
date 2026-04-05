# Developer Guide: same-file-deleter

This document covers build, run, and test procedures for developers.

## Running without building

If Go is installed, you can run the source directly without building:

```bash
cd /path/to/same-file-deleter
go run ./cmd/sfd --help
```

## Development workflow

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

`apply` defaults to dry-run. Add `--execute` to perform actual deletion.

## Running tests

```bash
go test ./...
```

## Formatting code

```bash
gofmt -w $(go list -f '{{.Dir}}' ./...)
```

## Building

Git tags are the single source of truth for versions. The version is embedded automatically at build time.

```bash
cd /path/to/same-file-deleter
go build -ldflags="-X main.version=$(git describe --tags --always)" ./cmd/sfd
./sfd version
```

If there is no tag, or if there are commits after the tag, the version string includes the commit hash:

```
v1.2.3              ← commit exactly at tag
v1.2.3-3-gabc1234   ← 3 commits after tag
abc1234             ← no tag
```

Building without `-ldflags` displays `dev`:

```bash
go build ./cmd/sfd
./sfd version   # → sfd version dev
```

## Release procedure

```bash
git tag v1.2.3
git push origin v1.2.3
# GitHub Actions will build and publish release binaries automatically
```

To install the built binary locally:

```bash
mkdir -p ~/bin
mv sfd ~/bin/sfd
~/bin/sfd --help
```

## Notes

- Go version: 1.22+ recommended
- Hash algorithm: blake3 (fixed)
- `.git` excluded by default, symlinks ignored, dry-run is the default
- `--out` is required
