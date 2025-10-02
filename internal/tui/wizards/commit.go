package wizards

import (
    "fmt"
    "strings"
    
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/interpretive-systems/diffium/internal/gitx"
)

// CommitResultMsg is sent when commit completes.
type CommitResultMsg struct {
    Err error
}

// CommitWizard handles the commit workflow.
type CommitWizard struct {
    repoRoot    string
    step        int // 0: select files, 1: message, 2: confirm
    files       []gitx.FileChange
    selected    map[string]bool
    index       int
    input       textinput.Model
    inputActive bool
    running     bool
    err         string
    done        bool
}

// NewCommitWizard creates a new commit wizard.
func NewCommitWizard() *CommitWizard {
    return &CommitWizard{
        selected: make(map[string]bool),
    }
}

// Init initializes the wizard.
func (w *CommitWizard) Init(repoRoot string, files []gitx.FileChange) tea.Cmd {
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
    w.inputActive = false
    return nil
}

// HandleKey processes keyboard input.
func (w *CommitWizard) HandleKey(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch w.step {
    case 0: // File selection
        return w.handleFileSelection(msg)
    case 1: // Message input
        return w.handleMessageInput(msg)
    case 2: // Confirm
        return w.handleConfirm(msg)
    }
    return ActionContinue, nil
}

func (w *CommitWizard) handleFileSelection(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch msg.String() {
    case "esc":
        return ActionClose, nil
    case "enter":
        w.step = 1
        ti := textinput.New()
        ti.Placeholder = "Commit message"
        ti.Prompt = "> "
        ti.CharLimit = 0
        w.input = ti
        w.inputActive = false
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

func (w *CommitWizard) handleMessageInput(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch msg.String() {
    case "esc":
        if w.inputActive {
            w.inputActive = false
            w.input.Blur()
            return ActionContinue, nil
        }
        return ActionClose, nil
    case "i":
        if !w.inputActive {
            w.inputActive = true
            w.input.Focus()
            return ActionContinue, nil
        }
    case "b":
        if !w.inputActive {
            w.step = 0
            return ActionContinue, nil
        }
    case "enter":
        if !w.inputActive {
            w.step = 2
            w.err = ""
            w.done = false
            w.running = false
            return ActionContinue, nil
        }
    }
    
    if w.inputActive {
        var cmd tea.Cmd
        w.input, cmd = w.input.Update(msg)
        return ActionContinue, cmd
    }
    
    return ActionContinue, nil
}

func (w *CommitWizard) handleConfirm(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch msg.String() {
    case "esc":
        if !w.running {
            return ActionClose, nil
        }
    case "b":
        if !w.running && !w.done {
            w.step = 1
            return ActionContinue, nil
        }
    case "y", "enter":
        if !w.running && !w.done {
            paths := w.selectedPaths()
            if len(paths) == 0 {
                w.err = "no files selected"
                return ActionContinue, nil
            }
            if strings.TrimSpace(w.input.Value()) == "" {
                w.err = "empty commit message"
                return ActionContinue, nil
            }
            w.err = ""
            w.running = true
            return ActionContinue, w.runCommit(paths, w.input.Value())
        }
    }
    return ActionContinue, nil
}

// Update processes messages.
func (w *CommitWizard) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case CommitResultMsg:
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
func (w *CommitWizard) RenderOverlay(width int) []string {
    lines := make([]string, 0, 64)
    lines = append(lines, strings.Repeat("─", width))
    
    switch w.step {
    case 0:
        lines = append(lines, w.renderFileSelection()...)
    case 1:
        lines = append(lines, w.renderMessageInput()...)
    case 2:
        lines = append(lines, w.renderConfirm()...)
    }
    
    return lines
}

func (w *CommitWizard) renderFileSelection() []string {
    title := lipgloss.NewStyle().Bold(true).
        Render("Commit — Select files (space: toggle, a: all, enter: continue, esc: cancel)")
    
    lines := []string{title}
    
    if len(w.files) == 0 {
        lines = append(lines, lipgloss.NewStyle().Faint(true).Render("No changes to commit"))
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

func (w *CommitWizard) renderMessageInput() []string {
    mode := "action"
    if w.inputActive {
        mode = "input"
    }
    escAction := "cancel"
    if w.inputActive {
        escAction = "leave input"
    }
    
    title := lipgloss.NewStyle().Bold(true).
        Render(fmt.Sprintf("Commit — Message (i: input, enter: continue, b: back, esc: %s) [%s]", escAction, mode))
    
    return []string{title, w.input.View()}
}

func (w *CommitWizard) renderConfirm() []string {
    title := lipgloss.NewStyle().Bold(true).
        Render("Commit — Confirm (y/enter: commit & push, b: back, esc: cancel)")
    
    lines := []string{title}
    
    sel := w.selectedPaths()
    lines = append(lines, fmt.Sprintf("Files: %d", len(sel)))
    lines = append(lines, "Message: "+w.input.Value())
    
    if w.running {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("63")).
            Render("Committing & pushing..."))
    }
    
    if w.err != "" {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Render("Error: ")+w.err)
    }
    
    return lines
}

// IsComplete returns true if wizard finished successfully.
func (w *CommitWizard) IsComplete() bool {
    return w.done
}

// Error returns any error message.
func (w *CommitWizard) Error() string {
    return w.err
}

func (w *CommitWizard) selectedPaths() []string {
    var out []string
    for _, f := range w.files {
        if w.selected[f.Path] {
            out = append(out, f.Path)
        }
    }
    return out
}

func (w *CommitWizard) runCommit(paths []string, message string) tea.Cmd {
    return func() tea.Msg {
        if err := gitx.StageFiles(w.repoRoot, paths); err != nil {
            return CommitResultMsg{Err: err}
        }
        if err := gitx.Commit(w.repoRoot, message); err != nil {
            return CommitResultMsg{Err: err}
        }
        if err := gitx.Push(w.repoRoot); err != nil {
            return CommitResultMsg{Err: err}
        }
        return CommitResultMsg{Err: nil}
    }
}

func fileStatusLabel(f gitx.FileChange) string {
    var tags []string
    if f.Deleted {
        tags = append(tags, "D")
    }
    if f.Untracked {
        tags = append(tags, "U")
    }
    if f.Staged {
        tags = append(tags, "S")
    }
    if f.Unstaged {
        tags = append(tags, "M")
    }
    if len(tags) == 0 {
        return "-"
    }
    return strings.Join(tags, "")
}
