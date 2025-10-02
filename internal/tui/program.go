package tui

import (
    "fmt"
	"strings"
	"time"
    
    tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
    "github.com/interpretive-systems/diffium/internal/prefs"
    "github.com/interpretive-systems/diffium/internal/tui/wizards"
	"github.com/interpretive-systems/diffium/internal/tui/components"
)

// Program is the main TUI program.
type Program struct {
    state      *State
    layout     *Layout
    keyHandler *KeyHandler
}

// Run starts the TUI program.
func Run(repoRoot string) error {
    state := NewState(repoRoot)
    p := &Program{
        state:      state,
        layout:     NewLayout(),
        keyHandler: NewKeyHandler(),
    }
    
    prog := tea.NewProgram(p, tea.WithAltScreen())
    if _, err := prog.Run(); err != nil {
        return err
    }
    return nil
}

// Init implements tea.Model.
func (p *Program) Init() tea.Cmd {
    return tea.Batch(
        loadFiles(p.state.RepoRoot, p.state.DiffMode),
        loadLastCommit(p.state.RepoRoot),
        loadCurrentBranch(p.state.RepoRoot),
        loadPrefs(p.state.RepoRoot),
        tickOnce(),
    )
}

// Update implements tea.Model.
func (p *Program) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return p.handleKeyMsg(msg)
        
    case tea.WindowSizeMsg:
        return p.handleWindowSize(msg)
        
    case tickMsg:
        return p.handleTick()
        
    case filesMsg:
        return p.handleFiles(msg)
        
    case diffMsg:
        return p.handleDiff(msg)
        
    case lastCommitMsg:
        return p.handleLastCommit(msg)
        
    case currentBranchMsg:
        return p.handleCurrentBranch(msg)
        
    case prefsMsg:
        return p.handlePrefs(msg)
    }
    
    // Forward wizard messages
    if p.state.ActiveWizard != "" {
        if wiz, ok := p.state.Wizards[p.state.ActiveWizard]; ok {
            cmd := wiz.Update(msg)
            if wiz.IsComplete() {
                p.state.ActiveWizard = ""
                return p, tea.Batch(cmd, p.refreshAfterWizard())
            }
            return p, cmd
        }
    }
    
    return p, nil
}

// View implements tea.Model.
func (p *Program) View() string {
    if p.state.Width == 0 || p.state.Height == 0 {
        return "Loading..."
    }
    
    // Calculate overlay height
    overlayHeight := p.calculateOverlayHeight()
    contentHeight := p.layout.ContentHeight(overlayHeight)
    
    // Update component dimensions
    p.state.FileList.EnsureVisible(contentHeight)
    p.state.DiffView.SetSize(p.layout.RightWidth(), contentHeight)
    
    // Render components
    leftLines := p.state.FileList.Render(contentHeight)
    
    // Render diff content
    selectedFile := p.state.FileList.SelectedFile()
    isBinary := selectedFile != nil && selectedFile.Binary
    diffContent := p.state.DiffView.RenderContent(p.layout.RightWidth(), isBinary)
    
    // Apply search highlights if active
    if p.state.SearchEngine.IsActive() && p.state.SearchEngine.Query() != "" {
        p.state.SearchEngine.SetContent(diffContent)
        diffContent = p.state.SearchEngine.HighlightedContent()
    }
    
    p.state.DiffView.SetContent(diffContent)
    rightView := p.state.DiffView.View()
    
    // Collect overlay lines
    overlayLines := p.collectOverlayLines()
    
    // Render top bar
    topLeft := p.renderTopLeft()
    topRight := p.renderTopRight()
    
    // Update status bar
    p.state.StatusBar.SetKeyBuffer(p.keyHandler.KeyBuffer())
    bottomBar := p.state.StatusBar.Render(p.state.Width)
    
    // Assemble frame
    return p.layout.RenderFrame(
        topLeft,
        topRight,
        leftLines,
        splitLines(rightView, contentHeight),
        overlayLines,
        bottomBar,
        p.state.Theme,
    )
}

