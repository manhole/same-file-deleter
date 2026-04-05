# Repository Guidelines

Guidelines for AI agents and developers.
Follow these conventions when making changes to this repository.

## Project Structure and Module Layout

A Go CLI tool that compares directories A and B and deletes duplicate files from B.
Three modes are supported: set mode, path-match mode (`--match-path`), and self-dedup mode (`--self`).

- `cmd/sfd/main.go`: CLI entry point (`index`, `plan`, `apply`)
- `internal/app/`: Use cases and command-level orchestration
- `internal/domain/`: Core models and matching rules
- `internal/infra/`: Filesystem scanning, JSONL I/O, hash computation, path safety
- `internal/*/*_test.go`: Unit and integration tests
- `DESIGN.md`, `ARCHITECTURE.md`: Functional design and implementation design

Place new code within the existing `internal/{app,domain,infra}` layer structure unless a clear boundary change is needed.

## Build, Test, and Development Commands

See [DEVELOPER.md](DEVELOPER.md) for details. Key commands:

- `go build ./cmd/sfd` — Build the CLI binary
- `go run ./cmd/sfd --help` — Run locally without building
- `go test ./...` — Run all tests
- `gofmt -w $(go list -f '{{.Dir}}' ./...)` — Format all Go files

## Coding Style and Naming Conventions

- Follow standard Go style; always run `gofmt`
- Package names: lowercase and concise (`app`, `domain`, `infra`)
- File names: role-based (e.g., `index_usecase.go`, `jsonl_reader.go`)
- Wrap errors explicitly (`fmt.Errorf("context: %w", err)`)
- CLI behavior must be deterministic and safety-first (dry-run by default, `--execute` required for deletion)

## Testing Guidelines

- Use the standard Go `testing` package
- Test names follow the `TestXxx` convention and are placed near the package under test
- Any behavior change affecting the following must include added or updated tests:
  - Path safety (`EnsureWithinRoot`)
  - Index/update behavior
  - End-to-end `index -> plan -> apply` flow

## Commit and Pull Request Guidelines

- Commit messages: short imperative form in English:
  - `Add architecture design for Go-based MVP`
  - `Finalize design decisions for A/B checksum workflow`
- PRs should include:
  - Purpose and scope
  - Key design or behavior changes
  - Test results (`go test ./...`)
  - Risk items (file deletion, path validation, backward compatibility)
