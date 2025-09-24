package tui

import (
    "fmt"
    "sort"
    "strings"
    "time"
	"unicode/utf8"

    "github.com/charmbracelet/bubbles/viewport"
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/x/ansi"
    "github.com/interpretive-systems/diffium/internal/diffview"
    "github.com/interpretive-systems/diffium/internal/gitx"
)

const (
    // Normal match: black on bright white
    searchMatchStartSeq        = "\x1b[30;107m"
    // Current match: black on yellow
    searchCurrentMatchStartSeq = "\x1b[30;43m"
    // Reset all styles
    searchMatchEndSeq          = "\x1b[0m"
)

type model struct {
    repoRoot    string
    theme       Theme
    files       []gitx.FileChange
    selected    int
    rows        []diffview.Row
    sideBySide  bool
    width       int
    height      int
    status      string
    lastRefresh time.Time
    showHelp    bool
    leftWidth   int
    rightVP     viewport.Model
	rightContent []string
    // commit wizard state
    showCommit  bool
    commitStep  int // 0: select files, 1: message, 2: confirm/progress
    cwFiles     []gitx.FileChange
    cwSelected  map[string]bool
    cwIndex     int
    cwInput     textinput.Model
    cwInputActive bool
    committing  bool
    commitErr   string
    commitDone  bool
    lastCommit  string
	// search state
	searchActive  bool
	searchInput   textinput.Model
	searchQuery   string
	searchMatches []int
	searchIndex   int
}

// messages
type tickMsg struct{}

type filesMsg struct {
    files []gitx.FileChange
    err   error
}

type diffMsg struct {
    path string
    rows []diffview.Row
    err  error
}

// Run instantiates and runs the Bubble Tea program.
func Run(repoRoot string) error {
    m := model{repoRoot: repoRoot, sideBySide: true, theme: loadThemeFromRepo(repoRoot)}
    p := tea.NewProgram(m, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        return err
    }
    return nil
}

