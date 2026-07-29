package github

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLatestReviewerDecisions(t *testing.T) {
	raw := []byte(`[
	  {"user":{"login":"alice"},"state":"COMMENTED","submitted_at":"2026-07-20T09:00:00Z"},
	  {"user":{"login":"alice"},"state":"APPROVED","submitted_at":"2026-07-20T10:00:00Z"},
	  {"user":{"login":"bob"},"state":"CHANGES_REQUESTED","submitted_at":"2026-07-21T10:00:00Z"},
	  {"user":{"login":"carol"},"state":"COMMENTED","submitted_at":"2026-07-21T11:00:00Z"},
	  {"user":{"login":"dave"},"state":"APPROVED","submitted_at":"2026-07-21T13:00:00Z"},
	  {"user":{"login":"dave"},"state":"DISMISSED","submitted_at":"2026-07-21T14:00:00Z"},
	  {"user":{"login":"carol_author"},"state":"APPROVED","submitted_at":"2026-07-21T12:00:00Z"}
	]`)
	got := latestReviewerDecisions(raw, "carol_author")
	// alice's latest decision is APPROVED (COMMENTED before it is ignored);
	// bob CHANGES_REQUESTED; carol COMMENTED only; dave APPROVED then DISMISSED
	// (dropped entirely); PR author excluded.
	m := map[string]string{}
	for _, d := range got {
		m[d.Login] = d.State
	}
	require.Equal(t, "approved", m["alice"])
	require.Equal(t, "changes_requested", m["bob"])
	require.Equal(t, "commented", m["carol"])
	_, hasDave := m["dave"]
	require.False(t, hasDave)
	_, hasAuthor := m["carol_author"]
	require.False(t, hasAuthor)
}
