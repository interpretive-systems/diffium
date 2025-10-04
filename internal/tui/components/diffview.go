package components

import (
    "strings"
    
    "github.com/charmbracelet/bubbles/viewport"
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/x/ansi"
    "github.com/interpretive-systems/diffium/internal/diffview"
    "github.com/interpretive-systems/diffium/internal/theme"
    tuiansi "github.com/interpretive-systems/diffium/internal/tui/ansi"
)

// DiffView manages the right pane diff viewer.
type DiffView struct {
    rows       []diffview.Row
    viewport   viewport.Model
    xOffset    int
    sideBySide bool
    wrapLines  bool
    curTheme   theme.Theme
    content    []string // Cached rendered content
}

// NewDiffView creates a new diff viewer.
func NewDiffView(defaultTheme theme.Theme) *DiffView {
    return &DiffView{
        curTheme:      defaultTheme,
        sideBySide: true,
    }
}

// SetRows updates the diff rows.
func (d *DiffView) SetRows(rows []diffview.Row) {
    d.rows = rows
}

// SetSize updates the viewport dimensions.
func (d *DiffView) SetSize(width, height int) {
    d.viewport.Width = width
    d.viewport.Height = height
}

func (d *DiffView) GetSideBySide() bool{
	return d.sideBySide
}

// SetSideBySide sets the display mode.
func (d *DiffView) SetSideBySide(sideBySide bool) {
    d.sideBySide = sideBySide
}

func (d *DiffView) GetWrap() bool{
	return d.wrapLines
}

// SetWrap sets line wrapping.
func (d *DiffView) SetWrap(wrap bool) {
    d.wrapLines = wrap
    if wrap {
        d.xOffset = 0
    }
}

// SetXOffset sets horizontal scroll offset.
func (d *DiffView) SetXOffset(offset int) {
    if offset < 0 {
        offset = 0
    }
    d.xOffset = offset
}

// XOffset returns the current horizontal offset.
func (d *DiffView) XOffset() int {
    return d.xOffset
}

// ScrollLeft scrolls left by delta.
func (d *DiffView) ScrollLeft(delta int) {
    if d.wrapLines {
        return
    }
    d.xOffset -= delta
    if d.xOffset < 0 {
        d.xOffset = 0
    }
}

// ScrollRight scrolls right by delta.
func (d *DiffView) ScrollRight(delta int) {
    if d.wrapLines {
        return
    }
    d.xOffset += delta
}

// ScrollHome resets horizontal scroll.
func (d *DiffView) ScrollHome() {
    d.xOffset = 0
}

// RenderContent generates the full content and caches it.
func (d *DiffView) RenderContent(width int, binary bool) []string {
    if binary {
        d.content = []string{lipgloss.NewStyle().Faint(true).Render("(Binary file; no text diff)")}
        return d.content
    }
    
    if d.rows == nil {
        d.content = []string{"Loading diff…"}
        return d.content
    }
    
    if d.sideBySide {
        d.content = d.renderSideBySide(width)
    } else {
        d.content = d.renderInline(width)
    }
    
    return d.content
}

// Content returns the cached content.
func (d *DiffView) Content() []string {
    return d.content
}

// SetContent updates the viewport content from rendered lines.
func (d *DiffView) SetContent(lines []string) {
    d.content = lines
    d.viewport.SetContent(strings.Join(lines, "\n"))
}

// View returns the viewport view.
func (d *DiffView) View() string {
    return d.viewport.View()
}

// Viewport returns the underlying viewport for direct manipulation.
func (d *DiffView) Viewport() *viewport.Model {
    return &d.viewport
}

func (d *DiffView) renderSideBySide(width int) []string {
    lines := make([]string, 0, len(d.rows))
    colsW := (width - 1) / 2
    if colsW < 10 {
        colsW = 10
    }
    mid := d.curTheme.DividerText("│")
    
    for _, r := range d.rows {
        switch r.Kind {
        case diffview.RowHunk:
            lines = append(lines, lipgloss.NewStyle().Faint(true).
                Render(strings.Repeat("·", width)))
        case diffview.RowMeta:
            // skip
        default:
            if d.wrapLines {
                lLines := d.renderSideCellWrap(r, "left", colsW)
                rLines := d.renderSideCellWrap(r, "right", colsW)
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
				l := d.renderSideCell(r, "left", colsW)
				rr := d.renderSideCell(r, "right", colsW)
				l = tuiansi.PadExact(l, colsW)
				rr = tuiansi.PadExact(rr, colsW)
				lines = append(lines, l+mid+rr)
            }
        }
    }
    
    return lines
}

