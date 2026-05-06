package vcsstore

import "database/sql"

// Store aggregates all VCS sub-repositories.
type Store struct {
	Auth           *AuthRepo
	Repos          *RepoRepo
	RepoPaths      *RepoPathRepo
	Branches       *BranchRepo
	PullRequests   *PRRepo
	ReviewRequests *ReviewRequestRepo
	Workspaces     *WorkspaceRepo
}

// NewStore constructs a Store backed by the provided *sql.DB.
func NewStore(db *sql.DB) *Store {
	return &Store{
		Auth:           NewAuthRepo(db),
		Repos:          NewRepoRepo(db),
		RepoPaths:      NewRepoPathRepo(db),
		Branches:       NewBranchRepo(db),
		PullRequests:   NewPRRepo(db),
		ReviewRequests: NewReviewRequestRepo(db),
		Workspaces:     NewWorkspaceRepo(db),
	}
}
