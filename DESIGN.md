# same-file-deleter Design

## 1. Purpose

Compare directory A and directory B, and delete from B any files whose content matches files in A.

- Designed for repeated A/B comparisons: checksum data is saved to files and reused to avoid re-reading file contents
- Deletion candidate extraction and actual deletion are separated, enabling a safe `plan -> dry-run -> execute` workflow
- For positioning relative to existing tools, see `EXISTING_TOOLS.md`

## 2. Terminology

- `checksum index file`: A file storing directory scan results (path, size, mtime, checksum, etc.).
- `plan file`: A file listing deletion candidates produced by comparing two indexes.

Note: This type of index file is commonly called a `manifest` in the industry.
This design uses `checksum index file` as the primary term for clarity, with `manifest` noted where relevant.

## 3. Representative Use Cases

1. **First run**:
   - Run `sfd index --out ...` for each of A and B, then `sfd plan` to generate deletion candidates.
   - Verify candidate count and paths with `sfd apply --dry-run`, then run `sfd apply --execute`.
2. **Repeated runs on the same A/B**:
   - After changes to A or B, re-run `sfd index --update` for the affected directory.
   - Unchanged files reuse existing checksums; only new or changed files are re-hashed. Then run `sfd plan` and `sfd apply`.
3. **Safety-first deletion**:
   - Always route deletion decisions through a plan file; never connect comparison results directly to deletion.
   - Make dry-run the standard step before actual deletion to minimize the impact of mistakes.
4. **Deduplication within a single directory (self-dedup)**:
   - `sfd index --dir ~/photos --out photos.jsonl` to build the index.
   - `sfd plan --a photos.jsonl --self --out dup-plan.jsonl` to detect duplicates within A.
   - `sfd apply --plan dup-plan.jsonl --execute` to delete duplicates.
   - One file (lexicographically smallest path) is kept per group; the rest become candidates.
5. **Removing unchanged files from a backup after an interrupted move (path-match mode)**:
   - Run `sfd index --out ...` for each of A and B.
   - `sfd plan --a A.jsonl --b B.jsonl --match-path --out plan.jsonl` to find files with the same path and content in both.
   - `sfd apply --plan plan.jsonl --execute` to delete them.
   - Files with the same path but different content (modified), or the same content but a different path, are not candidates.

## 4. Scope

- In scope: Regular files on the local filesystem
- Out of scope (initial version):
  - Remote storage integration (S3, etc.)
  - Deleting directories themselves

## 5. Requirements

### 5.1 Functional Requirements

1. Create/update a checksum index file for a specified directory
2. Compare checksum index files for A and B and extract deletion candidates from B
3. If multiple files on the B side match, all of them become deletion candidates
4. Delete files using a plan file
5. Verify candidates with a dry run before deletion
6. Detect duplicate files within a single directory and mark all but one per group as candidates

### 5.2 Non-functional Requirements

1. Handle large numbers of files (memory usage linear or better with file count)
2. Fast on re-runs (no re-hashing of unchanged files)
3. Safety (protection against accidental deletion: dry-run, plan review, path validation)
4. Reproducibility (same indexes always produce the same plan)
5. Low-cost cross-platform support (macOS primary, Windows if feasible)

## 6. Command Design

CLI command name: `sfd`.

### 6.1 `sfd index`

Scan a directory and create/update a checksum index file.

Example:
```bash
sfd index --dir /data/A --out .cache/A.checksums.jsonl
sfd index --dir /data/B --out .cache/B.checksums.jsonl
```

Key options:
- `--dir <path>`: Target directory
- `--out <path>`: Output file path (required)
- `--update`: Read the existing index and reuse checksums for unchanged files
- `--exclude <glob>`: Exclusion pattern (can be specified multiple times)
- `--include-all`: Disable default exclusions (`.git`, etc.) and include all files
- Symbolic links are always ignored

### 6.2 `sfd plan`

Compare checksum index files and generate a deletion candidate plan. Three modes are available.

**Set mode**: Delete from B files whose content matches any file in A (regardless of path).

```bash
sfd plan \
  --a .cache/A.checksums.jsonl \
  --b .cache/B.checksums.jsonl \
  --out .cache/A_to_B.delete-plan.jsonl
```

**Path-match mode (`--match-path`)**: Delete from B files with the same path and content as files in A.

```bash
sfd plan \
  --a .cache/A.checksums.jsonl \
  --b .cache/B.checksums.jsonl \
  --match-path \
  --out .cache/A_to_B.delete-plan.jsonl
```

**Self-dedup mode (`--self`)**: Detect duplicate files within A.

```bash
sfd plan \
  --a .cache/A.checksums.jsonl \
  --self \
  --out .cache/A.dedup-plan.jsonl
```

Key options:
- `--a <file>`: A-side checksum index file (required)
- `--b <file>`: B-side checksum index file (set mode and path-match mode only; mutually exclusive with `--self`)
- `--match-path`: Path-match mode. Only files with the same path and content in both A and B become candidates (mutually exclusive with `--self`)
- `--self`: Self-dedup mode. Keeps the lexicographically smallest path per group and marks the rest as candidates
- `--out <path>`: Plan output (required)

### 6.3 `sfd version`

Display the version number embedded at build time.

```bash
sfd version
```

The version is injected at build time using `git describe --tags --always`. Builds without a tag display `dev`.

### 6.4 `sfd apply`

Delete files according to a plan.

Example:
```bash
sfd apply --plan .cache/A_to_B.delete-plan.jsonl --dry-run
sfd apply --plan .cache/A_to_B.delete-plan.jsonl --execute
```

