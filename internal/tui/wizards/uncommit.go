package wizards

import (
    "fmt"
    "strings"
    
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/interpretive-systems/diffium/internal/gitx"
)

// UncommitResultMsg is sent when uncommit completes.
type UncommitResultMsg struct {
    Err error
}

// UncommitEligibleMsg contains files in last commit.
type UncommitEligibleMsg struct {
    Paths []string
    Err   error
}

// UncommitWizard handles the uncommit workflow.
type UncommitWizard struct {
    repoRoot    string
    step        int
    files       []gitx.FileChange
    selected    map[string]bool
    index       int
    eligible    map[string]bool
    running     bool
    err         string
    done        bool
}

// NewUncommitWizard creates a new uncommit wizard.
func NewUncommitWizard() *UncommitWizard {
    return &UncommitWizard{
        selected: make(map[string]bool),
        eligible: make(map[string]bool),
    }
}

// Init initializes the wizard.
func (w *UncommitWizard) Init(repoRoot string, files []gitx.FileChange) tea.Cmd {
    w.repoRoot = repoRoot
    w.step = 0
    w.files = append([]gitx.FileChange(nil), files...)
    w.selected = make(map[string]bool)
    for _, f := range w.files {
        w.selected[f.Path] = true
    }
    w.index = 0
    w.running = false
    w.err = ""
    w.done = false
    
    // Load eligible files
    return w.loadEligible()
}

// HandleKey processes keyboard input.
func (w *UncommitWizard) HandleKey(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch w.step {
    case 0: // File selection
        return w.handleFileSelection(msg)
    case 1: // Confirm
        return w.handleConfirm(msg)
    }
    return ActionContinue, nil
}

func (w *UncommitWizard) handleFileSelection(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch msg.String() {
    case "esc":
        return ActionClose, nil
    case "enter":
        w.step = 1
        w.err = ""
        w.done = false
        w.running = false
        return ActionContinue, nil
    case "j", "down":
        if len(w.files) > 0 && w.index < len(w.files)-1 {
            w.index++
        }
    case "k", "up":
        if w.index > 0 {
            w.index--
        }
    case " ":
        if len(w.files) > 0 {
            path := w.files[w.index].Path
            w.selected[path] = !w.selected[path]
        }
    case "a":
        all := true
        for _, f := range w.files {
            if !w.selected[f.Path] {
                all = false
                break
            }
        }
        set := !all
        for _, f := range w.files {
            w.selected[f.Path] = set
        }
    }
    return ActionContinue, nil
}

func (w *UncommitWizard) handleConfirm(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch msg.String() {
    case "esc":
        if !w.running {
            return ActionClose, nil
        }
    case "b":
        if !w.running && !w.done {
            w.step = 0
            return ActionContinue, nil
        }
    case "y", "enter":
        if !w.running && !w.done {
            paths := w.selectedPaths()
            if len(paths) == 0 {
                w.err = "no files selected"
                return ActionContinue, nil
            }
            w.err = ""
            w.running = true
            return ActionContinue, w.runUncommit(paths)
        }
    }
    return ActionContinue, nil
}

// Update processes messages.
func (w *UncommitWizard) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case UncommitResultMsg:
        w.running = false
        if msg.Err != nil {
            w.err = msg.Err.Error()
            w.done = false
        } else {
            w.err = ""
            w.done = true
        }
    case UncommitEligibleMsg:
        if msg.Err == nil {
            w.eligible = make(map[string]bool)
            for _, p := range msg.Paths {
                w.eligible[p] = true
            }
        }
    }
    return nil
}

// RenderOverlay renders the wizard UI.
func (w *UncommitWizard) RenderOverlay(width int) []string {
    lines := make([]string, 0, 64)
    lines = append(lines, strings.Repeat("─", width))
    
    switch w.step {
    case 0:
        lines = append(lines, w.renderFileSelection()...)
    case 1:
        lines = append(lines, w.renderConfirm()...)
    }
    
    return lines
}

func (w *UncommitWizard) renderFileSelection() []string {
    title := lipgloss.NewStyle().Bold(true).
        Render("Uncommit — Select files (space: toggle, a: all, enter: continue, esc: cancel)")
    
    lines := []string{title}
    
    if w.err != "" {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Render("Error: ")+w.err)
    }
    
    if len(w.files) == 0 {
        lines = append(lines, lipgloss.NewStyle().Faint(true).
            Render("No changes to choose from"))
        return lines
    }
    
    for i, f := range w.files {
        cur := "  "
        if i == w.index {
            cur = "> "
        }
        mark := "[ ]"
        if w.selected[f.Path] {
            mark = "[x]"
        }
        status := fileStatusLabel(f)
        lines = append(lines, fmt.Sprintf("%s%s %s %s", cur, mark, status, f.Path))
    }
    
    return lines
}

func (w *UncommitWizard) renderConfirm() []string {
    title := lipgloss.NewStyle().Bold(true).
        Render("Uncommit — Confirm (y/enter: uncommit, b: back, esc: cancel)")
    
    lines := []string{title}
    
    sel := w.selectedPaths()
    total := len(sel)
    elig := 0
    for _, p := range sel {
        if w.eligible[p] {
            elig++
        }
    }
    inelig := total - elig
    
    lines = append(lines, fmt.Sprintf(
        "Selected: %d  Eligible to uncommit: %d  Ignored: %d",
        total, elig, inelig,
    ))
    
    if w.running {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("63")).
            Render("Uncommitting…"))
    }
    
    if w.err != "" {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Render("Error: ")+w.err)
    }
    
    return lines
}

// IsComplete returns true if wizard finished successfully.
func (w *UncommitWizard) IsComplete() bool {
    return w.done
}

// Error returns any error message.
func (w *UncommitWizard) Error() string {
    return w.err
}

func (w *UncommitWizard) selectedPaths() []string {
    var out []string
    for _, f := range w.files {
        if w.selected[f.Path] {
            out = append(out, f.Path)
        }
    }
    return out
}

func (w *UncommitWizard) loadEligible() tea.Cmd {
    return func() tea.Msg {
        paths, err := gitx.FilesInLastCommit(w.repoRoot)
        return UncommitEligibleMsg{Paths: paths, Err: err}
    }
}

func (w *UncommitWizard) runUncommit(paths []string) tea.Cmd {
    return func() tea.Msg {
        // Filter to eligible only
        var eligible []string
        for _, p := range paths {
            if w.eligible[p] {
                eligible = append(eligible, p)
            }
        }
        
        if len(eligible) == 0 {
            return UncommitResultMsg{Err: fmt.Errorf("no selected files are in the last commit")}
        }
        
        if err := gitx.UncommitFiles(w.repoRoot, eligible); err != nil {
            return UncommitResultMsg{Err: err}
        }
        return UncommitResultMsg{Err: nil}
    }
}
