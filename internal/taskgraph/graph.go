// Package taskgraph turns cached tracker nodes/edges into the board graph:
// who blocks whom, what is ready to pick up, what is waiting.
package taskgraph

import "sort"

// Node is a graph node as the UI sees it.
type Node struct {
	Key          string `json:"key"`
	Summary      string `json:"summary"`
	Status       string `json:"status"`
	Type         string `json:"type"`
	Priority     string `json:"priority"`
	PriorityRank int    `json:"priorityRank"`
	Assignee     string `json:"assignee"`
	ParentKey    string `json:"parentKey"`
	Mine         bool   `json:"mine"`
	Terminal     bool   `json:"terminal"`
	Epic         bool   `json:"epic"`
	// Blocked: at least one open issue has a "blocks" edge into this node.
	Blocked   bool     `json:"blocked"`
	BlockedBy []string `json:"blockedBy,omitempty"`
	// Linked in-app context, when one exists.
	ContextID  string `json:"contextId,omitempty"`
	ContextKey string `json:"contextKey,omitempty"`
	URL        string `json:"url,omitempty"`
}

// Edge is a directed relation: parent (src's parent is dst), blocks (src
// blocks dst), relates.
type Edge struct {
	Src  string `json:"src"`
	Dst  string `json:"dst"`
	Kind string `json:"kind"`
}

// Graph is the board payload.
type Graph struct {
	Provider string `json:"provider"`
	Nodes    []Node `json:"nodes"`
	Edges    []Edge `json:"edges"`
	// Ready: my open, unblocked, non-epic issues, best first.
	Ready []string `json:"ready"`
	// Waiting: my open issues held back by an open blocker, best first.
	Waiting []string `json:"waiting"`
}

// Build computes Blocked/BlockedBy, Ready and Waiting. Nodes are returned in
// the input order; edges pointing at unknown keys are kept (the UI draws a
// stub) but never block anything.
func Build(provider string, nodes []Node, edges []Edge) Graph {
	byKey := make(map[string]int, len(nodes))
	for i, n := range nodes {
		byKey[n.Key] = i
		nodes[i].Blocked = false
		nodes[i].BlockedBy = nil
	}
	for _, e := range edges {
		if e.Kind != "blocks" {
			continue
		}
		si, ok := byKey[e.Src]
		di, ok2 := byKey[e.Dst]
		if !ok || !ok2 || nodes[si].Terminal {
			continue
		}
		nodes[di].Blocked = true
		nodes[di].BlockedBy = append(nodes[di].BlockedBy, e.Src)
	}
	var ready, waiting []Node
	for _, n := range nodes {
		if !n.Mine || n.Terminal || n.Epic {
			continue
		}
		if n.Blocked {
			waiting = append(waiting, n)
		} else {
			ready = append(ready, n)
		}
	}
	rank := func(a, b Node) bool {
		if a.PriorityRank != b.PriorityRank {
			return a.PriorityRank > b.PriorityRank
		}
		return a.Key < b.Key
	}
	sort.SliceStable(ready, func(i, j int) bool { return rank(ready[i], ready[j]) })
	sort.SliceStable(waiting, func(i, j int) bool { return rank(waiting[i], waiting[j]) })
	g := Graph{Provider: provider, Nodes: nodes, Edges: edges, Ready: []string{}, Waiting: []string{}}
	for _, n := range ready {
		g.Ready = append(g.Ready, n.Key)
	}
	for _, n := range waiting {
		g.Waiting = append(g.Waiting, n.Key)
	}
	return g
}
