package ansi

import (
    "strings"
    
    "github.com/charmbracelet/x/ansi"
)

// SliceHorizontal returns a substring starting at visual column start with at most width columns.
// Preserves ANSI escape sequences.
func SliceHorizontal(s string, start, width int) string {
    if start <= 0 {
        return ansi.Truncate(s, width, "")
    }
    head := ansi.Truncate(s, start+width, "")
    return ansi.TruncateLeft(head, start, "")
}

// ClipToWidth truncates string to at most w visual columns without ellipsis.
func ClipToWidth(s string, w int) string {
    if w <= 0 {
        return ""
    }
    return ansi.Truncate(s, w, "")
}

// PadExact pads string with spaces to exactly width w (ANSI-aware).
func PadExact(s string, w int) string {
    vw := VisualWidth(s)
    if vw >= w {
        return s
    }
    return s + strings.Repeat(" ", w-vw)
}

// TruncateToWidth truncates to width with ellipsis if needed.
func TruncateToWidth(s string, width int) string {
    return ansi.Truncate(s, width, "…")
}
