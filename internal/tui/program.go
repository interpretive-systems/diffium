package tui

import (
    "fmt"
    "sort"
    "strings"
    "time"

    "github.com/charmbracelet/bubbles/viewport"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/x/ansi"
    "github.com/interpretive-systems/diffium/internal/diffview"
    "github.com/interpretive-systems/diffium/internal/gitx"
)

type model struct {
    repoRoot    string
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
    m := model{repoRoot: repoRoot, sideBySide: true}
    p := tea.NewProgram(m, tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        return err
    }
    return nil
}

func (m model) Init() tea.Cmd {
    return tea.Batch(loadFiles(m.repoRoot), tickOnce())
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
                return m, nil
            default:
                return m, nil
            }
        }
        switch msg.String() {
        case "ctrl+c", "q":
            return m, tea.Quit
        case "h":
            m.showHelp = true
            return m, nil
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
                m.rightVP.GotoTop()
                return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path), m.recalcViewport())
            }
        case "g":
            if len(m.files) > 0 {
                m.selected = 0
                m.rightVP.GotoTop()
                return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path), m.recalcViewport())
            }
        case "G":
            if len(m.files) > 0 {
                m.selected = len(m.files) - 1
                m.rightVP.GotoTop()
                return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path), m.recalcViewport())
            }
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
    }
    return m, nil
}

func (m model) View() string {
    if m.showHelp {
        return m.viewHelp()
    }
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
    sep := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")

    // Row 1: top bar
    top := "Changes | " + m.topRightTitle()
    // Row 2: horizontal rule
    hr := strings.Repeat("─", m.width)

    // Row 3: columns
    contentHeight := m.height - 3 // top + rule + bottom bar
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
        mid := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")
        for _, r := range m.rows {
            switch r.Kind {
            case diffview.RowHunk:
                lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render(r.Meta))
            case diffview.RowMeta:
                // skip
            default:
                l := padToWidth(colorizeLeft(r), colsW)
                rr := padToWidth(colorizeRight(r), colsW)
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
    left := lipgloss.NewStyle().Faint(true).Render("h: help")
    right := lipgloss.NewStyle().Faint(true).Render("refreshed: " + m.lastRefresh.Format("15:04:05"))
    w := m.width
    // Compose with right-aligned timestamp
    leftW := runewidth.StringWidth(left)
    rightW := runewidth.StringWidth(right)
    if leftW+rightW >= w {
        return padToWidth(left, w)
    }
    return left + strings.Repeat(" ", w-leftW-rightW) + right
}

func fileStatusLabel(f gitx.FileChange) string {
    var tags []string
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
func (m model) recalcViewport() tea.Cmd {
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
    contentHeight := m.height - 3
    if contentHeight < 1 {
        contentHeight = 1
    }
    // Set dimensions
    m.rightVP.Width = rightW
    m.rightVP.Height = contentHeight
    // Build content
    content := strings.Join(m.rightBodyLinesAll(rightW), "\n")
    m.rightVP.SetContent(content)
    return nil
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
        mid := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("│")
        for _, r := range m.rows {
            switch r.Kind {
            case diffview.RowHunk:
                lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render(r.Meta))
            case diffview.RowMeta:
                // skip
            default:
                l := padToWidth(colorizeLeft(r), colsW)
                rr := padToWidth(colorizeRight(r), colsW)
                lines = append(lines, l+mid+rr)
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
                lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Render("+ "+r.Right))
            }
        }
    }
    return lines
}

func colorizeLeft(r diffview.Row) string {
    switch r.Kind {
    case diffview.RowContext:
        return r.Left
    case diffview.RowDel:
        return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(r.Left)
    case diffview.RowReplace:
        return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(r.Left)
    case diffview.RowAdd:
        return ""
    default:
        return r.Left
    }
}

func colorizeRight(r diffview.Row) string {
    switch r.Kind {
    case diffview.RowContext:
        return r.Right
    case diffview.RowAdd:
        return lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Render(r.Right)
    case diffview.RowReplace:
        return lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Render(r.Right)
    case diffview.RowDel:
        return ""
    default:
        return r.Right
    }
}