func (m model) Init() tea.Cmd {
    return tea.Batch(loadFiles(m.repoRoot), loadLastCommit(m.repoRoot), tickOnce())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
		if m.searchActive {
			return m.handleSearchKeys(msg)
		}
        if m.showHelp {
            switch msg.String() {
            case "q":
                return m, tea.Quit
            case "h", "esc":
                (&m).closeSearch()
                m.showHelp = false
                return m, m.recalcViewport()
            default:
                return m, nil
            }
        }
        if m.showCommit {
            return m.handleCommitKeys(msg)
        }
        switch msg.String() {
        case "ctrl+c", "q":
            return m, tea.Quit
        case "h":
			(&m).closeSearch()
            m.showHelp = true
            return m, m.recalcViewport()
        case "c":
            // Open commit wizard
			(&m).closeSearch()
            m.openCommitWizard()
            return m, m.recalcViewport()
		case "/":
			(&m).openSearch()
			return m, m.recalcViewport()	
        case "<", "H":
            if m.leftWidth == 0 {
                m.leftWidth = m.width / 3
            }
            m.leftWidth -= 2
            if m.leftWidth < 20 {
                m.leftWidth = 20
            }
            return m, m.recalcViewport()
        case ">", "L":
            if m.leftWidth == 0 {
                m.leftWidth = m.width / 3
            }
            m.leftWidth += 2
            maxLeft := m.width - 20
            if maxLeft < 20 {
                maxLeft = 20
            }
            if m.leftWidth > maxLeft {
                m.leftWidth = maxLeft
            }
            return m, m.recalcViewport()
        case "j", "down":
            if len(m.files) == 0 {
                return m, nil
            }
            if m.selected < len(m.files)-1 {
                m.selected++
                m.rows = nil
                // Reset scroll for new file
                m.rightVP.GotoTop()
                return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path), m.recalcViewport())
            }
        case "k", "up":
            if len(m.files) == 0 {
                return m, nil
            }
            if m.selected > 0 {
                m.selected--
                m.rows = nil
                m.rightVP.GotoTop()
                return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path), m.recalcViewport())
            }
        case "g":
            if len(m.files) > 0 {
                m.selected = 0
                m.rows = nil
                m.rightVP.GotoTop()
                return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path), m.recalcViewport())
            }
        case "G":
            if len(m.files) > 0 {
                m.selected = len(m.files) - 1
                m.rows = nil
                m.rightVP.GotoTop()
                return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path), m.recalcViewport())
            }
		case "n":
			return m, (&m).advanceSearch(1)
		case "N":
			return m, (&m).advanceSearch(-1)
        case "r":
            return m, tea.Batch(loadFiles(m.repoRoot), loadCurrentDiff(m))
        case "s":
            m.sideBySide = !m.sideBySide
            return m, m.recalcViewport()
        // Right pane scrolling
        case "pgdown":
            m.rightVP.PageDown()
            return m, nil
        case "pgup":
            m.rightVP.PageUp()
            return m, nil
        case "J", "ctrl+d":
            m.rightVP.HalfPageDown()
            return m, nil
        case "K", "ctrl+u":
            m.rightVP.HalfPageUp()
            return m, nil
        case "ctrl+e":
            m.rightVP.LineDown(1)
            return m, nil
        case "ctrl+y":
            m.rightVP.LineUp(1)
            return m, nil
        }
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        if m.leftWidth == 0 {
            // Initialize left width once
            m.leftWidth = m.width / 3
            if m.leftWidth < 24 {
                m.leftWidth = 24
            }
        }
        return m, m.recalcViewport()
    case tickMsg:
        // Periodic refresh
        return m, tea.Batch(loadFiles(m.repoRoot), tickOnce())
    case filesMsg:
        if msg.err != nil {
            m.status = fmt.Sprintf("status error: %v", msg.err)
            return m, nil
        }
        // Stable-sort files by path for deterministic UI
        sort.Slice(msg.files, func(i, j int) bool { return msg.files[i].Path < msg.files[j].Path })

        // Preserve selection by path if possible
        var selPath string
        if len(m.files) > 0 && m.selected >= 0 && m.selected < len(m.files) {
            selPath = m.files[m.selected].Path
        }
        m.files = msg.files
        m.lastRefresh = time.Now()

        // Reselect
        m.selected = 0
        if selPath != "" {
            for i, f := range m.files {
                if f.Path == selPath {
                    m.selected = i
                    break
                }
            }
        }
        // Load diff for selected if exists
        if len(m.files) > 0 {
            return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path), m.recalcViewport())
        }
        m.rows = nil
        return m, m.recalcViewport()
    case diffMsg:
        if msg.err != nil {
            m.status = fmt.Sprintf("diff error: %v", msg.err)
            m.rows = nil
            return m, m.recalcViewport()
        }
        // Only update if this diff is for the currently selected file
        if len(m.files) > 0 && m.files[m.selected].Path == msg.path {
            m.rows = msg.rows
        }
        return m, m.recalcViewport()
    case lastCommitMsg:
        if msg.err == nil {
            m.lastCommit = msg.summary
        }
        return m, nil
    case commitProgressMsg:
        m.committing = true
        m.commitErr = ""
        return m, nil
    case commitResultMsg:
        m.committing = false
        if msg.err != nil {
            m.commitErr = msg.err.Error()
            m.commitDone = false
            // refresh even on error (commit may have succeeded but push failed)
            return m, tea.Batch(loadFiles(m.repoRoot), loadLastCommit(m.repoRoot), m.recalcViewport())
        } else {
            m.commitErr = ""
            m.commitDone = true
            m.showCommit = false
            // refresh changes and last commit
            return m, tea.Batch(loadFiles(m.repoRoot), loadLastCommit(m.repoRoot), m.recalcViewport())
        }
        return m, nil
    }
    return m, nil
}

func (m model) View() string {
    // Layout
    if m.width == 0 || m.height == 0 {
        return "Loading..."
    }

    // Column widths
    leftW := m.leftWidth
    if leftW < 20 {
        leftW = 20
    }
    rightW := m.width - leftW - 1 // vertical divider column
    if rightW < 1 {
        rightW = 1
    }
    sep := m.theme.DividerText("│")

    // Row 1: top bar
    top := "Changes | " + m.topRightTitle()
    // Row 2: horizontal rule
    hr := m.theme.DividerText(strings.Repeat("─", m.width))

    // Row 3: columns, then optional overlays, then bottom rule + bar
    var overlay []string
    if m.showHelp {
        overlay = m.helpOverlayLines(m.width)
    }
    if m.showCommit {
        overlay = append(overlay, m.commitOverlayLines(m.width)...)
    }
	if m.searchActive { 
    	overlay = append(overlay, m.searchOverlayLines(m.width)...)
	}
    overlayH := len(overlay)

    contentHeight := m.height - 4 - overlayH // top + top rule + bottom rule + bottom bar
    if contentHeight < 1 {
        contentHeight = 1
    }

    leftLines := m.leftBodyLines(contentHeight)
    // Right viewport already holds content and scroll state; ensure dims
    // The viewport content is updated via recalcViewport()
    m.rightVP.Width = rightW
    m.rightVP.Height = contentHeight
    rightView := m.rightVP.View()
    rightLines := strings.Split(rightView, "\n")
    maxLines := contentHeight

    var b strings.Builder
    b.WriteString(top)
    b.WriteByte('\n')
    b.WriteString(hr)
    b.WriteByte('\n')
    for i := 0; i < maxLines; i++ {
        var l, r string
        if i < len(leftLines) {
            l = padToWidth(leftLines[i], leftW)
        } else {
            l = strings.Repeat(" ", leftW)
        }
        if i < len(rightLines) {
            r = rightLines[i]
        } else {
            r = ""
        }
        b.WriteString(l)
        b.WriteString(sep)
        b.WriteString(padToWidth(r, rightW))
        if i < maxLines-1 {
            b.WriteByte('\n')
        }
    }
    // Optional overlay right above bottom bar
    if overlayH > 0 {
        b.WriteByte('\n')
        for i, line := range overlay {
            b.WriteString(padToWidth(line, m.width))
            if i < overlayH-1 {
                b.WriteByte('\n')
            }
        }
    }
    // Bottom rule and bottom bar
    b.WriteByte('\n')
    b.WriteString(strings.Repeat("─", m.width))
    b.WriteByte('\n')
    b.WriteString(m.bottomBar())
    return b.String()
}

