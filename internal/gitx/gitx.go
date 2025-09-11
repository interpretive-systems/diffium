package gitx

import (
    "bytes"
    "errors"
    "fmt"
    "os/exec"
    "path/filepath"
    "sort"
    "strings"
)

// FileChange represents a changed file in the repo.
type FileChange struct {
    Path      string
    Staged    bool
    Unstaged  bool
    Untracked bool
    Binary    bool
    Deleted   bool
}

// RepoRoot resolves the git repository root from a given path (or current dir).
func RepoRoot(path string) (string, error) {
    if path == "" {
        path = "."
    }
    cmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
    out, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("rev-parse: %w", err)
    }
    root := strings.TrimSpace(string(out))
    if root == "" {
        return "", errors.New("empty git root")
    }
    return root, nil
}

// ChangedFiles lists files changed relative to HEAD, combining staged, unstaged, and untracked.
func ChangedFiles(repoRoot string) ([]FileChange, error) {
    // Unstaged vs index (include deletions)
    unstaged, err := listNames(repoRoot, []string{"diff", "--name-only", "--diff-filter=ACDMRTUXB"})
    if err != nil {
        return nil, err
    }
    // Staged vs HEAD
    staged, err := listNames(repoRoot, []string{"diff", "--name-only", "--cached", "--diff-filter=ACDMRTUXB"})
    if err != nil {
        return nil, err
    }
    // Untracked files
    untracked, err := listNames(repoRoot, []string{"ls-files", "--others", "--exclude-standard"})
    if err != nil {
        return nil, err
    }
    // Deletions detail
    deletedUnstaged, _ := listNames(repoRoot, []string{"ls-files", "-d"}) // deleted in WT, not staged
    deletedStaged, _ := listNames(repoRoot, []string{"diff", "--cached", "--name-only", "--diff-filter=D"})

    m := map[string]*FileChange{}
    mark := func(paths []string, fn func(fc *FileChange)) {
        for _, p := range paths {
            if p == "" { // skip any empties
                continue
            }
            if !filepath.IsAbs(p) {
                // Keep paths relative to repo root for display
            }
            fc := m[p]
            if fc == nil {
                fc = &FileChange{Path: p}
                m[p] = fc
            }
            fn(fc)
        }
    }
    mark(unstaged, func(fc *FileChange) { fc.Unstaged = true })
    mark(staged, func(fc *FileChange) { fc.Staged = true })
    mark(untracked, func(fc *FileChange) { fc.Untracked = true })
    mark(deletedUnstaged, func(fc *FileChange) { fc.Deleted = true; fc.Unstaged = true })
    mark(deletedStaged, func(fc *FileChange) { fc.Deleted = true; fc.Staged = true })

    // Determine potential binaries by probing diff header quickly
    paths := make([]string, 0, len(m))
    for p := range m {
        paths = append(paths, p)
    }
    sort.Strings(paths)
    out := make([]FileChange, 0, len(paths))
    for _, p := range paths {
        fc := m[p]
        // Lightweight binary check: if unified diff says Binary files differ
        if isBinary(repoRoot, p) {
            fc.Binary = true
        }
        out = append(out, *fc)
    }
    return out, nil
}

func listNames(repoRoot string, args []string) ([]string, error) {
    a := append([]string{"-C", repoRoot}, args...)
    cmd := exec.Command("git", a...)
    b, err := cmd.Output()
    if err != nil {
        // On empty sets git exits 0 with empty output; any non-0 means real error
        // Return empty result for safety only when output is empty
        return nil, fmt.Errorf("git %v: %w", strings.Join(args, " "), err)
    }
    lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
    out := make([]string, 0, len(lines))
    for _, l := range lines {
        l = strings.TrimSpace(l)
        if l != "" {
            out = append(out, l)
        }
    }
    return out, nil
}

