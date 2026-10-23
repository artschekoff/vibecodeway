package tooling

import (
	"context"

	jiraservice "github.com/artschekoff/vibecodeway/internal/jira/service"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/artschekoff/vibecodeway/internal/vcs"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
	"go.uber.org/zap"
)

// PRActions is the subset of VCS operations the PR tools need. The concrete
// *github.Runner satisfies it structurally; tests supply a fake. Write methods
// are added in Task 8.
type PRActions interface {
	ListReviewRequestsForUser(ctx context.Context, owner, repo, login string) ([]vcs.PRData, error)
	ListChangesRequestedForUser(ctx context.Context, owner, repo, login string) ([]vcs.PRData, error)
	ListAuthoredPRs(ctx context.Context, owner, repo, login string) ([]vcs.PRData, error)
	GetPRByNumber(ctx context.Context, owner, repo string, number int) (*vcs.PRData, error)
	GetPRComments(ctx context.Context, owner, repo string, number int) ([]vcs.CommentData, error)
	CountUnresolvedThreads(ctx context.Context, owner, repo string, prNumber int) (int, error)
	GetAuthUser(ctx context.Context) (string, error)

	ApprovePR(ctx context.Context, owner, repo string, number int, body string) error
	RequestChangesPR(ctx context.Context, owner, repo string, number int, body string) error
	CommentPR(ctx context.Context, owner, repo string, number int, body string) error
}

// RepoLocations lists tracked repositories together with their local checkout
// paths. *vcsstore.Store satisfies it structurally; tests supply a fake.
type RepoLocations interface {
	ListReposWithLocations(ctx context.Context) ([]vcsstore.RepoWithLocations, error)
}

// Deps are the dependencies every tool handler may close over.
type Deps struct {
	Store  *appstore.Store
	Svc    *jiraservice.ContextService
	VCS    PRActions
	Repos  RepoLocations
	Logger *zap.Logger
	// VCSStore is the app's VCS state (tracked repos + last-synced PR snapshots).
	// When set, the PR tools gain lazy-load behavior: reads default to the stored
	// snapshot and refreshes write the latest GitHub state back into it.
	VCSStore *vcsstore.Store
}

// NewRegistry builds the full tool set from the app's shared services. PR tools
// are only registered when a VCS backend is supplied.
func NewRegistry(d Deps) *Registry {
	r := &Registry{}
	registerContextTools(r, d)
	registerNotificationTools(r, d)
	if d.VCS != nil {
		registerPRTools(r, d)
	}
	if d.Repos != nil {
		registerCommitTools(r, d)
	}
	return r
}
