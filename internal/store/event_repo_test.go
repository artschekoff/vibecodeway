package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAddEventPersistsDomainEvent verifies that domain events can be persisted
// and retrieved from the event log.
func TestAddEventPersistsDomainEvent(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)

	// Create an organization and context to hold the event
	org, err := db.CreateOrganization(ctx, "test-org")
	require.NoError(t, err)
	require.NotNil(t, org)

	createdCtx, err := db.CreateContext(ctx, "test-context", org.ID)
	require.NoError(t, err)
	require.NotNil(t, createdCtx)

	// Add a domain event
	event := Event{
		ContextID:  createdCtx.ID,
		Type:       EventTypeCommentAdded,
		SourceType: EventSourceGitHub,
		SourceID:   "pr-123",
		Author:     "alice",
		Summary:    "New review comment on PR",
	}

	added, err := db.AddEvent(ctx, event)
	require.NoError(t, err)
	require.NotNil(t, added)
	require.NotEmpty(t, added.ID)
	require.Equal(t, EventTypeCommentAdded, added.Type)

	// Retrieve the event
	events, err := db.ListEvents(ctx, createdCtx.ID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, EventTypeCommentAdded, events[0].Type)
	require.Equal(t, "pr-123", events[0].SourceID)
}
