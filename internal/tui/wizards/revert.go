package wizards

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/interpretive-systems/diffium/internal/gitx"
)

const MaximumRenderableCommitsForRevert = 20

// NOTES: There might be no commit to revert to,
// There can be too much of a commit to render in terminal
type CommitListMsg struct {
	Commits   []string
	Err       error
	Overflown bool
}

type ReverResultMsg struct {
	Err string
}

type RevertWizard struct {
	repoRoot    string
	commits     []string
	step        int // 0: list commits, 1: accept/modify commit message, 2: confirm
	selected    int
	input       textinput.Model
	inputActive bool
	err         string
	done        bool
}

func NewRevertWizard() *RevertWizard {
	return &RevertWizard{}
}

func (w *RevertWizard) Init(repoRoot string, _ []gitx.FileChange) tea.Cmd {
	w.repoRoot = repoRoot
	w.step = 0
	w.commits = nil
	w.selected = 0
	w.err = ""
	w.done = false
	w.input = textinput.New()
	w.inputActive = false
	return w.loadCommits()
}

func (w *RevertWizard) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case CommitListMsg:
		{
			if msg.Err != nil {
				w.err = msg.Err.Error()
				w.commits = nil
				w.selected = 0
			} else {
				w.commits = msg.Commits
				w.selected = 0
				w.err = ""
			}
		}
	case ReverResultMsg:
		// TODO:
	case CommitResultMsg:
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
func (w *RevertWizard) RenderOverlay(width int) []string {
	lines := make([]string, 0, 128)
	lines = append(lines, strings.Repeat("─", width))

	switch w.step {
	case 0:
		lines = append(lines, w.renderCommitList()...)
	case 1:
		lines = append(lines, w.renderMessageInput()...)
	case 2:
		lines = append(lines, w.renderConfirm()...)
	}

	return lines
}
func (w *RevertWizard) renderConfirm() []string {
	title := lipgloss.NewStyle().Bold(true).
		Render("Revert — Confirm (y/enter: revert & push, b: back, esc: cancel)")

	lines := []string{title}

	// sel := w.selectedPaths()
	// lines = append(lines, fmt.Sprintf("Files: %d", len(sel)))
	// lines = append(lines, "Message: "+w.input.Value())

	// if w.running {
	//     lines = append(lines, lipgloss.NewStyle().
	//         Foreground(lipgloss.Color("63")).
	//         Render("Committing & pushing..."))
	// }

	if w.err != "" {
		lines = append(lines, lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render("Error: ")+w.err)
	}
	return lines
}
func (w *RevertWizard) handleConfirm(msg tea.KeyMsg) (Action, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// if !w.running {
		return ActionClose, nil
		// }
	case "b":
		// if !w.running && !w.done {
		w.step = 1
		return ActionContinue, nil
		// }
	case "y", "enter":
		// if !w.running && !w.done {
		//     paths := w.selectedPaths()
		//     if len(paths) == 0 {
		//         w.err = "no files selected"
		//         return ActionContinue, nil
		//     }
		if strings.TrimSpace(w.input.Value()) == "" {
			w.err = "empty commit message"
			return ActionContinue, nil
		}
		w.err = ""
		// w.running = true
		return ActionContinue, w.runRevert()
		// }
	}
	return ActionContinue, nil
}
func (w *RevertWizard) runRevert() tea.Cmd {
	return func() tea.Msg {
		// if err := gitx.StageFiles(w.repoRoot, paths); err != nil {
		//     return CommitResultMsg{Err: err}
		// }
		// if err := gitx.Commit(w.repoRoot, message); err != nil {
		//     return CommitResultMsg{Err: err}
		// }
		if err := gitx.Push(w.repoRoot); err != nil {
			return CommitResultMsg{Err: err}
		}
		return CommitResultMsg{Err: nil}
	}
}

func (w *RevertWizard) renderMessageInput() []string {
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
func (w *RevertWizard) HandleKey(msg tea.KeyMsg) (Action, tea.Cmd) {
	switch w.step {
	case 0:
		return w.handleBranchList(msg)
	case 1:
		return w.handleMessageInput(msg)
	case 2: // Confirm
		return w.handleConfirm(msg)
	}
	return ActionContinue, nil
}
func (w *RevertWizard) handleBranchList(msg tea.KeyMsg) (Action, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return ActionClose, nil
	case "j", "down":
		if len(w.commits) > 0 && w.selected < len(w.commits)-1 {
			w.selected++
		}
	case "k", "up":
		if w.selected > 0 {
			w.selected--
		}
	case "enter":
		if len(w.commits) == 0 {
			return ActionContinue, nil
		}
		w.step = 1
		w.err = ""
		w.done = false
		w.input.SetValue(`Revert todo

This reverts commit 25177c48b7704933a94a1e73ef7b248ddb0e8621.`)
		return ActionContinue, nil
	}
	return ActionContinue, nil
}

func (w *RevertWizard) renderCommitList() []string {
	title := lipgloss.NewStyle().Bold(true).Render("Commits - Select (enter: continue, esc: cancel)")
	lines := []string{title}
	if w.err != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: ")+w.err)
	}

	if len(w.commits) == 0 && w.err == "" {
		lines = append(lines, lipgloss.NewStyle().Faint(true).
			Render("Loading commits"))
		return lines
	}

	for i, n := range w.commits {
		cur := "  "
		if i == w.selected {
			cur = "> "
		}
		mark := "[ ]"
		if i == w.selected {
			mark = "[x]"
		}
		lines = append(lines, fmt.Sprintf("%s%s %s", cur, mark, n))
	}
	return lines
}

func (w *RevertWizard) handleMessageInput(msg tea.KeyMsg) (Action, tea.Cmd) {
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

func (w *RevertWizard) loadCommits() tea.Cmd {
	return func() tea.Msg {
		commits, err := gitx.Log(w.repoRoot, "--oneline")

		overflown := false
		if len(commits) > MaximumRenderableCommitsForRevert {
			overflown = true
			commits = commits[:MaximumRenderableCommitsForRevert] // shrink the slice
		}

		return CommitListMsg{
			Commits:   commits,
			Err:       err,
			Overflown: overflown,
		}
	}
}

func (w *RevertWizard) IsComplete() bool {
	return w.done
}

// Error returns any error message.
func (w *RevertWizard) Error() string {
	return w.err
}
