// Package provider defines the Provider interface that all data-source plugins must implement.
// To add a new data source, implement Provider and register it with service.New.
package provider

import (
	"context"
	"io"
	"time"
)

// Stats summarises what a Provider processed during a Refresh or Reload call.
// Semantic meaning of the fields depends on which method is called:
//
//	Refresh: ItemsRefreshed = existing items updated; NewItems = new sub-items (e.g. comments).
//	Reload:  ItemsRefreshed = newly discovered top-level items; NewItems = new sub-items.
type Stats struct {
	ItemsRefreshed   int
	NewItems         int
	// NewNotifications counts system-generated notifications created during this sync
	// (e.g. PR conflict detected, review status changed).
	NewNotifications int
}

// Provider is the interface every data-source plugin must satisfy.
// Register implementations with service.New to include them in cross-cutting operations
// (Refresh, Reload, FormatContext). Provider-specific APIs may remain on ContextService.
type Provider interface {
	// Name returns the unique lowercase slug for this provider (e.g. "jira", "slack").
	Name() string

	// IsConfigured reports whether this provider has data or configuration for contextID.
	IsConfigured(ctx context.Context, contextID string) (bool, error)

	// Refresh re-fetches and updates items already stored for contextID.
	// It does NOT discover or add new top-level items.
	Refresh(ctx context.Context, contextID string) (Stats, error)

	// Reload performs a full sync: discovers new top-level items and refreshes existing ones.
	Reload(ctx context.Context, contextID string) (Stats, error)

	// WriteContext appends a human-readable section for contextID to w.
	// Only items on or after startAt are included; pass a zero time for no filter.
	WriteContext(ctx context.Context, w io.Writer, contextID string, startAt time.Time) error
}
