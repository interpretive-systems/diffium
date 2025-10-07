package tui

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

// Theme defines customizable colors for rendering.
type Theme struct {
	AddColor     string `json:"addColor"`     // e.g. "34" or "#22c55e"
	DelColor     string `json:"delColor"`     // e.g. "196" or "#ef4444"
	MetaColor    string `json:"metaColor"`    // optional, currently unused
	DividerColor string `json:"dividerColor"` // e.g. "240"
}

func defaultTheme() Theme {
	return Theme{
		AddColor:     "34",
		DelColor:     "196",
		MetaColor:    "63",
		DividerColor: "240",
	}
}

// loadThemeFromRepo tries .diffium/theme.json at repoRoot.
func loadThemeFromRepo(repoRoot string) Theme {
	t := defaultTheme()
	path := filepath.Join(repoRoot, ".diffium", "theme.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return t
	}
	var u Theme
	if err := json.Unmarshal(b, &u); err != nil {
		return t
	}
	// Merge, keeping defaults for empty fields
	if u.AddColor != "" {
		t.AddColor = u.AddColor
	}
	if u.DelColor != "" {
		t.DelColor = u.DelColor
	}
	if u.MetaColor != "" {
		t.MetaColor = u.MetaColor
	}
	if u.DividerColor != "" {
		t.DividerColor = u.DividerColor
	}
	return t
}

func (t Theme) AddText(s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.AddColor)).Render(s)
}

func (t Theme) DelText(s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.DelColor)).Render(s)
}

func (t Theme) DividerText(s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.DividerColor)).Render(s)
}