func (p *Program) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    // Active wizard takes priority
    if p.state.ActiveWizard != "" {
        wiz := p.state.Wizards[p.state.ActiveWizard]
        action, cmd := wiz.HandleKey(msg)
        if action == wizards.ActionClose {
            p.state.ActiveWizard = ""
            return p, p.recalcViewport()
        }
        return p, cmd
    }
    
    // Search takes priority
    if p.state.SearchEngine.IsActive() {
        handled, cmd := p.state.SearchEngine.HandleKey(msg)
        if handled {
            if !p.state.SearchEngine.IsActive() {
                // Search closed
                return p, p.recalcViewport()
            }
            // Scroll to current match
            if p.state.SearchEngine.MatchCount() > 0 {
                line := p.state.SearchEngine.CurrentMatchLine()
                p.scrollToLine(line)
            }
            return p, cmd
        }
    }
    
    // Help screen
    if p.state.ShowHelp {
        return p.handleHelpKeys(msg)
    }
    
    // Normal key handling
    action, count := p.keyHandler.Handle(msg)
    return p.executeAction(action, count)
}

func (p *Program) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "q":
        return p, tea.Quit
    case "h", "esc":
        p.state.SearchEngine.Deactivate()
        p.state.ShowHelp = false
        return p, p.recalcViewport()
    }
    return p, nil
}

