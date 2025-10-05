package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"fmt" // <--- FIXED: Added missing fmt import

	"github.com/charmbracelet/lipgloss"
)

// Theme defines customizable colors for rendering.
type Theme struct {
	AddColor     string `json:"addColor"`
	DelColor     string `json:"delColor"`
	MetaColor    string `json:"metaColor"`
	DividerColor string `json:"dividerColor"`
	// NEW: Background colors
	AddBgColor string `json:"addBgColor"`
	DelBgColor string `json:"delBgColor"`
}

// --- Theme Definitions ---

func darkTheme() Theme {
	return Theme{
		AddColor:     "34",
		DelColor:     "196",
		MetaColor:    "63",
		DividerColor: "240",
		AddBgColor:   "235", // Subtle dark background
		DelBgColor:   "235", // Subtle dark background
	}
}

func lightTheme() Theme {
	return Theme{
		AddColor:     "22",  // Dark Green
		DelColor:     "9",   // Dark Red
		MetaColor:    "27",  // Dark Blue
		DividerColor: "244", // Medium Gray
		AddBgColor:   "255", // Very light background (e.g., White-Green tint)
		DelBgColor:   "255", // Very light background (e.g., White-Red tint)
	}
}

// GetTheme returns the requested base theme.
func GetTheme(name string) Theme {
	switch name {
	case "light":
		fmt.Printf("DEBUG: Loading Light Theme\n") // Temporary debug
		return lightTheme()
	default: // "dark" or any other value
		fmt.Printf("DEBUG: Loading Dark Theme\n") // Temporary debug
		return darkTheme()
	}
}

// loadThemeFromRepo tries .diffium/theme.json at repoRoot.
func loadThemeFromRepo(repoRoot, baseTheme string) Theme {
	t := GetTheme(baseTheme)
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
	if u.AddBgColor != "" { // <--- NEW: Merge background colors
		t.AddBgColor = u.AddBgColor
	}
	if u.DelBgColor != "" { // <--- NEW: Merge background colors
		t.DelBgColor = u.DelBgColor
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

// NEW: Methods to apply both foreground (text) and background color for entire lines
func (t Theme) AddLine(s string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.AddColor)).
		Background(lipgloss.Color(t.AddBgColor)).
		Render(s)
}

func (t Theme) DelLine(s string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.DelColor)).
		Background(lipgloss.Color(t.DelBgColor)).
		Render(s)
}
