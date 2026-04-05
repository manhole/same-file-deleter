# same-file-deleter

[![CI](https://github.com/manhole/same-file-deleter/actions/workflows/ci.yml/badge.svg)](https://github.com/manhole/same-file-deleter/actions/workflows/ci.yml)

`same-file-deleter` is a CLI tool that detects and deletes duplicate files by comparing file contents (checksums).
It operates in three steps: `index -> plan -> apply`.

- **Set mode**: Compare directories A and B; delete files from B whose contents match any file in A (regardless of path)
- **Path-match mode (`--match-path`)**: Delete files from B that have both the same path and the same contents as files in A
- **Self-dedup mode (`--self`)**: Detect duplicate files within directory A and delete all but one per group

## Installation

### go install

```bash
go install github.com/manhole/same-file-deleter/cmd/sfd@latest
```

### Binary download

Download the binary for your OS/architecture from the [Releases](https://github.com/manhole/same-file-deleter/releases) page.

On macOS/Linux, grant execute permission:

```bash
chmod +x sfd
./sfd --help
```

## Usage

### 1. Build a checksum index for each directory

```bash
sfd index --dir /path/A --out A.checksums.jsonl
sfd index --dir /path/B --out B.checksums.jsonl
```

Add `--update` to reuse an existing index and only re-hash changed files (faster on subsequent runs):

```bash
sfd index --dir /path/A --out A.checksums.jsonl --update
sfd index --dir /path/B --out B.checksums.jsonl --update
```

### 2. Generate a delete plan

**Set mode**: Delete from B any file whose content matches a file in A (path does not need to match)

```bash
sfd plan --a A.checksums.jsonl --b B.checksums.jsonl --out delete-plan.jsonl
```

**Path-match mode**: Delete from B files that have both the same path and the same content as files in A

```bash
sfd plan --a A.checksums.jsonl --b B.checksums.jsonl --match-path --out delete-plan.jsonl
```

Useful after an interrupted file move or to remove unchanged files from a folder backup.
Files with the same path but different content (modified) are not included.

**Self-dedup mode**: Detect and delete duplicates within A

```bash
sfd plan --a A.checksums.jsonl --self --out delete-plan.jsonl
```

In `--self` mode, the file with the lexicographically smallest path is kept per group; all others become deletion candidates.

### 3. Review and execute

```bash
sfd apply --plan delete-plan.jsonl --dry-run
sfd apply --plan delete-plan.jsonl --execute
```

### Notes

- `apply` defaults to dry-run (no deletion). Deletion only occurs when `--execute` is explicitly specified
- `--max-delete <n>` stops execution if the number of candidates exceeds n (protection against mistakes)
- `.git` is excluded by default; use `sfd index --include-all` to include it
- Symbolic links are not followed
- If multiple files in B match, all of them become deletion candidates

## Documentation

For background and detailed specifications, read in this order:

1. [EXISTING_TOOLS.md](EXISTING_TOOLS.md) — Why this tool exists (comparison with existing tools)
2. [DESIGN.md](DESIGN.md) — Feature requirements and specifications
3. [ARCHITECTURE.md](ARCHITECTURE.md) — Implementation architecture (for contributors)
4. [DEVELOPER.md](DEVELOPER.md) — Build and test instructions

Japanese versions: [README.ja.md](README.ja.md), [DESIGN.ja.md](DESIGN.ja.md), [ARCHITECTURE.ja.md](ARCHITECTURE.ja.md), [DEVELOPER.ja.md](DEVELOPER.ja.md), [EXISTING_TOOLS.ja.md](EXISTING_TOOLS.ja.md)

## License

MIT License — see [LICENSE](LICENSE)
