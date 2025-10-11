package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/interpretive-systems/diffium/internal/diffview"
	"github.com/interpretive-systems/diffium/internal/gitx"
	"github.com/interpretive-systems/diffium/internal/prefs"
)

const (
	// Normal match: black on bright white
	searchMatchStartSeq = "\x1b[30;107m"
	// Current match: black on yellow
	searchCurrentMatchStartSeq = "\x1b[30;43m"
	// Reset all styles
	searchMatchEndSeq = "\x1b[0m"
)

type model struct {
	repoRoot       string
	theme          Theme
	files          []gitx.FileChange
	selected       int
	rows           []diffview.Row
	sideBySide     bool
	diffMode       string
	width          int
	height         int
	status         string
	lastRefresh    time.Time
	showHelp       bool
	leftWidth      int
	savedLeftWidth int
	leftOffset     int
	rightVP        viewport.Model
	rightXOffset   int
	wrapLines      bool

	rightContent []string

	keyBuffer string
	// commit wizard state
	showCommit    bool
	commitStep    int // 0: select files, 1: message, 2: confirm/progress
	cwFiles       []gitx.FileChange
	cwSelected    map[string]bool
	cwIndex       int
	cwInput       textinput.Model
	cwInputActive bool
	committing    bool
	commitErr     string
	commitDone    bool
	lastCommit    string

	currentBranch string
	// uncommit wizard state
	showUncommit bool
	ucStep       int               // 0: select files, 1: confirm/progress
	ucFiles      []gitx.FileChange // list ALL current changes (like commit wizard)
	ucSelected   map[string]bool   // keyed by path
	ucIndex      int
	ucEligible   map[string]bool // paths that are part of HEAD (last commit)
	uncommitting bool
	uncommitErr  string
	uncommitDone bool

	// reset/clean wizard state
	showResetClean   bool
	rcStep           int // 0: select, 1: preview, 2: confirm (yellow), 3: confirm (red)
	rcDoReset        bool
	rcDoClean        bool
	rcIncludeIgnored bool
	rcIndex          int
	rcPreviewLines   []string // from git clean -dn
	rcPreviewErr     string
	rcRunning        bool
	rcErr            string
	rcDone           bool

	// branch switch wizard
	showBranch    bool
	brStep        int // 0: list, 1: confirm/progress
	brBranches    []string
	brCurrent     string
	brIndex       int
	brRunning     bool
	brErr         string
	brDone        bool
	brInput       textinput.Model
	brInputActive bool

	// pull wizard
	showPull  bool
	plRunning bool
	plErr     string
	plDone    bool
	plOutput  string

	// search state
	searchActive  bool
	searchInput   textinput.Model
	searchQuery   string
	searchMatches []int
	searchIndex   int
}

// messages
type tickMsg struct{}

