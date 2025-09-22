package tui

import (
    "fmt"
    "sort"
    "strings"
    "time"

    "github.com/charmbracelet/bubbles/textinput"
    "github.com/charmbracelet/bubbles/viewport"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/x/ansi"
    "github.com/interpretive-systems/diffium/internal/diffview"
    "github.com/interpretive-systems/diffium/internal/gitx"
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
    leftOffset  int
    rightVP     viewport.Model
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
    // uncommit wizard state
    showUncommit   bool
    ucStep         int // 0: select files, 1: confirm/progress
    ucFiles        []gitx.FileChange // list ALL current changes (like commit wizard)
    ucSelected     map[string]bool   // keyed by path
    ucIndex        int
    ucEligible     map[string]bool   // paths that are part of HEAD (last commit)
    uncommitting   bool
    uncommitErr    string
    uncommitDone   bool

    // reset/clean wizard state
    showResetClean bool
    rcStep         int // 0: select, 1: preview, 2: confirm (yellow), 3: confirm (red)
    rcDoReset      bool
    rcDoClean      bool
    rcIncludeIgnored bool
    rcIndex       int
    rcPreviewLines []string // from git clean -dn
    rcPreviewErr   string
    rcRunning      bool
    rcErr          string
    rcDone         bool
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
        if m.showHelp {
            switch msg.String() {
            case "q":
                return m, tea.Quit
            case "h", "esc":
                m.showHelp = false
                return m, m.recalcViewport()
            default:
                return m, nil
            }
        }
        if m.showCommit {
            return m.handleCommitKeys(msg)
        }
        if m.showUncommit {
            return m.handleUncommitKeys(msg)
        }
        if m.showResetClean {
            return m.handleResetCleanKeys(msg)
        }
        switch msg.String() {
        case "ctrl+c", "q":
            return m, tea.Quit
        case "h":
            m.showHelp = true
            return m, m.recalcViewport()
        case "c":
            // Open commit wizard
            m.openCommitWizard()
            return m, m.recalcViewport()
        case "u":
            // Open uncommit wizard
            m.openUncommitWizard()
            return m, tea.Batch(loadUncommitFiles(m.repoRoot), loadUncommitEligible(m.repoRoot), m.recalcViewport())
        case "R":
            // Open reset/clean wizard
            m.openResetCleanWizard()
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
        case "[":
            // Page up left pane
            vis := m.rightVP.Height
            if vis <= 0 { vis = 10 }
            step := vis - 1
            if step < 1 { step = 1 }
            newOffset := m.leftOffset - step
            if newOffset < 0 { newOffset = 0 }
            // Keep selection visible within new viewport
            if m.selected < newOffset {
                newOffset = m.selected
            }
            maxStart := len(m.files) - vis
            if maxStart < 0 { maxStart = 0 }
            if newOffset > maxStart { newOffset = maxStart }
            m.leftOffset = newOffset
            return m, m.recalcViewport()
        case "]":
            // Page down left pane
            vis := m.rightVP.Height
            if vis <= 0 { vis = 10 }
            step := vis - 1
            if step < 1 { step = 1 }
            maxStart := len(m.files) - vis
            if maxStart < 0 { maxStart = 0 }
            newOffset := m.leftOffset + step
            if newOffset > maxStart { newOffset = maxStart }
            // Keep selection visible within new viewport
            if m.selected >= newOffset+vis {
                newOffset = m.selected - vis + 1
                if newOffset < 0 { newOffset = 0 }
            }
            m.leftOffset = newOffset
            return m, m.recalcViewport()
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
    case rcPreviewMsg:
        m.rcPreviewErr = ""
        if msg.err != nil {
            m.rcPreviewErr = msg.err.Error()
            m.rcPreviewLines = nil
        } else {
            m.rcPreviewLines = msg.lines
        }
        return m, m.recalcViewport()
    case rcResultMsg:
        m.rcRunning = false
        if msg.err != nil {
            m.rcErr = msg.err.Error()
            m.rcDone = false
            return m, tea.Batch(loadFiles(m.repoRoot), m.recalcViewport())
        }
        m.rcErr = ""
        m.rcDone = true
        m.showResetClean = false
        return m, tea.Batch(loadFiles(m.repoRoot), m.recalcViewport())
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
    case uncommitFilesMsg:
        if msg.err != nil {
            m.uncommitErr = msg.err.Error()
            m.ucFiles = nil
            m.ucSelected = map[string]bool{}
            m.ucIndex = 0
            return m, m.recalcViewport()
        }
        m.ucFiles = msg.files
        m.ucSelected = map[string]bool{}
        for _, f := range m.ucFiles {
            m.ucSelected[f.Path] = true
        }
        m.ucIndex = 0
        return m, m.recalcViewport()
    case uncommitEligibleMsg:
        if msg.err != nil {
            // No parent commit or other issue; treat as no eligible files.
            m.ucEligible = map[string]bool{}
            return m, m.recalcViewport()
        }
        m.ucEligible = map[string]bool{}
        for _, p := range msg.paths {
            m.ucEligible[p] = true
        }
        return m, m.recalcViewport()
    case uncommitResultMsg:
        m.uncommitting = false
        if msg.err != nil {
            m.uncommitErr = msg.err.Error()
            m.uncommitDone = false
            return m, tea.Batch(loadFiles(m.repoRoot), loadLastCommit(m.repoRoot), m.recalcViewport())
        }
        m.uncommitErr = ""
        m.uncommitDone = true
        m.showUncommit = false
        return m, tea.Batch(loadFiles(m.repoRoot), loadLastCommit(m.repoRoot), m.recalcViewport())
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
    if m.showUncommit {
        overlay = append(overlay, m.uncommitOverlayLines(m.width)...)
    }
    if m.showResetClean {
        overlay = append(overlay, m.resetCleanOverlayLines(m.width)...)
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
    start := m.leftOffset
    if start < 0 { start = 0 }
    if start > len(m.files) { start = len(m.files) }
    end := start + max
    if end > len(m.files) { end = len(m.files) }
    for i := start; i < end; i++ {
        f := m.files[i]
        marker := "  "
        if i == m.selected {
            marker = "> "
        }
        status := fileStatusLabel(f)
        line := fmt.Sprintf("%s%s %s", marker, status, f.Path)
        lines = append(lines, line)
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
        "[/]            Page left file list",
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
    if m.showUncommit {
        overlayH += len(m.uncommitOverlayLines(m.width))
    }
    if m.showResetClean {
        overlayH += len(m.resetCleanOverlayLines(m.width))
    }
    contentHeight := m.height - 4 - overlayH
    if contentHeight < 1 {
        contentHeight = 1
    }
    // Clamp leftOffset and keep selection visible in left pane
    vis := contentHeight
    if vis < 1 { vis = 1 }
    if m.leftOffset < 0 { m.leftOffset = 0 }
    maxStart := len(m.files) - vis
    if maxStart < 0 { maxStart = 0 }
    if m.leftOffset > maxStart { m.leftOffset = maxStart }
    if len(m.files) > 0 {
        if m.selected < m.leftOffset {
            m.leftOffset = m.selected
        } else if m.selected >= m.leftOffset+vis {
            m.leftOffset = m.selected - vis + 1
            if m.leftOffset < 0 { m.leftOffset = 0 }
        }
        if m.leftOffset > maxStart { m.leftOffset = maxStart }
    }

    // Set dimensions
    m.rightVP.Width = rightW
    m.rightVP.Height = contentHeight
    // Build content
    content := strings.Join(m.rightBodyLinesAll(rightW), "\n")
    m.rightVP.SetContent(content)
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
        "[/]            Page left file list",
        "u              Uncommit (open wizard)",
        "R              Reset/Clean (open wizard)",
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

// --- Uncommit wizard ---

type uncommitFilesMsg struct {
    files []gitx.FileChange
    err   error
}

// --- Reset/Clean wizard ---

type rcPreviewMsg struct{
    lines []string
    err error
}

type rcResultMsg struct{ err error }

func (m *model) openResetCleanWizard() {
    m.showResetClean = true
    m.rcStep = 0
    m.rcDoReset = false
    m.rcDoClean = false
    m.rcIncludeIgnored = false
    m.rcIndex = 0
    m.rcPreviewLines = nil
    m.rcPreviewErr = ""
    m.rcRunning = false
    m.rcErr = ""
    m.rcDone = false
}

func (m model) resetCleanOverlayLines(width int) []string {
    if !m.showResetClean { return nil }
    lines := make([]string, 0, 128)
    lines = append(lines, strings.Repeat("─", width))
    switch m.rcStep {
    case 0: // select actions/options
        title := lipgloss.NewStyle().Bold(true).Render("Reset/Clean — Select actions (space: toggle, a: toggle both, enter: continue, esc: cancel)")
        lines = append(lines, title)
        items := []struct{ label string; on bool }{
            {"Reset working tree (git reset --hard)", m.rcDoReset},
            {"Clean untracked (git clean -d -f)", m.rcDoClean},
            {"Include ignored in clean (-x)", m.rcIncludeIgnored},
        }
        for i, it := range items {
            cur := "  "
            if i == m.rcIndex { cur = "> " }
            lines = append(lines, fmt.Sprintf("%s%s %s", cur, checkbox(it.on), it.label))
        }
        lines = append(lines, lipgloss.NewStyle().Faint(true).Render("A preview will be shown before confirmation"))
        if m.rcErr != "" {
            lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: ")+m.rcErr)
        }
    case 1: // preview
        title := lipgloss.NewStyle().Bold(true).Render("Reset/Clean — Preview (enter: continue, b: back, esc: cancel)")
        lines = append(lines, title)
        // Reset preview summary from current file list (tracked changes)
        if m.rcDoReset {
            tracked := 0
            for _, f := range m.files {
                if !f.Untracked && (f.Staged || f.Unstaged || f.Deleted) {
                    tracked++
                }
            }
            lines = append(lines, fmt.Sprintf("Reset would discard tracked changes for ~%d file(s)", tracked))
        } else {
            lines = append(lines, lipgloss.NewStyle().Faint(true).Render("Reset: (not selected)"))
        }
        // Clean preview
        if m.rcDoClean {
            if m.rcPreviewErr != "" {
                lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Clean preview error: ")+m.rcPreviewErr)
            } else if len(m.rcPreviewLines) == 0 {
                lines = append(lines, lipgloss.NewStyle().Faint(true).Render("Clean: nothing to remove"))
            } else {
                lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Clean would remove:"))
                max := 10
                for i, l := range m.rcPreviewLines {
                    if i >= max { break }
                    lines = append(lines, l)
                }
                if len(m.rcPreviewLines) > max {
                    lines = append(lines, fmt.Sprintf("… and %d more", len(m.rcPreviewLines)-max))
                }
                if m.rcIncludeIgnored {
                    lines = append(lines, lipgloss.NewStyle().Faint(true).Render("(including ignored files)"))
                }
            }
        } else {
            lines = append(lines, lipgloss.NewStyle().Faint(true).Render("Clean: (not selected)"))
        }
        // Show exact commands
        var cmds []string
        if m.rcDoReset { cmds = append(cmds, "git reset --hard") }
        if m.rcDoClean {
            c := "git clean -d -f"
            if m.rcIncludeIgnored { c += " -x" }
            cmds = append(cmds, c)
        }
        if len(cmds) > 0 {
            lines = append(lines, lipgloss.NewStyle().Faint(true).Render("Commands: "+strings.Join(cmds, "  &&  ")))
        } else {
            lines = append(lines, lipgloss.NewStyle().Faint(true).Render("No actions selected"))
        }
    case 2: // first (yellow) confirmation
        title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220")).Render("Confirm — This will discard local changes (enter: continue, b: back, esc: cancel)")
        lines = append(lines, title)
        lines = append(lines, "Proceed to final confirmation?")
    case 3: // final (red) confirmation
        title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Render("FINAL CONFIRMATION — Destructive action (y/enter: execute, b: back, esc: cancel)")
        lines = append(lines, title)
        if m.rcRunning {
            lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render("Running…"))
        }
        if m.rcErr != "" {
            lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: ")+m.rcErr)
        }
    }
    return lines
}

func checkbox(on bool) string { if on { return "[x]" } ; return "[ ]" }

func (m model) handleResetCleanKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch m.rcStep {
    case 0: // select
        switch key.String() {
        case "esc":
            m.showResetClean = false
            return m, m.recalcViewport()
        case "j", "down":
            if m.rcIndex < 2 { m.rcIndex++ }
            return m, nil
        case "k", "up":
            if m.rcIndex > 0 { m.rcIndex-- }
            return m, nil
        case " ":
            switch m.rcIndex {
            case 0:
                m.rcDoReset = !m.rcDoReset
            case 1:
                m.rcDoClean = !m.rcDoClean
            case 2:
                m.rcIncludeIgnored = !m.rcIncludeIgnored
            }
            return m, nil
        case "a":
            both := m.rcDoReset && m.rcDoClean
            m.rcDoReset = !both
            m.rcDoClean = !both
            return m, nil
        case "enter":
            if !m.rcDoReset && !m.rcDoClean {
                m.rcErr = "no actions selected"
                return m, m.recalcViewport()
            }
            m.rcStep = 1
            m.rcPreviewErr = ""
            m.rcPreviewLines = nil
            if m.rcDoClean {
                return m, loadRCPreview(m.repoRoot, m.rcIncludeIgnored)
            }
            return m, m.recalcViewport()
        }
    case 1: // preview
        switch key.String() {
        case "esc":
            m.showResetClean = false
            return m, m.recalcViewport()
        case "b":
            m.rcStep = 0
            return m, m.recalcViewport()
        case "enter":
            m.rcStep = 2
            return m, m.recalcViewport()
        }
    case 2: // yellow confirm
        switch key.String() {
        case "esc":
            m.showResetClean = false
            return m, m.recalcViewport()
        case "b":
            m.rcStep = 1
            return m, m.recalcViewport()
        case "enter":
            m.rcStep = 3
            return m, m.recalcViewport()
        }
    case 3: // red confirm
        switch key.String() {
        case "esc":
            if !m.rcRunning {
                m.showResetClean = false
                return m, m.recalcViewport()
            }
            return m, nil
        case "b":
            if !m.rcRunning {
                m.rcStep = 2
                return m, m.recalcViewport()
            }
            return m, nil
        case "y", "enter":
            if !m.rcRunning && !m.rcDone {
                m.rcRunning = true
                m.rcErr = ""
                return m, runResetClean(m.repoRoot, m.rcDoReset, m.rcDoClean, m.rcIncludeIgnored)
            }
            return m, nil
        }
    }
    return m, nil
}

func loadRCPreview(repoRoot string, includeIgnored bool) tea.Cmd {
    return func() tea.Msg {
        lines, err := gitx.CleanPreview(repoRoot, includeIgnored)
        return rcPreviewMsg{lines: lines, err: err}
    }
}

func runResetClean(repoRoot string, doReset, doClean bool, includeIgnored bool) tea.Cmd {
    return func() tea.Msg {
        if err := gitx.ResetAndClean(repoRoot, doReset, doClean, includeIgnored); err != nil {
            return rcResultMsg{err: err}
        }
        return rcResultMsg{err: nil}
    }
}

type uncommitResultMsg struct{ err error }

func loadUncommitFiles(repoRoot string) tea.Cmd {
    return func() tea.Msg {
        files, err := gitx.ChangedFiles(repoRoot)
        return uncommitFilesMsg{files: files, err: err}
    }
}

type uncommitEligibleMsg struct {
    paths []string
    err   error
}

func loadUncommitEligible(repoRoot string) tea.Cmd {
    return func() tea.Msg {
        ps, err := gitx.FilesInLastCommit(repoRoot)
        return uncommitEligibleMsg{paths: ps, err: err}
    }
}

func (m *model) openUncommitWizard() {
    m.showUncommit = true
    m.ucStep = 0
    m.ucFiles = nil
    m.ucSelected = map[string]bool{}
    m.ucIndex = 0
    m.uncommitting = false
    m.uncommitErr = ""
    m.uncommitDone = false
}

func (m model) uncommitOverlayLines(width int) []string {
    if !m.showUncommit {
        return nil
    }
    lines := make([]string, 0, 64)
    lines = append(lines, strings.Repeat("─", width))
    switch m.ucStep {
    case 0:
        title := lipgloss.NewStyle().Bold(true).Render("Uncommit — Select files (space: toggle, a: all, enter: continue, esc: cancel)")
        lines = append(lines, title)
        if len(m.ucFiles) == 0 && m.uncommitErr == "" {
            lines = append(lines, lipgloss.NewStyle().Faint(true).Render("Loading files…"))
            return lines
        }
        if m.uncommitErr != "" {
            lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: ")+m.uncommitErr)
        }
        if len(m.ucFiles) == 0 && m.uncommitErr == "" {
            lines = append(lines, lipgloss.NewStyle().Faint(true).Render("No changes to choose from"))
            return lines
        }
        for i, f := range m.ucFiles {
            cur := "  "
            if i == m.ucIndex { cur = "> " }
            mark := "[ ]"
            if m.ucSelected[f.Path] { mark = "[x]" }
            status := fileStatusLabel(f)
            lines = append(lines, fmt.Sprintf("%s%s %s %s", cur, mark, status, f.Path))
        }
    case 1:
        title := lipgloss.NewStyle().Bold(true).Render("Uncommit — Confirm (y/enter: uncommit, b: back, esc: cancel)")
        lines = append(lines, title)
        sel := m.uncommitSelectedPaths()
        total := len(sel)
        elig := 0
        if m.ucEligible != nil {
            for _, p := range sel { if m.ucEligible[p] { elig++ } }
        }
        inelig := total - elig
        lines = append(lines, fmt.Sprintf("Selected: %d  Eligible to uncommit: %d  Ignored: %d", total, elig, inelig))
        if m.ucEligible == nil {
            lines = append(lines, lipgloss.NewStyle().Faint(true).Render("(resolving eligibility…)"))
        }
        if m.uncommitting {
            lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render("Uncommitting…"))
        }
        if m.uncommitErr != "" {
            lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: ")+m.uncommitErr)
        }
    }
    return lines
}

func (m model) handleUncommitKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch m.ucStep {
    case 0:
        switch key.String() {
        case "esc":
            m.showUncommit = false
            return m, m.recalcViewport()
        case "enter":
            if len(m.ucFiles) == 0 {
                return m, nil
            }
            m.ucStep = 1
            m.uncommitErr = ""
            m.uncommitDone = false
            m.uncommitting = false
            return m, m.recalcViewport()
        case "j", "down":
            if len(m.ucFiles) > 0 && m.ucIndex < len(m.ucFiles)-1 { m.ucIndex++ }
            return m, nil
        case "k", "up":
            if m.ucIndex > 0 { m.ucIndex-- }
            return m, nil
        case " ":
            if len(m.ucFiles) > 0 {
                p := m.ucFiles[m.ucIndex].Path
                m.ucSelected[p] = !m.ucSelected[p]
            }
            return m, nil
        case "a":
            all := true
            for _, f := range m.ucFiles { if !m.ucSelected[f.Path] { all = false; break } }
            set := !all
            for _, f := range m.ucFiles { m.ucSelected[f.Path] = set }
            return m, nil
        }
    case 1:
        switch key.String() {
        case "esc":
            if !m.uncommitting {
                m.showUncommit = false
                return m, m.recalcViewport()
            }
            return m, nil
        case "b":
            if !m.uncommitting && !m.uncommitDone {
                m.ucStep = 0
                return m, m.recalcViewport()
            }
            return m, nil
        case "y", "enter":
            if !m.uncommitting && !m.uncommitDone {
                sel := m.uncommitSelectedPaths()
                if len(sel) == 0 {
                    m.uncommitErr = "no files selected"
                    return m, nil
                }
                m.uncommitErr = ""
                m.uncommitting = true
                return m, runUncommit(m.repoRoot, sel)
            }
            return m, nil
        }
    }
    return m, nil
}

func (m model) uncommitSelectedPaths() []string {
    var out []string
    for _, f := range m.ucFiles {
        if m.ucSelected[f.Path] { out = append(out, f.Path) }
    }
    return out
}

func runUncommit(repoRoot string, paths []string) tea.Cmd {
    return func() tea.Msg {
        // Filter to eligible paths (present in last commit)
        eligible, err := gitx.FilesInLastCommit(repoRoot)
        if err != nil {
            return uncommitResultMsg{err: err}
        }
        eligSet := map[string]bool{}
        for _, p := range eligible { eligSet[p] = true }
        var toUncommit []string
        for _, p := range paths { if eligSet[p] { toUncommit = append(toUncommit, p) } }
        if len(toUncommit) == 0 {
            return uncommitResultMsg{err: fmt.Errorf("no selected files are in the last commit")}
        }
        if err := gitx.UncommitFiles(repoRoot, toUncommit); err != nil {
            return uncommitResultMsg{err: err}
        }
        return uncommitResultMsg{err: nil}
    }
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
