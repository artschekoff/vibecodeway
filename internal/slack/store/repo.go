// Package slackstore provides Slack-specific SQLite repositories.
package slackstore

import "database/sql"

// Repo holds Slack-specific repositories backed by a shared *sql.DB.
type Repo struct {
	db *sql.DB
}

// NewRepo creates a new Repo using the given database connection.
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}
