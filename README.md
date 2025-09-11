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
  - Action mode by default: `i` to enter input mode, `enter` to continue, `b` back, `esc` cancel.
  - In input mode: type to edit; `esc` leaves input mode (does not cancel).
- Step 3: Confirm
  - `y`/`enter` to commit & push, `b` to go back, `esc` to cancel.

After commit & push, the overlay closes, file list refreshes, and the bottom bar shows `last: <hash subject>` next to `h: help`.

### Theming

Diffium supports simple, repo-local theming via `.diffium/theme.json` (relative to the repo root you are watching). Example:

```
{
  "addColor": "#22c55e",
  "delColor": "#ef4444",
  "dividerColor": "240"
}
```

Notes:
- Colors accept hex (e.g., `#22c55e`) or ANSI color indexes as strings (e.g., `"34"`, `"196"`).
- Omitted fields use defaults.
