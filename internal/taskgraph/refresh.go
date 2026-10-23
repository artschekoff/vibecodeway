package taskgraph

import (
	"context"
	"sync"
	"time"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/artschekoff/vibecodeway/internal/tracker"
)

// Store is the cache the refresher writes to (implemented by appstore.Store).
type Store interface {
	ResetTaskMine(ctx context.Context, provider string) error
	UpsertTaskNodes(ctx context.Context, provider string, nodes []appstore.TaskNode) error
	ReplaceTaskEdges(ctx context.Context, provider, owner string, edges []appstore.TaskEdge) error
	ListTaskGraph(ctx context.Context, provider string) ([]appstore.TaskNode, []appstore.TaskEdge, error)
	PruneTaskNodes(ctx context.Context, provider string) error
}

// Refresher pulls my issues from a provider, walks their parents up to the
// epic and their links, and caches everything in Store. Detail fetches are the
// expensive part (one call per issue), so they are rate-limited by TTL and
// run on a small worker pool.
type Refresher struct {
	Store    Store
	Workers  int
	MineTTL  time.Duration // re-fetch detail of my issues after this
	OtherTTL time.Duration // … of parents/blockers after this
	Progress func(current, total int, key string)
	Now      func() time.Time
	// Logf receives per-issue failures (a transient acli error must not abort
	// the whole refresh).
	Logf func(format string, args ...any)
}

const maxParentHops = 4

// Refresh updates the cache for p. force re-fetches every detail regardless of TTL.
func (r *Refresher) Refresh(ctx context.Context, p tracker.Provider, force bool) error {
	now := r.now()
	prov := p.Name()
	mine, err := p.Mine(ctx)
	if err != nil {
		return err
	}
	if err := r.Store.ResetTaskMine(ctx, prov); err != nil {
		return err
	}
	nodes := make([]appstore.TaskNode, 0, len(mine))
	for _, is := range mine {
		n := toNode(is)
		n.Mine, n.FetchedAt = true, stamp(now)
		nodes = append(nodes, n)
	}
	if err := r.Store.UpsertTaskNodes(ctx, prov, nodes); err != nil {
		return err
	}

	// Walk: my issues → their parents → … until every chain ends in an epic
	// (or a parentless issue). Each hop only fetches what the TTL allows.
	for hop := 0; hop < maxParentHops; hop++ {
		all, edges, err := r.Store.ListTaskGraph(ctx, prov)
		if err != nil {
			return err
		}
		need := r.needDetail(all, edges, p, force && hop == 0, now)
		if len(need) == 0 {
			break
		}
		if err := r.fetchDetails(ctx, p, need, now); err != nil {
			return err
		}
	}

	// Refresh the summary (status!) of every node that isn't mine so a blocker
	// closing or an epic moving shows up without a detail call.
	all, _, err := r.Store.ListTaskGraph(ctx, prov)
	if err != nil {
		return err
	}
	var others []string
	for _, n := range all {
		if !n.Mine {
			others = append(others, n.Key)
		}
	}
	for i := 0; i < len(others); i += 50 {
		chunk := others[i:min(i+50, len(others))]
		fetched, err := p.Fetch(ctx, chunk)
		if err != nil {
			r.logf("task graph: fetch %d neighbours failed: %v", len(chunk), err)
			continue
		}
		upd := make([]appstore.TaskNode, 0, len(fetched))
		for _, is := range fetched {
			n := toNode(is)
			n.FetchedAt = stamp(now)
			upd = append(upd, n)
		}
		if err := r.Store.UpsertTaskNodes(ctx, prov, upd); err != nil {
			return err
		}
	}
	return r.Store.PruneTaskNodes(ctx, prov)
}

// needDetail picks the keys whose detail is missing or stale: my open issues,
// plus non-epic parents (to keep climbing towards the epic).
func (r *Refresher) needDetail(nodes []appstore.TaskNode, edges []appstore.TaskEdge, p tracker.Provider, force bool, now time.Time) []string {
	isParent := map[string]bool{}
	for _, e := range edges {
		if e.Kind == "parent" {
			isParent[e.Dst] = true
		}
	}
	var need []string
	for _, n := range nodes {
		if n.Terminal || p.IsEpic(n.IssueType) {
			continue
		}
		var ttl time.Duration
		switch {
		case n.Mine:
			ttl = r.MineTTL
		case isParent[n.Key]:
			ttl = r.OtherTTL
		default:
			continue
		}
		if force || n.DetailAt == "" || stale(n.DetailAt, ttl, now) {
			need = append(need, n.Key)
		}
	}
	return need
}

