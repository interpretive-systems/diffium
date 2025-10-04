package tui

import (
    "sort"
    "time"
    
    tea "github.com/charmbracelet/bubbletea"
    "github.com/interpretive-systems/diffium/internal/diffview"
    "github.com/interpretive-systems/diffium/internal/gitx"
    "github.com/interpretive-systems/diffium/internal/prefs"
)

// loadFiles loads the changed files list.
func loadFiles(repoRoot, diffMode string) tea.Cmd {
    return func() tea.Msg {
        allFiles, err := gitx.ChangedFiles(repoRoot)
        if err != nil {
            return filesMsg{files: nil, err: err}
        }
        
        var filtered []gitx.FileChange
        for _, file := range allFiles {
            if diffMode == "staged" {
                if file.Staged {
                    filtered = append(filtered, file)
                }
            } else {
                if file.Unstaged || file.Untracked {
                    filtered = append(filtered, file)
                }
            }
        }
        
        // Stable sort for deterministic UI
        sort.Slice(filtered, func(i, j int) bool {
            return filtered[i].Path < filtered[j].Path
        })
        
        return filesMsg{files: filtered, err: nil}
    }
}

// loadDiff loads the diff for a specific file.
func loadDiff(repoRoot, path, diffMode string) tea.Cmd {
    return func() tea.Msg {
        var d string
        var err error
        if diffMode == "staged" {
            d, err = gitx.DiffStaged(repoRoot, path)
        } else {
            d, err = gitx.DiffHEAD(repoRoot, path)
        }
        if err != nil {
            return diffMsg{path: path, err: err}
        }
        rows := diffview.BuildRowsFromUnified(d)
        return diffMsg{path: path, rows: rows}
    }
}

// loadLastCommit loads the last commit summary.
func loadLastCommit(repoRoot string) tea.Cmd {
    return func() tea.Msg {
        s, err := gitx.LastCommitSummary(repoRoot)
        return lastCommitMsg{summary: s, err: err}
    }
}

// loadCurrentBranch loads the current branch name.
func loadCurrentBranch(repoRoot string) tea.Cmd {
    return func() tea.Msg {
        name, err := gitx.CurrentBranch(repoRoot)
        return currentBranchMsg{name: name, err: err}
    }
}

// loadPrefs loads user preferences.
func loadPrefs(repoRoot string) tea.Cmd {
    return func() tea.Msg {
        p := prefs.Load(repoRoot)
        return prefsMsg{p: p, err: nil}
    }
}

// tickOnce schedules a single tick after 1 second.
func tickOnce() tea.Cmd {
    return tea.Tick(time.Second, func(time.Time) tea.Msg {
        return tickMsg{}
    })
}