func (m model) leftBodyLines(max int) []string {
    lines := make([]string, 0, max)
    if len(m.files) == 0 {
        lines = append(lines, "No changes detected")
        return lines
    }
    for i, f := range m.files {
        marker := "  "
        if i == m.selected {
            marker = "> "
        }
        status := fileStatusLabel(f)
        line := fmt.Sprintf("%s%s %s", marker, status, f.Path)
        lines = append(lines, line)
        if len(lines) >= max {
            break
        }
    }
    return lines
}

func (m model) rightBodyLines(max, width int) []string {
    lines := make([]string, 0, max)
    if len(m.files) == 0 {
        return lines
    }
    if m.files[m.selected].Binary {
        lines = append(lines, lipgloss.NewStyle().Faint(true).Render("(Binary file; no text diff)"))
        return lines
    }
    if m.rows == nil {
        lines = append(lines, "Loading diff…")
        return lines
    }
    if m.sideBySide {
        colsW := (width - 1) / 2
        if colsW < 10 {
            colsW = 10
        }
    mid := m.theme.DividerText("│")
        for _, r := range m.rows {
            switch r.Kind {
            case diffview.RowHunk:
                lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render(r.Meta))
            case diffview.RowMeta:
                // skip
            default:
                l := padToWidth(m.colorizeLeft(r), colsW)
                rr := padToWidth(m.colorizeRight(r), colsW)
                lines = append(lines, l+mid+rr)
            }
            if len(lines) >= max {
                break
            }
        }
    } else {
        for _, r := range m.rows {
            switch r.Kind {
            case diffview.RowHunk:
                lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render(r.Meta))
            case diffview.RowContext:
                lines = append(lines, " "+r.Left)
            case diffview.RowAdd:
                lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Render("+ "+r.Right))
            case diffview.RowDel:
                lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("- "+r.Left))
            case diffview.RowReplace:
                lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("- "+r.Left))
                if len(lines) >= max { break }
                lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Render("+ "+r.Right))
            }
            if len(lines) >= max {
                break
            }
        }
    }
    return lines
}

func (m model) topRightTitle() string {
    if len(m.files) == 0 {
        return ""
    }
    header := fmt.Sprintf("%s (%s)", m.files[m.selected].Path, fileStatusLabel(m.files[m.selected]))
    return header
}

