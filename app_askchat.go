package main

import (
	"strings"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
)

const askChatTitleMax = 80

// SaveAskChat upserts an Ask AI conversation (the frontend's log JSON, stored
// verbatim) and prunes chats older than the configured retention.
func (a *App) SaveAskChat(id, title, logJSON string) error {
	title = strings.TrimSpace(title)
	if len([]rune(title)) > askChatTitleMax {
		title = string([]rune(title)[:askChatTitleMax]) + "…"
	}
	if err := a.db.UpsertAskChat(a.ctx, id, title, logJSON); err != nil {
		return err
	}
	a.pruneAskChats()
	return nil
}

// ListAskChats returns saved chats newest first (no log bodies), after pruning.
func (a *App) ListAskChats() ([]appstore.AskChat, error) {
	a.pruneAskChats()
	return a.db.ListAskChats(a.ctx)
}

// GetAskChat returns one chat with its full log so the UI can resume it.
func (a *App) GetAskChat(id string) (*appstore.AskChat, error) {
	return a.db.GetAskChat(a.ctx, id)
}

// DeleteAskChat removes a saved chat.
func (a *App) DeleteAskChat(id string) error {
	return a.db.DeleteAskChat(a.ctx, id)
}

// SetAskChatRetentionDays sets how long Ask AI chats are kept; 0 keeps forever.
func (a *App) SetAskChatRetentionDays(days int) error {
	if days < 0 {
		days = 0
	}
	current, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		return err
	}
	current.AskChatRetentionDays = days
	return a.db.SetAppSettings(a.ctx, *current)
}

// pruneAskChats is best-effort: a prune failure must not block saving/listing.
func (a *App) pruneAskChats() {
	settings, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		return
	}
	_, _ = a.db.PruneAskChats(a.ctx, settings.AskChatRetentionDays)
}
