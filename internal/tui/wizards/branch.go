package wizards

import (
    "fmt"
    "strings"
    
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/interpretive-systems/diffium/internal/gitx"
)

// BranchListMsg contains the list of branches.
type BranchListMsg struct {
    Names   []string
    Current string
    Err     error
}

// BranchResultMsg is sent when branch operation completes.
type BranchResultMsg struct {
    Err error
}

// BranchWizard handles branch operations.
type BranchWizard struct {
    repoRoot    string
    step        int // 0: list, 1: checkout confirm, 2: new name, 3: new confirm
    branches    []string
    current     string
    index       int
    input       textinput.Model
    inputActive bool
    running     bool
    err         string
    done        bool
}

// NewBranchWizard creates a new branch wizard.
func NewBranchWizard() *BranchWizard {
    return &BranchWizard{}
}

// Init initializes the wizard.
func (w *BranchWizard) Init(repoRoot string, files []gitx.FileChange) tea.Cmd {
    w.repoRoot = repoRoot
    w.step = 0
    w.branches = nil
    w.current = ""
    w.index = 0
    w.running = false
    w.err = ""
    w.done = false
    w.inputActive = false
    
    return w.loadBranches()
}

// HandleKey processes keyboard input.
func (w *BranchWizard) HandleKey(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch w.step {
    case 0:
        return w.handleBranchList(msg)
    case 1:
        return w.handleCheckoutConfirm(msg)
    case 2:
        return w.handleNewBranchName(msg)
    case 3:
        return w.handleNewBranchConfirm(msg)
    }
    return ActionContinue, nil
}

func (w *BranchWizard) handleBranchList(msg tea.KeyMsg) (Action, tea.Cmd) {
    switch msg.String() {
    case "esc":
        return ActionClose, nil
    case "j", "down":
        if len(w.branches) > 0 && w.index < len(w.branches)-1 {
            w.index++
        }
    case "k", "up":
        if w.index > 0 {
            w.index--
        }
    case "n":
        // New branch
        ti := textinput.New()
        ti.Placeholder = "Branch name"
        ti.Prompt = "> "
        w.input = ti
        w.inputActive = false
        w.step = 2
        w.err = ""
        return ActionContinue, nil
    case "enter":
        if len(w.branches) == 0 {
            return ActionContinue, nil
        }
        w.step = 1
        w.err = ""
        w.done = false
        w.running = false
        return ActionContinue, nil
    }
    return ActionContinue, nil
}

func (w *BranchWizard) handleCheckoutConfirm(msg tea.KeyMsg) (Action, tea.Cmd) {
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
            if len(w.branches) == 0 {
                return ActionContinue, nil
            }
            name := w.branches[w.index]
            w.running = true
            w.err = ""
            return ActionContinue, w.runCheckout(name)
        }
    }
    return ActionContinue, nil
}

func (w *BranchWizard) handleNewBranchName(msg tea.KeyMsg) (Action, tea.Cmd) {
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
            if strings.TrimSpace(w.input.Value()) == "" {
                w.err = "empty branch name"
                return ActionContinue, nil
            }
            w.step = 3
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

func (w *BranchWizard) handleNewBranchConfirm(msg tea.KeyMsg) (Action, tea.Cmd) {
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
            name := strings.TrimSpace(w.input.Value())
            if name == "" {
                w.err = "empty branch name"
                return ActionContinue, nil
            }
            w.running = true
            w.err = ""
            return ActionContinue, w.runCreateBranch(name)
        }
    }
    return ActionContinue, nil
}

// Update processes messages.
func (w *BranchWizard) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case BranchListMsg:
        if msg.Err != nil {
            w.err = msg.Err.Error()
            w.branches = nil
            w.current = ""
            w.index = 0
        } else {
            w.branches = msg.Names
            w.current = msg.Current
            w.err = ""
            // Focus current
            w.index = 0
            for i, n := range w.branches {
                if n == w.current {
                    w.index = i
                    break
                }
            }
        }
    case BranchResultMsg:
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
func (w *BranchWizard) RenderOverlay(width int) []string {
    lines := make([]string, 0, 128)
    lines = append(lines, strings.Repeat("─", width))
    
    switch w.step {
    case 0:
        lines = append(lines, w.renderBranchList()...)
    case 1:
        lines = append(lines, w.renderCheckoutConfirm()...)
    case 2:
        lines = append(lines, w.renderNewBranchName()...)
    case 3:
        lines = append(lines, w.renderNewBranchConfirm()...)
    }
    
    return lines
}