func (m model) bottomBar() string {
    leftText := "h: help"
    if m.lastCommit != "" {
        leftText += "  |  last: " + m.lastCommit
    }
    leftStyled := lipgloss.NewStyle().Faint(true).Render(leftText)
    right := lipgloss.NewStyle().Faint(true).Render("refreshed: " + m.lastRefresh.Format("15:04:05"))
    w := m.width
    // Ensure the right part is always visible; truncate left if needed
    rightW := lipgloss.Width(right)
    if rightW >= w {
        // Degenerate case: screen too small; just show right truncated
        return ansi.Truncate(right, w, "…")
    }
    avail := w - rightW - 1 // 1 space gap
    leftRendered := leftStyled
    if lipgloss.Width(leftRendered) > avail {
        leftRendered = ansi.Truncate(leftRendered, avail, "…")
    } else if lipgloss.Width(leftRendered) < avail {
        leftRendered = leftRendered + strings.Repeat(" ", avail-lipgloss.Width(leftRendered))
    }
    return leftRendered + " " + right
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

func loadFiles(repoRoot string) tea.Cmd {
    return func() tea.Msg {
        files, err := gitx.ChangedFiles(repoRoot)
        return filesMsg{files: files, err: err}
    }
}

func loadDiff(repoRoot, path string) tea.Cmd {
    return func() tea.Msg {
        d, err := gitx.DiffHEAD(repoRoot, path)
        if err != nil {
            return diffMsg{path: path, err: err}
        }
        rows := diffview.BuildRowsFromUnified(d)
        return diffMsg{path: path, rows: rows}
    }
}

func loadCurrentDiff(m model) tea.Cmd {
    if len(m.files) == 0 {
        return nil
    }
    return loadDiff(m.repoRoot, m.files[m.selected].Path)
}

func tickOnce() tea.Cmd {
    return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func padToWidth(s string, w int) string {
    width := lipgloss.Width(s)
    if width == w {
        return s
    }
    if width < w {
        return s + strings.Repeat(" ", w-width)
    }
    return ansi.Truncate(s, w, "…")
}

func (m model) viewHelp() string {
    // Full-screen simple help panel
    var b strings.Builder
    title := lipgloss.NewStyle().Bold(true).Render("Diffium Help")
    lines := []string{
        "",
        "j/k or arrows  Move selection",
        "J/K, PgDn/PgUp  Scroll diff",
        "</> or H/L      Adjust left pane width",
        "s              Toggle side-by-side / inline",
        "r              Refresh now",
        "g / G          Top / Bottom",
        "h or Esc       Close help",
        "q              Quit",
        "",
        "Press 'h' or 'Esc' to close.",
    }
    // Center-ish: add left padding
    pad := 4
    if m.width > 60 {
        pad = (m.width - 60) / 2
        if pad < 4 { pad = 4 }
    }
    leftPad := strings.Repeat(" ", pad)
    fmt.Fprintln(&b, leftPad+title)
    for _, l := range lines {
        fmt.Fprintln(&b, leftPad+l)
    }
    // Bottom hint
    hint := lipgloss.NewStyle().Faint(true).Render("h: help    refreshed: " + m.lastRefresh.Format("15:04:05"))
    fmt.Fprintln(&b)
    fmt.Fprint(&b, padToWidth(hint, m.width))
    return b.String()
}

// recalcViewport recalculates right viewport size and content based on current state.
func (m *model) recalcViewport() tea.Cmd {
    if m.width == 0 || m.height == 0 {
        return nil
    }
    leftW := m.leftWidth
    if leftW < 20 {
        leftW = 20
    }
    rightW := m.width - leftW - 1
    if rightW < 1 {
        rightW = 1
    }
    overlayH := 0
    if m.showHelp {
        overlayH += len(m.helpOverlayLines(m.width))
    }
    if m.showCommit {
        overlayH += len(m.commitOverlayLines(m.width))
    }
	if m.searchActive { 
    	overlayH += len(m.searchOverlayLines(m.width))
	}
    contentHeight := m.height - 4 - overlayH
    if contentHeight < 1 {
        contentHeight = 1
    }
    // Set dimensions
    m.rightVP.Width = rightW
    m.rightVP.Height = contentHeight
    // Build content
    m.rightContent = m.rightBodyLinesAll(rightW)

	// Update search matches + highlight state
	if m.searchQuery == "" {
		m.searchMatches = nil
		m.searchIndex = 0
	} else {
		m.recomputeSearchMatches(false)
	}
	m.refreshSearchHighlights()

    return nil
}

// helpOverlayLines returns the bottom overlay lines (without trailing newline).
func (m model) helpOverlayLines(width int) []string {
    if !m.showHelp {
        return nil
    }
    // Header
    title := lipgloss.NewStyle().Bold(true).Render("Help — press 'h' or Esc to close")
    // Keys
    keys := []string{
        "j/k or arrows  Move selection",
        "J/K, PgDn/PgUp  Scroll diff",
        "</> or H/L      Adjust left pane width",
        "c              Commit & push (open wizard)",
        "s              Toggle side-by-side / inline",
        "r              Refresh now",
        "g / G          Top / Bottom",
        "q              Quit",
    }
    lines := make([]string, 0, 2+len(keys))
    // Overlay top rule
    lines = append(lines, strings.Repeat("─", width))
    lines = append(lines, title)
    for _, k := range keys {
        lines = append(lines, k)
    }
    return lines
}

func (m model) commitOverlayLines(width int) []string {
    if !m.showCommit {
        return nil
    }
    lines := make([]string, 0, 64)
    lines = append(lines, strings.Repeat("─", width))
    switch m.commitStep {
    case 0:
        title := lipgloss.NewStyle().Bold(true).Render("Commit — Select files (space: toggle, a: all, enter: continue, esc: cancel)")
        lines = append(lines, title)
        if len(m.cwFiles) == 0 {
            lines = append(lines, lipgloss.NewStyle().Faint(true).Render("No changes to commit"))
            return lines
        }
        for i, f := range m.cwFiles {
            cur := "  "
            if i == m.cwIndex {
                cur = "> "
            }
            mark := "[ ]"
            if m.cwSelected[f.Path] {
                mark = "[x]"
            }
            status := fileStatusLabel(f)
            lines = append(lines, fmt.Sprintf("%s%s %s %s", cur, mark, status, f.Path))
        }
    case 1:
        mode := "action"
        if m.cwInputActive { mode = "input" }
        title := lipgloss.NewStyle().Bold(true).Render("Commit — Message (i: input, enter: continue, b: back, esc: " + map[bool]string{true:"leave input", false:"cancel"}[m.cwInputActive] + ") ["+mode+"]")
        lines = append(lines, title)
        lines = append(lines, m.cwInput.View())
    case 2:
        title := lipgloss.NewStyle().Bold(true).Render("Commit — Confirm (y/enter: commit & push, b: back, esc: cancel)")
        lines = append(lines, title)
        // Summary
        sel := m.selectedPaths()
        lines = append(lines, fmt.Sprintf("Files: %d", len(sel)))
        lines = append(lines, "Message: "+m.cwInput.Value())
        if m.committing {
            lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render("Committing & pushing..."))
        }
        if m.commitErr != "" {
            lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: ")+m.commitErr)
        }
    }
    return lines
}

func (m model) rightBodyLinesAll(width int) []string {
    lines := make([]string, 0, 1024)
    if len(m.files) == 0 {
        return lines
    }
    if m.files[m.selected].Binary {
        lines = append(lines, lipgloss.NewStyle().Faint(true).Render("(Binary file; no text diff)"))
        return lines
    }
    if m.rows == nil {
        lines = append(lines, "Loading diff…")
        return lines
    }
    if m.sideBySide {
        colsW := (width - 1) / 2
        if colsW < 10 {
            colsW = 10
        }
        mid := m.theme.DividerText("│")
        for _, r := range m.rows {
            switch r.Kind {
            case diffview.RowHunk:
                // Show a subtle separator instead of raw @@ header
                lines = append(lines, lipgloss.NewStyle().Faint(true).Render(strings.Repeat("·", width)))
            case diffview.RowMeta:
                // skip
            default:
                l := m.renderSideCell(r, "left", colsW)
                rr := m.renderSideCell(r, "right", colsW)
                lines = append(lines, l+mid+rr)
            }
        }
    } else {
        for _, r := range m.rows {
            switch r.Kind {
            case diffview.RowHunk:
                lines = append(lines, lipgloss.NewStyle().Faint(true).Render(strings.Repeat("·", width)))
            case diffview.RowContext:
                lines = append(lines, "  "+r.Left)
            case diffview.RowAdd:
                lines = append(lines, m.theme.AddText("+ "+r.Right))
            case diffview.RowDel:
                lines = append(lines, m.theme.DelText("- "+r.Left))
            case diffview.RowReplace:
                lines = append(lines, m.theme.DelText("- "+r.Left))
                lines = append(lines, m.theme.AddText("+ "+r.Right))
            }
        }
    }
    return lines
    
}

func (m *model) openSearch() {
	ti := textinput.New()
	ti.Placeholder = "Search diff"
	ti.Prompt = "/ "
	ti.CharLimit = 0
	ti.SetValue(m.searchQuery)
	ti.CursorEnd()
	ti.Focus()
	m.searchInput = ti
	m.searchActive = true
}

func (m *model) closeSearch() {
	if m.searchActive {
		m.searchInput.Blur()
	}
	m.searchActive = false
}

func (m model) handleSearchKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {

	if !m.searchActive {
		return m, nil
	}

    m.searchInput.Focus()

    switch key.String() {
        case "esc":
            m.closeSearch()
            return m, m.recalcViewport()
        case "ctrl+c":
            return m, tea.Quit    
    }
	
    
    // Navigation that does NOT leave input mode
    switch key.Type {
    case tea.KeyEnter:
        return m, (&m).advanceSearch(1)
    case tea.KeyDown: 
        return m, (&m).advanceSearch(1)
    case tea.KeyUp:
        return m, (&m).advanceSearch(-1)
    }
	 

    // Fallback: always let input handle it
    var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(key)
    m.searchQuery = m.searchInput.Value()
    m.recomputeSearchMatches(true)
    m.refreshSearchHighlights()

	if scrollCmd := m.scrollToCurrentMatch(); scrollCmd != nil {
		return m, tea.Batch(cmd, scrollCmd)
	}

    return m, cmd
}

func (m *model) advanceSearch(delta int) tea.Cmd {
	if len(m.searchMatches) == 0 {
		return nil
	}
	span := len(m.searchMatches)
	m.searchIndex = (m.searchIndex + delta) % span
	if m.searchIndex < 0 {
		m.searchIndex += span
	}
	m.refreshSearchHighlights()
	return m.scrollToCurrentMatch()
}

func (m *model) recomputeSearchMatches(resetIndex bool) {
	if m.searchQuery == "" {
		m.searchMatches = nil
		if resetIndex {
			m.searchIndex = 0
		}
		return
	}
	lowerQuery := strings.ToLower(m.searchQuery)
	matches := make([]int, 0, len(m.rightContent))
	for i, line := range m.rightContent {
		plain := strings.ToLower(ansi.Strip(line))
		if strings.Contains(plain, lowerQuery) {
			matches = append(matches, i)
		}
	}
	m.searchMatches = matches
	if len(matches) == 0 {
		if resetIndex {
			m.searchIndex = 0
		}
		return
	}
	if resetIndex || m.searchIndex >= len(matches) {
		m.searchIndex = 0
	}
}

func (m *model) refreshSearchHighlights() {
	if len(m.rightContent) == 0 {
		m.rightVP.SetContent("")
		return
	}
	lines := m.rightContent
	if m.searchQuery != "" {
		lines = m.highlightSearchLines(lines)
	}
	m.rightVP.SetContent(strings.Join(lines, "\n"))
}

type runeRange struct {
	start int
	end   int
}

func (m model) highlightSearchLines(lines []string) []string {
	if len(lines) == 0 || m.searchQuery == "" {
		return lines
	}
	lineHasMatch := make(map[int]struct{}, len(m.searchMatches))
	for _, idx := range m.searchMatches {
		if idx >= 0 && idx < len(lines) {
			lineHasMatch[idx] = struct{}{}
		}
	}
	currentLine := -1
	if len(m.searchMatches) > 0 && m.searchIndex >= 0 && m.searchIndex < len(m.searchMatches) {
		currentLine = m.searchMatches[m.searchIndex]
	}
	result := make([]string, len(lines))
	for i, line := range lines {
		if _, ok := lineHasMatch[i]; !ok {
			result[i] = line
			continue
		}
		ranges := findQueryRanges(line, m.searchQuery)
		if len(ranges) == 0 {
			result[i] = line
			continue
		}
		result[i] = applyANSIRangeHighlight(line, ranges, i == currentLine)
	}
	return result
}

func findQueryRanges(line, query string) []runeRange {
	plain := ansi.Strip(line)
	if plain == "" || query == "" {
		return nil
	}
	lowerRunes := []rune(strings.ToLower(plain))
	queryRunes := []rune(strings.ToLower(query))
	if len(queryRunes) == 0 || len(queryRunes) > len(lowerRunes) {
		return nil
	}
	ranges := make([]runeRange, 0, 4)
	for i := 0; i <= len(lowerRunes)-len(queryRunes); i++ {
		match := true
		for j := 0; j < len(queryRunes); j++ {
			if lowerRunes[i+j] != queryRunes[j] {
				match = false
				break
			}
		}
		if match {
			ranges = append(ranges, runeRange{start: i, end: i + len(queryRunes)})
		}
	}
	if len(ranges) == 0 {
		return nil
	}
	return mergeRuneRanges(ranges)
}

func mergeRuneRanges(ranges []runeRange) []runeRange {
	if len(ranges) <= 1 {
		return ranges
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].start == ranges[j].start {
			return ranges[i].end < ranges[j].end
		}
		return ranges[i].start < ranges[j].start
	})
	merged := make([]runeRange, 0, len(ranges))
	cur := ranges[0]
	for _, r := range ranges[1:] {
		if r.start <= cur.end { // overlap or adjacent
			if r.end > cur.end {
				cur.end = r.end
			}
			continue
		}
		merged = append(merged, cur)
		cur = r
	}
	merged = append(merged, cur)
	return merged
}

