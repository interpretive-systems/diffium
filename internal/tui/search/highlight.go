package search

import (
    "sort"
    "strings"
    "unicode/utf8"
    
    "github.com/interpretive-systems/diffium/internal/tui/ansi"
)

const (
    // Normal match: black on bright white
    matchStartSeq        = "\x1b[30;107m"
    // Current match: black on yellow
    currentMatchStartSeq = "\x1b[30;43m"
    // Reset all styles
    matchEndSeq          = "\x1b[0m"
)

// Highlighter applies search highlights to text.
type Highlighter struct{}

// NewHighlighter creates a new highlighter.
func NewHighlighter() *Highlighter {
    return &Highlighter{}
}

// HighlightLines applies highlights to matching lines.
func (h *Highlighter) HighlightLines(lines []string, query string, matches []int, currentLine int) []string {
    if len(lines) == 0 || query == "" {
        return lines
    }
    
    // Build set of matching lines
    matchSet := make(map[int]struct{}, len(matches))
    for _, idx := range matches {
        if idx >= 0 && idx < len(lines) {
            matchSet[idx] = struct{}{}
        }
    }
    
    result := make([]string, len(lines))
    for i, line := range lines {
        if _, hasMatch := matchSet[i]; !hasMatch {
            result[i] = line
            continue
        }
        
        ranges := findQueryRanges(line, query)
        if len(ranges) == 0 {
            result[i] = line
            continue
        }
        
        result[i] = h.applyRangeHighlight(line, ranges, i == currentLine)
    }
    
    return result
}

// RuneRange represents a range of runes in a string.
type RuneRange struct {
    Start int
    End   int
}

// findQueryRanges finds all occurrences of query in line (case-insensitive).
func findQueryRanges(line, query string) []RuneRange {
    plain := ansi.Strip(line)
    if plain == "" || query == "" {
        return nil
    }
    
    lowerRunes := []rune(strings.ToLower(plain))
    queryRunes := []rune(strings.ToLower(query))
    
    if len(queryRunes) == 0 || len(queryRunes) > len(lowerRunes) {
        return nil
    }
    
    ranges := make([]RuneRange, 0, 4)
    for i := 0; i <= len(lowerRunes)-len(queryRunes); i++ {
        match := true
        for j := 0; j < len(queryRunes); j++ {
            if lowerRunes[i+j] != queryRunes[j] {
                match = false
                break
            }
        }
        if match {
            ranges = append(ranges, RuneRange{Start: i, End: i + len(queryRunes)})
        }
    }
    
    if len(ranges) == 0 {
        return nil
    }
    
    return mergeRanges(ranges)
}

// mergeRanges merges overlapping or adjacent ranges.
func mergeRanges(ranges []RuneRange) []RuneRange {
    if len(ranges) <= 1 {
        return ranges
    }
    
    sort.Slice(ranges, func(i, j int) bool {
        if ranges[i].Start == ranges[j].Start {
            return ranges[i].End < ranges[j].End
        }
        return ranges[i].Start < ranges[j].Start
    })
    
    merged := make([]RuneRange, 0, len(ranges))
    cur := ranges[0]
    
    for _, r := range ranges[1:] {
        if r.Start <= cur.End {
            if r.End > cur.End {
                cur.End = r.End
            }
            continue
        }
        merged = append(merged, cur)
        cur = r
    }
    merged = append(merged, cur)
    
    return merged
}

// applyRangeHighlight applies ANSI highlight codes to ranges in the line.
func (h *Highlighter) applyRangeHighlight(line string, ranges []RuneRange, isCurrent bool) string {
    if len(ranges) == 0 {
        return line
    }
    
    startSeq := matchStartSeq
    if isCurrent {
        startSeq = currentMatchStartSeq
    }
    
    var b strings.Builder
    matchIdx := 0
    inMatch := false
    runePos := 0
    
    for i := 0; i < len(line); {
        // Handle ANSI escape sequences
        if line[i] == 0x1b {
            next := ansi.ConsumeEscape(line, i)
            b.WriteString(line[i:next])
            i = next
            continue
        }
        
        r, size := utf8.DecodeRuneInString(line[i:])
        _ = r
        
        // Close match if we've passed the end
        if inMatch {
            for matchIdx < len(ranges) && runePos >= ranges[matchIdx].End {
                b.WriteString(matchEndSeq)
                inMatch = false
                matchIdx++
            }
        }
        
        // Skip past completed ranges
        for !inMatch && matchIdx < len(ranges) && runePos >= ranges[matchIdx].End {
            matchIdx++
        }
        
        // Start match if we're at the start of a range
        if !inMatch && matchIdx < len(ranges) && runePos == ranges[matchIdx].Start {
            b.WriteString(startSeq)
            inMatch = true
        }
        
        b.WriteString(line[i : i+size])
        runePos++
        i += size
    }
    
    if inMatch {
        b.WriteString(matchEndSeq)
    }
    
    return b.String()
}