func (w *BranchWizard) renderBranchList() []string {
    title := lipgloss.NewStyle().Bold(true).
        Render("Branches — Select (enter: continue, n: new, esc: cancel)")
    
    lines := []string{title}
    
    if w.err != "" {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Render("Error: ")+w.err)
    }
    
    if len(w.branches) == 0 && w.err == "" {
        lines = append(lines, lipgloss.NewStyle().Faint(true).
            Render("Loading branches…"))
        return lines
    }
    
    for i, n := range w.branches {
        cur := "  "
        if i == w.index {
            cur = "> "
        }
        mark := "   "
        if n == w.current {
            mark = "[*]"
        }
        lines = append(lines, fmt.Sprintf("%s%s %s", cur, mark, n))
    }
    
    lines = append(lines, lipgloss.NewStyle().Faint(true).
        Render("[*] current branch"))
    
    return lines
}

func (w *BranchWizard) renderCheckoutConfirm() []string {
    title := lipgloss.NewStyle().Bold(true).
        Render("Checkout — Confirm (y/enter: checkout, b: back, esc: cancel)")
    
    lines := []string{title}
    
    if len(w.branches) > 0 {
        name := w.branches[w.index]
        lines = append(lines, fmt.Sprintf("Branch: %s", name))
    }
    
    if w.running {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("63")).
            Render("Checking out…"))
    }
    
    if w.err != "" {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Render("Error: ")+w.err)
    }
    
    return lines
}

func (w *BranchWizard) renderNewBranchName() []string {
    mode := "action"
    if w.inputActive {
        mode = "input"
    }
    escAction := "cancel"
    if w.inputActive {
        escAction = "leave input"
    }
    
    title := lipgloss.NewStyle().Bold(true).
        Render(fmt.Sprintf("New Branch — Name (i: input, enter: continue, b: back, esc: %s) [%s]", escAction, mode))
    
    lines := []string{title, w.input.View()}
    
    if w.err != "" {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Render("Error: ")+w.err)
    }
    
    return lines
}

func (w *BranchWizard) renderNewBranchConfirm() []string {
    title := lipgloss.NewStyle().Bold(true).
        Render("New Branch — Confirm (y/enter: create, b: back, esc: cancel)")
    
    lines := []string{title}
    lines = append(lines, "Name: "+w.input.Value())
    
    if w.running {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("63")).
            Render("Creating…"))
    }
    
    if w.err != "" {
        lines = append(lines, lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Render("Error: ")+w.err)
    }
    
    return lines
}

// IsComplete returns true if wizard finished successfully.
func (w *BranchWizard) IsComplete() bool {
    return w.done
}

// Error returns any error message.
func (w *BranchWizard) Error() string {
    return w.err
}

func (w *BranchWizard) loadBranches() tea.Cmd {
    return func() tea.Msg {
        names, current, err := gitx.ListBranches(w.repoRoot)
        return BranchListMsg{Names: names, Current: current, Err: err}
    }
}

func (w *BranchWizard) runCheckout(branch string) tea.Cmd {
    return func() tea.Msg {
        if err := gitx.Checkout(w.repoRoot, branch); err != nil {
            return BranchResultMsg{Err: err}
        }
        return BranchResultMsg{Err: nil}
    }
}

func (w *BranchWizard) runCreateBranch(name string) tea.Cmd {
    return func() tea.Msg {
        if err := gitx.CheckoutNew(w.repoRoot, name); err != nil {
            return BranchResultMsg{Err: err}
        }
        return BranchResultMsg{Err: nil}
    }
}
