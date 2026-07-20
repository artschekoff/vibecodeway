package main

import (
	appstore "github.com/artschekoff/context-consolidator/internal/store"
)

const defaultActivityFeedLimit = 100

// GetActivityFeed returns the most recent activity (state-change events) across
// all contexts, newest first. limit <= 0 uses defaultActivityFeedLimit.
func (a *App) GetActivityFeed(limit int) ([]appstore.ActivityFeedItem, error) {
	if limit <= 0 {
		limit = defaultActivityFeedLimit
	}
	return a.db.ListRecentActivity(a.ctx, limit)
}
