package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// safeJoin resolves path relative to repoPath and guarantees the result stays
// inside repoPath, even in the presence of symlinks. It first rejects any
// lexical escape (e.g. "..") before repoPath is resolved on disk, then walks
// up from the target to the nearest ancestor that actually exists and
// verifies that ancestor's canonical (symlink-resolved) location is still
// within the canonical repo root. This handles the case where an intermediate
// path component is a symlink pointing outside the repo (e.g. a symlinked
// directory), while still allowing legitimate new files/directories that
// don't exist on disk yet (SaveRepoFile creates new files).
func safeJoin(repoPath, path string) (string, error) {
	full := filepath.Clean(filepath.Join(repoPath, path))
	rel, err := filepath.Rel(repoPath, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes repository")
	}

	root, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		return "", fmt.Errorf("resolve repo root: %w", err)
	}

	// Walk up from full to the nearest ancestor that exists on disk, then
	// resolve symlinks on that ancestor and confirm it is still within root.
	ancestor := full
	for {
		if _, statErr := os.Lstat(ancestor); statErr == nil {
			break
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			// Reached filesystem root without finding an existing ancestor.
			break
		}
		ancestor = parent
	}

	realAncestor, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", fmt.Errorf("resolve ancestor path: %w", err)
	}

	relAncestor, err := filepath.Rel(root, realAncestor)
	if err != nil || relAncestor == ".." || strings.HasPrefix(relAncestor, ".."+string(filepath.Separator)) {
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
	// Route path through the same containment guard as ReadRepoFile/SaveRepoFile
	// for consistency, even though this only shells out to `git show` and never
	// touches the filesystem directly. An escaping path (via ".." or a symlink)
	// is treated the same as a path that simply isn't present at HEAD: we return
	// ("", nil) rather than an error, preserving the existing "not readable at
	// HEAD" contract that callers (the diff view) already rely on for
	// new/untracked files.
	if _, err := safeJoin(repoPath, path); err != nil {
		return "", nil
	}
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