func (d *DiffView) renderInline(width int) []string {
    lines := make([]string, 0, len(d.rows))
    
    for _, r := range d.rows {
        switch r.Kind {
        case diffview.RowHunk:
            lines = append(lines, lipgloss.NewStyle().Faint(true).
                Render(strings.Repeat("·", width)))
        case diffview.RowContext:
            base := "  " + r.Left
            if d.wrapLines {
                wrapped := tuiansi.WrapLine(base, width)
                lines = append(lines, wrapped...)
            } else {
                line := base
                if d.xOffset > 0 {
                    line = tuiansi.SliceHorizontal(line, d.xOffset, width)
                    line = tuiansi.PadExact(line, width)
                }
                lines = append(lines, line)
            }
        case diffview.RowAdd:
            base := d.curTheme.AddText("+ " + r.Right)
            if d.wrapLines {
                wrapped := tuiansi.WrapLine(base, width)
                lines = append(lines, wrapped...)
            } else {
                line := base
                if d.xOffset > 0 {
                    line = tuiansi.SliceHorizontal(line, d.xOffset, width)
                    line = tuiansi.PadExact(line, width)
                }
                lines = append(lines, line)
            }
        case diffview.RowDel:
            base := d.curTheme.DelText("- " + r.Left)
            if d.wrapLines {
                wrapped := tuiansi.WrapLine(base, width)
                lines = append(lines, wrapped...)
            } else {
                line := base
                if d.xOffset > 0 {
                    line = tuiansi.SliceHorizontal(line, d.xOffset, width)
                    line = tuiansi.PadExact(line, width)
                }
                lines = append(lines, line)
            }
        case diffview.RowReplace:
            base1 := d.curTheme.DelText("- " + r.Left)
            base2 := d.curTheme.AddText("+ " + r.Right)
            if d.wrapLines {
                wrapped1 := tuiansi.WrapLine(base1, width)
                wrapped2 := tuiansi.WrapLine(base2, width)
                lines = append(lines, wrapped1...)
                lines = append(lines, wrapped2...)
            } else {
                line1 := base1
                if d.xOffset > 0 {
                    line1 = tuiansi.SliceHorizontal(line1, d.xOffset, width)
                    line1 = tuiansi.PadExact(line1, width)
                }
                lines = append(lines, line1)
                line2 := base2
                if d.xOffset > 0 {
                    line2 = tuiansi.SliceHorizontal(line2, d.xOffset, width)
                    line2 = tuiansi.PadExact(line2, width)
                }
                lines = append(lines, line2)
            }
        }
    }
    
    return lines
}

func (d *DiffView) renderSideCell(r diffview.Row, side string, width int) string {
    marker := " "
    content := ""
    
    switch side {
    case "left":
        content = r.Left
        switch r.Kind {
        case diffview.RowContext:
            marker = " "
        case diffview.RowDel, diffview.RowReplace:
            marker = d.curTheme.DelText("-")
            content = d.curTheme.DelText(content)
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
            marker = d.curTheme.AddText("+")
            content = d.curTheme.AddText(content)
        case diffview.RowDel:
            marker = " "
            content = ""
        }
    }
    
    if width <= 2 {
        return ansi.Truncate(marker+" ", width, "")
    }
    
    bodyW := width - 2

	clipped := tuiansi.SliceHorizontal(content, d.xOffset, bodyW)
    return marker + " " + clipped
}

func (d *DiffView) renderSideCellWrap(r diffview.Row, side string, width int) []string {
    marker := " "
    content := ""
    
    switch side {
    case "left":
        content = r.Left
        switch r.Kind {
        case diffview.RowContext:
            marker = " "
        case diffview.RowDel, diffview.RowReplace:
            marker = d.curTheme.DelText("-")
            content = d.curTheme.DelText(content)
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
            marker = d.curTheme.AddText("+")
            content = d.curTheme.AddText(content)
        case diffview.RowDel:
            marker = " "
            content = ""
        }
    }
    
    if width <= 2 {
        return []string{ansi.Truncate(marker+" ", width, "")}
    }
    
    bodyW := width - 2
    wrapped := tuiansi.WrapLine(content, bodyW)
    out := make([]string, 0, len(wrapped))
    for _, p := range wrapped {
        out = append(out, marker+" "+tuiansi.PadExact(p, bodyW))
    }
    if len(out) == 0 {
        out = append(out, marker+" "+strings.Repeat(" ", bodyW))
    }
    return out
}
