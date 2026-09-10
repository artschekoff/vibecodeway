package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const appSettingsSingletonID = 1

// defaultAppSettings returns the zero-value defaults used when no row exists.
func defaultAppSettings() *AppSettings {
	return &AppSettings{
		ContextRefetchEnabled:        false,
		ContextRefetchIntervalMin:    30,
		ReviewsRefetchEnabled:        false,
		ReviewsRefetchIntervalMin:    30,
		TerminalAppPath:              "",
		CodingAgentApp:               "cursor",
		AutoLinkReposOnContextCreate: true,
		PRStaleThresholdDays:         7,
		WorkdayHours:                 8,
		TaskHours:                    2,
	}
}

// GetAppSettings returns the current application settings.
// Returns defaults when the row has not been created yet.
func (s *Store) GetAppSettings(ctx context.Context) (*AppSettings, error) {
	var (
		contextEnabled     int
		contextIntervalMin int
		reviewsEnabled     int
		reviewsIntervalMin int
		terminalAppPath    string
		codingAgentApp     string
		autoLink           int
		editorSettings     string
		prStaleThreshold   int
		workdayHours       float64
		taskHours          float64
		statsAuthorName    string
		updatedAt          string
	)
	err := s.db.QueryRowContext(
		ctx,
		`SELECT context_refetch_enabled, context_refetch_interval_min,
		        reviews_refetch_enabled, reviews_refetch_interval_min,
		        terminal_app_path, coding_agent_app,
		        auto_link_repos_on_context_create, editor_settings,
		        pr_stale_threshold_days, workday_hours, task_hours,
		        stats_author_name, updated_at
		 FROM app_settings WHERE id = ?`,
		appSettingsSingletonID,
	).Scan(&contextEnabled, &contextIntervalMin, &reviewsEnabled, &reviewsIntervalMin,
		&terminalAppPath, &codingAgentApp, &autoLink, &editorSettings, &prStaleThreshold,
		&workdayHours, &taskHours, &statsAuthorName, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultAppSettings(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading app settings: %w", err)
	}
	return &AppSettings{
		ContextRefetchEnabled:        contextEnabled != 0,
		ContextRefetchIntervalMin:    contextIntervalMin,
		ReviewsRefetchEnabled:        reviewsEnabled != 0,
		ReviewsRefetchIntervalMin:    reviewsIntervalMin,
		TerminalAppPath:              terminalAppPath,
		CodingAgentApp:               codingAgentApp,
		AutoLinkReposOnContextCreate: autoLink != 0,
		EditorSettings:               editorSettings,
		PRStaleThresholdDays:         prStaleThreshold,
		WorkdayHours:                 workdayHours,
		TaskHours:                    taskHours,
		StatsAuthorName:              statsAuthorName,
		UpdatedAt:                    updatedAt,
	}, nil
}

// SetAppSettings upserts the singleton app_settings row.
func (s *Store) SetAppSettings(ctx context.Context, settings AppSettings) error {
	now := time.Now().UTC().Format(time.RFC3339)

	contextEnabled := 0
	if settings.ContextRefetchEnabled {
		contextEnabled = 1
	}
	reviewsEnabled := 0
	if settings.ReviewsRefetchEnabled {
		reviewsEnabled = 1
	}
	autoLink := 0
	if settings.AutoLinkReposOnContextCreate {
		autoLink = 1
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO app_settings
		     (id, context_refetch_enabled, context_refetch_interval_min,
		      reviews_refetch_enabled, reviews_refetch_interval_min,
		      terminal_app_path, coding_agent_app,
		      auto_link_repos_on_context_create, editor_settings,
		      pr_stale_threshold_days, workday_hours, task_hours,
		      stats_author_name, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		     context_refetch_enabled           = excluded.context_refetch_enabled,
		     context_refetch_interval_min      = excluded.context_refetch_interval_min,
		     reviews_refetch_enabled           = excluded.reviews_refetch_enabled,
		     reviews_refetch_interval_min      = excluded.reviews_refetch_interval_min,
		     terminal_app_path                 = excluded.terminal_app_path,
		     coding_agent_app                  = excluded.coding_agent_app,
		     auto_link_repos_on_context_create = excluded.auto_link_repos_on_context_create,
		     editor_settings                   = excluded.editor_settings,
		     pr_stale_threshold_days           = excluded.pr_stale_threshold_days,
		     workday_hours                     = excluded.workday_hours,
		     task_hours                        = excluded.task_hours,
		     stats_author_name                 = excluded.stats_author_name,
		     updated_at                        = excluded.updated_at`,
		appSettingsSingletonID,
		contextEnabled, settings.ContextRefetchIntervalMin,
		reviewsEnabled, settings.ReviewsRefetchIntervalMin,
		settings.TerminalAppPath,
		settings.CodingAgentApp,
		autoLink,
		settings.EditorSettings,
		settings.PRStaleThresholdDays,
		settings.WorkdayHours,
		settings.TaskHours,
		settings.StatsAuthorName,
		now,
	)
	if err != nil {
		return fmt.Errorf("saving app settings: %w", err)
	}
	return nil
}
