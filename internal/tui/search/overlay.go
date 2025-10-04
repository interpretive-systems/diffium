package search

import (
    "fmt"
    "strings"
    
    "github.com/charmbracelet/lipgloss"
    "github.com/interpretive-systems/diffium/internal/tui/ansi"
)

// RenderOverlay renders the search overlay UI.
func (e *Engine) RenderOverlay(width int, dividerColor string) []string {
    if !e.active || width <= 0 {
        return nil
    }
    
    lines := make([]string, 0, 3)
    
    // Divider
    divider := lipgloss.NewStyle().
        Foreground(lipgloss.Color(dividerColor)).
        Render(strings.Repeat("?", width))
    lines = append(lines, divider)
    
    // Input
    inputView := e.InputView()
    lines = append(lines, ansi.PadExact(inputView, width))
    
    // Status
    status := "Type to search (esc: close, enter: finish typing)"
    if e.query != "" {
        if len(e.matches) == 0 {
            status = "No matches (esc: close)"
        } else {
            status = fmt.Sprintf(
                "Match %d of %d  (Enter/↓: next, ↑: prev, Esc: close)",
                e.CurrentMatchIndex(),
                e.MatchCount(),
            )
        }
    }
    
    statusStyled := lipgloss.NewStyle().Faint(true).Render(status)
    lines = append(lines, ansi.PadExact(statusStyled, width))
    
    return lines
}
