// Package repofiles provides bounded, path-safe reads of a repo checkout for
// injecting code context into AI prompts. All reads are confined to a root dir.
package repofiles

import (
	"fmt"
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

// ReadFiles reads each requested rel path (must be a regular file inside root),
// truncating per maxFileBytes and stopping additions once maxTotalBytes is hit.
func ReadFiles(root string, rels []string) (map[string]string, error) {
	out := make(map[string]string, len(rels))
	total := 0
	for _, rel := range rels {
		abs, err := safeJoin(root, rel)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(abs)
		if err != nil || !info.Mode().IsRegular() {
			continue // missing/non-regular: skip silently, not an error
		}
		if total >= maxTotalBytes {
			break
		}
		b, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		s := string(b)
		if len(s) > maxFileBytes {
			s = s[:maxFileBytes] + "…[truncated]"
		}
		out[filepath.ToSlash(rel)] = s
		total += len(s)
	}
	return out, nil
}
