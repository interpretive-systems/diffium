package ansi

import (
    "strings"
    
    "github.com/charmbracelet/x/ansi"
)

// WrapLine wraps a single line to the given width, preserving ANSI codes.
func WrapLine(s string, width int) []string {
    if width <= 0 {
        return []string{""}
    }
    wrapped := ansi.Hardwrap(s, width, false)
    return strings.Split(wrapped, "\n")
}

// WrapLines wraps multiple lines.
func WrapLines(lines []string, width int) []string {
    result := make([]string, 0, len(lines)*2)
    for _, line := range lines {
        wrapped := WrapLine(line, width)
        result = append(result, wrapped...)
    }
    return result
}