type clearStatusMsg struct{}

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
	m := model{repoRoot: repoRoot, sideBySide: true, diffMode: "head", theme: loadThemeFromRepo(repoRoot)}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func (m model) Init() tea.Cmd {
	return tea.Batch(loadFiles(m.repoRoot, m.diffMode), loadLastCommit(m.repoRoot), loadCurrentBranch(m.repoRoot), loadPrefs(m.repoRoot), tickOnce())
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
		if m.showUncommit {
			return m.handleUncommitKeys(msg)
		}
		if m.showResetClean {
			return m.handleResetCleanKeys(msg)
		}
		if m.showBranch {
			return m.handleBranchKeys(msg)
		}
		if m.showPull {
			return m.handlePullKeys(msg)
		}

		key := msg.String()

		if isNumericKey(key) {
			m.keyBuffer += key
			return m, nil
		}

		if !isNumericKey(key) && !isMovementKey(key) {
			m.keyBuffer = ""
		}

		switch key {
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
		case "u":
			// Open uncommit wizard
			m.openUncommitWizard()
			return m, tea.Batch(loadUncommitFiles(m.repoRoot), loadUncommitEligible(m.repoRoot), m.recalcViewport())
		case "b":
			m.openBranchWizard()
			return m, tea.Batch(loadBranches(m.repoRoot), m.recalcViewport())
		case "p":
			m.openPullWizard()
			return m, m.recalcViewport()
		case "R":
			// Open reset/clean wizard
			m.openResetCleanWizard()
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
			_ = prefs.SaveLeftWidth(m.repoRoot, m.leftWidth)
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
			_ = prefs.SaveLeftWidth(m.repoRoot, m.leftWidth)
			return m, m.recalcViewport()
		case "j", "down":
			if len(m.files) == 0 {
				return m, nil
			}
			if m.selected < len(m.files)-1 {
				if m.keyBuffer == "" {
					m.selected++
				} else {
					jump, err := strconv.Atoi(m.keyBuffer)
					if err != nil {
						m.selected++
					} else {
						m.selected += jump
						m.selected = min(m.selected, len(m.files)-1)
					}
					m.keyBuffer = ""
				}
				m.rows = nil
				// Reset scroll for new file
				m.rightVP.GotoTop()
				return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path, m.diffMode), m.recalcViewport())
			}
		case "k", "up":
			if len(m.files) == 0 {
				m.keyBuffer = ""
			}
			if m.selected > 0 {
				if m.keyBuffer == "" {
					m.selected--
				} else {
					jump, err := strconv.Atoi(m.keyBuffer)
					if err != nil {
						m.selected--
					} else {
						m.selected -= jump
						m.selected = max(m.selected, 0)
					}
					m.keyBuffer = ""
				}
				m.rows = nil
				m.rightVP.GotoTop()
				return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path, m.diffMode), m.recalcViewport())
			}
		case "g":
			if len(m.files) > 0 {
				m.selected = 0
				m.rows = nil
				m.rightVP.GotoTop()
				return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path, m.diffMode), m.recalcViewport())
			}
		case "G":
			if len(m.files) > 0 {
				m.selected = len(m.files) - 1
				m.rows = nil
				m.rightVP.GotoTop()
				return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path, m.diffMode), m.recalcViewport())
			}
		case "[":
			// Page up left pane
			vis := m.rightVP.Height
			if vis <= 0 {
				vis = 10
			}
			step := vis - 1
			if step < 1 {
				step = 1
			}
			newOffset := m.leftOffset - step
			if newOffset < 0 {
				newOffset = 0
			}
			// Keep selection visible within new viewport
			if m.selected < newOffset {
				newOffset = m.selected
			}
			maxStart := len(m.files) - vis
			if maxStart < 0 {
				maxStart = 0
			}
			if newOffset > maxStart {
				newOffset = maxStart
			}
			m.leftOffset = newOffset
			return m, m.recalcViewport()
		case "]":
			// Page down left pane
			vis := m.rightVP.Height
			if vis <= 0 {
				vis = 10
			}
			step := vis - 1
			if step < 1 {
				step = 1
			}
			maxStart := len(m.files) - vis
			if maxStart < 0 {
				maxStart = 0
			}
			newOffset := m.leftOffset + step
			if newOffset > maxStart {
				newOffset = maxStart
			}
			// Keep selection visible within new viewport
			if m.selected >= newOffset+vis {
				newOffset = m.selected - vis + 1
				if newOffset < 0 {
					newOffset = 0
				}
			}
			m.leftOffset = newOffset
			return m, m.recalcViewport()
		case "n":
			return m, (&m).advanceSearch(1)
		case "N":
			return m, (&m).advanceSearch(-1)
		case "r":
			return m, tea.Batch(loadFiles(m.repoRoot, m.diffMode), loadCurrentDiff(m))
		case "s":
			m.sideBySide = !m.sideBySide
			_ = prefs.SaveSideBySide(m.repoRoot, m.sideBySide)
			return m, m.recalcViewport()
		case "t":
			if m.diffMode == "head" {
				m.diffMode = "staged"
			} else {
				m.diffMode = "head"
			}
			m.rows = nil
			m.selected = 0
			m.rightVP.GotoTop()
			return m, tea.Batch(loadFiles(m.repoRoot, m.diffMode), m.recalcViewport())
		case "w":
			// Toggle wrap in diff pane
			m.wrapLines = !m.wrapLines
			if m.wrapLines {
				m.rightXOffset = 0
			}
			_ = prefs.SaveWrap(m.repoRoot, m.wrapLines)
			return m, m.recalcViewport()
		// Horizontal scroll for right pane
		case "left", "{":
			if m.wrapLines {
				return m, nil
			}
			if m.rightXOffset > 0 {
				m.rightXOffset -= 4
				if m.rightXOffset < 0 {
					m.rightXOffset = 0
				}
				return m, m.recalcViewport()
			}
			return m, nil
		case "right", "}":
			if m.wrapLines {
				return m, nil
			}
			m.rightXOffset += 4
			return m, m.recalcViewport()
		case "home":
			if m.rightXOffset != 0 {
				m.rightXOffset = 0
				return m, m.recalcViewport()
			}
			return m, nil
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
			if m.savedLeftWidth > 0 {
				m.leftWidth = m.savedLeftWidth
			} else {
				m.leftWidth = m.width / 3
			}
			if m.leftWidth < 24 {
				m.leftWidth = 24
			}
			// Also ensure it doesn't exceed available
			maxLeft := m.width - 20
			if maxLeft < 20 {
				maxLeft = 20
			}
			if m.leftWidth > maxLeft {
				m.leftWidth = maxLeft
			}
		}
		return m, m.recalcViewport()
	case clearStatusMsg:
		m.status = ""
		return m, nil
	case tickMsg:
		// Periodic refresh
		return m, tea.Batch(loadFiles(m.repoRoot, m.diffMode), loadCurrentBranch(m.repoRoot), tickOnce())
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
			return m, tea.Batch(loadDiff(m.repoRoot, m.files[m.selected].Path, m.diffMode), m.recalcViewport())
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
	case currentBranchMsg:
		if msg.err == nil {
			m.currentBranch = msg.name
		}
		return m, nil
	case prefsMsg:
		if msg.err == nil {
			if msg.p.SideSet {
				m.sideBySide = msg.p.SideBySide
			}
			if msg.p.WrapSet {
				m.wrapLines = msg.p.Wrap
				if m.wrapLines {
					m.rightXOffset = 0
				}
			}
			if msg.p.LeftSet {
				m.savedLeftWidth = msg.p.LeftWidth
				// If we already know the window size, apply immediately.
				if m.width > 0 {
					lw := m.savedLeftWidth
					if lw < 24 {
						lw = 24
					}
					maxLeft := m.width - 20
					if maxLeft < 20 {
						maxLeft = 20
					}
					if lw > maxLeft {
						lw = maxLeft
					}
					m.leftWidth = lw
					return m, m.recalcViewport()
				}
			}
		}
		return m, nil
	case pullResultMsg:
		m.plRunning = false
		// Always show result output in overlay; close with enter/esc
		m.plOutput = msg.out
		if msg.err != nil {
			m.plErr = msg.err.Error()
		} else {
			m.plErr = ""
		}
		m.plDone = true
		m.showPull = true
		// Refresh repo state after pull
		return m, tea.Batch(loadFiles(m.repoRoot, m.diffMode), loadLastCommit(m.repoRoot), loadCurrentBranch(m.repoRoot), m.recalcViewport())
	case branchListMsg:
		if msg.err != nil {
			m.brErr = msg.err.Error()
			m.brBranches = nil
			m.brCurrent = ""
			m.brIndex = 0
			return m, m.recalcViewport()
		}
		m.brBranches = msg.names
		m.brCurrent = msg.current
		m.brErr = ""
		// Focus current if present
		m.brIndex = 0
		for i, n := range m.brBranches {
			if n == m.brCurrent {
				m.brIndex = i
				break
			}
		}
		return m, m.recalcViewport()
	case branchResultMsg:
		if msg.err != nil {
			m.brErr = msg.err.Error()
			m.status = msg.out
			return m, m.recalcViewport()
		}

		// successful branch change
		statusText := strings.TrimSpace(msg.out)
		if statusText == "" {
			if m.brStep == 3 {
				statusText = fmt.Sprintf("Created and switched to branch '%s'", m.brInput.Value())
			} else {
				statusText = fmt.Sprintf("Switched to branch '%s'", m.brBranches[m.brIndex])
			}
		} else {
			statusText = "" + statusText
		}
		// Add diff summary
		if summary, err := gitx.DiffSummary(m.repoRoot); err == nil && summary != "" {
			statusText += "\n" + summary
		}
		m.status = statusText
		m.showBranch = false
		// Clear status after delay using bubble tea timer
		return m, tea.Batch(
			loadFiles(m.repoRoot, m.diffMode),
			loadLastCommit(m.repoRoot),
			loadCurrentBranch(m.repoRoot),
			m.recalcViewport(),
			tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
				return clearStatusMsg{}
			}),
		)
		m.brIndex = 0
		return m, tea.Batch(loadFiles(m.repoRoot, m.diffMode), loadLastCommit(m.repoRoot), loadCurrentBranch(m.repoRoot), m.recalcViewport())
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
			return m, tea.Batch(loadFiles(m.repoRoot, m.diffMode), m.recalcViewport())
		}
		m.rcErr = ""
		m.rcDone = true
		m.showResetClean = false
		return m, tea.Batch(loadFiles(m.repoRoot, m.diffMode), m.recalcViewport())
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
			return m, tea.Batch(loadFiles(m.repoRoot, m.diffMode), loadLastCommit(m.repoRoot), m.recalcViewport())
		} else {
			m.commitErr = ""
			m.commitDone = true
			m.showCommit = false
			// refresh changes and last commit
			return m, tea.Batch(loadFiles(m.repoRoot, m.diffMode), loadLastCommit(m.repoRoot), m.recalcViewport())
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
			return m, tea.Batch(loadFiles(m.repoRoot, m.diffMode), loadLastCommit(m.repoRoot), m.recalcViewport())
		}
		m.uncommitErr = ""
		m.uncommitDone = true
		m.showUncommit = false
		return m, tea.Batch(loadFiles(m.repoRoot, m.diffMode), loadLastCommit(m.repoRoot), m.recalcViewport())
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

	// Row 1: top bar with right-aligned current branch
	leftTop := "Changes | " + m.topRightTitle()
	rightTop := m.currentBranch
	if rightTop != "" {
		rightTop = lipgloss.NewStyle().Faint(true).Render(rightTop)
	}
	// Compose with right part visible and left truncated if needed
	{
		rightW := lipgloss.Width(rightTop)
		if rightW >= m.width {
			leftTop = ansi.Truncate(rightTop, m.width, "…")
		} else {
			avail := m.width - rightW - 1
			if lipgloss.Width(leftTop) > avail {
				leftTop = ansi.Truncate(leftTop, avail, "…")
			} else if lipgloss.Width(leftTop) < avail {
				leftTop = leftTop + strings.Repeat(" ", avail-lipgloss.Width(leftTop))
			}
			leftTop = leftTop + " " + rightTop
		}
	}
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
	if m.showBranch {
		overlay = append(overlay, m.branchOverlayLines(m.width)...)
	}
	if m.showPull {
		overlay = append(overlay, m.pullOverlayLines(m.width)...)
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
	b.WriteString(leftTop)
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
	if start < 0 {
		start = 0
	}
	if start > len(m.files) {
		start = len(m.files)
	}
	end := start + max
	if end > len(m.files) {
		end = len(m.files)
	}
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
				if len(lines) >= max {
					break
				}
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
		return fmt.Sprintf("[%s]", strings.ToUpper(m.diffMode))
	}
	header := fmt.Sprintf("%s (%s) [%s]", m.files[m.selected].Path, fileStatusLabel(m.files[m.selected]), strings.ToUpper(m.diffMode))
	return header
}