Key options:
- `--plan <path>`: Plan file
- `--dry-run`: List candidates without deleting (default)
- `--execute`: Perform actual deletion
- `--max-delete <n>`: Stop if the number of candidates exceeds n (0 = unlimited)

## 7. Data Formats

JSONL (one JSON object per line) is used to handle large datasets.

### 7.1 Checksum Index Record Example

```json
{"path":"sub/x.txt","size":1234,"mtime_ns":1739420000000000000,"algo":"blake3","checksum":"ab12...","type":"file"}
```

Fields:
- `path`: Relative path from the scan root
- `size`: File size in bytes
- `mtime_ns`: Last modified time (nanoseconds)
- `algo`: Hash algorithm
- `checksum`: File content hash
- `type`: Currently `file` only

### 7.2 Plan Record Examples

Set mode:

```json
{"b_root":"/data/B","path":"sub/x.txt","reason":"checksum_match_with_A","checksum":"ab12...","size":1234}
```

`--match-path` mode:

```json
{"b_root":"/data/B","path":"sub/x.txt","reason":"path_and_checksum_match","checksum":"ab12...","size":1234}
```

`--self` mode (includes a `kept_path` field):

```json
{"b_root":"/data/D","path":"b/photo.jpg","reason":"self_duplicate","checksum":"ab12...","size":1234,"kept_path":"a/photo.jpg"}
```

- `kept_path`: Relative path of the file to keep within the group (`--self` mode only; omitted in A/B modes)

### 7.3 File Naming Conventions

- Recommended extension: `checksums.jsonl` (JSONL only)
- When handling A and B in the same working directory, `A.checksums.jsonl` / `B.checksums.jsonl` is also acceptable
- Output path must be specified explicitly via `--out` (required)

## 8. Processing Details

### 8.1 Index Processing

1. Recursively scan the directory
2. Retrieve `path`, `size`, `mtime_ns` for each file
3. Exclude `.git`; ignore symbolic links
4. With `--update`: load the existing index as a dictionary; reuse checksums where `size+mtime_ns` match
5. Files inside the target directory that happen to be checksum files are not automatically excluded
6. Re-hash only changed or new files
7. Hash algorithm: `blake3` fixed (MVP)
8. Write the new index atomically (write to temp file, then rename)

### 8.2 Plan Processing

**Set mode:**

1. Read A-index; build a set keyed by `(algo, checksum, size)`
2. Stream B-index; write each record whose key exists in A's set to the plan
3. All matching B-side files are output (including duplicates), except those inside `#recycle`
4. Output statistics (match count, total size, skipped count)

**Path-match mode (`--match-path`):**

1. Read A-index; build a map keyed by `path`
2. Stream B-index; write each record where the `path` exists in A and `(algo, checksum, size)` also matches
3. Exclude files inside `#recycle`
4. Output statistics

**Self-dedup mode (`--self`):**

1. Load the entire A-index; build a map of `(algo, checksum, size)` → `[]IndexRecord`
2. Groups with 2 or more records are considered duplicates
3. For each group, keep the file with the lexicographically smallest path and output the rest as `PlanRecord{b_root=A_root, reason="self_duplicate", kept_path=keep.Path}`
4. Set `b_root` in PlanRecord to A's root directory (used by `apply` to construct the deletion path)
5. Output statistics

Handling of `#recycle` folders (Synology NAS trash):
- Files inside `#recycle` are never made deletion candidates
- For a group containing `#recycle` files: candidates are extracted only when there are 2 or more files **outside** `#recycle`
  - Example: 1 inside `#recycle` + 1 outside → no deletion
  - Example: 1 inside `#recycle` + 2 outside → 1 outside becomes a candidate

### 8.3 Apply Processing

1. Verify that each path in the plan is under B's root (path traversal prevention)
2. `--dry-run`: display candidate list and count only
3. `--execute`: delete files (failures are aggregated and reported at the end)
4. No re-checksum verification after plan creation

## 9. Safety Design

- Default: dry-run
- Deletion only when `--execute` is explicitly specified
- Plans are stored in human-readable JSONL
- Stop when `--max-delete` is exceeded
- Paths outside B's root are immediately rejected
- Deletion is a direct file removal, not a trash/recycle bin operation

## 10. Performance Design

- Hash computation is parallelized using a worker pool (based on CPU core count)
- File reads use streaming
- Index comparison uses a hash set on the A side for O(1) lookup
- Match check uses `checksum+size` (size comparison is a low-cost additional guard)

## 11. Error Handling Policy

- A single file failure does not stop the entire run (aggregated and summarized at exit)
- Unreadable files are logged and processing continues; exits with non-zero at the end
- Index I/O failure or format corruption causes immediate abort
- Exit codes:
  - `0`: Success
  - `1`: Runtime error (including individual file errors)
  - `2`: Invalid input (option or format consistency error)

## 12. Test Strategy

1. Unit tests
   - Checksum computation
   - Index read/write
   - Plan matching logic
2. Integration tests
   - Small A/B samples through `index -> plan -> apply`
   - Diff verification between `dry-run` and `execute`
3. Load tests
   - Large number of small files
   - Small number of large files

## 13. MVP Scope

1. `sfd index` (with `--update`)
2. `sfd plan` (A/B fixed)
3. `sfd apply` (dry-run/execute)
4. JSONL index/plan

## 14. Future Extensions

- Additional hash algorithms (sha256, etc.)
- SQLite storage (faster incremental updates)
- Standardized trash/isolation instead of direct deletion
- TUI/GUI frontend
