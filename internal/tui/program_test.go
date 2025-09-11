package tui

import (
    "strings"
    "testing"
    "time"

    "github.com/charmbracelet/x/ansi"
    "github.com/interpretive-systems/diffium/internal/diffview"
    "github.com/interpretive-systems/diffium/internal/gitx"
)

func baseModelForTest() model {
    m := model{}
    m.repoRoot = "."
    m.theme = defaultTheme()
    m.files = []gitx.FileChange{
        {Path: "file1.txt", Unstaged: true},
        {Path: "file2.txt", Staged: true},
    }
    m.selected = 0
    m.width = 80
    m.height = 16
    m.leftWidth = 24
    m.lastRefresh = time.Date(2024, 10, 1, 12, 34, 56, 0, time.UTC)
    m.showHelp = false
    m.showCommit = false
    return m
}

func sampleUnified() string {
    return "@@ -1,3 +1,3 @@\n line1\n-line2\n+line2 changed\n line3\n"
}

func TestView_SideBySide_Render(t *testing.T) {
    m := baseModelForTest()
    m.sideBySide = true
    m.rows = diffview.BuildRowsFromUnified(sampleUnified())
    (&m).recalcViewport()
    out := m.View()
    plain := ansi.Strip(out)

    // Basic snapshot-like assertions
    if !strings.HasPrefix(plain, "Changes | file1.txt (M)") {
        t.Fatalf("unexpected header: %q", strings.SplitN(plain, "\n", 2)[0])
    }
    if !strings.Contains(plain, "│") {
        t.Fatalf("expected vertical divider in view")
    }
    if !strings.Contains(plain, "line2 changed") {
        t.Fatalf("expected changed text in right pane")
    }
    if !strings.Contains(plain, "refreshed: 12:34:56") {
        t.Fatalf("expected bottom bar timestamp, got: %q", plain)
    }
}

func TestView_Inline_Render(t *testing.T) {
    m := baseModelForTest()
    m.sideBySide = false
    m.rows = diffview.BuildRowsFromUnified(sampleUnified())
    (&m).recalcViewport()
    out := m.View()
    plain := ansi.Strip(out)

    if !strings.Contains(plain, "+ line2 changed") {
        t.Fatalf("expected inline added line, got: %q", plain)
    }
    if !strings.Contains(plain, "- line2") {
        t.Fatalf("expected inline deleted line, got: %q", plain)
    }
}