func applyANSIRangeHighlight(line string, ranges []runeRange, isCurrent bool) string {
	if len(ranges) == 0 {
		return line
	}
	startSeq := searchMatchStartSeq
	if isCurrent {
		startSeq = searchCurrentMatchStartSeq
	}
	endSeq := searchMatchEndSeq
	var b strings.Builder
	matchIdx := 0
	inMatch := false
	runePos := 0
	for i := 0; i < len(line); {
		if line[i] == 0x1b {
			next := consumeANSIEscape(line, i)
			b.WriteString(line[i:next])
			i = next
			continue
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		_ = r // rune value unused beyond size and counting
		if inMatch {
			for matchIdx < len(ranges) && runePos >= ranges[matchIdx].end {
				b.WriteString(endSeq)
				inMatch = false
				matchIdx++
			}
		}
		for !inMatch && matchIdx < len(ranges) && runePos >= ranges[matchIdx].end {
			matchIdx++
		}
		if !inMatch && matchIdx < len(ranges) && runePos == ranges[matchIdx].start {
			b.WriteString(startSeq)
			inMatch = true
		}
		b.WriteString(line[i : i+size])
		runePos++
		i += size
	}
	if inMatch {
		b.WriteString(endSeq)
	}
	return b.String()
}

func consumeANSIEscape(s string, i int) int {
	if i >= len(s) || s[i] != 0x1b {
		if i+1 > len(s) {
			return len(s)
		}
		return i + 1
	}
	j := i + 1
	if j >= len(s) {
		return j
	}
	switch s[j] {
	case '[': // CSI
		j++
		for j < len(s) {
			c := s[j]
			if c >= 0x40 && c <= 0x7e {
				j++
				break
			}
			j++
		}
	case ']': // OSC
		j++
		for j < len(s) && s[j] != 0x07 {
			j++
		}
		if j < len(s) {
			j++
		}
	case 'P', 'X', '^', '_': // DCS, SOS, PM, APC
		j++
		for j < len(s) {
			if s[j] == 0x1b {
				j++
				break
			}
			j++
		}
	default:
		j++
	}
	if j <= i {
		return i + 1
	}
	return j
}

func (m *model) scrollToCurrentMatch() tea.Cmd {
	if len(m.searchMatches) == 0 {
		return nil
	}
	target := m.searchMatches[m.searchIndex]
	if target < 0 {
		target = 0
	}
	offset := target
	if m.rightVP.Height > 0 {
		offset = target - m.rightVP.Height/2
		if offset < 0 {
			offset = 0
		}
	}
	maxOffset := len(m.rightContent) - m.rightVP.Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	m.rightVP.SetYOffset(offset)
	return nil
}

func (m model) searchOverlayLines(width int) []string {
	if !m.searchActive || width <= 0 {
		return nil
	}
	lines := make([]string, 0, 3)
	lines = append(lines, m.theme.DividerText(strings.Repeat("?", width)))
	lines = append(lines, padToWidth(m.searchInput.View(), width))
	status := "Type to search (esc: close, enter: finish typing)"
	if m.searchQuery != "" {
		if len(m.searchMatches) == 0 {
			status = "No matches (esc: close)"
		} else {
			status = fmt.Sprintf("Match %d of %d  (Enter/↓: next, ↑: prev, Esc: close)", m.searchIndex+1, len(m.searchMatches))
		}
	}
	lines = append(lines, padToWidth(lipgloss.NewStyle().Faint(true).Render(status), width))
	return lines
}

// --- Commit wizard ---

type lastCommitMsg struct{
    summary string
    err error
}

func loadLastCommit(repoRoot string) tea.Cmd {
    return func() tea.Msg {
        s, err := gitx.LastCommitSummary(repoRoot)
        return lastCommitMsg{summary: s, err: err}
    }
}

func (m *model) openCommitWizard() {
    m.showCommit = true
    m.commitStep = 0
    // snapshot files list
    m.cwFiles = append([]gitx.FileChange(nil), m.files...)
    m.cwSelected = map[string]bool{}
    for _, f := range m.cwFiles {
        m.cwSelected[f.Path] = true // default include all
    }
    m.cwIndex = 0
    m.commitDone = false
    m.commitErr = ""
    m.committing = false
    m.cwInput = textinput.Model{}
    m.cwInput.Placeholder = "Commit message"
    m.cwInput.CharLimit = 0
    m.cwInputActive = false
}

// handle commit wizard keys
func (m model) handleCommitKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch m.commitStep {
    case 0: // select files
        switch key.String() {
        case "esc":
            m.showCommit = false
            return m, m.recalcViewport()
        case "enter":
            m.commitStep = 1
            // focus text input
            ti := textinput.New()
            ti.Placeholder = "Commit message"
            ti.Prompt = "> "
            ti.Focus()
            m.cwInput = ti
            return m, m.recalcViewport()
        case "j", "down":
            if len(m.cwFiles) > 0 && m.cwIndex < len(m.cwFiles)-1 {
                m.cwIndex++
            }
            return m, nil
        case "k", "up":
            if m.cwIndex > 0 {
                m.cwIndex--
            }
            return m, nil
        case " ":
            if len(m.cwFiles) > 0 {
                p := m.cwFiles[m.cwIndex].Path
                m.cwSelected[p] = !m.cwSelected[p]
            }
            return m, nil
        case "a":
            all := true
            for _, f := range m.cwFiles {
                if !m.cwSelected[f.Path] { all = false; break }
            }
            // toggle all
            set := !all
            for _, f := range m.cwFiles {
                m.cwSelected[f.Path] = set
            }
            return m, nil
        }
    case 1: // message (input/action modes)
        switch key.String() {
        case "esc":
            if m.cwInputActive {
                // leave input mode
                m.cwInputActive = false
                return m, m.recalcViewport()
            }
            m.showCommit = false
            return m, m.recalcViewport()
        case "i":
            if !m.cwInputActive {
                m.cwInputActive = true
                m.cwInput.Focus()
                return m, m.recalcViewport()
            }
            // if already active, treat as input
        case "b":
            if !m.cwInputActive {
                m.commitStep = 0
                return m, m.recalcViewport()
            }
            // in input mode, 'b' is literal
        case "enter":
            if !m.cwInputActive {
                m.commitStep = 2
                m.commitDone = false
                m.commitErr = ""
                m.committing = false
                return m, m.recalcViewport()
            }
            // in input mode, forward to input
        }
        // Default: if input mode, forward to text input; else ignore
        if m.cwInputActive {
            var cmd tea.Cmd
            m.cwInput, cmd = m.cwInput.Update(key)
            return m, cmd
        }
        return m, nil
    case 2: // confirm/progress
        switch key.String() {
        case "esc":
            // can't cancel mid-commit, but if not running: exit
            if !m.committing {
                m.showCommit = false
                return m, m.recalcViewport()
            }
            return m, nil
        case "b":
            if !m.committing && !m.commitDone {
                m.commitStep = 1
                return m, m.recalcViewport()
            }
            return m, nil
        case "y", "enter":
            if !m.committing && !m.commitDone {
                sel := m.selectedPaths()
                if len(sel) == 0 {
                    m.commitErr = "no files selected"
                    return m, nil
                }
                if strings.TrimSpace(m.cwInput.Value()) == "" {
                    m.commitErr = "empty commit message"
                    return m, nil
                }
                m.commitErr = ""
                m.committing = true
                return m, runCommit(m.repoRoot, sel, m.cwInput.Value())
            }
            return m, nil
        }
    }
    return m, nil
}