// DiffHEAD returns a unified diff between HEAD and the working tree for a single file.
func DiffHEAD(repoRoot, path string) (string, error) {
    var args []string
    if isTracked(repoRoot, path) {
        args = []string{"-C", repoRoot, "diff", "--no-color", "--text", "HEAD", "--", path}
    } else {
        // For untracked files, show diff vs /dev/null
        args = []string{"-C", repoRoot, "diff", "--no-color", "--no-index", "--text", "/dev/null", path}
    }
    cmd := exec.Command("git", args...)
    b, err := cmd.CombinedOutput()
    if err != nil {
        if len(b) == 0 {
            return "", fmt.Errorf("git diff: %w", err)
        }
    }
    return string(b), nil
}

func isBinary(repoRoot, path string) bool {
    var args []string
    if isTracked(repoRoot, path) {
        args = []string{"-C", repoRoot, "diff", "--numstat", "HEAD", "--", path}
    } else {
        args = []string{"-C", repoRoot, "diff", "--numstat", "--no-index", "/dev/null", path}
    }
    cmd := exec.Command("git", args...)
    b, _ := cmd.Output()
    line := strings.TrimSpace(string(b))
    if line == "" {
        return false
    }
    // numstat returns "-\t-\tpath" for binary files
    parts := strings.Split(line, "\t")
    if len(parts) >= 2 && (parts[0] == "-" || parts[1] == "-") {
        return true
    }
    // Fallback: detect textual mention just in case
    return bytes.Contains(b, []byte("-\t-\t"))
}

func isTracked(repoRoot, path string) bool {
    cmd := exec.Command("git", "-C", repoRoot, "ls-files", "--error-unmatch", "--", path)
    if err := cmd.Run(); err != nil {
        return false
    }
    return true
}

// StageFiles stages the provided file paths.
func StageFiles(repoRoot string, paths []string) error {
    if len(paths) == 0 {
        return nil
    }
    // Use -A to ensure deletions are staged too, but still scoped to pathspecs
    args := append([]string{"-C", repoRoot, "add", "-A", "--"}, paths...)
    cmd := exec.Command("git", args...)
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("git add: %w: %s", err, string(out))
    }
    return nil
}

// Commit performs a git commit with the given message.
func Commit(repoRoot, message string) error {
    if strings.TrimSpace(message) == "" {
        return errors.New("empty commit message")
    }
    cmd := exec.Command("git", "-C", repoRoot, "commit", "-m", message)
    if out, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("git commit: %w: %s", err, string(out))
    }
    return nil
}

// Push attempts to push the current branch. If no upstream is set,
// it falls back to pushing to the first remote (or origin) with -u.
func Push(repoRoot string) error {
    // Try simple push first
    cmd := exec.Command("git", "-C", repoRoot, "push")
    if out, err := cmd.CombinedOutput(); err == nil {
        return nil
    } else {
        // Fallback logic
        // Get current branch
        bcmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
        bOut, bErr := bcmd.Output()
        if bErr != nil {
            return fmt.Errorf("git push: %w: %s", err, string(out))
        }
        branch := strings.TrimSpace(string(bOut))
        // Choose remote
        rcmd := exec.Command("git", "-C", repoRoot, "remote")
        rOut, _ := rcmd.Output()
        remotes := strings.Fields(string(rOut))
        remote := "origin"
        if len(remotes) > 0 {
            remote = remotes[0]
        }
        cmd2 := exec.Command("git", "-C", repoRoot, "push", "-u", remote, branch)
        if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
            return fmt.Errorf("git push: %w: %s", err2, string(out2))
        }
    }
    return nil
}

// LastCommitSummary returns short hash and subject of last commit.
func LastCommitSummary(repoRoot string) (string, error) {
    cmd := exec.Command("git", "-C", repoRoot, "log", "-1", "--pretty=format:%h %s")
    b, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("git log: %w", err)
    }
    return strings.TrimSpace(string(b)), nil
}