func (m model) bottomBar() string {
	var leftRendered string
	if m.status != "" {
		// If we have a status message, show it prominently
		leftRendered = lipgloss.NewStyle().Bold(true).Render(m.status)
	} else {
		baseText := "h: help"
		if m.keyBuffer != "" {
			baseText = m.keyBuffer
		}
		if m.lastCommit != "" {
			baseText += "  |  last: " + m.lastCommit
		}
		leftRendered = lipgloss.NewStyle().Faint(true).Render(baseText)
	}
	right := lipgloss.NewStyle().Faint(true).Render("refreshed: " + m.lastRefresh.Format("15:04:05"))
	w := m.width
	// Ensure the right part is always visible; truncate left if needed
	rightW := lipgloss.Width(right)
	if rightW >= w {
		// Degenerate case: screen too small; just show right truncated
		return ansi.Truncate(right, w, "…")
	}
	avail := w - rightW - 1 // 1 space gap
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

func loadFiles(repoRoot, diffMode string) tea.Cmd {
	return func() tea.Msg {
		allFiles, err := gitx.ChangedFiles(repoRoot)
		if err != nil {
			return filesMsg{files: nil, err: err}
		}

		// Filter files based on diff mode
		var filteredFiles []gitx.FileChange
		for _, file := range allFiles {
			if diffMode == "staged" {
				if file.Staged {
					filteredFiles = append(filteredFiles, file)
				}
			} else {
				if file.Unstaged || file.Untracked {
					filteredFiles = append(filteredFiles, file)
				}
			}
		}

		return filesMsg{files: filteredFiles, err: nil}
	}
}

func loadDiff(repoRoot, path, diffMode string) tea.Cmd {
	return func() tea.Msg {
		var d string
		var err error
		if diffMode == "staged" {
			d, err = gitx.DiffStaged(repoRoot, path)
		} else {
			d, err = gitx.DiffHEAD(repoRoot, path)
		}
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
	return loadDiff(m.repoRoot, m.files[m.selected].Path, m.diffMode)
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
		"{/}            Horizontal scroll (diff)",
		"b              Switch branch (open wizard)",
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
		if pad < 4 {
			pad = 4
		}
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
	if m.showBranch {
		overlayH += len(m.branchOverlayLines(m.width))
	}
	if m.showPull {
		overlayH += len(m.pullOverlayLines(m.width))
	}
	if m.searchActive {
		overlayH += len(m.searchOverlayLines(m.width))
	}
	contentHeight := m.height - 4 - overlayH
	if contentHeight < 1 {
		contentHeight = 1
	}
	// Clamp leftOffset and keep selection visible in left pane
	vis := contentHeight
	if vis < 1 {
		vis = 1
	}
	if m.leftOffset < 0 {
		m.leftOffset = 0
	}
	maxStart := len(m.files) - vis
	if maxStart < 0 {
		maxStart = 0
	}
	if m.leftOffset > maxStart {
		m.leftOffset = maxStart
	}
	if len(m.files) > 0 {
		if m.selected < m.leftOffset {
			m.leftOffset = m.selected
		} else if m.selected >= m.leftOffset+vis {
			m.leftOffset = m.selected - vis + 1
			if m.leftOffset < 0 {
				m.leftOffset = 0
			}
		}
		if m.leftOffset > maxStart {
			m.leftOffset = maxStart
		}
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
		if m.cwInputActive {
			mode = "input"
		}
		title := lipgloss.NewStyle().Bold(true).Render("Commit — Message (i: input, enter: continue, b: back, esc: " + map[bool]string{true: "leave input", false: "cancel"}[m.cwInputActive] + ") [" + mode + "]")
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

type rcPreviewMsg struct {
	lines []string
	err   error
}

// --- Branch switch wizard ---

type branchListMsg struct {
	names   []string
	current string
	err     error
}

// --- Pull wizard ---

type pullResultMsg struct {
	out string
	err error
}

func (m *model) openPullWizard() {
	m.showPull = true
	m.plRunning = false
	m.plErr = ""
	m.plDone = false
	m.plOutput = ""
}

func (m model) pullOverlayLines(width int) []string {
	if !m.showPull {
		return nil
	}
	lines := make([]string, 0, 32)
	lines = append(lines, strings.Repeat("─", width))
	if m.plDone {
		title := lipgloss.NewStyle().Bold(true).Render("Pull — Result (enter/esc: close)")
		lines = append(lines, title)
		if m.plErr != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: ")+m.plErr)
		}
		if m.plOutput != "" {
			// Show up to 12 lines of output
			outLines := strings.Split(strings.TrimRight(m.plOutput, "\n"), "\n")
			max := 12
			for i, l := range outLines {
				if i >= max {
					break
				}
				lines = append(lines, l)
			}
			if len(outLines) > max {
				lines = append(lines, fmt.Sprintf("… and %d more", len(outLines)-max))
			}
		} else if m.plErr == "" {
			lines = append(lines, lipgloss.NewStyle().Faint(true).Render("(no output)"))
		}
	} else {
		title := lipgloss.NewStyle().Bold(true).Render("Pull — Confirm (y/enter: pull, esc: cancel)")
		lines = append(lines, title)
		if m.plRunning {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render("Pulling…"))
		}
		if m.plErr != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: ")+m.plErr)
		}
	}
	return lines
}

func (m model) handlePullKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		if m.plDone && !m.plRunning {
			m.showPull = false
			m.plDone = false
			m.plOutput = ""
			m.plErr = ""
			return m, m.recalcViewport()
		}
		if !m.plRunning {
			m.showPull = false
			return m, m.recalcViewport()
		}
		return m, nil
	case "y", "enter":
		if m.plDone && !m.plRunning {
			// Close results view
			m.showPull = false
			m.plDone = false
			m.plOutput = ""
			m.plErr = ""
			return m, m.recalcViewport()
		}
		if !m.plRunning && !m.plDone {
			m.plRunning = true
			m.plErr = ""
			return m, runPull(m.repoRoot)
		}
		return m, nil
	}
	return m, nil
}

