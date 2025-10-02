package components

import (
    _ "fmt"
    "strings"
    "time"
    
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/x/ansi"
)

// StatusBar manages the bottom status bar.
type StatusBar struct {
    lastRefresh time.Time
    lastCommit  string
    keyBuffer   string
}

// NewStatusBar creates a new status bar.
func NewStatusBar() *StatusBar {
    return &StatusBar{}
}

// SetLastRefresh updates the refresh timestamp.
func (s *StatusBar) SetLastRefresh(t time.Time) {
    s.lastRefresh = t
}

// SetLastCommit updates the last commit message.
func (s *StatusBar) SetLastCommit(msg string) {
    s.lastCommit = msg
}

// SetKeyBuffer updates the key buffer display.
func (s *StatusBar) SetKeyBuffer(buf string) {
    s.keyBuffer = buf
}

// Render renders the status bar.
func (s *StatusBar) Render(width int) string {
    leftText := "h: help"
    if s.keyBuffer != "" {
        leftText = s.keyBuffer
    }
    if s.lastCommit != "" {
        leftText += "  |  last: " + s.lastCommit
    }
    
    leftStyled := lipgloss.NewStyle().Faint(true).Render(leftText)
    right := lipgloss.NewStyle().Faint(true).
        Render("refreshed: " + s.lastRefresh.Format("15:04:05"))
    
    // Ensure right part is always visible
    rightW := lipgloss.Width(right)
    if rightW >= width {
        return ansi.Truncate(right, width, "…")
    }
    
    avail := width - rightW - 1
    leftRendered := leftStyled
    if lipgloss.Width(leftRendered) > avail {
        leftRendered = ansi.Truncate(leftRendered, avail, "…")
    } else if lipgloss.Width(leftRendered) < avail {
        leftRendered = leftRendered + strings.Repeat(" ", avail-lipgloss.Width(leftRendered))
    }
    
    return leftRendered + " " + right
}
