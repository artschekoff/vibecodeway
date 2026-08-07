package tooling

import (
	"context"

	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/artschekoff/context-consolidator/internal/vcs"
	"go.uber.org/zap"
)

// PRActions is the subset of VCS operations the PR tools need. The concrete
// *github.Runner satisfies it structurally; tests supply a fake. Write methods
// are added in Task 8.
type PRActions interface {
	ListReviewRequestsForUser(ctx context.Context, owner, repo, login string) ([]vcs.PRData, error)
	ListChangesRequestedForUser(ctx context.Context, owner, repo, login string) ([]vcs.PRData, error)
	GetPRByNumber(ctx context.Context, owner, repo string, number int) (*vcs.PRData, error)
	GetPRComments(ctx context.Context, owner, repo string, number int) ([]vcs.CommentData, error)
}

// Deps are the dependencies every tool handler may close over.
type Deps struct {
	Store  *appstore.Store
	Svc    *jiraservice.ContextService
	VCS    PRActions
	Logger *zap.Logger
}

// NewRegistry builds the full tool set from the app's shared services. PR tools
// are only registered when a VCS backend is supplied.
func NewRegistry(d Deps) *Registry {
	r := &Registry{}
	registerContextTools(r, d)
	if d.VCS != nil {
		registerPRTools(r, d)
	}
	return r
}