func runPull(repoRoot string) tea.Cmd {
	return func() tea.Msg {
		out, err := gitx.PullWithOutput(repoRoot)
		if err != nil {
			return pullResultMsg{out: out, err: err}
		}
		return pullResultMsg{out: out, err: nil}
	}
}

type branchResultMsg struct {
	err error
	out string
}

func loadBranches(repoRoot string) tea.Cmd {
	return func() tea.Msg {
		names, current, err := gitx.ListBranches(repoRoot)
		return branchListMsg{names: names, current: current, err: err}
	}
}

func (m *model) openBranchWizard() {
	m.showBranch = true
	m.brStep = 0
	m.brBranches = nil
	m.brCurrent = ""
	m.brIndex = 0
	m.brRunning = false
	m.brErr = ""
	m.brDone = false
	m.brInput = textinput.Model{}
	m.brInputActive = false
}

func (m model) branchOverlayLines(width int) []string {
	if !m.showBranch {
		return nil
	}
	lines := make([]string, 0, 128)
	lines = append(lines, strings.Repeat("─", width))
	switch m.brStep {
	case 0:
		title := lipgloss.NewStyle().Bold(true).Render("Branches — Select (enter: continue, n: new, esc: cancel)")
		lines = append(lines, title)
		if m.brErr != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: ")+m.brErr)
		}
		if len(m.brBranches) == 0 && m.brErr == "" {
			lines = append(lines, lipgloss.NewStyle().Faint(true).Render("Loading branches…"))
			return lines
		}
		for i, n := range m.brBranches {
			cur := "  "
			if i == m.brIndex {
				cur = "> "
			}
			mark := "   "
			if n == m.brCurrent {
				mark = "[*]"
			}
			lines = append(lines, fmt.Sprintf("%s%s %s", cur, mark, n))
		}
		lines = append(lines, lipgloss.NewStyle().Faint(true).Render("[*] current branch"))
	case 1:
		title := lipgloss.NewStyle().Bold(true).Render("Checkout — Confirm (y/enter: checkout, b: back, esc: cancel)")
		lines = append(lines, title)
		if len(m.brBranches) > 0 {
			lines = append(lines, fmt.Sprintf("Branch: %s", m.brBranches[m.brIndex]))
		}
		if m.brRunning {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render("Checking out…"))
		}
		if m.brErr != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: ")+m.brErr)
		}
	case 2:
		// New branch: name input
		mode := "action"
		if m.brInputActive {
			mode = "input"
		}
		title := lipgloss.NewStyle().Bold(true).Render("New Branch — Name (i: input, enter: continue, b: back, esc: " + map[bool]string{true: "leave input", false: "cancel"}[m.brInputActive] + ") [" + mode + "]")
		lines = append(lines, title)
		lines = append(lines, m.brInput.View())
		if m.brErr != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: ")+m.brErr)
		}
	case 3:
		// New branch: confirm
		title := lipgloss.NewStyle().Bold(true).Render("New Branch — Confirm (y/enter: create, b: back, esc: cancel)")
		lines = append(lines, title)
		lines = append(lines, "Name: "+m.brInput.Value())
		if m.brRunning {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render("Creating…"))
		}
		if m.brErr != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: ")+m.brErr)
		}
	}
	return lines
}

