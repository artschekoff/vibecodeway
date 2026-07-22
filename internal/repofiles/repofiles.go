// Package repofiles provides bounded, path-safe reads of a repo checkout for
// injecting code context into AI prompts. All reads are confined to a root dir.
package repofiles

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxTreeEntries = 2000
	maxFileBytes   = 20000
	maxTotalBytes  = 120000
)

var ignoredDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, ".next": true, "target": true, ".venv": true, "__pycache__": true,
}

// FileTree returns sorted repo-root-relative regular-file paths, skipping ignored
// directories, capped at maxTreeEntries.
func FileTree(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries rather than aborting
		}
		if d.IsDir() {
			if ignoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil // skip symlinks entirely: never list them, never descend
		}
		rel, rErr := filepath.Rel(root, p)
		if rErr != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking repo: %w", err)
	}
	sort.Strings(out)
	if len(out) > maxTreeEntries {
		out = out[:maxTreeEntries]
	}
	return out, nil
}

// safeJoin resolves rel against root and guarantees the result stays inside root.
// This is a purely lexical check (filepath.Clean); it does not resolve symlinks.
// Callers that read file contents must additionally verify the resolved
// (symlink-free) path stays inside root — see ReadFiles.
func safeJoin(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path not allowed: %q", rel)
	}
	clean := filepath.Clean(filepath.Join(root, rel))
	rootClean := filepath.Clean(root) + string(os.PathSeparator)
	if clean != filepath.Clean(root) && !strings.HasPrefix(clean+string(os.PathSeparator), rootClean) {
		return "", fmt.Errorf("path escapes repo root: %q", rel)
	}
	return clean, nil
}

// withinResolvedRoot reports whether resolved is rootResolved itself or lives
// underneath it, using the same separator-suffixed prefix check as safeJoin.
func withinResolvedRoot(resolved, rootResolved string) bool {
	if resolved == rootResolved {
		return true
	}
	rootPrefixed := rootResolved + string(os.PathSeparator)
	return strings.HasPrefix(resolved+string(os.PathSeparator), rootPrefixed)
}

// ReadFiles reads each requested rel path (must be a regular file inside root),
// truncating per maxFileBytes and stopping additions once maxTotalBytes is hit.
//
// safeJoin only rejects lexical traversal (".."/absolute paths); it does not
// resolve symlinks. A symlink committed inside root (or an intermediate
// symlinked directory) could otherwise point outside root with no ".." in its
// name. To close that hole, every resolved path is re-verified against the
// resolved root before it is read.
func ReadFiles(root string, rels []string) (map[string]string, error) {
	rootResolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolving repo root: %w", err)
	}
	rootResolved = filepath.Clean(rootResolved)

	out := make(map[string]string, len(rels))
	total := 0
	for _, rel := range rels {
		abs, err := safeJoin(root, rel)
		if err != nil {
			return nil, err
		}

		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			continue // missing/broken symlink or path: skip silently, not an error
		}
		resolved = filepath.Clean(resolved)
		if !withinResolvedRoot(resolved, rootResolved) {
			// A symlink (direct or via an intermediate component) escaped
			// root. Unlike a missing file, this is an observable attack
			// attempt, so it is a hard error, consistent with lexical
			// traversal/absolute-path rejection in safeJoin.
			return nil, fmt.Errorf("path escapes repo root via symlink: %q", rel)
		}

		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			continue // missing/non-regular: skip silently, not an error
		}
		if total >= maxTotalBytes {
			break
		}

		s, err := readBounded(resolved, maxFileBytes)
		if err != nil {
			continue
		}
		out[filepath.ToSlash(rel)] = s
		total += len(s)
	}
	return out, nil
}

// readBounded reads at most limit+1 bytes from path, never loading the whole
// file into memory regardless of its actual size. If the read hits the limit,
// the content is trimmed to limit bytes and a truncation marker is appended.
func readBounded(path string, limit int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	b, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return "", err
	}
	if len(b) > limit {
		return string(b[:limit]) + "…[truncated]", nil
	}
	return string(b), nil
}
