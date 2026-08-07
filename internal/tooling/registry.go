package tooling

import (
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"go.uber.org/zap"
)

// PRActions is widened in Task 7. Declared here so Deps is stable.
type PRActions interface{}

// Deps are the dependencies every tool handler may close over.
type Deps struct {
	Store  *appstore.Store
	Svc    *jiraservice.ContextService
	VCS    PRActions
	Logger *zap.Logger
}

// NewRegistry builds the full tool set from the app's shared services.
func NewRegistry(d Deps) *Registry {
	r := &Registry{}
	registerContextTools(r, d)
	// registerPRTools(r, d) // added in Task 7
	return r
}
