package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/interpretive-systems/diffium/internal/diffview"
	"github.com/interpretive-systems/diffium/internal/gitx"
	"github.com/interpretive-systems/diffium/internal/theme"
	"github.com/interpretive-systems/diffium/internal/tui/components"
	"github.com/interpretive-systems/diffium/internal/tui/search"
)

func baseModelForTest() Program {
	filesChanged := []gitx.FileChange{
		{Path: "file1.txt", Unstaged: true},
		{Path: "file2.txt", Staged: true},
	}

	sb := components.NewStatusBar()
	curTime, _ := time.Parse(time.TimeOnly, "12:34:56")
	sb.SetLastRefresh(curTime)

	fl := components.NewFileList()
	fl.SetFiles(filesChanged)

	m := Program{
		state: &State{
			Width:        80,
			Height:       16,
			RepoRoot:     ".",
			SearchEngine: search.New(),
			DiffView:     components.NewDiffView(theme.LoadThemeFromRepo(".")),
			Theme:        theme.DefaultTheme(),
			Files:        filesChanged,
			FileList:     fl,
			StatusBar:    sb,
			LastRefresh:  time.Date(2024, 10, 1, 12, 34, 56, 0, time.UTC),
			ShowHelp:     false,
		},
		layout: &Layout{
			width:     80,
			height:    16,
			leftWidth: 24,
		},
		keyHandler: &KeyHandler{},
	}

	return m
}

func sampleUnified() string {
	return "@@ -1,3 +1,3 @@\n line1\n-line2\n+line2 changed\n line3\n"
}

func TestView_SideBySide_Render(t *testing.T) {
	m := baseModelForTest()
	m.state.DiffView.SetSideBySide(true)
	m.state.DiffView.SetRows(diffview.BuildRowsFromUnified(sampleUnified()))
	m.recalcViewport()

	out := m.View()
	plain := ansi.Strip(out)

	// Snapshot-like assertions
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
	m.state.DiffView.SetSideBySide(false)
	m.state.DiffView.SetRows(diffview.BuildRowsFromUnified(sampleUnified()))
	m.recalcViewport()

	out := m.View()
	plain := ansi.Strip(out)

	if !strings.Contains(plain, "+ line2 changed") {
		t.Fatalf("expected inline added line, got: %q", plain)
	}
	if !strings.Contains(plain, "- line2") {
		t.Fatalf("expected inline deleted line, got: %q", plain)
	}
}
