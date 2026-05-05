package vcs_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	vcs "github.com/artschekoff/context-consolidator/internal/vcs"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}
	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("hello"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
	exec.Command("git", "-C", dir, "checkout", "-b", "feature/test").Run()
	exec.Command("git", "-C", dir, "checkout", "main").Run()
	return dir
}

func TestGitCheckout_Success(t *testing.T) {
	dir := initTestRepo(t)
	if err := vcs.GitCheckout(dir, "feature/test"); err != nil {
		t.Fatalf("GitCheckout: %v", err)
	}
	out, _ := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if string(out) != "feature/test\n" {
		t.Errorf("expected feature/test, got %q", string(out))
	}
}

func TestGitCheckout_InvalidBranch(t *testing.T) {
	dir := initTestRepo(t)
	err := vcs.GitCheckout(dir, "no-such-branch")
	if err == nil {
		t.Error("expected error for nonexistent branch, got nil")
	}
}