func (r *Refresher) fetchDetails(ctx context.Context, p tracker.Provider, keys []string, now time.Time) error {
	prov := p.Name()
	workers := r.Workers
	if workers <= 0 {
		workers = 1
	}
	var (
		wg      sync.WaitGroup
		storeMu sync.Mutex // SQLite: serialise writes from the pool
		done    int
		doneMu  sync.Mutex
		firstEr error
	)
	jobs := make(chan string)
	for i := 0; i < min(workers, len(keys)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range jobs {
				is, err := p.Detail(ctx, key)
				doneMu.Lock()
				done++
				cur := done
				doneMu.Unlock()
				if r.Progress != nil {
					r.Progress(cur, len(keys), key)
				}
				if err != nil {
					r.logf("task graph: detail %s failed: %v", key, err)
					continue
				}
				nodes, edges := explode(is, now)
				storeMu.Lock()
				if err := r.Store.UpsertTaskNodes(ctx, prov, nodes); err == nil {
					err = r.Store.ReplaceTaskEdges(ctx, prov, key, edges)
				} else if firstEr == nil {
					firstEr = err
				}
				storeMu.Unlock()
			}
		}()
	}
	for _, k := range keys {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case jobs <- k:
		}
	}
	close(jobs)
	wg.Wait()
	return firstEr
}

// explode turns one detailed issue into the nodes it mentions (itself, its
// parent, link targets, subtasks — the others as stubs) and the edges it owns.
func explode(is *tracker.Issue, now time.Time) ([]appstore.TaskNode, []appstore.TaskEdge) {
	self := toNode(*is)
	self.FetchedAt, self.DetailAt = stamp(now), stamp(now)
	nodes := []appstore.TaskNode{self}
	var edges []appstore.TaskEdge
	if is.ParentKey != "" {
		nodes = append(nodes, appstore.TaskNode{Key: is.ParentKey, IssueType: is.ParentType})
		edges = append(edges, appstore.TaskEdge{Src: is.Key, Dst: is.ParentKey, Kind: "parent"})
	}
	for _, l := range is.Links {
		nodes = append(nodes, appstore.TaskNode{
			Key: l.Key, Summary: l.Summary, Status: l.Status, IssueType: l.Type,
			Terminal: appstore.IsTerminalTicketStatus(l.Status),
		})
		switch l.Kind {
		case tracker.LinkBlocks:
			edges = append(edges, appstore.TaskEdge{Src: is.Key, Dst: l.Key, Kind: "blocks"})
		case tracker.LinkBlockedBy:
			edges = append(edges, appstore.TaskEdge{Src: l.Key, Dst: is.Key, Kind: "blocks"})
		default:
			edges = append(edges, appstore.TaskEdge{Src: is.Key, Dst: l.Key, Kind: "relates"})
		}
	}
	for _, s := range is.Subtasks {
		nodes = append(nodes, appstore.TaskNode{
			Key: s.Key, Summary: s.Summary, Status: s.Status,
			Terminal: appstore.IsTerminalTicketStatus(s.Status),
		})
		edges = append(edges, appstore.TaskEdge{Src: s.Key, Dst: is.Key, Kind: "parent"})
	}
	return nodes, edges
}

// FromStore converts cached rows into graph nodes/edges (Epic flag resolved
// through the provider).
func FromStore(nodes []appstore.TaskNode, edges []appstore.TaskEdge, isEpic func(string) bool) ([]Node, []Edge) {
	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, Node{
			Key: n.Key, Summary: n.Summary, Status: n.Status, Type: n.IssueType,
			Priority: n.Priority, PriorityRank: n.PriorityRank, Assignee: n.Assignee,
			ParentKey: n.ParentKey, Mine: n.Mine, Terminal: n.Terminal, Epic: isEpic(n.IssueType),
		})
	}
	es := make([]Edge, 0, len(edges))
	for _, e := range edges {
		es = append(es, Edge{Src: e.Src, Dst: e.Dst, Kind: e.Kind})
	}
	return out, es
}

func toNode(is tracker.Issue) appstore.TaskNode {
	return appstore.TaskNode{
		Key: is.Key, Summary: is.Summary, Status: is.Status, IssueType: is.Type,
		Priority: is.Priority, PriorityRank: is.PriorityRank, Assignee: is.Assignee,
		ParentKey: is.ParentKey, Terminal: is.Terminal,
	}
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func stale(at string, ttl time.Duration, now time.Time) bool {
	t, err := time.Parse(time.RFC3339, at)
	return err != nil || now.Sub(t) > ttl
}

func (r *Refresher) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Refresher) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
	}
}
