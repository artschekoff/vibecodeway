package analysis

// This file documents (via a failing/RED test) the duplicate-event bug in
// GenerateEventsFromMessages.
//
// Mechanism: GenerateEventsFromMessages computes its delta window as
// since = max(existing event Timestamp), then calls listMessages(ctx, contextID, since),
// which delegates to ListMessagesSince and filters on the message's created_at
// (WHERE m.created_at >= since). But `since` is derived from EVENT timestamps —
// a different clock than message created_at. When an extracted event's timestamp
// is EARLIER than the message's created_at (the normal case: an event mirrors a
// source's original time, e.g. a Jira comment from days ago, while the message
// row's created_at is insertion time = now), `since` never advances past the
// message's created_at. A second call to GenerateEventsFromMessages therefore
// re-selects the same message, re-runs extraction, and inserts a DUPLICATE event.
//
// Fixed by Task 12 (a message-processed watermark independent of event timestamps).
// This test is intentionally RED until then — do not "fix" it here.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/artschekoff/vibecodeway/internal/openai"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeExtractAI is a scripted chatClient double returning one fixed event whose
// timestamp is deliberately EARLIER than the seeded message's created_at (which
// defaults to "now" via UpsertMessage).
type fakeExtractAI struct{}

func (fakeExtractAI) ChatCompletion(_ context.Context, _ []openai.Message) (*openai.Response, error) {
	return &openai.Response{Content: `[{"type":"comment_added","timestamp":"2020-01-01T00:00:00Z","source_type":"github","source_id":"c1","author":"alice","summary":"a comment"}]`}, nil
}

func (fakeExtractAI) DescribeSettings(_ context.Context) (*appstore.AISettings, openai.Source, error) {
	return &appstore.AISettings{}, openai.SourceNone, nil
}

func (fakeExtractAI) EmbedText(_ context.Context, _ string) ([]float32, error) { return nil, nil }

func TestReanalyzeWithoutNewMessagesAddsNoEvents(t *testing.T) {
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)
	c, err := s.CreateContext(ctx, "JH-1", org.ID)
	require.NoError(t, err)

	// Seed ONE message (created_at defaults to now — later than the fake event's 2020 timestamp).
	_, err = s.UpsertMessage(ctx, appstore.UpsertMessageArgs{
		ContextID: c.ID, OrganizationID: org.ID, SourceID: "github",
		ExternalID: "m1", RawAuthor: "alice", Body: "please review this",
	})
	require.NoError(t, err)

	svc := &Service{store: s, ai: fakeExtractAI{}, logger: zap.NewNop()}

	// First run: extracts and saves 1 event.
	_, err = svc.GenerateEventsFromMessages(ctx, c.ID)
	require.NoError(t, err)
	after1, err := s.ListEvents(ctx, c.ID)
	require.NoError(t, err)

	// Second run with NO new messages: MUST add nothing.
	_, err = svc.GenerateEventsFromMessages(ctx, c.ID)
	require.NoError(t, err)
	after2, err := s.ListEvents(ctx, c.ID)
	require.NoError(t, err)

	require.Equal(t, len(after1), len(after2),
		"re-analysis with no new messages must not add duplicate events (added %d)", len(after2)-len(after1))
}
