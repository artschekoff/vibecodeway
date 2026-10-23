// Package tracker is the provider-neutral view of an issue tracker (Jira today,
// Linear or others later). The task graph, ranking and board code only talk to
// Provider; each backend lives in its own sub-package and registers itself.
package tracker

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// LinkKind is the relation between an issue and a linked key, from the issue's
// point of view.
type LinkKind string

const (
	LinkBlocks    LinkKind = "blocks"     // this issue blocks Key
	LinkBlockedBy LinkKind = "blocked_by" // Key blocks this issue
	LinkRelates   LinkKind = "relates"    // any other link type
)

// Link is one issue link with whatever the provider already knows about the
// other side (saves a fetch for the common "is my blocker done?" question).
type Link struct {
	Kind    LinkKind `json:"kind"`
	Key     string   `json:"key"`
	Summary string   `json:"summary"`
	Status  string   `json:"status"`
	Type    string   `json:"type"`
}

// Issue is a tracker item. Mine/Fetch fill the summary fields; Detail adds
// parent, links and subtasks.
type Issue struct {
	Key      string `json:"key"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Priority string `json:"priority"`
	// PriorityRank normalises provider priorities for colouring/sorting:
	// 5 highest … 1 lowest, 0 unknown.
	PriorityRank int    `json:"priorityRank"`
	Assignee     string `json:"assignee"`
	ParentKey    string `json:"parentKey"`
	ParentType   string `json:"parentType"`
	Terminal     bool   `json:"terminal"`
	Links        []Link `json:"links,omitempty"`
	Subtasks     []Link `json:"subtasks,omitempty"`
}

// Provider is what a tracker backend must implement for the task graph.
type Provider interface {
	// Name is the registry slug, e.g. "jira", "linear".
	Name() string
	// Mine returns the open issues assigned to the current user (summary fields).
	Mine(ctx context.Context) ([]Issue, error)
	// Fetch returns summary fields for the given keys (batch; unknown keys skipped).
	Fetch(ctx context.Context, keys []string) ([]Issue, error)
	// Detail returns one issue with parent, links and subtasks.
	Detail(ctx context.Context, key string) (*Issue, error)
	// IssueURL is the web link for a key.
	IssueURL(ctx context.Context, key string) string
	// IsEpic reports whether an issue type is the top-level grouping node.
	IsEpic(issueType string) bool
}

var (
	mu       sync.RWMutex
	registry = map[string]Provider{}
)

// Register adds a provider under its Name. Later registrations replace earlier ones.
func Register(p Provider) {
	mu.Lock()
	defer mu.Unlock()
	registry[p.Name()] = p
}

// Get returns the provider registered under name.
func Get(name string) (Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown tracker provider %q", name)
	}
	return p, nil
}

// Names lists registered providers, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
