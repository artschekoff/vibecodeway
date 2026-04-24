package contextlog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Object key prefixes. Keep these stable — they are serialised into diff_json in the DB.
const (
	KeyPrefixJiraIssue   = "jira_issue:"
	KeyPrefixJiraComment = "jira_comment:"
	KeyPrefixSlackThread = "slack_thread:"
	KeyPrefixContextText = "context_text:"
)

// SnapshotFull reads every context object (jira_issues, jira_comments, slack_threads,
// context_texts) for contextID in one pass and returns a map keyed by object key.
// Only active rows (is_active = 1) are included — deactivated items are treated as absent.
func (r *Repo) SnapshotFull(ctx context.Context, contextID string) (Snapshot, error) {
	snap := make(Snapshot)

	if err := r.collectIssues(ctx, contextID, snap); err != nil {
		return nil, err
	}
	if err := r.collectComments(ctx, contextID, snap); err != nil {
		return nil, err
	}
	if err := r.collectSlackThreads(ctx, contextID, snap); err != nil {
		return nil, err
	}
	if err := r.collectContextTexts(ctx, contextID, snap); err != nil {
		return nil, err
	}
	return snap, nil
}

func (r *Repo) collectIssues(ctx context.Context, contextID string, out Snapshot) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT jira_key, summary, status, assignee, issue_type, description, jira_updated
		FROM jira_issues
		WHERE context_id = ? AND is_active = 1`, contextID)
	if err != nil {
		return fmt.Errorf("snapshot jira issues: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var key, summary, status, assignee, issueType, description, jiraUpdated string
		if err := rows.Scan(&key, &summary, &status, &assignee, &issueType, &description, &jiraUpdated); err != nil {
			return fmt.Errorf("scan jira issue: %w", err)
		}
		content := fmt.Sprintf("key=%s\nsummary=%s\nstatus=%s\nassignee=%s\ntype=%s\njira_updated=%s\n\n%s",
			key, summary, status, assignee, issueType, jiraUpdated, description)
		out[KeyPrefixJiraIssue+key] = ObjectSnapshot{Hash: hashOf(content), Content: content}
	}
	return rows.Err()
}

func (r *Repo) collectComments(ctx context.Context, contextID string, out Snapshot) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.author, c.body, c.jira_created, i.jira_key
		FROM jira_comments c
		JOIN jira_issues i ON i.id = c.issue_id
		WHERE i.context_id = ? AND c.is_active = 1`, contextID)
	if err != nil {
		return fmt.Errorf("snapshot jira comments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id, author, body, jiraCreated, issueKey string
		if err := rows.Scan(&id, &author, &body, &jiraCreated, &issueKey); err != nil {
			return fmt.Errorf("scan jira comment: %w", err)
		}
		content := fmt.Sprintf("issue=%s\nauthor=%s\njira_created=%s\n\n%s",
			issueKey, author, jiraCreated, body)
		out[KeyPrefixJiraComment+id] = ObjectSnapshot{Hash: hashOf(content), Content: content}
	}
	return rows.Err()
}

func (r *Repo) collectSlackThreads(ctx context.Context, contextID string, out Snapshot) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, workspace, channel_id, channel_name, thread_ts, permalink, content
		FROM slack_threads
		WHERE context_id = ? AND is_active = 1`, contextID)
	if err != nil {
		return fmt.Errorf("snapshot slack threads: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id, workspace, channelID, channelName, threadTS, permalink, threadContent string
		if err := rows.Scan(&id, &workspace, &channelID, &channelName, &threadTS, &permalink, &threadContent); err != nil {
			return fmt.Errorf("scan slack thread: %w", err)
		}
		content := fmt.Sprintf("workspace=%s\nchannel=%s (%s)\nthread_ts=%s\npermalink=%s\n\n%s",
			workspace, channelName, channelID, threadTS, permalink, threadContent)
		out[KeyPrefixSlackThread+id] = ObjectSnapshot{Hash: hashOf(content), Content: content}
	}
	return rows.Err()
}

func (r *Repo) collectContextTexts(ctx context.Context, contextID string, out Snapshot) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, content
		FROM context_texts
		WHERE context_id = ? AND is_active = 1`, contextID)
	if err != nil {
		return fmt.Errorf("snapshot context texts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id, name, textContent string
		if err := rows.Scan(&id, &name, &textContent); err != nil {
			return fmt.Errorf("scan context text: %w", err)
		}
		content := fmt.Sprintf("name=%s\n\n%s", name, textContent)
		out[KeyPrefixContextText+id] = ObjectSnapshot{Hash: hashOf(content), Content: content}
	}
	return rows.Err()
}

// Diff compares two snapshots and returns added/removed/changed object changes.
// The output lists are sorted by key for deterministic output.
func Diff(before, after Snapshot) DiffPayload {
	var out DiffPayload

	for key, newVal := range after {
		oldVal, existed := before[key]
		switch {
		case !existed:
			out.Added = append(out.Added, ObjectChange{Key: key, After: newVal})
		case oldVal.Hash != newVal.Hash:
			out.Changed = append(out.Changed, ObjectChange{Key: key, Before: oldVal, After: newVal})
		}
	}
	for key, oldVal := range before {
		if _, stillThere := after[key]; !stillThere {
			out.Removed = append(out.Removed, ObjectChange{Key: key, Before: oldVal})
		}
	}
	sortChanges(out.Added)
	sortChanges(out.Removed)
	sortChanges(out.Changed)
	return out
}

func sortChanges(changes []ObjectChange) {
	sort.Slice(changes, func(i, j int) bool { return changes[i].Key < changes[j].Key })
}

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// CategoryOf returns the prefix category of an object key (e.g. "jira_issue").
// Returns "unknown" for malformed keys.
func CategoryOf(key string) string {
	if i := strings.Index(key, ":"); i > 0 {
		return key[:i]
	}
	return "unknown"
}
