package wizards

import (
    "strings"
    
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/interpretive-systems/diffium/internal/gitx"
)

// PullResultMsg is sent when pull completes.
type PullResultMsg struct {
    Output string
    Err    error
}

// PullWizard handles the pull workflow.
type PullWizard struct {
    repoRoot string
    running  bool
    err      string
    output   string
    done     bool
}

// NewPullWizard creates a new pull wizard.
func NewPullWizard() *PullWizard {
    return &PullWizard{}
}

// Init initializes the wizard.
func (w *PullWizard) Init(repoRoot string, files []gitx.FileChange) tea.Cmd {
    w.repoRoot = repoRoot
    w.running = false
    w.err = ""
    w.output = ""
    w.done = false
    return nil
}

// HandleKey processes keyboard input.
func (w *PullWizard) HandleKey(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch msg.String() {
    case "esc":
        if w.done && !w.running {
            return ActionClose, nil
        }
        if !w.running {
            return ActionClose, nil
        }
    case "y", "enter":
        if w.done && !w.running {
            return ActionClose, nil
        }
        if !w.running && !w.done {
            w.running = true
            w.err = ""
            return ActionContinue, w.runPull()
        }
    }
    return ActionContinue, nil
}

// Update processes messages.
func (w *PullWizard) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case PullResultMsg:
        w.running = false
        w.output = msg.Output
        if msg.Err != nil {
            w.err = msg.Err.Error()
        } else {
            w.err = ""
        }
        w.done = true
    }
    return nil
}

// RenderOverlay renders the wizard UI.
func (w *PullWizard) RenderOverlay(width int) []string {
    lines := make([]string, 0, 32)
    lines = append(lines, strings.Repeat("─", width))
    
    if w.done {
        title := lipgloss.NewStyle().Bold(true).
            Render("Pull — Result (enter/esc: close)")
        lines = append(lines, title)
        
        if w.err != "" {
            lines = append(lines, lipgloss.NewStyle().
                Foreground(lipgloss.Color("196")).
                Render("Error: ")+w.err)
        }
        
        if w.output != "" {
            outLines := strings.Split(strings.TrimRight(w.output, "\n"), "\n")
            max := 12
            for i, l := range outLines {
                if i >= max {
                    break
                }
                lines = append(lines, l)
            }
            if len(outLines) > max {
                lines = append(lines, lipgloss.NewStyle().Faint(true).
                    Render("… and more"))
            }
        } else if w.err == "" {
            lines = append(lines, lipgloss.NewStyle().Faint(true).
                Render("(no output)"))
        }
    } else {
        title := lipgloss.NewStyle().Bold(true).
            Render("Pull — Confirm (y/enter: pull, esc: cancel)")
        lines = append(lines, title)
        
        if w.running {
            lines = append(lines, lipgloss.NewStyle().
                Foreground(lipgloss.Color("63")).
                Render("Pulling…"))
        }
        
        if w.err != "" {
            lines = append(lines, lipgloss.NewStyle().
                Foreground(lipgloss.Color("196")).
                Render("Error: ")+w.err)
        }
    }
    
    return lines
}

// IsComplete returns true if wizard finished successfully.
func (w *PullWizard) IsComplete() bool {
    return w.done && w.err == ""
}

// Error returns any error message.
func (w *PullWizard) Error() string {
    return w.err
}

func (w *PullWizard) runPull() tea.Cmd {
    return func() tea.Msg {
        out, err := gitx.PullWithOutput(w.repoRoot)
        return PullResultMsg{Output: out, Err: err}
    }
}