func (p *Program) executeAction(action KeyAction, count int) (tea.Model, tea.Cmd) {
    switch action {
    case ActionQuit:
        return p, tea.Quit
        
    case ActionToggleHelp:
        p.state.SearchEngine.Deactivate()
        p.state.ShowHelp = !p.state.ShowHelp
        return p, p.recalcViewport()
        
    case ActionOpenCommit:
        p.state.SearchEngine.Deactivate()
		cmd := p.openWizard("commit")
        return p, tea.Batch(cmd, p.recalcViewport())
        
    case ActionOpenUncommit:
		cmd := p.openWizard("uncommit")
        return p, tea.Batch(cmd, p.recalcViewport())
        
    case ActionOpenBranch:
		cmd := p.openWizard("branch")
        return p, tea.Batch(cmd, p.recalcViewport())
        
    case ActionOpenPull:
		cmd := p.openWizard("pull")
        return p, tea.Batch(cmd, p.recalcViewport())
        
    case ActionOpenResetClean:
		cmd := p.openWizard("resetclean")
        return p, tea.Batch(cmd, p.recalcViewport())
        
    case ActionOpenSearch:
        p.state.SearchEngine.Activate()
        return p, p.recalcViewport()
        
    case ActionRefresh:
        return p, tea.Batch(
            loadFiles(p.state.RepoRoot, p.state.DiffMode),
            p.loadCurrentDiff(),
        )
        
    case ActionToggleSideBySide:
        sideBySide := !p.state.DiffView.GetSideBySide() // Toggle current mode
        p.state.DiffView.SetSideBySide(sideBySide)
        _ = prefs.SaveSideBySide(p.state.RepoRoot, sideBySide)
        return p, p.recalcViewport()
        
    case ActionToggleDiffMode:
        if p.state.DiffMode == "head" {
            p.state.DiffMode = "staged"
        } else {
            p.state.DiffMode = "head"
        }
        p.state.FileList.GoToTop()
        p.state.DiffView.Viewport().GotoTop()
        return p, tea.Batch(
            loadFiles(p.state.RepoRoot, p.state.DiffMode),
            p.recalcViewport(),
        )
        
    case ActionToggleWrap:
        wrap := !p.state.DiffView.GetWrap()// Invert current
        p.state.DiffView.SetWrap(wrap)
        _ = prefs.SaveWrap(p.state.RepoRoot, wrap)
        return p, p.recalcViewport()
        
    case ActionMoveDown:
        if p.state.FileList.MoveSelection(count) {
            p.state.DiffView.Viewport().GotoTop()
            return p, tea.Batch(p.loadCurrentDiff(), p.recalcViewport())
        }
        
    case ActionMoveUp:
        if p.state.FileList.MoveSelection(-count) {
            p.state.DiffView.Viewport().GotoTop()
            return p, tea.Batch(p.loadCurrentDiff(), p.recalcViewport())
        }
        
    case ActionGoToTop:
        if p.state.FileList.GoToTop() {
            p.state.DiffView.Viewport().GotoTop()
            return p, tea.Batch(p.loadCurrentDiff(), p.recalcViewport())
        }
        
    case ActionGoToBottom:
        if p.state.FileList.GoToBottom() {
            p.state.DiffView.Viewport().GotoTop()
            return p, tea.Batch(p.loadCurrentDiff(), p.recalcViewport())
        }
        
    case ActionPageUpLeft:
        p.state.FileList.PageUp(p.layout.ContentHeight(0))
        return p, p.recalcViewport()
        
    case ActionPageDownLeft:
        p.state.FileList.PageDown(p.layout.ContentHeight(0))
        return p, p.recalcViewport()
        
    case ActionScrollLeft:
        p.state.DiffView.ScrollLeft(1)
        return p, p.recalcViewport()
        
    case ActionScrollRight:
        p.state.DiffView.ScrollRight(1)
        return p, p.recalcViewport()
        
    case ActionScrollHome:
        p.state.DiffView.ScrollHome()
        return p, p.recalcViewport()
        
    case ActionPageDown:
        p.state.DiffView.Viewport().PageDown()
        
    case ActionPageUp:
        p.state.DiffView.Viewport().PageUp()
        
    case ActionHalfPageDown:
        p.state.DiffView.Viewport().HalfPageDown()
        
    case ActionHalfPageUp:
        p.state.DiffView.Viewport().HalfPageUp()
        
    case ActionLineDown:
        p.state.DiffView.Viewport().LineDown(1)
        
    case ActionLineUp:
        p.state.DiffView.Viewport().LineUp(1)
        
    case ActionAdjustLeftNarrower:
        p.layout.AdjustLeftWidth(-2)
        p.state.LeftWidth = p.layout.LeftWidth()
        _ = prefs.SaveLeftWidth(p.state.RepoRoot, p.state.LeftWidth)
        return p, p.recalcViewport()
        
    case ActionAdjustLeftWider:
        p.layout.AdjustLeftWidth(2)
        p.state.LeftWidth = p.layout.LeftWidth()
        _ = prefs.SaveLeftWidth(p.state.RepoRoot, p.state.LeftWidth)
        return p, p.recalcViewport()
        
    case ActionSearchNext:
        p.state.SearchEngine.Next()
        if p.state.SearchEngine.MatchCount() > 0 {
            line := p.state.SearchEngine.CurrentMatchLine()
            p.scrollToLine(line)
        }
        
    case ActionSearchPrevious:
        p.state.SearchEngine.Previous()
        if p.state.SearchEngine.MatchCount() > 0 {
            line := p.state.SearchEngine.CurrentMatchLine()
            p.scrollToLine(line)
        }
    case ActionOpenRevert:
		cmd := p.openWizard("revert")
        return p, tea.Batch(cmd, p.recalcViewport())
    }
    return p, nil
}

func (p *Program) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
    p.state.Width = msg.Width
    p.state.Height = msg.Height
    p.layout.SetSize(msg.Width, msg.Height)
    
    if p.state.LeftWidth == 0 {
        if p.state.SavedLeftWidth > 0 {
            p.state.LeftWidth = p.state.SavedLeftWidth
        } else {
            p.state.LeftWidth = msg.Width / 3
        }
        if p.state.LeftWidth < 24 {
            p.state.LeftWidth = 24
        }
        maxLeft := msg.Width - 20
        if maxLeft < 20 {
            maxLeft = 20
        }
        if p.state.LeftWidth > maxLeft {
            p.state.LeftWidth = maxLeft
        }
        p.layout.SetLeftWidth(p.state.LeftWidth)
    }
    
    return p, p.recalcViewport()
}

