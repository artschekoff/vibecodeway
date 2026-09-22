package symbols

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Symbol is one ctags tag. Path is relative to the worktree root.
type Symbol struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Language string `json:"language"`
	Line     int    `json:"line"`
}

// indexTTL bounds staleness: a lookup on an entry older than this rebuilds
// first. var (not const) so tests can shrink it.
var indexTTL = 5 * time.Minute

type indexEntry struct {
	symbols []Symbol
	byName  map[string][]int // exact-case name -> indexes into symbols
	builtAt time.Time
}

// Service builds and serves per-worktree symbol indexes. Safe for concurrent
// use; the ctags subprocess runs outside the lock.
type Service struct {
	logger    *zap.Logger
	findCtags func() (string, error)

	mu    sync.Mutex
	cache map[string]*indexEntry
}

func NewService(logger *zap.Logger) *Service {
	return &Service{logger: logger, findCtags: DefaultFindCtags, cache: map[string]*indexEntry{}}
}

// putForTest seeds the cache without running ctags. Test-only by convention.
func (s *Service) putForTest(worktree string, syms []Symbol) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[worktree] = newEntry(syms)
}

func newEntry(syms []Symbol) *indexEntry {
	byName := make(map[string][]int, len(syms))
	for i, sym := range syms {
		byName[sym.Name] = append(byName[sym.Name], i)
	}
	return &indexEntry{symbols: syms, byName: byName, builtAt: time.Now()}
}

// parseCtagsJSON converts `--output-format=json` lines to Symbols, skipping
// pseudo-tags and unparsable lines (ctags emits one JSON object per line).
func parseCtagsJSON(out []byte) []Symbol {
	var syms []Symbol
	for _, line := range bytes.Split(out, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var row struct {
			Type     string `json:"_type"`
			Name     string `json:"name"`
			Kind     string `json:"kind"`
			Path     string `json:"path"`
			Language string `json:"language"`
			Line     int    `json:"line"`
		}
		if err := json.Unmarshal(line, &row); err != nil || row.Type != "tag" {
			continue
		}
		syms = append(syms, Symbol{Name: row.Name, Kind: row.Kind, Path: row.Path, Language: row.Language, Line: row.Line})
	}
	return syms
}

// build runs git ls-files | ctags over the worktree and returns the parsed
// symbols. git ls-files respects .gitignore, so node_modules etc. never reach
// ctags.
func (s *Service) build(worktree string) ([]Symbol, error) {
	bin, err := s.findCtags()
	if err != nil {
		return nil, err
	}
	files, err := exec.Command("git", "-C", worktree, "ls-files").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %w", worktree, err)
	}
	cmd := exec.Command(bin, "--output-format=json", "--fields=+nl", "-L", "-")
	cmd.Dir = worktree
	cmd.Stdin = bytes.NewReader(files)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// ctags exits non-zero on some unreadable files but still emits tags;
		// only fail when nothing was produced.
		if stdout.Len() == 0 {
			return nil, fmt.Errorf("ctags: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		s.logger.Warn("symbols: ctags finished with errors",
			zap.String("worktree", worktree), zap.String("stderr", stderr.String()), zap.Error(err))
	}
	return parseCtagsJSON(stdout.Bytes()), nil
}

// Index force-rebuilds the worktree's index.
func (s *Service) Index(worktree string) (int, int, error) {
	start := time.Now()
	syms, err := s.build(worktree)
	if err != nil {
		return 0, 0, err
	}
	s.mu.Lock()
	s.cache[worktree] = newEntry(syms)
	s.mu.Unlock()
	return len(syms), int(time.Since(start).Milliseconds()), nil
}

// entryFor returns a fresh-enough index, lazily building one when missing or
// older than indexTTL.
func (s *Service) entryFor(worktree string) (*indexEntry, error) {
	s.mu.Lock()
	e, ok := s.cache[worktree]
	s.mu.Unlock()
	if ok && time.Since(e.builtAt) < indexTTL {
		return e, nil
	}
	if _, _, err := s.Index(worktree); err != nil {
		// A stale entry beats a hard failure (e.g. ctags uninstalled mid-run).
		if ok {
			s.logger.Warn("symbols: index rebuild failed, serving stale entry",
				zap.String("worktree", worktree), zap.Error(err))
			return e, nil
		}
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cache[worktree], nil
}

// Lookup returns exact-case matches for name, falling back to
// case-insensitive matches when there are none.
func (s *Service) Lookup(worktree, name string) ([]Symbol, error) {
	e, err := s.entryFor(worktree)
	if err != nil {
		return nil, err
	}
	if idxs := e.byName[name]; len(idxs) > 0 {
		out := make([]Symbol, 0, len(idxs))
		for _, i := range idxs {
			out = append(out, e.symbols[i])
		}
		return out, nil
	}
	var out []Symbol
	for _, sym := range e.symbols {
		if strings.EqualFold(sym.Name, name) {
			out = append(out, sym)
		}
	}
	return out, nil
}

// Search returns case-insensitive substring matches ranked prefix-first, then
// by name length, then alphabetically for stability.
func (s *Service) Search(worktree, query string, limit int) ([]Symbol, error) {
	e, err := s.entryFor(worktree)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	q := strings.ToLower(query)
	type ranked struct {
		sym    Symbol
		prefix bool
	}
	var hits []ranked
	for _, sym := range e.symbols {
		low := strings.ToLower(sym.Name)
		if !strings.Contains(low, q) {
			continue
		}
		hits = append(hits, ranked{sym: sym, prefix: strings.HasPrefix(low, q)})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].prefix != hits[j].prefix {
			return hits[i].prefix
		}
		if len(hits[i].sym.Name) != len(hits[j].sym.Name) {
			return len(hits[i].sym.Name) < len(hits[j].sym.Name)
		}
		return hits[i].sym.Name < hits[j].sym.Name
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]Symbol, len(hits))
	for i, h := range hits {
		out[i] = h.sym
	}
	return out, nil
}
