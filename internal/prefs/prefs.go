package prefs

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Prefs represents persisted UI preferences.
type Prefs struct {
	Wrap       bool
	WrapSet    bool
	SideBySide bool
	SideSet    bool
	LeftWidth  int
	LeftSet    bool
}

const (
	keyWrap       = "diffium.wrap"
	keySideBySide = "diffium.sideBySide"
	keyLeftWidth  = "diffium.leftWidth"
)

// Load reads preferences from git local config.
func Load(repoRoot string) Prefs {
	var p Prefs
	if s, ok := get(repoRoot, keyWrap); ok {
		p.WrapSet = true
		p.Wrap = parseBool(s)
	}
	if s, ok := get(repoRoot, keySideBySide); ok {
		p.SideSet = true
		p.SideBySide = parseBool(s)
	}
	if s, ok := get(repoRoot, keyLeftWidth); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > 0 {
			p.LeftSet = true
			p.LeftWidth = n
		}
	}
	return p
}

// SaveWrap persists wrap pref.
func SaveWrap(repoRoot string, v bool) error {
	return set(repoRoot, keyWrap, boolStr(v))
}

// SaveSideBySide persists side-by-side pref.
func SaveSideBySide(repoRoot string, v bool) error {
	return set(repoRoot, keySideBySide, boolStr(v))
}

// SaveLeftWidth persists left column width.
func SaveLeftWidth(repoRoot string, w int) error {
	if w <= 0 {
		return fmt.Errorf("invalid left width: %d", w)
	}
	return set(repoRoot, keyLeftWidth, strconv.Itoa(w))
}

func get(repoRoot, key string) (string, bool) {
	cmd := exec.Command("git", "-C", repoRoot, "config", "--get", key)
	b, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

func set(repoRoot, key, value string) error {
	cmd := exec.Command("git", "-C", repoRoot, "config", "--local", key, value)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config %s: %w: %s", key, err, string(out))
	}
	return nil
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
