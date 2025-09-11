package gitx

import (
    "os"
    "os/exec"
    "path/filepath"
    "testing"
)

func TestPushToBareRemote(t *testing.T) {
    dir := t.TempDir()
    mustRunT(t, dir, "git", "-c", "init.defaultBranch=main", "init", "-q")
    mustRunT(t, dir, "git", "config", "user.email", "test@example.com")
    mustRunT(t, dir, "git", "config", "user.name", "Test User")

    writeT(t, filepath.Join(dir, "f.txt"), "hello\n")
    mustRunT(t, dir, "git", "add", ".")
    mustRunT(t, dir, "git", "commit", "-q", "-m", "init")

    remote := filepath.Join(dir, "remote.git")
    mustRunT(t, dir, "git", "init", "--bare", remote)
    mustRunT(t, dir, "git", "remote", "add", "origin", remote)

    // change and commit via our helpers
    writeT(t, filepath.Join(dir, "f.txt"), "hello world\n")
    if err := StageFiles(dir, []string{"f.txt"}); err != nil { t.Fatal(err) }
    if err := Commit(dir, "update"); err != nil { t.Fatal(err) }
    if err := Push(dir); err != nil { t.Fatalf("push failed: %v", err) }
}

func mustRunT(t *testing.T, dir string, name string, args ...string) {
    t.Helper()
    cmd := exec.Command(name, args...)
    cmd.Dir = dir
    if out, err := cmd.CombinedOutput(); err != nil {
        t.Fatalf("command %s %v failed: %v\n%s", name, args, err, out)
    }
}

func writeT(t *testing.T, path, content string) {
    t.Helper()
    if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
        t.Fatal(err)
    }
}
