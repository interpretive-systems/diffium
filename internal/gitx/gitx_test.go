package gitx

import (
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

func TestChangedFiles_AndDiffHEAD(t *testing.T) {
    dir := t.TempDir()

    mustRun(t, dir, "git", "init", "-q")
    mustRun(t, dir, "git", "config", "user.email", "test@example.com")
    mustRun(t, dir, "git", "config", "user.name", "Test User")

    // initial commit
    write(t, filepath.Join(dir, "f1.txt"), "one\nline\n")
    write(t, filepath.Join(dir, "del.txt"), "to delete\n")
    mustRun(t, dir, "git", "add", ".")
    mustRun(t, dir, "git", "commit", "-q", "-m", "init")

    // modify f1 (unstaged), create new (untracked), delete del.txt (unstaged)
    write(t, filepath.Join(dir, "f1.txt"), "one\nline changed\n")
    write(t, filepath.Join(dir, "new.txt"), "brand new\n")
    if err := os.Remove(filepath.Join(dir, "del.txt")); err != nil {
        t.Fatal(err)
    }

    files, err := ChangedFiles(dir)
    if err != nil {
        t.Fatalf("ChangedFiles error: %v", err)
    }
    // Collect map for assertions
    m := map[string]FileChange{}
    for _, f := range files {
        m[f.Path] = f
    }
    if !m["f1.txt"].Unstaged {
        t.Fatalf("expected f1.txt to be unstaged modified, got %+v", m["f1.txt"])
    }
    if !m["new.txt"].Untracked {
        t.Fatalf("expected new.txt to be untracked, got %+v", m["new.txt"])
    }
    if !(m["del.txt"].Deleted && m["del.txt"].Unstaged) {
        t.Fatalf("expected del.txt to be deleted unstaged, got %+v", m["del.txt"])
    }

    // DiffHEAD for modified file should be non-empty
    d, err := DiffHEAD(dir, "f1.txt")
    if err != nil {
        t.Fatalf("DiffHEAD error: %v", err)
    }
    if !strings.Contains(d, "-line") || !strings.Contains(d, "+line changed") {
        t.Fatalf("unexpected diff: %s", d)
    }

    // Stage all three and commit using StageFiles + Commit
    if err := StageFiles(dir, []string{"f1.txt", "new.txt", "del.txt"}); err != nil {
        t.Fatalf("StageFiles error: %v", err)
    }
    if err := Commit(dir, "test commit"); err != nil {
        t.Fatalf("Commit error: %v", err)
    }
    // After commit, ChangedFiles should be empty
    files2, err := ChangedFiles(dir)
    if err != nil {
        t.Fatalf("ChangedFiles(2) error: %v", err)
    }
    if len(files2) != 0 {
        t.Fatalf("expected no changes after commit, got %v", files2)
    }
}

func mustRun(t *testing.T, dir string, name string, args ...string) {
    t.Helper()
    cmd := exec.Command(name, args...)
    cmd.Dir = dir
    if out, err := cmd.CombinedOutput(); err != nil {
        t.Fatalf("command %s %v failed: %v\n%s", name, args, err, out)
    }
}

func write(t *testing.T, path, content string) {
    t.Helper()
    if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
        t.Fatal(err)
    }
}

