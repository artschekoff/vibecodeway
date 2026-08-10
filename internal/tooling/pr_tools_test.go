package tooling

import (
	"context"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/vcs"
)

// fakePR is a test double for PRActions (extended with write methods in Task 8).
type fakePR struct {
	reviewReq  []vcs.PRData
	changesReq []vcs.PRData
	pr         *vcs.PRData
	comments   []vcs.CommentData

	lastLogin  string
	approved   int    // PR number approved
	changesFor int    // PR number changes-requested
	commented  int    // PR number commented
	lastBody   string // body of the last write action
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