func (p *Program) handleTick() (tea.Model, tea.Cmd) {
    return p, tea.Batch(
        loadFiles(p.state.RepoRoot, p.state.DiffMode),
        loadCurrentBranch(p.state.RepoRoot),
        tickOnce(),
    )
}

func (p *Program) handleFiles(msg filesMsg) (tea.Model, tea.Cmd) {
    if msg.err != nil {
        return p, nil
    }
    
    // Preserve selection by path
    var selPath string
    if sel := p.state.FileList.SelectedFile(); sel != nil {
        selPath = sel.Path
    }
    
    p.state.FileList.SetFiles(msg.files)
    p.state.Files = msg.files
    p.state.StatusBar.SetLastRefresh(time.Now())
    
    // Restore selection
    if selPath != "" {
        for i, f := range msg.files {
            if f.Path == selPath {
                p.state.FileList.MoveSelection(i - p.state.FileList.Selected())
                break
            }
        }
    }
    
    // Load diff for selected
    if len(msg.files) > 0 {
        return p, tea.Batch(p.loadCurrentDiff(), p.recalcViewport())
    }
    
    return p, p.recalcViewport()
}

func (p *Program) handleDiff(msg diffMsg) (tea.Model, tea.Cmd) {
    if msg.err != nil {
        return p, nil
    }
    
    // Only update if this is for the current file
    if sel := p.state.FileList.SelectedFile(); sel != nil && sel.Path == msg.path {
        p.state.DiffView.SetRows(msg.rows)
    }
    
    return p, p.recalcViewport()
}

func (p *Program) handleLastCommit(msg lastCommitMsg) (tea.Model, tea.Cmd) {
    if msg.err == nil {
        p.state.LastCommit = msg.summary
        p.state.StatusBar.SetLastCommit(msg.summary)
    }
    return p, nil
}

func (p *Program) handleCurrentBranch(msg currentBranchMsg) (tea.Model, tea.Cmd) {
    if msg.err == nil {
        p.state.CurrentBranch = msg.name
    }
    return p, nil
}

func (p *Program) handlePrefs(msg prefsMsg) (tea.Model, tea.Cmd) {
    if msg.err != nil {
        return p, nil
    }
    
    if msg.p.SideSet {
        p.state.DiffView.SetSideBySide(msg.p.SideBySide)
    }
    if msg.p.WrapSet {
        p.state.DiffView.SetWrap(msg.p.Wrap)
    }
    if msg.p.LeftSet {
        p.state.SavedLeftWidth = msg.p.LeftWidth
        if p.state.Width > 0 {
            lw := p.state.SavedLeftWidth
            if lw < 24 {
                lw = 24
            }
            maxLeft := p.state.Width - 20
            if maxLeft < 20 {
                maxLeft = 20
            }
            if lw > maxLeft {
                lw = maxLeft
            }
            p.state.LeftWidth = lw
            p.layout.SetLeftWidth(lw)
            return p, p.recalcViewport()
        }
    }
    
    return p, nil
}

func (p *Program) openWizard(name string) tea.Cmd{
    p.state.ActiveWizard = name
    wiz := p.state.Wizards[name]
    return wiz.Init(p.state.RepoRoot, p.state.Files)
}

func (p *Program) loadCurrentDiff() tea.Cmd {
    if sel := p.state.FileList.SelectedFile(); sel != nil {
        return loadDiff(p.state.RepoRoot, sel.Path, p.state.DiffMode)
    }
    return nil
}

func (p *Program) recalcViewport() tea.Cmd {
    // Force re-render
    return nil
}

