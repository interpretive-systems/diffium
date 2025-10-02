package tui

import (
    "strconv"
    
    tea "github.com/charmbracelet/bubbletea"
)

// KeyAction represents an action triggered by a key press.
type KeyAction int

const (
    ActionNone KeyAction = iota
    ActionQuit
    ActionToggleHelp
    ActionOpenCommit
    ActionOpenUncommit
    ActionOpenBranch
    ActionOpenPull
    ActionOpenResetClean
    ActionOpenSearch
    ActionRefresh
    ActionToggleSideBySide
    ActionToggleDiffMode
    ActionToggleWrap
    ActionMoveUp
    ActionMoveDown
    ActionGoToTop
    ActionGoToBottom
    ActionPageUpLeft
    ActionPageDownLeft
    ActionScrollLeft
    ActionScrollRight
    ActionScrollHome
    ActionPageDown
    ActionPageUp
    ActionHalfPageDown
    ActionHalfPageUp
    ActionLineDown
    ActionLineUp
    ActionAdjustLeftNarrower
    ActionAdjustLeftWider
    ActionSearchNext
    ActionSearchPrevious
    ActionOpenRevert
)

// KeyHandler handles key input and maintains key buffer.
type KeyHandler struct {
    keyBuffer string
}

// NewKeyHandler creates a new key handler.
func NewKeyHandler() *KeyHandler {
    return &KeyHandler{}
}

// Handle processes a key message and returns the action.
func (k *KeyHandler) Handle(msg tea.KeyMsg) (KeyAction, int) {
    key := msg.String()
    
    // Numeric keys build up the buffer
    if isNumericKey(key) {
        k.keyBuffer += key
        return ActionNone, 0
    }
    
    // Get count from buffer
    count := 1
    if k.keyBuffer != "" {
        if n, err := strconv.Atoi(k.keyBuffer); err == nil {
            count = n
        }
    }
    
    // Non-movement keys clear the buffer
    if !isMovementKey(key) {
        k.keyBuffer = ""
    }

    action := k.keyToAction(key)
    
    // Clear buffer after movement
    if isMovementKey(key) {
        k.keyBuffer = ""
    }
    
    return action, count
}

// KeyBuffer returns the current key buffer.
func (k *KeyHandler) KeyBuffer() string {
    return k.keyBuffer
}

// ClearBuffer clears the key buffer.
func (k *KeyHandler) ClearBuffer() {
    k.keyBuffer = ""
}

func (k *KeyHandler) keyToAction(key string) KeyAction {
    switch key {
    case "ctrl+c", "q":
        return ActionQuit
    case "h":
        return ActionToggleHelp
    case "c":
        return ActionOpenCommit
    case "u":
        return ActionOpenUncommit
    case "b":
        return ActionOpenBranch
    case "p":
        return ActionOpenPull
    case "R":
        return ActionOpenResetClean
    case "/":
        return ActionOpenSearch
    case "r":
        return ActionRefresh
    case "s":
        return ActionToggleSideBySide
    case "t":
        return ActionToggleDiffMode
    case "w":
        return ActionToggleWrap
    case "j", "down":
        return ActionMoveDown
    case "k", "up":
        return ActionMoveUp
    case "g":
        return ActionGoToTop
    case "G":
        return ActionGoToBottom
    case "[":
        return ActionPageUpLeft
    case "]":
        return ActionPageDownLeft
    case "left", "{":
        return ActionScrollLeft
    case "right", "}":
        return ActionScrollRight
    case "home":
        return ActionScrollHome
    case "pgdown":
        return ActionPageDown
    case "pgup":
        return ActionPageUp
    case "J", "ctrl+d":
        return ActionHalfPageDown
    case "K", "ctrl+u":
        return ActionHalfPageUp
    case "ctrl+e":
        return ActionLineDown
    case "ctrl+y":
        return ActionLineUp
    case ">":
        return ActionAdjustLeftWider
	case "<":
		return ActionAdjustLeftNarrower
    case "n":
        return ActionSearchNext
    case "N":
        return ActionSearchPrevious
    case "V":
        return ActionOpenRevert
    default:
        return ActionNone
    }
}

func isNumericKey(key string) bool {
    return len(key) == 1 && key >= "0" && key <= "9"
}

func isMovementKey(key string) bool {
    return key == "j" || key == "k" || key == "down" || key == "up"
}
