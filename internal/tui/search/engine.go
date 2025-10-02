package search

import (
    "strings"
    
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/interpretive-systems/diffium/internal/tui/ansi"
)

// Engine manages search state and operations.
type Engine struct {
    query       string
    matches     []int          // Line indices with matches
    index       int            // Current match index
    input       textinput.Model
    active      bool
    highlighter *Highlighter
    content     []string       // Current content being searched
}

// New creates a new search engine.
func New() *Engine {
    ti := textinput.New()
    ti.Placeholder = "Search diff"
    ti.Prompt = "/ "
    ti.CharLimit = 0
    
    return &Engine{
        highlighter: NewHighlighter(),
        input:      ti,
    }
}

// Activate opens the search input.
func (e *Engine) Activate() {
    e.active = true
    e.input.Focus()
}

// Deactivate closes search.
func (e *Engine) Deactivate() {
    e.active = false
    e.input.Blur()
}

// IsActive returns whether search is active.
func (e *Engine) IsActive() bool {
    return e.active
}

// HandleKey processes key input for search.
func (e *Engine) HandleKey(msg tea.KeyMsg) (bool, tea.Cmd) {
    switch msg.String() {
    case "esc":
        e.Deactivate()
        return true, nil
    case "enter", "down":
        e.Next()
        return true, nil
    case "up":
        e.Previous()
        return true, nil
    }
    
    var cmd tea.Cmd
    e.input, cmd = e.input.Update(msg)
    e.query = e.input.Value()
    e.recomputeMatches()
    
    return true, cmd
}

// SetContent updates the content to search through.
func (e *Engine) SetContent(lines []string) {
    e.content = lines
    e.recomputeMatches()
}

// Query returns the current search query.
func (e *Engine) Query() string {
    return e.query
}

// recomputeMatches finds all matching lines.
func (e *Engine) recomputeMatches() {
    if e.query == "" {
        e.matches = nil
        e.index = 0
        return
    }
    
    lowerQuery := strings.ToLower(e.query)
    matches := make([]int, 0, len(e.content))
    
    for i, line := range e.content {
        plain := strings.ToLower(ansi.Strip(line))
        if strings.Contains(plain, lowerQuery) {
            matches = append(matches, i)
        }
    }
    
    e.matches = matches
    if len(matches) > 0 && e.index >= len(matches) {
        e.index = 0
    }
}

// Next advances to the next match.
func (e *Engine) Next() {
    if len(e.matches) == 0 {
        return
    }
    e.index = (e.index + 1) % len(e.matches)
}

// Previous moves to the previous match.
func (e *Engine) Previous() {
    if len(e.matches) == 0 {
        return
    }
    e.index = (e.index - 1 + len(e.matches)) % len(e.matches)
}

// CurrentMatchLine returns the line index of the current match.
func (e *Engine) CurrentMatchLine() int {
    if len(e.matches) == 0 {
        return -1
    }
    return e.matches[e.index]
}

// HighlightedContent returns content with search highlights applied.
func (e *Engine) HighlightedContent() []string {
    if e.query == "" || len(e.content) == 0 {
        return e.content
    }
    
    currentLine := e.CurrentMatchLine()
    return e.highlighter.HighlightLines(e.content, e.query, e.matches, currentLine)
}

// MatchCount returns the number of matches.
func (e *Engine) MatchCount() int {
    return len(e.matches)
}

// CurrentMatchIndex returns the current match index (1-based).
func (e *Engine) CurrentMatchIndex() int {
    if len(e.matches) == 0 {
        return 0
    }
    return e.index + 1
}

// InputView returns the text input view.
func (e *Engine) InputView() string {
    return e.input.View()
}
