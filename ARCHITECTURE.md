# same-file-deleter Architecture

## 1. Purpose

Translate the specifications in `DESIGN.md` into a concrete module structure, data flow, and execution model.

## 2. Technology Choices

- Language: Go (1.22+)
- Rationale:
  - Easy single-binary distribution
  - Low-cost macOS/Windows cross-platform support
  - Parallel hash computation is straightforward with goroutines
- Hash algorithm: `blake3` fixed (MVP)

## 3. System Boundary

- Standalone CLI application (local execution)
- No external services or databases (MVP)
- Inputs:
  - A/B directories on the local filesystem
  - Existing `checksums.jsonl` / `plan.jsonl` files
- Outputs:
  - `checksums.jsonl`
  - `delete-plan.jsonl`
  - Summary on stdout; error details on stderr

## 4. Layer Structure

- `cmd`: Entry point and argument parsing
- `internal/app`: Use case execution (index/plan/apply)
- `internal/domain`: Entities, matching rules, policies
- `internal/infra`: Filesystem, JSONL I/O, hash computation

Dependency direction:
- `cmd -> app -> domain`
- `app -> infra`
- `domain` does not reference `infra`

## 5. Directory Layout

```text
cmd/
  sfd/
    main.go
internal/
  app/
    index_usecase.go
    plan_usecase.go
    apply_usecase.go
    errors.go
    e2e_test.go
  domain/
    model.go
    matcher.go
    matcher_test.go
    policy.go
  infra/
    fswalker.go
    jsonl_reader.go
    jsonl_writer.go
    blake3_hasher.go
    path_guard.go
    path_guard_test.go
    atomic_write.go
```

## 6. Per-Command Flow

### 6.1 `sfd index`

1. Validate arguments (`--dir`, `--out` required)
2. Recursively scan the target directory (default exclusion `.git` applied; disabled by `--include-all`; symlinks ignored)
3. With `--update`: read the existing index and build a `path -> (size, mtime_ns, checksum)` map
4. Files where `size+mtime_ns` match reuse the existing checksum; others are re-hashed
5. Write JSONL to a temp file, then atomically rename to the output path
6. Print summary (scanned, reused, re-hashed, errors)

### 6.2 `sfd plan`

**Set mode (when `--b` is specified):**

1. Validate arguments (`--a`, `--b`, `--out` required)
2. Read A-index; build a set of `MatchKey` values
3. Stream B-index; write matching records to the plan
4. All matching B records are output (including duplicates)
5. Print summary (match count, total matched size)

**Path-match mode (when `--match-path` is specified):**

1. Validate arguments (`--a`, `--b`, `--out` required; `--self` is disallowed)
2. Read A-index; build a `path -> IndexRecord` map
3. Stream B-index; write records where the path exists in A and `MatchKey` also matches
4. Print summary

**Self-dedup mode (when `--self` is specified):**

1. Validate arguments (`--a`, `--out` required; `--b` is disallowed)
2. Load the entire A-index; build a `MatchKey -> []IndexRecord` map
3. For each group, exclude `#recycle` files; treat groups with 2 or more non-`#recycle` files as duplicates
4. Keep the lexicographically smallest path per group; output the rest as `PlanRecord{b_root=A_root, reason="self_duplicate", kept_path=keep.Path}`
5. Print group list (kept/deleted)
6. Print summary

### 6.3 `sfd apply`

1. Validate arguments (`--plan` required)
2. Default to `--dry-run`; perform deletion only when `--execute` is explicitly specified
3. For each plan line, normalize `b_root/path` and verify it is within B's root
4. `--dry-run`: display list and count only
5. `--execute`: delete files directly (no re-checksum verification)
6. Print summary (succeeded, failed, total deleted size)

## 7. Concurrency and Performance

- Parallel hash computation is planned for `index` only (**currently unimplemented — runs single-threaded**).
- Planned approach:
  - 1 scanner goroutine
  - N hash worker goroutines (`N = runtime.NumCPU()`)
- `plan`/`apply` are primarily streaming operations; I/O-bound, so a simple implementation is preferred.
- Memory usage:
  - `index --update`: existing index held as a `path`-keyed map (O(files_in_dir))
  - `plan` set mode: only A-side key set held in memory (O(files_in_A))
  - `plan --match-path`: A-side `path -> IndexRecord` map (O(files_in_A))
  - `plan --self`: entire A-index as `MatchKey -> []IndexRecord` map (O(files_in_A), including path strings)

## 8. Path Safety

- `apply` uses `filepath.Clean` + `filepath.Rel` to reject access outside B's root
- Prevents absolute paths and `..` injection
- The same logic works correctly on Windows

## 9. JSONL I/O Policy

- Stream read/write with one JSON object per line
- Per-line read failures are reported with line numbers
- Compatibility:
  - Extra fields are silently ignored
  - Missing required fields are treated as malformed lines

## 10. Cross-platform Policy

- MVP target: macOS
- Low-cost Windows support:
  - Use `filepath` for all path operations
  - Normalize path separator differences internally
  - No OS-specific APIs

## 11. Test Architecture

- Unit tests:
  - `matcher` (match logic)
  - `path_guard` (B-root boundary enforcement)
  - `jsonl_reader/writer`
- Integration tests:
  - Full `index -> plan -> apply` flow with a small A/B sample
  - Diff verification between `dry-run` and `execute`
  - Continued operation and exit code when unreadable files are present
- Load tests:
  - Large number of small files; verify `--update` reuse rate
