package tui

import (
    "time"
    
    "github.com/interpretive-systems/diffium/internal/gitx"
    "github.com/interpretive-systems/diffium/internal/tui/components"
    "github.com/interpretive-systems/diffium/internal/tui/search"
    "github.com/interpretive-systems/diffium/internal/tui/wizards"
	"github.com/interpretive-systems/diffium/internal/theme"
)

// State holds all application state.
type State struct {
    // Repository
    RepoRoot      string
    Files         []gitx.FileChange
    CurrentBranch string
    DiffMode      string // "head" or "staged"
    LastCommit    string
    
    // UI State
    Width         int
    Height        int
    LeftWidth     int
    SavedLeftWidth int
    ShowHelp      bool
    LastRefresh   time.Time
    
    // Active Wizard
    ActiveWizard string // "", "commit", "uncommit", "branch", "resetclean", "pull"
    
    // Components
    FileList    *components.FileList
    DiffView    *components.DiffView
    StatusBar   *components.StatusBar
    SearchEngine *search.Engine
    
    // Wizards
    Wizards map[string]wizards.Wizard
    
    // Theme
    Theme theme.Theme
    
    // Key handling
    KeyBuffer string
}

// NewState creates initial application state.
func NewState(repoRoot string) *State {
    curTheme := theme.LoadThemeFromRepo(repoRoot)
    
    return &State{
        RepoRoot:     repoRoot,
        DiffMode:     "head",
        Theme:        curTheme,
        FileList:     components.NewFileList(),
        DiffView:     components.NewDiffView(curTheme),
        StatusBar:    components.NewStatusBar(),
        SearchEngine: search.New(),
        Wizards: map[string]wizards.Wizard{
            "commit":     wizards.NewCommitWizard(),
            "uncommit":   wizards.NewUncommitWizard(),
            "branch":     wizards.NewBranchWizard(),
            "resetclean": wizards.NewResetCleanWizard(),
            "pull":       wizards.NewPullWizard(),
        },
    }
}
