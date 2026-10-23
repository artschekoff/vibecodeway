package tooling

import (
	"context"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/vcs"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
)

// fakePR is a test double for PRActions (extended with write methods in Task 8).
type fakePR struct {
	reviewReq  []vcs.PRData
	changesReq []vcs.PRData
	authored   []vcs.PRData
	pr         *vcs.PRData
	comments   []vcs.CommentData
	authUser   string
	unresolved int

	lastLogin     string
	approved      int    // PR number approved
	changesFor    int    // PR number changes-requested
	commented     int    // PR number commented
	lastBody      string // body of the last write action
	getPRCalls    int    // GetPRByNumber invocations (lazy-load cache assertions)
	authoredCalls int    // ListAuthoredPRs invocations
}

func (f *fakePR) ListAuthoredPRs(_ context.Context, _, _, login string) ([]vcs.PRData, error) {
	f.lastLogin = login
	f.authoredCalls++
	return f.authored, nil
}

func (f *fakePR) CountUnresolvedThreads(_ context.Context, _, _ string, _ int) (int, error) {
	return f.unresolved, nil
}

func (f *fakePR) GetAuthUser(_ context.Context) (string, error) { return f.authUser, nil }

func (f *fakePR) ListReviewRequestsForUser(_ context.Context, _, _, login string) ([]vcs.PRData, error) {
	f.lastLogin = login
	return f.reviewReq, nil
}

func (f *fakePR) ListChangesRequestedForUser(_ context.Context, _, _, login string) ([]vcs.PRData, error) {
	f.lastLogin = login
	return f.changesReq, nil
}

func (f *fakePR) GetPRByNumber(_ context.Context, _, _ string, _ int) (*vcs.PRData, error) {
	f.getPRCalls++
	return f.pr, nil
}
func (f *fakePR) UpdatePRBranch(_ context.Context, _, _ string, _ int) error { return nil }
func (f *fakePR) GetPRComments(_ context.Context, _, _ string, _ int) ([]vcs.CommentData, error) {
	return f.comments, nil
}

func (f *fakePR) ApprovePR(_ context.Context, _, _ string, number int, body string) error {
	f.approved, f.lastBody = number, body
	return nil
}

func (f *fakePR) RequestChangesPR(_ context.Context, _, _ string, number int, body string) error {
	f.changesFor, f.lastBody = number, body
	return nil
}

func (f *fakePR) CommentPR(_ context.Context, _, _ string, number int, body string) error {
	f.commented, f.lastBody = number, body
	return nil
}

func TestListReviewRequestsTool(t *testing.T) {
	fake := &fakePR{reviewReq: []vcs.PRData{
		{Number: 7, Title: "Add search", Author: "octo", ReviewStatus: "REVIEW_REQUIRED", URL: "u"},
	}}
	reg := NewRegistry(Deps{VCS: fake})

	out, err := reg.Call(context.Background(), "list_review_requests",
		map[string]any{"repo": "acme/app", "user": "octocat"}, Ctx{})
	if err != nil {
		t.Fatalf("list_review_requests: %v", err)
	}
	m := out.(map[string]any)
	if m["count"].(int) != 1 {
		t.Fatalf("expected 1 PR, got %v", m)
	}
	if fake.lastLogin != "octocat" {
		t.Fatalf("expected login octocat passed through, got %q", fake.lastLogin)
	}
	if tool, _ := reg.Get("list_review_requests"); tool.Write {
		t.Fatal("list_review_requests must be a read tool")
	}
}

func TestGetPRTool(t *testing.T) {
	fake := &fakePR{pr: &vcs.PRData{Number: 42, Title: "Fix bug", Author: "a", Mergeable: "CONFLICTING"}}
	reg := NewRegistry(Deps{VCS: fake})

	out, err := reg.Call(context.Background(), "get_pr",
		map[string]any{"repo": "acme/app", "number": float64(42)}, Ctx{})
	if err != nil {
		t.Fatalf("get_pr: %v", err)
	}
	m := out.(map[string]any)
	if m["number"].(int) != 42 || m["mergeable"] != "CONFLICTING" {
		t.Fatalf("get_pr wrong payload: %v", m)
	}
}

func TestApprovePRTool(t *testing.T) {
	fake := &fakePR{}
	reg := NewRegistry(Deps{VCS: fake})

	out, err := reg.Call(context.Background(), "approve_pr",
		map[string]any{"repo": "acme/app", "number": float64(9), "body": "ship it"}, Ctx{})
	if err != nil {
		t.Fatalf("approve_pr: %v", err)
	}
	if out.(map[string]any)["status"] != "approved" {
		t.Fatalf("unexpected payload: %v", out)
	}
	if fake.approved != 9 || fake.lastBody != "ship it" {
		t.Fatalf("approve not passed through: num=%d body=%q", fake.approved, fake.lastBody)
	}
	if tool, _ := reg.Get("approve_pr"); !tool.Write {
		t.Fatal("approve_pr must be a write tool")
	}
}

