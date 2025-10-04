package wizards

import (
    "fmt"
    "strings"
    
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/interpretive-systems/diffium/internal/gitx"
)

// ResetCleanPreviewMsg contains preview output.
type ResetCleanPreviewMsg struct {
    Lines []string
    Err   error
}

// ResetCleanResultMsg is sent when operation completes.
type ResetCleanResultMsg struct {
    Err error
}

// ResetCleanWizard handles reset/clean operations.
type ResetCleanWizard struct {
    repoRoot       string
    step           int // 0: select, 1: preview, 2: yellow confirm, 3: red confirm
    doReset        bool
    doClean        bool
    includeIgnored bool
    index          int
    previewLines   []string
    previewErr     string
    running        bool
    err            string
    done           bool
    files          []gitx.FileChange
}

// NewResetCleanWizard creates a new reset/clean wizard.
func NewResetCleanWizard() *ResetCleanWizard {
    return &ResetCleanWizard{}
}

// Init initializes the wizard.
func (w *ResetCleanWizard) Init(repoRoot string, files []gitx.FileChange) tea.Cmd {
    w.repoRoot = repoRoot
    w.files = files
    w.step = 0
    w.doReset = false
    w.doClean = false
    w.includeIgnored = false
    w.index = 0
    w.previewLines = nil
    w.previewErr = ""
    w.running = false
    w.err = ""
    w.done = false
    return nil
}

// HandleKey processes keyboard input.
func (w *ResetCleanWizard) HandleKey(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch w.step {
    case 0:
        return w.handleSelect(msg)
    case 1:
        return w.handlePreview(msg)
    case 2:
        return w.handleYellowConfirm(msg)
    case 3:
        return w.handleRedConfirm(msg)
    }
    return ActionContinue, nil
}

func (w *ResetCleanWizard) handleSelect(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch msg.String() {
    case "esc":
        return ActionClose, nil
    case "j", "down":
        if w.index < 2 {
            w.index++
        }
    case "k", "up":
        if w.index > 0 {
            w.index--
        }
    case " ":
        switch w.index {
        case 0:
            w.doReset = !w.doReset
        case 1:
            w.doClean = !w.doClean
        case 2:
            w.includeIgnored = !w.includeIgnored
        }
    case "a":
        both := w.doReset && w.doClean
        w.doReset = !both
        w.doClean = !both
    case "enter":
        if !w.doReset && !w.doClean {
            w.err = "no actions selected"
            return ActionContinue, nil
        }
        w.step = 1
        w.previewErr = ""
        w.previewLines = nil
        if w.doClean {
            return ActionContinue, w.loadPreview()
        }
        return ActionContinue, nil
    }
    return ActionContinue, nil
}

func (w *ResetCleanWizard) handlePreview(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch msg.String() {
    case "esc":
        return ActionClose, nil
    case "b":
        w.step = 0
        return ActionContinue, nil
    case "enter":
        w.step = 2
        return ActionContinue, nil
    }
    return ActionContinue, nil
}

func (w *ResetCleanWizard) handleYellowConfirm(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch msg.String() {
    case "esc":
        return ActionClose, nil
    case "b":
        w.step = 1
        return ActionContinue, nil
    case "enter":
        w.step = 3
        return ActionContinue, nil
    }
    return ActionContinue, nil
}

func (w *ResetCleanWizard) handleRedConfirm(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch msg.String() {
    case "esc":
        if !w.running {
            return ActionClose, nil
        }
    case "b":
        if !w.running && !w.done {
            w.step = 2
            return ActionContinue, nil
        }
    case "y", "enter":
        if !w.running && !w.done {
            w.running = true
            w.err = ""
            return ActionContinue, w.runResetClean()
        }
    }
    return ActionContinue, nil
}

// Update processes messages.
func (w *ResetCleanWizard) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case ResetCleanPreviewMsg:
        w.previewErr = ""
        if msg.Err != nil {
            w.previewErr = msg.Err.Error()
            w.previewLines = nil
        } else {
            w.previewLines = msg.Lines
        }
    case ResetCleanResultMsg:
        w.running = false
        if msg.Err != nil {
            w.err = msg.Err.Error()
            w.done = false
        } else {
            w.err = ""
            w.done = true
        }
    }
    return nil
}

// RenderOverlay renders the wizard UI.
func (w *ResetCleanWizard) RenderOverlay(width int) []string {
    lines := make([]string, 0, 128)
    lines = append(lines, strings.Repeat("─", width))
    
    switch w.step {
    case 0:
        lines = append(lines, w.renderSelect()...)
    case 1:
        lines = append(lines, w.renderPreview()...)
    case 2:
        lines = append(lines, w.renderYellowConfirm()...)
    case 3:
        lines = append(lines, w.renderRedConfirm()...)
    }
    
    return lines
}

