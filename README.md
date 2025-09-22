# diffium
Diffium is a CLI tool to help you better understand git diffs, with the main intention to make co-development with AI coding agents easier.

## Dependencies

go: go1.25.1
git

brew install go
brew install git

## Stage 1 (WIP)

Watcher TUI that shows changed files on the left and a side-by-side diff on the right.

### Run

- From a git repository, run: `go run ./cmd/diffium watch`
- Optional: `-r, --repo` to point at another repo path

### Keys

- `j/k` or arrow keys: move selection
- `J/K`, `PgDn/PgUp`: scroll diff
- `s`: toggle side-by-side vs inline
- `u`: open uncommit wizard (remove selected files from last commit; shows all current changes for selection)
- `R`: open reset/clean wizard (repo-wide): select reset `git reset --hard`, clean `git clean -d -f`, optionally include ignored; shows preview, then two confirmations (yellow + red)
- `r`: refresh now (auto-refresh runs every second)
- `g/G`: top/bottom
- `h`: help panel
- `<`/`>` or `H`/`L`: adjust left pane width
- `c`: open commit flow (overlay)
- `q`: quit

The top bar shows `Changes | <file>` with a horizontal rule below. The bottom bar shows `h: help` on the left and the last `refreshed` time on the right. Requires `git` in PATH. Binary files are listed but not rendered as text diffs yet.

## Quick Demo

Fastest way to demo Diffium on macOS:

- Open the watcher: `go run ./cmd/diffium watch`
- In another terminal, edit the `testing` file in this repo:
  - Add a line: `echo "New demo line" >> testing`
  - Change a word (macOS sed): `sed -i '' 's/brown/green/' testing`
  - Delete the last line: `sed -i '' -e '$d' testing`
- Watch the left pane update and the right pane show a side-by-side diff.
- Press `s` to toggle inline/side-by-side.
- Press `c` to open the commit overlay; select files, enter a message, confirm to commit & push.

Alternative demo with the Next.js sample:

- `cd basic-nextjs`
- Create a change: `printf "\n// demo change\n" >> pages/index.js`
- Switch back to Diffium to review and commit.

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
