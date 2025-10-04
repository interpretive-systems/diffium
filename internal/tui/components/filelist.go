package components

import (
    "fmt"
    "strings"
    
    "github.com/interpretive-systems/diffium/internal/gitx"
)

// FileList manages the left pane file list.
type FileList struct {
    files    []gitx.FileChange
    selected int
    offset   int
}

// NewFileList creates a new file list.
func NewFileList() *FileList {
    return &FileList{}
}

// SetFiles updates the file list.
func (f *FileList) SetFiles(files []gitx.FileChange) {
    f.files = files
    if f.selected >= len(files) {
        f.selected = len(files) - 1
    }
    if f.selected < 0 {
        f.selected = 0
    }
}

// Files returns the current file list.
func (f *FileList) Files() []gitx.FileChange {
    return f.files
}

// Selected returns the currently selected file index.
func (f *FileList) Selected() int {
    return f.selected
}

// SelectedFile returns the currently selected file.
func (f *FileList) SelectedFile() *gitx.FileChange {
    if len(f.files) == 0 || f.selected < 0 || f.selected >= len(f.files) {
        return nil
    }
    return &f.files[f.selected]
}

// MoveSelection moves the selection by delta.
func (f *FileList) MoveSelection(delta int) bool {
    if len(f.files) == 0 {
        return false
    }
    
    newSel := f.selected + delta
    if newSel < 0 {
        newSel = 0
    }
    if newSel >= len(f.files) {
        newSel = len(f.files) - 1
    }
    
    changed := newSel != f.selected
    f.selected = newSel
    return changed
}

// GoToTop moves selection to the first file.
func (f *FileList) GoToTop() bool {
    if len(f.files) == 0 || f.selected == 0 {
        return false
    }
    f.selected = 0
    return true
}

// GoToBottom moves selection to the last file.
func (f *FileList) GoToBottom() bool {
    if len(f.files) == 0 {
        return false
    }
    last := len(f.files) - 1
    if f.selected == last {
        return false
    }
    f.selected = last
    return true
}

// PageUp scrolls up one page.
func (f *FileList) PageUp(visibleCount int) {
    if visibleCount <= 0 {
        visibleCount = 10
    }
    step := visibleCount - 1
    if step < 1 {
        step = 1
    }
    
    newOffset := f.offset - step
    if newOffset < 0 {
        newOffset = 0
    }
    
    if f.selected < newOffset {
        newOffset = f.selected
    }
    
    maxStart := len(f.files) - visibleCount
    if maxStart < 0 {
        maxStart = 0
    }
    if newOffset > maxStart {
        newOffset = maxStart
    }
    
    f.offset = newOffset
}

// PageDown scrolls down one page.
func (f *FileList) PageDown(visibleCount int) {
    if visibleCount <= 0 {
        visibleCount = 10
    }
    step := visibleCount - 1
    if step < 1 {
        step = 1
    }
    
    maxStart := len(f.files) - visibleCount
    if maxStart < 0 {
        maxStart = 0
    }
    
    newOffset := f.offset + step
    if newOffset > maxStart {
        newOffset = maxStart
    }
    
    if f.selected >= newOffset+visibleCount {
        newOffset = f.selected - visibleCount + 1
        if newOffset < 0 {
            newOffset = 0
        }
    }
    
    f.offset = newOffset
}

// EnsureVisible ensures the selected item is visible.
func (f *FileList) EnsureVisible(visibleCount int) {
    if len(f.files) == 0 || visibleCount <= 0 {
        return
    }
    
    if f.offset < 0 {
        f.offset = 0
    }
    
    maxStart := len(f.files) - visibleCount
    if maxStart < 0 {
        maxStart = 0
    }
    if f.offset > maxStart {
        f.offset = maxStart
    }
    
    if f.selected < f.offset {
        f.offset = f.selected
    } else if f.selected >= f.offset+visibleCount {
        f.offset = f.selected - visibleCount + 1
        if f.offset < 0 {
            f.offset = 0
        }
    }
    
    if f.offset > maxStart {
        f.offset = maxStart
    }
}

// Render renders the file list to lines.
func (f *FileList) Render(height int) []string {
    lines := make([]string, 0, height)
    
    if len(f.files) == 0 {
        lines = append(lines, "No changes detected")
        return lines
    }
    
    f.EnsureVisible(height)
    
    start := f.offset
    end := start + height
    if end > len(f.files) {
        end = len(f.files)
    }
    
    for i := start; i < end; i++ {
        file := f.files[i]
        marker := "  "
        if i == f.selected {
            marker = "> "
        }
        status := FileStatusLabel(file)
        line := fmt.Sprintf("%s%s %s", marker, status, file.Path)
        lines = append(lines, line)
    }
    
    return lines
}

// FileStatusLabel returns a short status label for a file.
func FileStatusLabel(f gitx.FileChange) string {
    var tags []string
    if f.Deleted {
        tags = append(tags, "D")
    }
    if f.Untracked {
        tags = append(tags, "U")
    }
    if f.Staged {
        tags = append(tags, "S")
    }
    if f.Unstaged {
        tags = append(tags, "M")
    }
    if len(tags) == 0 {
        return "-"
    }
    return strings.Join(tags, "")
}
