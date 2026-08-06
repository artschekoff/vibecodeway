package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"github.com/artschekoff/context-consolidator/internal/vcs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

// bgRefreshState holds the cancellation function for the running background
// refresh goroutine and the mutex that protects it.
type bgRefreshState struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

// --- Wails API ---

// GetAppSettings returns the current application settings.
func (a *App) GetAppSettings() (*appstore.AppSettings, error) {
	return a.db.GetAppSettings(a.ctx)
}

// SetAppSettings persists new application settings and immediately restarts the
// background refresh goroutine so changes take effect without an app restart.
func (a *App) SetAppSettings(
	contextEnabled bool, contextIntervalMin int,
	reviewsEnabled bool, reviewsIntervalMin int,
	terminalAppPath string,
	codingAgentApp string,
) error {
	if contextIntervalMin < 1 {
		contextIntervalMin = 1
	}
	if reviewsIntervalMin < 1 {
		reviewsIntervalMin = 1
	}
	codingAgentApp = strings.TrimSpace(codingAgentApp)
	current, _ := a.db.GetAppSettings(a.ctx)
	settings := appstore.AppSettings{
		ContextRefetchEnabled:     contextEnabled,
		ContextRefetchIntervalMin: contextIntervalMin,
		ReviewsRefetchEnabled:     reviewsEnabled,
		ReviewsRefetchIntervalMin: reviewsIntervalMin,
		TerminalAppPath:           terminalAppPath,
		CodingAgentApp:            codingAgentApp,
	}
	if current != nil {
		settings.AutoLinkReposOnContextCreate = current.AutoLinkReposOnContextCreate
	}
	if err := a.db.SetAppSettings(a.ctx, settings); err != nil {
		return err
	}
	a.restartBackgroundRefresh(settings)
	return nil
}

// SetAutoLinkRepos toggles whether matched repos are auto-linked when a new
// context is created. Persisted in app_settings; preserves all other fields.
func (a *App) SetAutoLinkRepos(enabled bool) error {
	current, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		return err
	}
	current.AutoLinkReposOnContextCreate = enabled
	return a.db.SetAppSettings(a.ctx, *current)
}

// --- Background refresh lifecycle ---

// startBackgroundRefresh reads the persisted settings and starts the goroutine.
// Safe to call multiple times — stops any previous goroutine first.
func (a *App) startBackgroundRefresh() {
	settings, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		a.logger.Warn("app_settings: could not load settings for background refresh", zap.Error(err))
		settings = &appstore.AppSettings{
			ContextRefetchIntervalMin: 15,
			ReviewsRefetchIntervalMin: 15,
		}
	}
	a.restartBackgroundRefresh(*settings)
}

// stopBackgroundRefresh cancels the running goroutine, if any.
func (a *App) stopBackgroundRefresh() {
	a.bg.mu.Lock()
	defer a.bg.mu.Unlock()
	if a.bg.cancel != nil {
		a.bg.cancel()
		a.bg.cancel = nil
	}
}

// restartBackgroundRefresh stops any running goroutine and starts a new one
// with the given settings. If both refresh types are disabled the goroutine
// is stopped and no new one is launched.
func (a *App) restartBackgroundRefresh(settings appstore.AppSettings) {
	a.bg.mu.Lock()
	if a.bg.cancel != nil {
		a.bg.cancel()
		a.bg.cancel = nil
	}

	if !settings.ContextRefetchEnabled && !settings.ReviewsRefetchEnabled {
		a.bg.mu.Unlock()
		return
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.bg.cancel = cancel
	a.bg.mu.Unlock()

	go a.backgroundRefreshLoop(ctx, settings)
}

// GetKeybindings returns all user-configurable keyboard shortcuts.
func (a *App) GetKeybindings() ([]appstore.Keybinding, error) {
	return a.db.GetKeybindings(context.Background())
}

// UpdateKeybinding saves a new modifier+key+sequence combo for a command and,
// for the "quick_attach" command, re-registers the global hotkey immediately.
// modifierKeys is a comma-separated string (e.g. "cmd,shift"), keyCode is e.g. "c".
func (a *App) UpdateKeybinding(command, modifierKeys, keyCode, sequence string) error {
	if err := a.db.UpdateKeybinding(context.Background(), command, modifierKeys, keyCode, sequence); err != nil {
		return err
	}
	if command == "quick_attach" {
		if err := a.hotkeyService.Reconfigure(modifierKeys, keyCode); err != nil {
			return fmt.Errorf("reconfigure hotkey: %w", err)
		}
	}
	return nil
}

// ResetKeybinding restores a shortcut to its default chord.
func (a *App) ResetKeybinding(command string) error {
	return a.db.ResetKeybinding(context.Background(), command)
}

// backgroundRefreshLoop runs two independent tickers — one for contexts and one
// for code reviews — and calls the matching refresh method when each fires.
// It exits cleanly when ctx is cancelled.
func (a *App) backgroundRefreshLoop(ctx context.Context, settings appstore.AppSettings) {
	var (
		ctxTicker     *time.Ticker
		reviewsTicker *time.Ticker
		ctxC          <-chan time.Time
		reviewsC      <-chan time.Time
	)

	if settings.ContextRefetchEnabled {
		ctxTicker = time.NewTicker(time.Duration(settings.ContextRefetchIntervalMin) * time.Minute)
		ctxC = ctxTicker.C
		defer ctxTicker.Stop()
	}
	if settings.ReviewsRefetchEnabled {
		reviewsTicker = time.NewTicker(time.Duration(settings.ReviewsRefetchIntervalMin) * time.Minute)
		reviewsC = reviewsTicker.C
		defer reviewsTicker.Stop()
	}

	a.logger.Info("background refresh started",
		zap.Bool("contexts", settings.ContextRefetchEnabled),
		zap.Int("contexts_interval_min", settings.ContextRefetchIntervalMin),
		zap.Bool("reviews", settings.ReviewsRefetchEnabled),
		zap.Int("reviews_interval_min", settings.ReviewsRefetchIntervalMin),
	)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("background refresh stopped")
			return

		case <-ctxC:
			a.logger.Info("background refresh: refreshing all contexts")
			runtime.EventsEmit(a.ctx, "bg-refresh:contexts:start", nil)
			results, err := a.svc.RefreshAllContexts(ctx, func(idx, total int, contextKey, _ string) {
				runtime.EventsEmit(a.ctx, "bg-refresh:contexts:progress", map[string]interface{}{
					"current":    idx,
					"total":      total,
					"contextKey": contextKey,
				})
			})
			if err != nil {
				a.logger.Warn("background refresh: contexts error", zap.Error(err))
			}
			errCount := 0
			for _, r := range results {
				if r.Error != "" {
					errCount++
				}
			}
			runtime.EventsEmit(a.ctx, "bg-refresh:contexts:done", map[string]interface{}{
				"total":  len(results),
				"errors": errCount,
			})
			runtime.EventsEmit(a.ctx, "bg-refresh:contexts")

		case <-reviewsC:
			a.logger.Info("background refresh: refreshing pending reviews")
			if err := a.vcsSvc.SyncReviewRequests(ctx, vcs.PlatformGitHub); err != nil {
				a.logger.Warn("background refresh: reviews error", zap.Error(err))
			}
			runtime.EventsEmit(a.ctx, "bg-refresh:reviews")
		}
	}
}