func (w *ResetCleanWizard) renderSelect() []string {
    title := lipgloss.NewStyle().Bold(true).
        Render("Reset/Clean — Select actions (space: toggle, a: toggle both, enter: continue, esc: cancel)")
    
    lines := []string{title}
    
    items := []struct {
        label string
        on    bool
    }{
        {"Reset working tree (git reset --hard)", w.doReset},
        {"Clean untracked (git clean -d -f)", w.doClean},
        {"Include ignored in clean (-x)", w.includeIgnored},
    }
    
    for i, it := range items {
        cur := "  "
        if i == w.index {
            cur = "> "
        }
        check := "[ ]"
        if it.on {
            check = "[x]"
        }
        lines = append(lines, fmt.Sprintf("%s%s %s", cur, check, it.label))
    }
    
    lines = append(lines, lipgloss.NewStyle().Faint(true).
        Render("A preview will be shown before confirmation"))
    
    if w.err != "" {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Render("Error: ")+w.err)
    }
    
    return lines
}

func (w *ResetCleanWizard) renderPreview() []string {
    title := lipgloss.NewStyle().Bold(true).
        Render("Reset/Clean — Preview (enter: continue, b: back, esc: cancel)")
    
    lines := []string{title}
    
    // Reset preview
    if w.doReset {
        tracked := 0
        for _, f := range w.files {
            if !f.Untracked && (f.Staged || f.Unstaged || f.Deleted) {
                tracked++
            }
        }
        lines = append(lines, fmt.Sprintf("Reset would discard tracked changes for ~%d file(s)", tracked))
    } else {
        lines = append(lines, lipgloss.NewStyle().Faint(true).
            Render("Reset: (not selected)"))
    }
    
    // Clean preview
    if w.doClean {
        if w.previewErr != "" {
            lines = append(lines, lipgloss.NewStyle().
                Foreground(lipgloss.Color("196")).
                Render("Clean preview error: ")+w.previewErr)
        } else if len(w.previewLines) == 0 {
            lines = append(lines, lipgloss.NewStyle().Faint(true).
                Render("Clean: nothing to remove"))
        } else {
            lines = append(lines, lipgloss.NewStyle().Bold(true).
                Render("Clean would remove:"))
            max := 10
            for i, l := range w.previewLines {
                if i >= max {
                    break
                }
                lines = append(lines, l)
            }
            if len(w.previewLines) > max {
                lines = append(lines, fmt.Sprintf("… and %d more", len(w.previewLines)-max))
            }
            if w.includeIgnored {
                lines = append(lines, lipgloss.NewStyle().Faint(true).
                    Render("(including ignored files)"))
            }
        }
    } else {
        lines = append(lines, lipgloss.NewStyle().Faint(true).
            Render("Clean: (not selected)"))
    }
    
    // Commands
    var cmds []string
    if w.doReset {
        cmds = append(cmds, "git reset --hard")
    }
    if w.doClean {
        c := "git clean -d -f"
        if w.includeIgnored {
            c += " -x"
        }
        cmds = append(cmds, c)
    }
    if len(cmds) > 0 {
        lines = append(lines, lipgloss.NewStyle().Faint(true).
            Render("Commands: "+strings.Join(cmds, "  &&  ")))
    }
    
    return lines
}

func (w *ResetCleanWizard) renderYellowConfirm() []string {
    title := lipgloss.NewStyle().Bold(true).
        Foreground(lipgloss.Color("220")).
        Render("Confirm — This will discard local changes (enter: continue, b: back, esc: cancel)")
    
    lines := []string{title}
    lines = append(lines, "Proceed to final confirmation?")
    
    return lines
}

func (w *ResetCleanWizard) renderRedConfirm() []string {
    title := lipgloss.NewStyle().Bold(true).
        Foreground(lipgloss.Color("196")).
        Render("FINAL CONFIRMATION — Destructive action (y/enter: execute, b: back, esc: cancel)")
    
    lines := []string{title}
    
    if w.running {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("63")).
            Render("Running…"))
    }
    
    if w.err != "" {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Render("Error: ")+w.err)
    }
    
    return lines
}

// IsComplete returns true if wizard finished successfully.
func (w *ResetCleanWizard) IsComplete() bool {
    return w.done
}

// Error returns any error message.
func (w *ResetCleanWizard) Error() string {
    return w.err
}

func (w *ResetCleanWizard) loadPreview() tea.Cmd {
    return func() tea.Msg {
        lines, err := gitx.CleanPreview(w.repoRoot, w.includeIgnored)
        return ResetCleanPreviewMsg{Lines: lines, Err: err}
    }
}

func (w *ResetCleanWizard) runResetClean() tea.Cmd {
    return func() tea.Msg {
        if err := gitx.ResetAndClean(w.repoRoot, w.doReset, w.doClean, w.includeIgnored); err != nil {
            return ResetCleanResultMsg{Err: err}
        }
        return ResetCleanResultMsg{Err: nil}
    }
}
