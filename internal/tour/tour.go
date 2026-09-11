// Package tour implements the file-contract side of the code tour feature:
// reading, validating, freshness-checking, and consuming the
// `.vibecodeway/tour.json` file an agent may leave behind after finishing an
// implementation. See docs/superpowers/specs/2026-09-11-code-tour-design.md
// §2 for the authoritative schema, freshness rule, and error semantics.
package tour

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
)

// ErrInvalid is wrapped into the error returned by Load when tour.json exists
// but fails schema validation (wrong version, no steps, or a step missing a
// required field). Callers use errors.Is(err, ErrInvalid) to distinguish
// this from other I/O errors and fall back to generation per spec §5.
var ErrInvalid = errors.New("invalid tour file")

// contractRelPath is the file contract's location relative to the repo root.
const contractRelPath = ".vibecodeway/tour.json"

// File is the parsed contents of tour.json.
type File struct {
	Version int                 `json:"version"`
	Title   string              `json:"title"`
	Steps   []appstore.TourStep `json:"steps"`
}

// contractPath returns the absolute path to the tour contract file for
// repoPath.
func contractPath(repoPath string) string {
	return filepath.Join(repoPath, contractRelPath)
}

// Load reads and validates the tour contract file at
// <repoPath>/.vibecodeway/tour.json.
//
// It returns (nil, nil) when the file does not exist — that is the normal
// "no contract left behind" case, not an error. A parse failure or schema
// validation failure is returned as an error wrapping ErrInvalid so the
// caller can distinguish "no file" from "invalid file" and fall back to
// generation in both non-existence and invalidity cases per spec §5.
func Load(repoPath string) (*File, error) {
	raw, err := os.ReadFile(contractPath(repoPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read tour contract: %w", err)
	}

	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse tour contract: %w: %w", ErrInvalid, err)
	}

	if err := validate(&f); err != nil {
		return nil, err
	}

	return &f, nil
}

// validate applies the schema rules from spec §2: version must be 1, there
// must be at least one step, and every step must carry a non-empty file,
// anchor, and what (why may be empty).
func validate(f *File) error {
	if f.Version != 1 {
		return fmt.Errorf("%w: unsupported version %d", ErrInvalid, f.Version)
	}
	if len(f.Steps) == 0 {
		return fmt.Errorf("%w: no steps", ErrInvalid)
	}
	for i, step := range f.Steps {
		if strings.TrimSpace(step.File) == "" {
			return fmt.Errorf("%w: step %d missing file", ErrInvalid, i)
		}
		if strings.TrimSpace(step.Anchor) == "" {
			return fmt.Errorf("%w: step %d missing anchor", ErrInvalid, i)
		}
		if strings.TrimSpace(step.What) == "" {
			return fmt.Errorf("%w: step %d missing what", ErrInvalid, i)
		}
	}
	return nil
}

// Fresh reports whether the tour contract file at repoPath is fresh: its
// mtime must be newer than the repository's last commit time (spec §2), so a
// stale tour left over from a previous task never resurfaces. A repository
// with zero commits (git log has nothing to compare against) counts as
// fresh.
func Fresh(repoPath string, fileModTime time.Time) (bool, error) {
	commitTime, ok, err := lastCommitTime(repoPath)
	if err != nil {
		return false, fmt.Errorf("determine last commit time: %w", err)
	}
	if !ok {
		// No commits yet: nothing to be stale relative to.
		return true, nil
	}
	return fileModTime.After(commitTime), nil
}

// lastCommitTime runs `git log -1 --format=%cI` in repoPath and returns the
// last commit's committer time. ok is false when the repository has no
// commits yet (empty output, not an error).
func lastCommitTime(repoPath string) (t time.Time, ok bool, err error) {
	out, err := exec.Command("git", "-C", repoPath, "log", "-1", "--format=%cI").Output()
	if err != nil {
		// A brand-new repo with no commits exits non-zero here ("does not
		// have any commits yet"); treat that as "no commits" rather than a
		// hard error, matching the spec's "zero commits counts as fresh".
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("git log in %s: %w", repoPath, err)
	}

	str := strings.TrimSpace(string(out))
	if str == "" {
		return time.Time{}, false, nil
	}

	parsed, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse commit time %q: %w", str, err)
	}
	return parsed, true, nil
}

// Consume deletes the tour contract file after a successful import, per
// spec §2 ("After successful import the file is deleted"). A missing file
// is not an error, so Consume is idempotent.
func Consume(repoPath string) error {
	err := os.Remove(contractPath(repoPath))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove tour contract: %w", err)
	}
	return nil
}
