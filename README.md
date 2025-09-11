# diffium
Diffium is a CLI tool to help you better understand git diffs, with the main intention to make co-development with AI coding agents easier.

## Stage 1 (WIP)

Watcher TUI that shows changed files on the left and a side-by-side diff on the right.

### Run

- From a git repository, run: `go run ./cmd/diffium watch`
- Optional: `-r, --repo` to point at another repo path

### Keys

- `j/k` or arrow keys: move selection
- `s`: toggle side-by-side vs inline
- `r`: refresh now (auto-refresh runs every second)
- `g/G`: top/bottom
- `q`: quit

Notes: Requires `git` in PATH. Binary files are listed but not rendered as text diffs yet.
