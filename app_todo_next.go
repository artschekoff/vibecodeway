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

// AdjustTodoNext revises the persisted next-action list per a free-text comment,
// letting the AI use discovery tools, then persists and returns the new list.
func (a *App) AdjustTodoNext(comment string) ([]appstore.AITodoItem, error) {
	current, err := a.db.ListTodoItems(a.ctx)
	if err != nil {
		return nil, err
	}
	revised, err := a.agent.AdjustTodos(a.ctx, current, comment, "")
	if err != nil {
		return nil, err
	}
	if err := a.db.ReplaceTodoItems(a.ctx, revised); err != nil {
		return nil, err
	}
	return a.db.ListTodoItems(a.ctx)
}

// ReorderTodoNext persists a new display order and returns the updated list.
func (a *App) ReorderTodoNext(orderedIDs []string) ([]appstore.AITodoItem, error) {
	if err := a.db.ReorderTodoItems(a.ctx, orderedIDs); err != nil {
		return nil, err
	}
	return a.db.ListTodoItems(a.ctx)
}

// DeleteTodoNextItem removes one item and returns the updated list.
func (a *App) DeleteTodoNextItem(id string) ([]appstore.AITodoItem, error) {
	if err := a.db.DeleteTodoItem(a.ctx, id); err != nil {
		return nil, err
	}
	return a.db.ListTodoItems(a.ctx)
}

// AddTodoNextItem appends a user-authored item at the end and returns the list.
func (a *App) AddTodoNextItem(kind, title, refType, refKey, priority string) ([]appstore.AITodoItem, error) {
	existing, err := a.db.ListTodoItems(a.ctx)
	if err != nil {
		return nil, err
	}
	if priority == "" {
		priority = "medium"
	}
	if _, err := a.db.AddTodoItem(a.ctx, appstore.AITodoItem{
		Kind: kind, Title: title, RefType: refType, RefKey: refKey,
		Priority: priority, Position: len(existing), Status: "pending", Source: "user",
	}); err != nil {
		return nil, err
	}
	return a.db.ListTodoItems(a.ctx)
}
