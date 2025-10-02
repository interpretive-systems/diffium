package tui

import (
    _ "fmt"
    "strings"
    
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/x/ansi"
	"github.com/interpretive-systems/diffium/internal/theme"
)

// Layout manages screen layout calculations.
type Layout struct {
    width      int
    height     int
    leftWidth  int
}

// NewLayout creates a new layout manager.
func NewLayout() *Layout {
    return &Layout{}
}

// SetSize updates the layout dimensions.
func (l *Layout) SetSize(width, height int) {
    l.width = width
    l.height = height
}

// SetLeftWidth sets the left pane width.
func (l *Layout) SetLeftWidth(width int) {
    l.leftWidth = width
}

// Width returns the total width.
func (l *Layout) Width() int {
    return l.width
}

// Height returns the total height.
func (l *Layout) Height() int {
    return l.height
}

// LeftWidth returns the left pane width.
func (l *Layout) LeftWidth() int {
    if l.leftWidth < 20 {
        return 20
    }
    return l.leftWidth
}

// RightWidth returns the right pane width.
func (l *Layout) RightWidth() int {
    leftW := l.LeftWidth()
    rightW := l.width - leftW - 1 // 1 for divider
    if rightW < 1 {
        rightW = 1
    }
    return rightW
}

// ContentHeight returns the height available for content.
func (l *Layout) ContentHeight(overlayHeight int) int {
    // top bar + top rule + bottom rule + bottom bar + overlays
    h := l.height - 4 - overlayHeight
    if h < 1 {
        h = 1
    }
    return h
}

// AdjustLeftWidth adjusts the left width by delta.
func (l *Layout) AdjustLeftWidth(delta int) {
    newWidth := l.leftWidth + delta
    if newWidth < 20 {
        newWidth = 20
    }
    maxLeft := l.width - 20
    if maxLeft < 20 {
        maxLeft = 20
    }
    if newWidth > maxLeft {
        newWidth = maxLeft
    }
    l.leftWidth = newWidth
}

// RenderFrame renders the main frame with top bar, rules, and columns.
func (l *Layout) RenderFrame(
    topLeft, topRight string,
    leftLines, rightLines []string,
    overlayLines []string,
    bottomBar string,
    theme theme.Theme,
) string {
    var b strings.Builder
    
    // Row 1: Top bar
    topBar := l.renderTopBar(topLeft, topRight)
    b.WriteString(topBar)
    b.WriteByte('\n')
    
    // Row 2: Horizontal rule
    hr := theme.DividerText(strings.Repeat("─", l.width))
    b.WriteString(hr)
    b.WriteByte('\n')
    
    // Row 3: Content columns
    leftW := l.LeftWidth()
    rightW := l.RightWidth()
    sep := theme.DividerText("│")
    
    contentHeight := len(leftLines)
    if len(rightLines) > contentHeight {
        contentHeight = len(rightLines)
    }
    
    for i := 0; i < contentHeight; i++ {
        var left, right string
        if i < len(leftLines) {
            left = padToWidth(leftLines[i], leftW)
        } else {
            left = strings.Repeat(" ", leftW)
        }
        if i < len(rightLines) {
            right = rightLines[i]
        } else {
            right = ""
        }
        b.WriteString(left)
        b.WriteString(sep)
        b.WriteString(padToWidth(right, rightW))
        if i < contentHeight-1 {
            b.WriteByte('\n')
        }
    }
    
    // Optional overlay
    if len(overlayLines) > 0 {
        b.WriteByte('\n')
        for i, line := range overlayLines {
            b.WriteString(padToWidth(line, l.width))
            if i < len(overlayLines)-1 {
                b.WriteByte('\n')
            }
        }
    }
    
    // Bottom rule and bar
    b.WriteByte('\n')
    b.WriteString(strings.Repeat("─", l.width))
    b.WriteByte('\n')
    b.WriteString(bottomBar)
    
    return b.String()
}

func (l *Layout) renderTopBar(left, right string) string {
    rightW := lipgloss.Width(right)
    if rightW >= l.width {
        return ansi.Truncate(right, l.width, "…")
    }
    
    avail := l.width - rightW - 1
    if lipgloss.Width(left) > avail {
        left = ansi.Truncate(left, avail, "…")
    } else if lipgloss.Width(left) < avail {
        left = left + strings.Repeat(" ", avail-lipgloss.Width(left))
    }
    
    return left + " " + right
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
