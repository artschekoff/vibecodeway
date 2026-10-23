package taskgraph

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/artschekoff/vibecodeway/internal/tracker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider: JH-1 (my subtask) → JH-10 (story) → JH-100 (epic);
// JH-1 is blocked by JH-2 (open); JH-3 is mine and free.
type fakeProvider struct{ details, fetches int }

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Mine(context.Context) ([]tracker.Issue, error) {
	return []tracker.Issue{
		{Key: "JH-1", Summary: "sub", Status: "To Do", Type: "Subtask", Priority: "High", PriorityRank: 4},
		{Key: "JH-3", Summary: "free", Status: "To Do", Type: "Task", Priority: "Highest", PriorityRank: 5},
	}, nil
}
func (f *fakeProvider) Fetch(_ context.Context, keys []string) ([]tracker.Issue, error) {
	f.fetches++
	var out []tracker.Issue
	for _, k := range keys {
		switch k {
		case "JH-10":
			out = append(out, tracker.Issue{Key: k, Summary: "story", Status: "In Progress", Type: "Story"})
		case "JH-100":
			out = append(out, tracker.Issue{Key: k, Summary: "epic", Status: "In Progress", Type: "Epic"})
		case "JH-2":
			out = append(out, tracker.Issue{Key: k, Summary: "blocker", Status: "In Progress", Type: "Task"})
		}
	}
	return out, nil
}
func (f *fakeProvider) Detail(_ context.Context, key string) (*tracker.Issue, error) {
	f.details++
	switch key {
	case "JH-1":
		return &tracker.Issue{Key: key, Summary: "sub", Status: "To Do", Type: "Subtask", Priority: "High", PriorityRank: 4,
			ParentKey: "JH-10", ParentType: "Story",
			Links: []tracker.Link{{Kind: tracker.LinkBlockedBy, Key: "JH-2", Status: "In Progress", Summary: "blocker"}}}, nil
	case "JH-3":
		return &tracker.Issue{Key: key, Summary: "free", Status: "To Do", Type: "Task", Priority: "Highest", PriorityRank: 5}, nil
	case "JH-10":
		return &tracker.Issue{Key: key, Summary: "story", Status: "In Progress", Type: "Story", ParentKey: "JH-100", ParentType: "Epic",
			Subtasks: []tracker.Link{{Key: "JH-1", Status: "To Do"}}}, nil
	}
	return nil, assert.AnError
}
func (f *fakeProvider) IssueURL(_ context.Context, key string) string { return "https://x/" + key }
func (f *fakeProvider) IsEpic(t string) bool                          { return t == "Epic" }

func TestRefreshClimbsToEpicAndMarksBlocked(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	defer s.Close() //nolint:errcheck
	p := &fakeProvider{}
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	r := &Refresher{Store: s, Workers: 2, MineTTL: time.Hour, OtherTTL: 24 * time.Hour, Now: func() time.Time { return now }}
	ctx := context.Background()

	require.NoError(t, r.Refresh(ctx, p, false))
	nodes, edges, err := s.ListTaskGraph(ctx, "fake")
	require.NoError(t, err)
	gn, ge := FromStore(nodes, edges, p.IsEpic)
	g := Build("fake", gn, ge)

	keys := map[string]Node{}
	for _, n := range g.Nodes {
		keys[n.Key] = n
	}
	assert.True(t, keys["JH-100"].Epic, "story's parent resolved to the epic")
	assert.Equal(t, "In Progress", keys["JH-100"].Status, "neighbour summaries refreshed via Fetch")
	assert.True(t, keys["JH-1"].Blocked)
	assert.Equal(t, []string{"JH-3"}, g.Ready)
	assert.Equal(t, []string{"JH-1"}, g.Waiting)
	assert.Equal(t, 3, p.details, "JH-1, JH-3 and the story; the epic is never detailed")

	// Second refresh within TTL: no new detail calls.
	require.NoError(t, r.Refresh(ctx, p, false))
	assert.Equal(t, 3, p.details)
	// Forced: every detail again — my two issues plus the story.
	require.NoError(t, r.Refresh(ctx, p, true))
	assert.Equal(t, 6, p.details)
}
