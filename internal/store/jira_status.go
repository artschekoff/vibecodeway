package store

import (
	"context"
	"fmt"
	"strings"
)

// terminalTicketStatuses are Jira statuses meaning the work is finished. This is
// the canonical set shared across the app — board decluttering (analysis),
// context auto-archival, and the manual-deactivate guard all resolve "done"
// through here so the taxonomy has a single source of truth. Keys are already
// lowercased/trimmed; look them up via IsTerminalTicketStatus.
var terminalTicketStatuses = map[string]bool{
	"done": true, "closed": true, "resolved": true, "cancelled": true, "canceled": true,
	"won't do": true, "wont do": true, "duplicate": true, "merged": true, "deployed": true,
	"released": true, "verified": true, "shipped": true,
}

// IsTerminalTicketStatus reports whether a Jira status means the work is finished
// (done/closed/resolved/…). Case- and whitespace-insensitive.
func IsTerminalTicketStatus(status string) bool {
	return terminalTicketStatuses[strings.ToLower(strings.TrimSpace(status))]
}

// ListKnownJiraStatuses returns every distinct, non-blank status seen on synced
// Jira issues, sorted alphabetically. acli exposes no way to enumerate a
// project's workflow statuses or an issue's available transitions, so the
// statuses already synced from Jira are the only real source for the
// status-change dropdown — the alternative is inventing names Jira rejects.
func (s *Store) ListKnownJiraStatuses(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT status FROM jira_issues
		 WHERE TRIM(status) <> '' ORDER BY status COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("listing known jira statuses: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var st string
		if err := rows.Scan(&st); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// contextTicketCounts returns how many Jira issues are linked to the context
// (total) and how many of those are non-terminal ("open"). Contexts hold few
// issues, so statuses are classified in Go against the canonical set rather than
// duplicating the list into SQL.
func (s *Store) contextTicketCounts(ctx context.Context, contextID string) (total, open int, err error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT status FROM jira_issues WHERE context_id = ?`, contextID)
	if err != nil {
		return 0, 0, fmt.Errorf("context ticket counts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var st string
		if err := rows.Scan(&st); err != nil {
			return 0, 0, err
		}
		total++
		if !IsTerminalTicketStatus(st) {
			open++
		}
	}
	return total, open, rows.Err()
}

// CountOpenTickets returns the number of non-terminal Jira issues in the context.
// Used to warn before a manual deactivation that would hide still-open work.
func (s *Store) CountOpenTickets(ctx context.Context, contextID string) (int, error) {
	_, open, err := s.contextTicketCounts(ctx, contextID)
	return open, err
}

// ContextAllTicketsDone reports whether the context has at least one Jira issue
// and every one of them is terminal. Contexts with no tickets return false so
// they are never auto-archived on a vacuously-true "all done".
func (s *Store) ContextAllTicketsDone(ctx context.Context, contextID string) (bool, error) {
	total, open, err := s.contextTicketCounts(ctx, contextID)
	if err != nil {
		return false, err
	}
	return total > 0 && open == 0, nil
}
