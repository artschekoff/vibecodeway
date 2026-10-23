package main

import (
	"fmt"
	"time"

	"github.com/artschekoff/vibecodeway/internal/taskgraph"
	"github.com/artschekoff/vibecodeway/internal/tracker"
	"github.com/artschekoff/vibecodeway/internal/tracker/jiratracker"
)

// Task graph: my issues from a tracker provider, their epics, blockers and
// contexts, as the Board's graph view. Provider-neutral via internal/tracker.

const (
	taskGraphWorkers  = 4
	taskGraphMineTTL  = time.Hour
	taskGraphOtherTTL = 24 * time.Hour
)

// ListTaskProviders returns the registered tracker providers ("jira", …).
func (a *App) ListTaskProviders() []string { return tracker.Names() }

// RefreshTaskGraph pulls fresh data from provider (force ignores the detail
// TTLs) and returns the rebuilt graph. Emits "taskgraph:progress"
// {current,total,key} for each per-issue detail fetch.
func (a *App) RefreshTaskGraph(provider string, force bool) (*taskgraph.Graph, error) {
	p, err := tracker.Get(provider)
	if err != nil {
		return nil, err
	}
	r := &taskgraph.Refresher{
		Store: a.db, Workers: taskGraphWorkers, MineTTL: taskGraphMineTTL, OtherTTL: taskGraphOtherTTL,
		Logf: func(format string, args ...any) { a.logger.Sugar().Warnf(format, args...) },
		Progress: func(current, total int, key string) {
			if a.emit != nil {
				a.emit("taskgraph:progress", map[string]any{"current": current, "total": total, "key": key})
			}
		},
	}
	if err := r.Refresh(a.ctx, p, force); err != nil {
		return nil, fmt.Errorf("refresh task graph: %w", err)
	}
	return a.GetTaskGraph(provider)
}

// GetTaskGraph returns the cached graph with blocked/ready/waiting computed
// and in-app contexts attached.
func (a *App) GetTaskGraph(provider string) (*taskgraph.Graph, error) {
	p, err := tracker.Get(provider)
	if err != nil {
		return nil, err
	}
	rows, edgeRows, err := a.db.ListTaskGraph(a.ctx, provider)
	if err != nil {
		return nil, err
	}
	nodes, edges := taskgraph.FromStore(rows, edgeRows, p.IsEpic)
	keys := make([]string, 0, len(nodes))
	for _, n := range nodes {
		keys = append(keys, n.Key)
	}
	// Context linkage is per provider table; Jira is the only one today.
	contexts := map[string]struct{ ID, Key string }{}
	if provider == jiratracker.Name {
		refs, err := a.db.FindActiveContextsByJiraKeys(a.ctx, keys)
		if err != nil {
			return nil, err
		}
		for k, r := range refs {
			contexts[k] = struct{ ID, Key string }{r.ID, r.Key}
		}
	}
	for i := range nodes {
		if c, ok := contexts[nodes[i].Key]; ok {
			nodes[i].ContextID, nodes[i].ContextKey = c.ID, c.Key
		}
		nodes[i].URL = p.IssueURL(a.ctx, nodes[i].Key)
	}
	g := taskgraph.Build(provider, nodes, edges)
	return &g, nil
}
