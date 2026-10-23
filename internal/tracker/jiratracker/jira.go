// Package jiratracker adapts the Jira context service to tracker.Provider.
package jiratracker

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/artschekoff/vibecodeway/internal/jira"
	jiraservice "github.com/artschekoff/vibecodeway/internal/jira/service"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/artschekoff/vibecodeway/internal/tracker"
)

// Name is the registry slug.
const Name = "jira"

// Tracker is the Jira implementation of tracker.Provider.
type Tracker struct {
	svc *jiraservice.ContextService

	baseMu sync.Mutex
	base   string // instance URL, resolved once (acli auth status is a subprocess)
}

// New wraps svc.
func New(svc *jiraservice.ContextService) *Tracker { return &Tracker{svc: svc} }

func (t *Tracker) Name() string { return Name }

// Mine: open issues assigned to me. Done work is noise on the graph and would
// multiply the per-issue detail calls.
func (t *Tracker) Mine(ctx context.Context) ([]tracker.Issue, error) {
	rs, err := t.svc.SearchJQL(ctx, "assignee = currentUser() AND statusCategory != Done ORDER BY priority DESC, created DESC")
	if err != nil {
		return nil, err
	}
	return fromSearch(rs), nil
}

func (t *Tracker) Fetch(ctx context.Context, keys []string) ([]tracker.Issue, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	rs, err := t.svc.SearchJQL(ctx, fmt.Sprintf("key in (%s)", strings.Join(keys, ", ")))
	if err != nil {
		return nil, err
	}
	return fromSearch(rs), nil
}

func (t *Tracker) Detail(ctx context.Context, key string) (*tracker.Issue, error) {
	d, err := t.svc.ViewIssue(ctx, key)
	if err != nil {
		return nil, err
	}
	is := &tracker.Issue{
		Key: d.Key, Summary: d.Summary, Status: d.Status, Type: d.IssueType,
		Priority: d.Priority, PriorityRank: PriorityRank(d.Priority), Assignee: d.Assignee,
		ParentKey: d.ParentKey, ParentType: d.ParentType,
		Terminal: appstore.IsTerminalTicketStatus(d.Status),
	}
	for _, l := range d.Links {
		is.Links = append(is.Links, tracker.Link{
			Kind: linkKind(l), Key: l.LinkedKey, Summary: l.Summary, Status: l.Status, Type: l.IssueType,
		})
	}
	for _, s := range d.Subtasks {
		is.Subtasks = append(is.Subtasks, tracker.Link{Key: s.Key, Summary: s.Summary, Status: s.Status})
	}
	return is, nil
}

// IssueURL builds the web link. The instance URL comes from `acli jira auth
// status` — one subprocess — so it is resolved once per Tracker, not once per
// node (a 120-node graph used to spawn acli 120 times per GetTaskGraph).
func (t *Tracker) IssueURL(ctx context.Context, key string) string {
	t.baseMu.Lock()
	defer t.baseMu.Unlock()
	if t.base == "" {
		t.base = strings.TrimRight(t.svc.GetJiraInstanceURL(ctx), "/")
	}
	if t.base == "" {
		return ""
	}
	return t.base + "/browse/" + key
}

func (t *Tracker) IsEpic(issueType string) bool { return strings.EqualFold(issueType, "Epic") }

// linkKind maps Jira link types to the neutral kinds. Any type whose name
// contains "block" (Blocks, Blocked, Dependency-style custom types) is a
// blocking relation; outward = this issue blocks the other, inward = the
// other blocks this issue.
func linkKind(l jira.IssueLink) tracker.LinkKind {
	if !strings.Contains(strings.ToLower(l.TypeName), "block") {
		return tracker.LinkRelates
	}
	if l.Direction == "inward" {
		return tracker.LinkBlockedBy
	}
	return tracker.LinkBlocks
}

// PriorityRank normalises Jira's default priority scheme (5 highest … 1).
func PriorityRank(p string) int {
	switch strings.ToLower(p) {
	case "highest", "blocker":
		return 5
	case "high", "critical":
		return 4
	case "medium", "major":
		return 3
	case "low", "minor":
		return 2
	case "lowest", "trivial":
		return 1
	}
	return 0
}

func fromSearch(rs []jira.SearchResult) []tracker.Issue {
	out := make([]tracker.Issue, 0, len(rs))
	for _, r := range rs {
		out = append(out, tracker.Issue{
			Key: r.Key, Summary: r.Summary, Status: r.Status, Type: r.IssueType,
			Priority: r.Priority, PriorityRank: PriorityRank(r.Priority), Assignee: r.Assignee,
			Terminal: appstore.IsTerminalTicketStatus(r.Status),
		})
	}
	return out
}
