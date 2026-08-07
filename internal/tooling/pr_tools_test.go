package tooling

import (
	"context"
	"testing"

	"github.com/artschekoff/context-consolidator/internal/vcs"
)

// fakePR is a test double for PRActions (extended with write methods in Task 8).
type fakePR struct {
	reviewReq  []vcs.PRData
	changesReq []vcs.PRData
	pr         *vcs.PRData
	comments   []vcs.CommentData

	lastLogin string
}

func (f *fakePR) ListReviewRequestsForUser(_ context.Context, _, _, login string) ([]vcs.PRData, error) {
	f.lastLogin = login
	return f.reviewReq, nil
}
func (f *fakePR) ListChangesRequestedForUser(_ context.Context, _, _, login string) ([]vcs.PRData, error) {
	f.lastLogin = login
	return f.changesReq, nil
}
func (f *fakePR) GetPRByNumber(_ context.Context, _, _ string, _ int) (*vcs.PRData, error) {
	return f.pr, nil
}
func (f *fakePR) GetPRComments(_ context.Context, _, _ string, _ int) ([]vcs.CommentData, error) {
	return f.comments, nil
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

func TestBadRepoArg(t *testing.T) {
	reg := NewRegistry(Deps{VCS: &fakePR{}})
	if _, err := reg.Call(context.Background(), "get_pr",
		map[string]any{"repo": "not-a-repo", "number": float64(1)}, Ctx{}); err == nil {
		t.Fatal("expected error for malformed repo")
	}
}