func (m model) handleBranchKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.brStep {
	case 0:
		switch key.String() {
		case "esc":
			m.showBranch = false
			return m, m.recalcViewport()
		case "j", "down":
			if len(m.brBranches) > 0 && m.brIndex < len(m.brBranches)-1 {
				m.brIndex++
			}
			return m, nil
		case "k", "up":
			if m.brIndex > 0 {
				m.brIndex--
			}
			return m, nil
		case "n":
			// New branch flow
			ti := textinput.New()
			ti.Placeholder = "Branch name"
			ti.Prompt = "> "
			// Start in action mode; 'i' toggles input focus
			m.brInput = ti
			m.brInputActive = false
			m.brStep = 2
			m.brErr = ""
			m.brDone = false
			m.brRunning = false
			return m, m.recalcViewport()
		case "enter":
			if len(m.brBranches) == 0 {
				return m, nil
			}
			m.brStep = 1
			m.brErr = ""
			m.brDone = false
			m.brRunning = false
			return m, m.recalcViewport()
		}
	case 1:
		switch key.String() {
		case "esc":
			if !m.brRunning {
				m.showBranch = false
				return m, m.recalcViewport()
			}
			return m, nil
		case "b":
			if !m.brRunning && !m.brDone {
				m.brStep = 0
				return m, m.recalcViewport()
			}
			return m, nil
		case "y", "enter":
			if !m.brRunning && !m.brDone {
				if len(m.brBranches) == 0 {
					return m, nil
				}
				// name := m.brBranches[m.brIndex]
				m.brRunning = true
				m.brErr = ""
				return m, m.checkoutBranch()
			}
			return m, nil
		}
	case 2: // new branch name
		switch key.String() {
		case "esc":
			if m.brInputActive {
				m.brInputActive = false
				return m, m.recalcViewport()
			}
			m.showBranch = false
			return m, m.recalcViewport()
		case "i":
			if !m.brInputActive {
				m.brInputActive = true
				m.brInput.Focus()
				return m, m.recalcViewport()
			}
			// already active, treat as input
		case "b":
			if !m.brInputActive {
				m.brStep = 0
				return m, m.recalcViewport()
			}
			// else forward to input
		case "enter":
			if !m.brInputActive {
				if strings.TrimSpace(m.brInput.Value()) == "" {
					m.brErr = "empty branch name"
					return m, nil
				}
				m.brStep = 3
				m.brErr = ""
				m.brDone = false
				m.brRunning = false
				return m, m.recalcViewport()
			}
			// in input mode, forward to text input
		}
		if m.brInputActive {
			var cmd tea.Cmd
			m.brInput, cmd = m.brInput.Update(key)
			return m, cmd
		}
		return m, nil
	case 3: // confirm new branch
		switch key.String() {
		case "esc":
			if !m.brRunning {
				m.showBranch = false
				return m, m.recalcViewport()
			}
			return m, nil
		case "b":
			if !m.brRunning && !m.brDone {
				m.brStep = 2
				return m, m.recalcViewport()
			}
			return m, nil
		case "y", "enter":
			if !m.brRunning && !m.brDone {
				name := strings.TrimSpace(m.brInput.Value())
				if name == "" {
					m.brErr = "empty branch name"
					return m, nil
				}
				m.brRunning = true
				m.brErr = ""
				return m, m.checkoutNewBranch()
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *model) checkoutBranch() tea.Cmd {
	return func() tea.Msg {
		out, err := gitx.CheckoutBranch(m.repoRoot, m.brBranches[m.brIndex])
		if err != nil {
			return branchResultMsg{err: err, out: out}
		}
		return branchResultMsg{err: nil, out: out}
	}
}

func (m *model) checkoutNewBranch() tea.Cmd {
	return func() tea.Msg {
		out, err := gitx.CheckoutNewBranch(m.repoRoot, m.brInput.Value())
		if err != nil {
			return branchResultMsg{err: err, out: out}
		}
		return branchResultMsg{err: nil, out: out}
	}
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
	if !m.showResetClean {
		return nil
	}
	lines := make([]string, 0, 128)
	lines = append(lines, strings.Repeat("─", width))
	switch m.rcStep {
	case 0: // select actions/options
		title := lipgloss.NewStyle().Bold(true).Render("Reset/Clean — Select actions (space: toggle, a: toggle both, enter: continue, esc: cancel)")
		lines = append(lines, title)
		items := []struct {
			label string
			on    bool
		}{
			{"Reset working tree (git reset --hard)", m.rcDoReset},
			{"Clean untracked (git clean -d -f)", m.rcDoClean},
			{"Include ignored in clean (-x)", m.rcIncludeIgnored},
		}
		for i, it := range items {
			cur := "  "
			if i == m.rcIndex {
				cur = "> "
			}
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
					if i >= max {
						break
					}
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
		if m.rcDoReset {
			cmds = append(cmds, "git reset --hard")
		}
		if m.rcDoClean {
			c := "git clean -d -f"
			if m.rcIncludeIgnored {
				c += " -x"
			}
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

func checkbox(on bool) string {
	if on {
		return "[x]"
	}
	return "[ ]"
}

func (m model) handleResetCleanKeys(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.rcStep {
	case 0: // select
		switch key.String() {
		case "esc":
			m.showResetClean = false
			return m, m.recalcViewport()
		case "j", "down":
			if m.rcIndex < 2 {
				m.rcIndex++
			}
			return m, nil
		case "k", "up":
			if m.rcIndex > 0 {
				m.rcIndex--
			}
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
			if i == m.ucIndex {
				cur = "> "
			}
			mark := "[ ]"
			if m.ucSelected[f.Path] {
				mark = "[x]"
			}
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
			for _, p := range sel {
				if m.ucEligible[p] {
					elig++
				}
			}
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
			if len(m.ucFiles) > 0 && m.ucIndex < len(m.ucFiles)-1 {
				m.ucIndex++
			}
			return m, nil
		case "k", "up":
			if m.ucIndex > 0 {
				m.ucIndex--
			}
			return m, nil
		case " ":
			if len(m.ucFiles) > 0 {
				p := m.ucFiles[m.ucIndex].Path
				m.ucSelected[p] = !m.ucSelected[p]
			}
			return m, nil
		case "a":
			all := true
			for _, f := range m.ucFiles {
				if !m.ucSelected[f.Path] {
					all = false
					break
				}
			}
			set := !all
			for _, f := range m.ucFiles {
				m.ucSelected[f.Path] = set
			}
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
		if m.ucSelected[f.Path] {
			out = append(out, f.Path)
		}
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
		for _, p := range eligible {
			eligSet[p] = true
		}
		var toUncommit []string
		for _, p := range paths {
			if eligSet[p] {
				toUncommit = append(toUncommit, p)
			}
		}
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
				// subtle separator fills full width
				lines = append(lines, lipgloss.NewStyle().Faint(true).Render(strings.Repeat("·", width)))
			case diffview.RowMeta:
				// skip
			default:
				if m.wrapLines {
					lLines := m.renderSideCellWrap(r, "left", colsW)
					rLines := m.renderSideCellWrap(r, "right", colsW)
					n := len(lLines)
					if len(rLines) > n {
						n = len(rLines)
					}
					for i := 0; i < n; i++ {
						var l, rr string
						if i < len(lLines) {
							l = lLines[i]
						} else {
							l = strings.Repeat(" ", colsW)
						}
						if i < len(rLines) {
							rr = rLines[i]
						} else {
							rr = strings.Repeat(" ", colsW)
						}
						lines = append(lines, l+mid+rr)
					}
				} else {
					l := m.renderSideCell(r, "left", colsW)
					rr := m.renderSideCell(r, "right", colsW)
					l = padExact(l, colsW)
					rr = padExact(rr, colsW)
					lines = append(lines, l+mid+rr)
				}
			}
		}
	} else {
		for _, r := range m.rows {
			switch r.Kind {
			case diffview.RowHunk:
				lines = append(lines, lipgloss.NewStyle().Faint(true).Render(strings.Repeat("·", width)))
			case diffview.RowContext:
				base := "  " + r.Left
				if m.wrapLines {
					wrapped := ansi.Hardwrap(base, width, false)
					lines = append(lines, strings.Split(wrapped, "\n")...)
				} else {
					line := base
					if m.rightXOffset > 0 {
						line = sliceANSI(line, m.rightXOffset, width)
						line = padExact(line, width)
					}
					lines = append(lines, line)
				}
			case diffview.RowAdd:
				base := m.theme.AddText("+ " + r.Right)
				if m.wrapLines {
					wrapped := ansi.Hardwrap(base, width, false)
					lines = append(lines, strings.Split(wrapped, "\n")...)
				} else {
					line := base
					if m.rightXOffset > 0 {
						line = sliceANSI(line, m.rightXOffset, width)
						line = padExact(line, width)
					}
					lines = append(lines, line)
				}
			case diffview.RowDel:
				base := m.theme.DelText("- " + r.Left)
				if m.wrapLines {
					wrapped := ansi.Hardwrap(base, width, false)
					lines = append(lines, strings.Split(wrapped, "\n")...)
				} else {
					line := base
					if m.rightXOffset > 0 {
						line = sliceANSI(line, m.rightXOffset, width)
						line = padExact(line, width)
					}
					lines = append(lines, line)
				}
			case diffview.RowReplace:
				base1 := m.theme.DelText("- " + r.Left)
				base2 := m.theme.AddText("+ " + r.Right)
				if m.wrapLines {
					wrapped1 := strings.Split(ansi.Hardwrap(base1, width, false), "\n")
					wrapped2 := strings.Split(ansi.Hardwrap(base2, width, false), "\n")
					lines = append(lines, wrapped1...)
					lines = append(lines, wrapped2...)
				} else {
					line1 := base1
					if m.rightXOffset > 0 {
						line1 = sliceANSI(line1, m.rightXOffset, width)
						line1 = padExact(line1, width)
					}
					lines = append(lines, line1)
					line2 := base2
					if m.rightXOffset > 0 {
						line2 = sliceANSI(line2, m.rightXOffset, width)
						line2 = padExact(line2, width)
					}
					lines = append(lines, line2)
				}
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

type lastCommitMsg struct {
	summary string
	err     error
}

func loadLastCommit(repoRoot string) tea.Cmd {
	return func() tea.Msg {
		s, err := gitx.LastCommitSummary(repoRoot)
		return lastCommitMsg{summary: s, err: err}
	}
}

type currentBranchMsg struct {
	name string
	err  error
}

func loadCurrentBranch(repoRoot string) tea.Cmd {
	return func() tea.Msg {
		name, err := gitx.CurrentBranch(repoRoot)
		return currentBranchMsg{name: name, err: err}
	}
}

type prefsMsg struct {
	p   prefs.Prefs
	err error
}

func loadPrefs(repoRoot string) tea.Cmd {
	return func() tea.Msg {
		// Loading never errors for now; returns zero-vals on missing keys
		p := prefs.Load(repoRoot)
		return prefsMsg{p: p, err: nil}
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
				if !m.cwSelected[f.Path] {
					all = false
					break
				}
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

	clipped := sliceANSI(content, m.rightXOffset, bodyW)

	return marker + " " + clipped
}

// renderSideCellWrap renders a cell like renderSideCell but wraps the content
// to the given width and returns multiple visual lines. The marker is repeated
// on each wrapped line.
func (m model) renderSideCellWrap(r diffview.Row, side string, width int) []string {
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
	// Reserve 2 cols for marker and a space
	if width <= 2 {
		return []string{ansi.Truncate(marker+" ", width, "")}
	}
	bodyW := width - 2
	wrapped := ansi.Hardwrap(content, bodyW, false)
	parts := strings.Split(wrapped, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, marker+" "+padExact(p, bodyW))
	}
	if len(out) == 0 {
		out = append(out, marker+" "+strings.Repeat(" ", bodyW))
	}
	return out
}

// sliceANSI returns a substring of s starting at visual column `start` with at most `w` columns, preserving ANSI escapes.
func sliceANSI(s string, start, w int) string {
	if start <= 0 {
		return ansi.Truncate(s, w, "")
	}
	// First keep only the left portion up to start+w, then drop the first `start` columns.
	head := ansi.Truncate(s, start+w, "")
	return ansi.TruncateLeft(head, start, "")
}

// padExact pads s with spaces to exactly width w (ANSI-aware width).
func padExact(s string, w int) string {
	sw := lipgloss.Width(s)
	if sw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-sw)
}

func isMovementKey(key string) bool {
	return key == "j" || key == "k"
}

func isNumericKey(key string) bool {
	return key <= "9" && key >= "0"
}
