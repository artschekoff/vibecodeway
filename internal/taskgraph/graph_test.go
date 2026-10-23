package taskgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildBlockedReadyWaiting(t *testing.T) {
	nodes := []Node{
		{Key: "E-1", Type: "Epic", Epic: true, Mine: true, PriorityRank: 5},
		{Key: "T-1", Mine: true, PriorityRank: 5}, // blocked by open X-1
		{Key: "T-2", Mine: true, PriorityRank: 3}, // blocked by closed X-2 → ready
		{Key: "T-3", Mine: true, PriorityRank: 4},
		{Key: "T-4", Mine: true, PriorityRank: 5, Terminal: true}, // done, ignored
		{Key: "X-1", Status: "In Progress"},
		{Key: "X-2", Status: "Done", Terminal: true},
	}
	edges := []Edge{
		{Src: "X-1", Dst: "T-1", Kind: "blocks"},
		{Src: "X-2", Dst: "T-2", Kind: "blocks"},
		{Src: "T-1", Dst: "E-1", Kind: "parent"},
		{Src: "GHOST", Dst: "T-3", Kind: "blocks"}, // unknown blocker never blocks
	}
	g := Build("jira", nodes, edges)

	assert.Equal(t, []string{"T-3", "T-2"}, g.Ready, "highest unblocked first, epic and done excluded")
	assert.Equal(t, []string{"T-1"}, g.Waiting)
	assert.Equal(t, []string{"X-1"}, g.Nodes[1].BlockedBy)
	assert.False(t, g.Nodes[2].Blocked, "closed blocker releases the task")
}