func (m model) selectedPaths() []string {
    var out []string
    for _, f := range m.cwFiles {
        if m.cwSelected[f.Path] {
            out = append(out, f.Path)
        }
    }
    return out
}

type commitProgressMsg struct{}
type commitResultMsg struct{ err error }

func runCommit(repoRoot string, paths []string, message string) tea.Cmd {
    return func() tea.Msg {
        // Stage selected files
        if err := gitx.StageFiles(repoRoot, paths); err != nil {
            return commitResultMsg{err: err}
        }
        // Commit
        if err := gitx.Commit(repoRoot, message); err != nil {
            return commitResultMsg{err: err}
        }
        // Push
        if err := gitx.Push(repoRoot); err != nil {
            return commitResultMsg{err: err}
        }
        return commitResultMsg{err: nil}
    }
}



func (m model) colorizeLeft(r diffview.Row) string {
    switch r.Kind {
    case diffview.RowContext:
        return r.Left
    case diffview.RowDel:
        return m.theme.DelText(r.Left)
    case diffview.RowReplace:
        return m.theme.DelText(r.Left)
    case diffview.RowAdd:
        return ""
    default:
        return r.Left
    }
}

func (m model) colorizeRight(r diffview.Row) string {
    switch r.Kind {
    case diffview.RowContext:
        return r.Right
    case diffview.RowAdd:
        return m.theme.AddText(r.Right)
    case diffview.RowReplace:
        return m.theme.AddText(r.Right)
    case diffview.RowDel:
        return ""
    default:
        return r.Right
    }
}

// renderSideCell renders a left or right cell with a colored marker and padding.
// side is "left" or "right". width is the total cell width.
func (m model) renderSideCell(r diffview.Row, side string, width int) string {
    marker := " "
    content := ""
    switch side {
    case "left":
        content = r.Left
        switch r.Kind {
        case diffview.RowContext:
            marker = " "
        case diffview.RowDel, diffview.RowReplace:
            marker = m.theme.DelText("-")
            content = m.theme.DelText(content)
        case diffview.RowAdd:
            marker = " "
            content = ""
        }
    case "right":
        content = r.Right
        switch r.Kind {
        case diffview.RowContext:
            marker = " "
        case diffview.RowAdd, diffview.RowReplace:
            marker = m.theme.AddText("+")
            content = m.theme.AddText(content)
        case diffview.RowDel:
            marker = " "
            content = ""
        }
    }
    // Reserve 2 cols: marker + space
    if width <= 2 {
        return ansi.Truncate(marker+" ", width, "")
    }
    bodyW := width - 2
    body := padToWidth(content, bodyW)
    return marker + " " + body
}
