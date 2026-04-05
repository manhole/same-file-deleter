# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [1.1.0] - 2026-04-05

### Added
- `--match-path` mode: delete files from B that have both the same path and content as files in A
- `--include-all` flag: disable default exclusions (`.git`, etc.) in `sfd index`
- MIT License
- GitHub Actions: CI workflow (runs `go test` on push/PR to main)
- GitHub Actions: release workflow (builds cross-platform binaries on version tags)
- English documentation; Japanese versions retained as `*.ja.md`
- `go install` instructions in README

### Changed
- Renamed "A/B比較モード" to "集合モード" (set mode) in documentation and tests

### Fixed
- `sfd plan` now rejects `--a` and `--b` pointing to the same file

## [1.0.0] - initial release

- `sfd index`: build/update checksum index for a directory
- `sfd plan`: generate deletion plan (set mode and self-dedup mode)
- `sfd apply`: execute or dry-run a deletion plan
- JSONL format for index and plan files
