package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func safeJoin(repoPath, path string) (string, error) {
	full := filepath.Clean(filepath.Join(repoPath, path))
	rel, err := filepath.Rel(repoPath, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes repository")
	}
	return full, nil
}

func (a *App) ReadRepoFile(repoPath, path string) (string, error) {
	full, err := safeJoin(repoPath, path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(b), nil
}

func (a *App) ReadRepoFileHead(repoPath, path string) (string, error) {
	out, err := exec.Command("git", "-C", repoPath, "show", "HEAD:"+path).Output()
	if err != nil {
		// Not in HEAD (new/untracked/added) -> empty original for the diff.
		return "", nil
	}
	return string(out), nil
}

func (a *App) SaveRepoFile(repoPath, path, content string) error {
	full, err := safeJoin(repoPath, path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}
