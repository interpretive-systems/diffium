package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// KeyBindings stores all customizable key bindings
type KeyBindings struct {
	Quit             []string `json:"quit"`
	Help             []string `json:"help"`
	Commit           []string `json:"commit"`
	Uncommit         []string `json:"uncommit"`
	BranchSwitch     []string `json:"branch_switch"`
	Pull             []string `json:"pull"`
	Reset            []string `json:"reset"`
	Search           []string `json:"search"`
	Refresh          []string `json:"refresh"`
	SideBySide       []string `json:"side_by_side"`
	ToggleWrap       []string `json:"toggle_wrap"`
	ToggleDiffMode   []string `json:"toggle_diff_mode"`
	NavigateUp       []string `json:"navigate_up"`
	NavigateDown     []string `json:"navigate_down"`
	NavigatePageUp   []string `json:"navigate_page_up"`
	NavigatePageDown []string `json:"navigate_page_down"`
}

// DefaultKeyBindings returns the default key bindings
func DefaultKeyBindings() KeyBindings {
	return KeyBindings{
		Quit:             []string{"ctrl+c", "q"},
		Help:             []string{"h"},
		Commit:           []string{"c"},
		Uncommit:         []string{"u"},
		BranchSwitch:     []string{"b"},
		Pull:             []string{"p"},
		Reset:            []string{"R"},
		Search:           []string{"/"},
		Refresh:          []string{"r"},
		SideBySide:       []string{"s"},
		ToggleWrap:       []string{"w"},
		ToggleDiffMode:   []string{"t"},
		NavigateUp:       []string{"k", "up"},
		NavigateDown:     []string{"j", "down"},
		NavigatePageUp:   []string{"K", "pgup"},
		NavigatePageDown: []string{"J", "pgdown"},
	}
}

// LoadKeyBindings loads key bindings
func LoadKeyBindings(configDir string) (KeyBindings, error) {
	configPath := filepath.Join(configDir, "keybindings.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		kb := DefaultKeyBindings()
		if err := SaveKeyBindings(configDir, kb); err != nil {
			return kb, fmt.Errorf("failed to save default config: %w", err)
		}
		return kb, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return DefaultKeyBindings(), fmt.Errorf("failed to read config: %w", err)
	}

	var kb KeyBindings
	if err := json.Unmarshal(data, &kb); err != nil {
		return DefaultKeyBindings(), fmt.Errorf("failed to parse config: %w", err)
	}

	return kb, nil
}

// SaveKeyBindings
func SaveKeyBindings(configDir string, kb KeyBindings) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(kb, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(configDir, "keybindings.json")
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
