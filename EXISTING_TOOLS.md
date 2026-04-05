# Comparison with Existing Tools

## Why this tool exists

This tool is a CLI designed to run the "fixed A/B", "keep A, delete only from B", "leave the comparison result as a plan file" workflow with minimal friction.

- `checksums.jsonl` is treated as an explicit artifact, making it easy to avoid re-reading file contents on repeated runs against the same A/B pair
- Existing tools can approximate this workflow, but their core focus is duplicate-group processing or general scanning, not the A/B deletion workflow that this tool treats as a first-class operation

## Positioning

Both `rmlint` and `fclones` are capable duplicate-file tools, but this tool is not a replacement for either. Its purpose is to simplify a fixed A/B deletion workflow.

| Aspect | same-file-deleter | rmlint | fclones |
| --- | --- | --- | --- |
| Primary purpose | Delete from B files whose content matches files in A | Detect and process duplicate files and various filesystem lint | Find duplicate file groups and delete, move, or hardlink them |
| Pre-comparison data | Explicitly saves and reuses per-directory `checksums.jsonl` | Primarily uses scan results produced at runtime | Primarily builds duplicate groups from a runtime scan |
| Re-run cost model | `index --update` avoids re-hashing unchanged files | Fast re-scanning and some caching exist, but persistent per-directory A/B indexes are not the primary model | Report reuse is possible, but the focus is on in-session duplicate discovery |
| Safety model | Separates `plan` and `apply`; dry-run is the default | Rich output formats, but fixed A/B deletion plan files are not central | `group` and `remove` can be separated, but the target is the full duplicate group |
| Intended workflow | Repeatedly compare A as the reference set against B as the deletion target | Broadly scan arbitrary directories for duplicates and unwanted files | Extract duplicate groups from arbitrary directories and organize them |

## When to use existing tools instead

- You want to search for duplicates across a broad set of directories, not a fixed A/B pair
- You need operations other than deletion, such as moving, hardlinking, or duplicate-directory detection
- You prefer the rich output formats and ecosystem of existing tools

## References

- `rmlint`: https://rmlint.readthedocs.io/ , https://github.com/sahib/rmlint
- `fclones`: https://github.com/pkolaczk/fclones
