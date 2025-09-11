# Testing Diffium

This project uses Go’s standard `testing` package. Tests run entirely offline and shell out to your local `git` binary.

## Prerequisites

- Go 1.25+
- Git CLI available in `PATH` (`git --version`)

## Quick start

- Run all tests:
  - `go test ./...`
  - or `make test`

## Useful flags

- Verbose output: `go test -v ./...`
- Race detector: `go test -race ./...`
- Single package: `go test ./internal/gitx -v`
- Single test: `go test ./internal/gitx -run TestChangedFiles_AndDiffHEAD -v`
- Coverage HTML report:
  - `go test -coverprofile=coverage.out ./...`
  - `go tool cover -html=coverage.out`

## Notes

- Git-dependent tests create temporary repositories under `t.TempDir()` and set the necessary repo-local config (`user.name`, `user.email`). Your global Git config is not modified.
- No network access is required. Tests rely only on the local Git executable.
- If you encounter errors like “git not found”, ensure Git is installed and accessible in your shell.