func TestRequestChangesRequiresBody(t *testing.T) {
	reg := NewRegistry(Deps{VCS: &fakePR{}})
	if _, err := reg.Call(context.Background(), "request_changes_pr",
		map[string]any{"repo": "acme/app", "number": float64(9)}, Ctx{}); err == nil {
		t.Fatal("request_changes_pr must require a body")
	}
	for _, name := range []string{"request_changes_pr", "comment_pr"} {
		if tool, _ := reg.Get(name); !tool.Write {
			t.Fatalf("%s must be a write tool", name)
		}
	}
}

func TestBadRepoArg(t *testing.T) {
	reg := NewRegistry(Deps{VCS: &fakePR{}})
	if _, err := reg.Call(context.Background(), "get_pr",
		map[string]any{"repo": "not-a-repo", "number": float64(1)}, Ctx{}); err == nil {
		t.Fatal("expected error for malformed repo")
	}
}

func TestLazyLoadPRStatusTools(t *testing.T) {
	s := newTestStore(t)
	vs := vcsstore.NewStore(s.DB())
	ctx := context.Background()
	if _, err := vs.Repos.EnsureRepo(ctx, "github", "octo", "hello"); err != nil {
		t.Fatalf("ensure repo: %v", err)
	}

	pr := vcs.PRData{Number: 7, Title: "Add search", Author: "me", State: "open",
		ReviewStatus: "review_required", MergeStateStatus: "BEHIND", CIStatus: "FAILURE", CreatedAt: "2026-10-01T00:00:00Z"}
	fake := &fakePR{authUser: "me", pr: &pr, authored: []vcs.PRData{pr}, unresolved: 2}
	reg := NewRegistry(Deps{VCS: fake, VCSStore: vs})

	// First get_pr_status: uncached → fetched from GitHub and snapshotted.
	res, err := reg.Call(ctx, "get_pr_status", map[string]any{"repo": "octo/hello", "number": 7}, Ctx{})
	if err != nil {
		t.Fatalf("get_pr_status: %v", err)
	}
	out := res.(map[string]any)
	if out["source"] != "github" || fake.getPRCalls != 1 {
		t.Fatalf("expected live fetch, got %v (calls=%d)", out["source"], fake.getPRCalls)
	}
	brief := out["pullRequest"].(map[string]any)
	if brief["unresolvedComments"] != 2 || brief["updateBranchNeeded"] != true {
		t.Fatalf("snapshot brief wrong: %v", brief)
	}

	// Second call: answered from the snapshot, GitHub untouched.
	res, err = reg.Call(ctx, "get_pr_status", map[string]any{"repo": "octo/hello", "number": 7}, Ctx{})
	if err != nil {
		t.Fatalf("get_pr_status cached: %v", err)
	}
	if res.(map[string]any)["source"] != "snapshot" || fake.getPRCalls != 1 {
		t.Fatalf("expected snapshot read, got %v (calls=%d)", res.(map[string]any)["source"], fake.getPRCalls)
	}

	// refresh=true forces a GitHub fetch and snapshot update.
	res, err = reg.Call(ctx, "get_pr_status", map[string]any{"repo": "octo/hello", "number": 7, "refresh": true}, Ctx{})
	if err != nil {
		t.Fatalf("get_pr_status refresh: %v", err)
	}
	if res.(map[string]any)["source"] != "github" || fake.getPRCalls != 2 {
		t.Fatalf("expected refresh fetch, got %v (calls=%d)", res.(map[string]any)["source"], fake.getPRCalls)
	}

	// list_author_prs: snapshot already has the PR → no GitHub call.
	res, err = reg.Call(ctx, "list_author_prs", map[string]any{"repo": "octo/hello"}, Ctx{})
	if err != nil {
		t.Fatalf("list_author_prs: %v", err)
	}
	out = res.(map[string]any)
	if out["source"] != "snapshot" || out["count"] != 1 || fake.authoredCalls != 0 {
		t.Fatalf("expected snapshot list, got %v", out)
	}

	// refresh=true re-syncs the list from GitHub.
	res, err = reg.Call(ctx, "list_author_prs", map[string]any{"repo": "octo/hello", "refresh": true}, Ctx{})
	if err != nil {
		t.Fatalf("list_author_prs refresh: %v", err)
	}
	if res.(map[string]any)["source"] != "github" || fake.authoredCalls != 1 {
		t.Fatalf("expected live list, got %v (calls=%d)", res.(map[string]any)["source"], fake.authoredCalls)
	}
}
