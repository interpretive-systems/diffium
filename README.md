# diffium
Diffium is a CLI tool to help you better understand git diffs, with the main intention to make co-development with AI coding agents easier.

## Stage 1 (WIP)

Watcher TUI that shows changed files on the left and a side-by-side diff on the right.

### Run

- From a git repository, run: `go run ./cmd/diffium watch`
- Optional: `-r, --repo` to point at another repo path

### Keys

- `j/k` or arrow keys: move selection
- `J/K`, `PgDn/PgUp`: scroll diff
- `s`: toggle side-by-side vs inline
- `r`: refresh now (auto-refresh runs every second)
- `g/G`: top/bottom
- `h`: help panel
- `<`/`>` or `H`/`L`: adjust left pane width
- `c`: open commit flow (overlay)
- `q`: quit

The top bar shows `Changes | <file>` with a horizontal rule below. The bottom bar shows `h: help` on the left and the last `refreshed` time on the right. Requires `git` in PATH. Binary files are listed but not rendered as text diffs yet.

### Commit Flow

- Press `c` to open the commit overlay.
- Step 1: Select files
  - `j/k` to move, `space` to toggle file, `a` toggle all, `enter` to continue, `esc` to cancel.
- Step 2: Commit message
  - Type your message, `enter` to continue, `b` to go back, `esc` to cancel.
- Step 3: Confirm
  - `y`/`enter` to commit, `b` to go back, `esc` to cancel.

After commit, the overlay closes, file list refreshes, and the bottom bar shows `last: <hash subject>` next to `h: help`.
