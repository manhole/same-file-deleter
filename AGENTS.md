# Repository Guidelines

## Project Structure & Module Organization
This repository is a Go CLI project for comparing two directories and deleting duplicates from B based on A.

- `cmd/sfd/main.go`: CLI entrypoint (`index`, `plan`, `apply`).
- `internal/app/`: use cases and command-level orchestration.
- `internal/domain/`: core models and matching rules.
- `internal/infra/`: filesystem walking, JSONL I/O, hashing, path safety.
- `internal/*/*_test.go`: unit and integration-style tests.
- `DESIGN.md`, `ARCHITECTURE.md`: product and architecture decisions.

Keep new code inside the existing `internal/{app,domain,infra}` layering unless a clear boundary change is required.

## Build, Test, and Development Commands
- `go build ./cmd/sfd`  
Builds the CLI binary.
- `go run ./cmd/sfd --help`  
Runs the tool locally.
- `go test ./...`  
Runs all tests.
- `gofmt -w $(rg --files -g '*.go')`  
Formats all Go files.

Example flow:
`sfd index --dir /data/A --out A.checksums.jsonl`  
`sfd plan --a A.checksums.jsonl --b B.checksums.jsonl --out plan.jsonl`  
`sfd apply --plan plan.jsonl --execute`

## Coding Style & Naming Conventions
- Follow standard Go style; always run `gofmt`.
- Package names are lowercase and concise (`app`, `domain`, `infra`).
- Use descriptive file names by role (e.g., `index_usecase.go`, `jsonl_reader.go`).
- Prefer explicit error wrapping (`fmt.Errorf("context: %w", err)`).
- Keep CLI behavior deterministic and safety-first (`dry-run` default, explicit `--execute`).

## Testing Guidelines
- Use Go’s built-in `testing` package.
- Name tests `TestXxx` and keep them close to the package under test.
- Add/adjust tests for every behavior change, especially:
  - path safety (`EnsureWithinRoot`)
  - index/update behavior
  - end-to-end `index -> plan -> apply` flow

## Commit & Pull Request Guidelines
- Use short, imperative commit messages (seen in history):  
  - `Add architecture design for Go-based MVP`  
  - `Finalize design decisions for A/B checksum workflow`
- PRs should include:
  - purpose and scope
  - key design/behavior changes
  - test evidence (`go test ./...`)
  - any risk notes (file deletion, path validation, backward compatibility)
