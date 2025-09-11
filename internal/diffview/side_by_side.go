package diffview

import (
    "bufio"
    "strings"
)

// RowKind represents the semantic type of a side-by-side row.
type RowKind int

const (
    RowContext RowKind = iota
    RowAdd
    RowDel
    RowReplace
    RowHunk
    RowMeta
)

// Row represents a single visual row for side-by-side rendering.
type Row struct {
    Left  string
    Right string
    Kind  RowKind
    Meta  string // for hunk header text
}

// BuildRowsFromUnified parses a unified diff string into side-by-side rows.
// It uses a simple pairing strategy within each hunk: deletions are paired
// with subsequent additions as replacements; any remaining lines are shown
// as left-only (deletions) or right-only (additions).
func BuildRowsFromUnified(unified string) []Row {
    s := bufio.NewScanner(strings.NewReader(unified))
    s.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // allow large lines

    rows := make([]Row, 0, 256)
    pendingDel := make([]string, 0)

    flushPending := func() {
        for _, dl := range pendingDel {
            rows = append(rows, Row{Left: trimPrefix(dl), Right: "", Kind: RowDel})
        }
        pendingDel = pendingDel[:0]
    }

    inHunk := false
    for s.Scan() {
        line := s.Text()
        if strings.HasPrefix(line, "diff --git ") || strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
            // Metadata; flush any pending deletions
            flushPending()
            rows = append(rows, Row{Kind: RowMeta, Meta: line})
            continue
        }
        if strings.HasPrefix(line, "@@ ") {
            flushPending()
            rows = append(rows, Row{Kind: RowHunk, Meta: line})
            inHunk = true
            continue
        }
        if !inHunk {
            // Outside hunks, we don't have meaningful line-level info; skip
            continue
        }

        if len(line) == 0 {
            // blank line inside hunk: treat as context
            flushPending()
            rows = append(rows, Row{Left: "", Right: "", Kind: RowContext})
            continue
        }

        switch line[0] {
        case ' ':
            flushPending()
            t := trimPrefix(line)
            rows = append(rows, Row{Left: t, Right: t, Kind: RowContext})
        case '-':
            pendingDel = append(pendingDel, line)
        case '+':
            if len(pendingDel) > 0 {
                // Pair with the earliest pending deletion
                dl := pendingDel[0]
                pendingDel = pendingDel[1:]
                rows = append(rows, Row{Left: trimPrefix(dl), Right: trimPrefix(line), Kind: RowReplace})
            } else {
                rows = append(rows, Row{Left: "", Right: trimPrefix(line), Kind: RowAdd})
            }
        default:
            // Unknown line; ignore
        }
    }
    flushPending()
    return rows
}

func trimPrefix(s string) string {
    if s == "" {
        return s
    }
    if s[0] == ' ' || s[0] == '+' || s[0] == '-' {
        return s[1:]
    }
    return s
}

