# Contributing to Diffium

Thanks for your interest in contributing! This document outlines how to propose a change and our working conventions.

## Getting Started

- Requires Go 1.25+ and Git CLI.
- Build: `go build ./...`
- Test: `go test ./...`
- Run: `go run ./cmd/diffium watch`

## Issues and Assignment

- Before starting work, please open an issue describing the problem or feature.
- Assign yourself to the issue. Our CI enforces that PRs have an assignee and that linked issues are assigned.
- Reference the issue in your PR description or title using closing keywords, e.g. `Closes #123`. CI will fail if no linked issue is found.

## Branching and PRs

- Create a branch from `main` (e.g., `feat/side-by-side-tweaks`, `fix/diff-crash`).
- Add tests for your change when applicable.
- Keep PRs focused and small; include a clear summary.
- Fill in the PR template checklist and ensure CI passes.

## Code Style

- Keep dependencies minimal and prefer the standard library.
- Match the existing project structure and naming.
- Favor small, testable units of code.

## Tests

- Unit tests: add to the corresponding package under `internal/.../*_test.go`.
- For git-related code, use temporary repositories (`t.TempDir()`) and shell out to `git` (as the code does).

## Theming and UX

- Theme values are read from `.diffium/theme.json` per-repo.
- When extending the UI, consider ANSI-aware width handling for alignment.

## Conduct

Please be respectful and inclusive. See `CODE_OF_CONDUCT.md`.
