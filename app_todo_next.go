package main

import (
	"time"

	analysis "github.com/artschekoff/context-consolidator/internal/analysis"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
)

// GenerateTodoNext gathers the current board and asks the AI for the prioritized
// next-action list, persisting and returning it.
func (a *App) GenerateTodoNext(personName string) ([]appstore.AITodoItem, error) {
	board, err := a.GetMyBoard(personName)
	if err != nil {
		return nil, err
	}
	reviews, err := a.GetPendingCodeReviews()
	if err != nil {
		return nil, err
	}
	settings, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		return nil, err
	}
	input := analysis.TodoInput{
		JiraIssues:     board.MyAssignedJiraIssues,
		MyPRs:          board.MyOpenPRs,
		ReviewRequests: reviews,
		ThresholdDays:  settings.PRStaleThresholdDays,
		Now:            time.Now().UTC(),
	}
	return a.analysisSvc.GenerateTodoNext(a.ctx, input)
}

// ListTodoNext returns the persisted next-action list.
func (a *App) ListTodoNext() ([]appstore.AITodoItem, error) {
	return a.db.ListTodoItems(a.ctx)
}