func (p *Program) refreshAfterWizard() tea.Cmd {
    return tea.Batch(
        loadFiles(p.state.RepoRoot, p.state.DiffMode),
        loadLastCommit(p.state.RepoRoot),
        loadCurrentBranch(p.state.RepoRoot),
        p.recalcViewport(),
    )
}

func (p *Program) calculateOverlayHeight() int {
    height := 0
    
    if p.state.ShowHelp {
        height += 18 // Help has fixed height
    }
    
    if p.state.ActiveWizard != "" {
        wiz := p.state.Wizards[p.state.ActiveWizard]
        height += len(wiz.RenderOverlay(p.state.Width))
    }
    
    if p.state.SearchEngine.IsActive() {
        height += 3 // Search overlay
    }
    
    return height
}

func (p *Program) collectOverlayLines() []string {
    var lines []string
    
    if p.state.ShowHelp {
        lines = append(lines, p.renderHelpOverlay()...)
    }
    
    if p.state.ActiveWizard != "" {
        wiz := p.state.Wizards[p.state.ActiveWizard]
        lines = append(lines, wiz.RenderOverlay(p.state.Width)...)
    }
    
    if p.state.SearchEngine.IsActive() {
        lines = append(lines, p.state.SearchEngine.RenderOverlay(
            p.state.Width,
            p.state.Theme.DividerColor,
        )...)
    }
    
    return lines
}

func (p *Program) renderHelpOverlay() []string {
    lines := make([]string, 0, 20)
    lines = append(lines, strings.Repeat("─", p.state.Width))
    
    title := lipgloss.NewStyle().Bold(true).
        Render("Help — press 'h' or Esc to close")
    lines = append(lines, title)
    
    keys := []string{
        "j/k or arrows  Move selection",
        "J/K, PgDn/PgUp  Scroll diff",
        "{/}            Horizontal scroll (diff)",
        "</> or H/L      Adjust left pane width",
        "[/]            Page left file list",
        "b              Switch branch (open wizard)",
        "p              Pull (open wizard)",
        "u              Uncommit (open wizard)",
        "R              Reset/Clean (open wizard)",
        "c              Commit & push (open wizard)",
        "s              Toggle side-by-side / inline",
        "t              Toggle HEAD / staged diffs",
        "w              Toggle line wrap (diff)",
        "/              Search in diff",
        "n/N            Next/previous search match",
        "r              Refresh now",
        "g / G          Top / Bottom",
        "q              Quit",
    }
    
    lines = append(lines, keys...)
    return lines
}

func (p *Program) renderTopLeft() string {
    var title string
    if sel := p.state.FileList.SelectedFile(); sel != nil {
        status := components.FileStatusLabel(*sel)
        title = fmt.Sprintf("Changes | %s (%s) [%s]",
            sel.Path, status, strings.ToUpper(p.state.DiffMode))
    } else {
        title = fmt.Sprintf("Changes | [%s]", strings.ToUpper(p.state.DiffMode))
    }
    return title
}

func (p *Program) renderTopRight() string {
    if p.state.CurrentBranch != "" {
        return lipgloss.NewStyle().Faint(true).Render(p.state.CurrentBranch)
    }
    return ""
}

func (p *Program) scrollToLine(line int) {
    if line < 0 {
        return
    }
    
    vp := p.state.DiffView.Viewport()
    offset := line
    
    if vp.Height > 0 {
        offset = line - vp.Height/2
        if offset < 0 {
            offset = 0
        }
    }
    
    maxOffset := len(p.state.DiffView.Content()) - vp.Height
    if maxOffset < 0 {
        maxOffset = 0
    }
    if offset > maxOffset {
        offset = maxOffset
    }
    
    vp.SetYOffset(offset)
}

func splitLines(s string, maxLines int) []string {
    if s == "" {
        return nil
    }
    lines := strings.Split(s, "\n")
    if len(lines) > maxLines {
        return lines[:maxLines]
    }
    return lines
}
