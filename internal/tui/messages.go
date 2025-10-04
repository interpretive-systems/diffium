package tui

import (
    _ "time"
    
    "github.com/interpretive-systems/diffium/internal/diffview"
    "github.com/interpretive-systems/diffium/internal/gitx"
    "github.com/interpretive-systems/diffium/internal/prefs"
)

// tickMsg triggers periodic refresh.
type tickMsg struct{}

// filesMsg contains loaded file changes.
type filesMsg struct {
    files []gitx.FileChange
    err   error
}

// diffMsg contains loaded diff rows.
type diffMsg struct {
    path string
    rows []diffview.Row
    err  error
}

// lastCommitMsg contains the last commit summary.
type lastCommitMsg struct {
    summary string
    err     error
}

// currentBranchMsg contains the current branch name.
type currentBranchMsg struct {
    name string
    err  error
}

// prefsMsg contains loaded preferences.
type prefsMsg struct {
    p   prefs.Prefs
    err error
}
