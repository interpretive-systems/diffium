package wizards

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/interpretive-systems/diffium/internal/gitx"
)

// Action represents what the wizard wants the parent to do.
type Action int

const (
    ActionContinue Action = iota // Continue processing in wizard
    ActionClose                  // Close the wizard
    ActionBack                   // Go back a step (internal)
)

// Wizard is the interface all wizards implement.
type Wizard interface {
    // Init initializes the wizard with repo and file state.
    Init(repoRoot string, files []gitx.FileChange) tea.Cmd
    
    // HandleKey processes keyboard input.
    // Returns the action to take and any commands.
    HandleKey(msg tea.KeyMsg) (Action, tea.Cmd)
    
    // Update processes tea messages (for async results).
    Update(msg tea.Msg) tea.Cmd
    
    // RenderOverlay returns the wizard UI lines.
    RenderOverlay(width int) []string
    
    // IsComplete returns true if wizard finished successfully.
    IsComplete() bool
    
    // Error returns any error message.
    Error() string
}
