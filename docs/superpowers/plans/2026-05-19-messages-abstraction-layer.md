# Messages Abstraction Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce a unified `messages` table as a base abstraction for all provider items (Jira issues/comments, Slack threads, GitHub PRs/comments), so the AI analysis pipeline reads only from `messages` and remains provider-agnostic. Authors are resolved to org-level `users` records via a `author_mappings` lookup table; unknown authors trigger a post-refresh modal asking the user to assign or create org users.

**Architecture:**
- `sources` — dictionary table (`id = slug`, e.g. `"jira"`) for all provider names.
- `users` — org-level people only (no source-system accounts). One row per real human in the org.
- `author_mappings` — translates `(source_id, raw_name_from_source) → users.id`. Populated by the user via the resolution modal.
- `messages` — base content table. `source_id FK sources`, `author_id FK users` (NULL = unresolved), `raw_author TEXT` (verbatim name from source for display + matching), `parent_id` for threading.
- Provider tables keep their specific columns and add nullable `message_id FK messages`.
- After every `RefreshContext`, the frontend checks for unresolved authors and shows a modal: select existing user or create new, then call `AssignAuthor`.
- AI pipeline reads `messages` only via `store.ListMessagesSince`; `formatMessages` renders `raw_author` when `author_id` is not yet resolved.

**Tech Stack:** Go 1.22, SQLite (goose), `github.com/google/uuid`, React 19, Wails IPC.

---

## File Map

| Status | Path | Change |
|--------|------|--------|
| Create | `internal/store/migrations/055_users_and_messages.sql` | `sources`, `users`, `author_mappings`, `messages` DDL |
| Create | `internal/store/migrations/056_provider_message_ids.sql` | Add `message_id` column to 5 provider tables |
| Modify | `internal/store/models.go` | Add `Source`, `User`, `AuthorMapping`, `Message`, arg structs |
| Create | `internal/store/user_repo.go` | `CreateUser`, `ListUsers`, `GetUser` |
| Create | `internal/store/author_repo.go` | `ResolveAuthor`, `CreateAuthorMapping`, `GetUnresolvedAuthors`, `AssignAuthor` |
| Create | `internal/store/message_repo.go` | `UpsertMessage`, `ListMessagesSince`, `BackfillMessages` |
| Create | `internal/store/message_writer.go` | `MessageWriter` interface |
| Modify | `internal/jira/store/models.go` | Add `MessageID *string` to `Issue`, `Comment` |
| Modify | `internal/jira/store/issue_repo.go` | Add `SetIssueMessageID` |
| Modify | `internal/jira/store/comment_repo.go` | Add `SetCommentMessageID` |
| Modify | `internal/jira/service/jira_provider.go` | Inject `MessageWriter`, write messages with `raw_author` |
| Modify | `internal/slack/store/models.go` | Add `MessageID *string` to `SlackThread`, `SlackDMConfig` |
| Modify | `internal/slack/store/slack_repo.go` | Add `SetThreadMessageID`, `SetDMConfigMessageID` |
| Modify | `internal/jira/service/slack_provider.go` | Inject `MessageWriter`, write messages with `raw_author` |
| Modify | `internal/vcs/store/models.go` | Add `MessageID *string` to `PullRequest`, `PRComment` |
| Modify | `internal/vcs/store/pr_repo.go` | Add `SetPRMessageID`, `SetPRCommentMessageID` |
| Modify | `internal/vcs/service/vcs_provider.go` | Inject `MessageWriter`, write messages with `raw_author` |
| Modify | `internal/analysis/service.go` | Replace 4× `FormatContext` with `listMessages` + `formatMessages` |
| Modify | `internal/jira/service/service.go` | Pass `MessageWriter` to provider constructors |
| Modify | `app.go` | Wire `MessageWriter`, add `GetUnresolvedAuthors`, `AssignAuthor`, `CreateUserAndAssign`, `ListOrgUsers`, `ListMessages`, `BackfillMessages` on startup |
| Create | `frontend/src/components/UnresolvedAuthorsModal.tsx` | Post-refresh modal for author assignment |
| Modify | `frontend/src/components/ContextDetail.tsx` (or wherever RefreshContext is called) | Trigger modal after refresh if unresolved authors exist |
| Modify | `internal/analysis/service.go:521-571` | `GeneratePersonAssignments` — fetch org users via `ListUsers`, inject `{{org_users}}` into prompt |

---

## Task 1: Create `sources`, `users`, `author_mappings`, and `messages` tables

**Files:**
- Create: `internal/store/migrations/055_users_and_messages.sql`

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up

-- Provider / origin dictionary. id is the slug used as FK everywhere.
CREATE TABLE sources (
    id         TEXT PRIMARY KEY,   -- "jira" | "slack" | "github" | "user"
    label      TEXT NOT NULL,      -- human-readable, e.g. "Jira"
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO sources (id, label) VALUES
    ('jira',   'Jira'),
    ('slack',  'Slack'),
    ('github', 'GitHub'),
    ('user',   'User Note');

-- Org-level people. No source-system accounts here — one row per real human.
CREATE TABLE users (
    id           TEXT PRIMARY KEY,
    org_id       TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    display_name TEXT NOT NULL DEFAULT '',
    email        TEXT NOT NULL DEFAULT '',
    avatar_url   TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Maps a (source, verbatim author name/id from that source) → org user.
-- Populated by the user via the author-resolution modal after each refresh.
CREATE TABLE author_mappings (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_id    TEXT NOT NULL REFERENCES sources(id),
    external_name TEXT NOT NULL,  -- verbatim string as it appears in source data
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (source_id, external_name)
);

CREATE INDEX author_mappings_user_idx ON author_mappings(user_id);

-- Base message abstraction for all content items across all providers.
CREATE TABLE messages (
    id              TEXT PRIMARY KEY,
    author_id       TEXT REFERENCES users(id) ON DELETE SET NULL,  -- NULL = unresolved
    raw_author      TEXT NOT NULL DEFAULT '',  -- verbatim author string from source (for modal + fallback display)
    context_id      TEXT REFERENCES contexts(id) ON DELETE CASCADE,
    organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL,
    parent_id       TEXT REFERENCES messages(id) ON DELETE SET NULL,
    title           TEXT NOT NULL DEFAULT '',
    body            TEXT NOT NULL DEFAULT '',
    source_id       TEXT NOT NULL REFERENCES sources(id),
    external_id     TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    inserted_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX messages_context_idx    ON messages(context_id, created_at DESC);
CREATE INDEX messages_parent_idx     ON messages(parent_id);
CREATE INDEX messages_source_idx     ON messages(source_id, external_id);
CREATE INDEX messages_unresolved_idx ON messages(context_id, author_id) WHERE author_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS messages_unresolved_idx;
DROP INDEX IF EXISTS messages_source_idx;
DROP INDEX IF EXISTS messages_parent_idx;
DROP INDEX IF EXISTS messages_context_idx;
DROP TABLE IF EXISTS messages;
DROP INDEX IF EXISTS author_mappings_user_idx;
DROP TABLE IF EXISTS author_mappings;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS sources;
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator
go build ./...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/store/migrations/055_users_and_messages.sql
git commit -m "feat(db): add sources, users, author_mappings, messages abstraction tables (055)"
```

---

## Task 2: Add `message_id` column to provider tables

**Files:**
- Create: `internal/store/migrations/056_provider_message_ids.sql`

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up
ALTER TABLE jira_issues       ADD COLUMN message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE jira_comments     ADD COLUMN message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE slack_threads     ADD COLUMN message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE vcs_pull_requests ADD COLUMN message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE vcs_pr_comments   ADD COLUMN message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;

-- +goose Down
-- SQLite: additive-only; no rollback needed for nullable FK columns.
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/store/migrations/056_provider_message_ids.sql
git commit -m "feat(db): add message_id FK column to provider tables (056)"
```

---

## Task 3: Add Go models

**Files:**
- Modify: `internal/store/models.go`

- [ ] **Step 1: Append to the end of `internal/store/models.go`**

```go
// Source is a row from the sources dictionary table.
type Source struct {
	ID        string `json:"id"`    // slug, e.g. "jira"
	Label     string `json:"label"` // e.g. "Jira"
	CreatedAt string `json:"createdAt"`
}

// User is an org-level person. Not linked to any specific source system.
type User struct {
	ID          string  `json:"id"`
	OrgID       *string `json:"orgId,omitempty"`
	DisplayName string  `json:"displayName"`
	Email       string  `json:"email"`
	AvatarURL   string  `json:"avatarUrl"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

// AuthorMapping links a verbatim source-system name to an org User.
type AuthorMapping struct {
	ID           string `json:"id"`
	UserID       string `json:"userId"`
	SourceID     string `json:"sourceId"`
	ExternalName string `json:"externalName"` // verbatim name as it appears in source data
	CreatedAt    string `json:"createdAt"`
}

// UnresolvedAuthor is one unknown author found in a context's messages.
// Returned by GetUnresolvedAuthors for the assignment modal.
type UnresolvedAuthor struct {
	SourceID     string `json:"sourceId"`
	SourceLabel  string `json:"sourceLabel"`
	RawAuthor    string `json:"rawAuthor"`
	MessageCount int    `json:"messageCount"`
}

// Message is the base abstraction for any content item from any provider.
// AuthorDisplayName and SourceLabel are populated by JOINs at query time.
type Message struct {
	ID              string  `json:"id"`
	AuthorID        *string `json:"authorId,omitempty"`
	AuthorDisplayName string `json:"authorDisplayName,omitempty"` // joined from users
	RawAuthor       string  `json:"rawAuthor"`                   // verbatim from source
	ContextID       *string `json:"contextId,omitempty"`
	OrganizationID  *string `json:"organizationId,omitempty"`
	ParentID        *string `json:"parentId,omitempty"`
	Title           string  `json:"title"`
	Body            string  `json:"body"`
	SourceID        string  `json:"sourceId"`
	SourceLabel     string  `json:"sourceLabel,omitempty"` // joined from sources
	ExternalID      string  `json:"externalId"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
	InsertedAt      string  `json:"insertedAt"`
}

// CreateUserArgs holds fields for creating a new org user.
type CreateUserArgs struct {
	OrgID       string
	DisplayName string
	Email       string
	AvatarURL   string
}

// UpsertMessageArgs holds data for creating or updating a Message record.
type UpsertMessageArgs struct {
	ContextID      string
	OrganizationID string // optional → NULL when empty
	ParentID       string // optional → NULL when empty
	AuthorID       string // optional → NULL when empty (unresolved)
	RawAuthor      string // verbatim name from source; always set when author is known
	SourceID       string // FK to sources.id, e.g. "jira"
	ExternalID     string
	Title          string
	Body           string
	CreatedAt      string // RFC3339
	UpdatedAt      string // RFC3339
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/store/...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/store/models.go
git commit -m "feat(store): add Source, User, AuthorMapping, Message models"
```

---

## Task 4: Create `UserRepo`, `AuthorRepo`, and `MessageRepo`

**Files:**
- Create: `internal/store/user_repo.go`
- Create: `internal/store/author_repo.go`
- Create: `internal/store/message_repo.go`

- [ ] **Step 1: Write `user_repo.go`**

```go
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateUser inserts a new org-level user and returns it.
func (s *Store) CreateUser(ctx context.Context, args CreateUserArgs) (*User, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	u := &User{
		ID:          uuid.New().String(),
		DisplayName: args.DisplayName,
		Email:       args.Email,
		AvatarURL:   args.AvatarURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	var orgPtr *string
	if args.OrgID != "" {
		orgPtr = &args.OrgID
		u.OrgID = orgPtr
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, org_id, display_name, email, avatar_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, u.ID, orgPtr, u.DisplayName, u.Email, u.AvatarURL, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// ListUsers returns all org users, optionally filtered by orgID (empty = all orgs).
func (s *Store) ListUsers(ctx context.Context, orgID string) ([]User, error) {
	q := `SELECT id, org_id, display_name, email, avatar_url, created_at, updated_at FROM users`
	var args []any
	if orgID != "" {
		q += " WHERE org_id = ?"
		args = append(args, orgID)
	}
	q += " ORDER BY display_name ASC"

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.OrgID, &u.DisplayName, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
```

- [ ] **Step 2: Write `author_repo.go`**

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ResolveAuthor looks up the org user ID for a (sourceID, externalName) pair.
// Returns empty string when no mapping exists — caller should leave author_id NULL.
func (s *Store) ResolveAuthor(ctx context.Context, sourceID, externalName string) string {
	if externalName == "" {
		return ""
	}
	var userID string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id FROM author_mappings WHERE source_id = ? AND external_name = ?`,
		sourceID, externalName,
	).Scan(&userID)
	if err != nil {
		return ""
	}
	return userID
}

// CreateAuthorMapping records that (sourceID, externalName) belongs to userID.
// Safe to call multiple times — updates the user_id on conflict.
func (s *Store) CreateAuthorMapping(ctx context.Context, sourceID, externalName, userID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO author_mappings (id, user_id, source_id, external_name, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(source_id, external_name) DO UPDATE SET user_id = excluded.user_id
	`, id, userID, sourceID, externalName, now)
	if err != nil {
		return fmt.Errorf("create author mapping: %w", err)
	}
	return nil
}

// GetUnresolvedAuthors returns distinct (source, raw_author) pairs in contextID
// whose messages have no author_id set.
func (s *Store) GetUnresolvedAuthors(ctx context.Context, contextID string) ([]UnresolvedAuthor, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.source_id, COALESCE(src.label, m.source_id), m.raw_author, COUNT(*) AS cnt
		FROM messages m
		LEFT JOIN sources src ON src.id = m.source_id
		WHERE m.context_id = ?
		  AND m.author_id IS NULL
		  AND m.raw_author != ''
		GROUP BY m.source_id, m.raw_author
		ORDER BY cnt DESC, m.raw_author ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("get unresolved authors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []UnresolvedAuthor
	for rows.Next() {
		var a UnresolvedAuthor
		if err := rows.Scan(&a.SourceID, &a.SourceLabel, &a.RawAuthor, &a.MessageCount); err != nil {
			return nil, fmt.Errorf("scan unresolved author: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AssignAuthor creates an author_mapping for (sourceID, rawAuthor) → userID
// and back-fills author_id on all matching messages.
func (s *Store) AssignAuthor(ctx context.Context, sourceID, rawAuthor, userID string) error {
	if err := s.CreateAuthorMapping(ctx, sourceID, rawAuthor, userID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE messages SET author_id = ?
		WHERE source_id = ? AND raw_author = ? AND author_id IS NULL
	`, userID, sourceID, rawAuthor)
	if err != nil {
		return fmt.Errorf("backfill author_id on messages: %w", err)
	}
	return nil
}

// AssignAuthorBatch runs AssignAuthor for each mapping in a single call.
// Used by the modal "Confirm" button to submit all assignments at once.
func (s *Store) AssignAuthorBatch(ctx context.Context, assignments []AuthorAssignment) error {
	for _, a := range assignments {
		if err := s.AssignAuthor(ctx, a.SourceID, a.RawAuthor, a.UserID); err != nil {
			return err
		}
	}
	return nil
}

// AuthorAssignment is one row of input from the resolution modal.
type AuthorAssignment struct {
	SourceID  string `json:"sourceId"`
	RawAuthor string `json:"rawAuthor"`
	UserID    string `json:"userId"`
}

// CreateUserAndAssign creates a new org user, then calls AssignAuthor.
// Returns the new User so the frontend can add it to its local list.
func (s *Store) CreateUserAndAssign(ctx context.Context, args CreateUserArgs, sourceID, rawAuthor string) (*User, error) {
	u, err := s.CreateUser(ctx, args)
	if err != nil {
		return nil, err
	}
	if err := s.AssignAuthor(ctx, sourceID, rawAuthor, u.ID); err != nil {
		return nil, err
	}
	return u, nil
}

// GetAuthorMappings returns all mappings for a user — useful for the user profile page.
func (s *Store) GetAuthorMappings(ctx context.Context, userID string) ([]AuthorMapping, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, source_id, external_name, created_at
		FROM author_mappings WHERE user_id = ? ORDER BY source_id, external_name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get author mappings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []AuthorMapping
	for rows.Next() {
		var m AuthorMapping
		if err := rows.Scan(&m.ID, &m.UserID, &m.SourceID, &m.ExternalName, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan author mapping: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
```

- [ ] **Step 3: Write `message_repo.go`**

```go
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UpsertMessage creates or updates a message identified by (source_id, external_id, context_id).
// Returns the internal message ID.
func (s *Store) UpsertMessage(ctx context.Context, args UpsertMessageArgs) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var contextIDPtr, orgIDPtr, parentIDPtr, authorIDPtr *string
	if args.ContextID != "" {
		contextIDPtr = &args.ContextID
	}
	if args.OrganizationID != "" {
		orgIDPtr = &args.OrganizationID
	}
	if args.ParentID != "" {
		parentIDPtr = &args.ParentID
	}
	if args.AuthorID != "" {
		authorIDPtr = &args.AuthorID
	}

	createdAt := args.CreatedAt
	if createdAt == "" {
		createdAt = now
	}
	updatedAt := args.UpdatedAt
	if updatedAt == "" {
		updatedAt = now
	}

	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM messages WHERE source_id = ? AND external_id = ? AND context_id IS ?`,
		args.SourceID, args.ExternalID, contextIDPtr,
	).Scan(&id)

	if err != nil {
		id = uuid.New().String()
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO messages
			  (id, author_id, raw_author, context_id, organization_id, parent_id,
			   title, body, source_id, external_id, created_at, updated_at, inserted_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, authorIDPtr, args.RawAuthor, contextIDPtr, orgIDPtr, parentIDPtr,
			args.Title, args.Body, args.SourceID, args.ExternalID,
			createdAt, updatedAt, now)
		if err != nil {
			return "", fmt.Errorf("insert message: %w", err)
		}
		return id, nil
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE messages
		SET author_id = ?, raw_author = ?, organization_id = ?, parent_id = ?,
		    title = ?, body = ?, updated_at = ?
		WHERE id = ?
	`, authorIDPtr, args.RawAuthor, orgIDPtr, parentIDPtr,
		args.Title, args.Body, updatedAt, id)
	if err != nil {
		return "", fmt.Errorf("update message: %w", err)
	}
	return id, nil
}

// ListMessagesSince returns messages for contextID on or after since,
// joined with users for AuthorDisplayName and sources for SourceLabel.
// Pass sourceIDs to filter by provider (empty = all). Ordered by created_at ASC.
func (s *Store) ListMessagesSince(ctx context.Context, contextID string, since time.Time, sourceIDs ...string) ([]Message, error) {
	q := `
		SELECT m.id, m.author_id, COALESCE(u.display_name, ''), m.raw_author,
		       m.context_id, m.organization_id, m.parent_id,
		       m.title, m.body, m.source_id, COALESCE(src.label, m.source_id),
		       m.external_id, m.created_at, m.updated_at, m.inserted_at
		FROM messages m
		LEFT JOIN users u     ON u.id = m.author_id
		LEFT JOIN sources src ON src.id = m.source_id
		WHERE m.context_id = ?`
	args := []any{contextID}

	if !since.IsZero() {
		q += " AND m.created_at >= ?"
		args = append(args, since.UTC().Format(time.RFC3339))
	}
	if len(sourceIDs) > 0 {
		ph := ""
		for i, src := range sourceIDs {
			if i > 0 {
				ph += ","
			}
			ph += "?"
			args = append(args, src)
		}
		q += " AND m.source_id IN (" + ph + ")"
	}
	q += " ORDER BY m.created_at ASC"

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(
			&m.ID, &m.AuthorID, &m.AuthorDisplayName, &m.RawAuthor,
			&m.ContextID, &m.OrganizationID, &m.ParentID,
			&m.Title, &m.Body, &m.SourceID, &m.SourceLabel,
			&m.ExternalID, &m.CreatedAt, &m.UpdatedAt, &m.InsertedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// BackfillMessages creates message records for provider items with message_id IS NULL.
// Idempotent — safe to call at startup.
func (s *Store) BackfillMessages(ctx context.Context) error {
	if err := s.backfillJiraIssues(ctx); err != nil {
		return fmt.Errorf("backfill jira issues: %w", err)
	}
	if err := s.backfillJiraComments(ctx); err != nil {
		return fmt.Errorf("backfill jira comments: %w", err)
	}
	if err := s.backfillSlackThreads(ctx); err != nil {
		return fmt.Errorf("backfill slack threads: %w", err)
	}
	if err := s.backfillVCSPRs(ctx); err != nil {
		return fmt.Errorf("backfill vcs prs: %w", err)
	}
	if err := s.backfillVCSPRComments(ctx); err != nil {
		return fmt.Errorf("backfill vcs pr comments: %w", err)
	}
	return nil
}

func (s *Store) backfillJiraIssues(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, jira_key, summary, COALESCE(description,''),
		       COALESCE(assignee,''), COALESCE(jira_updated,''), created_at
		FROM jira_issues WHERE message_id IS NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type row struct{ id, contextID, jiraKey, summary, description, assignee, jiraUpdated, createdAt string }
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.contextID, &r.jiraKey, &r.summary, &r.description, &r.assignee, &r.jiraUpdated, &r.createdAt); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range items {
		ts := r.jiraUpdated
		if ts == "" {
			ts = r.createdAt
		}
		authorID := s.ResolveAuthor(ctx, "jira", r.assignee)
		msgID, err := s.UpsertMessage(ctx, UpsertMessageArgs{
			ContextID:  r.contextID,
			SourceID:   "jira",
			ExternalID: r.jiraKey,
			Title:      r.summary,
			Body:       r.description,
			RawAuthor:  r.assignee,
			AuthorID:   authorID,
			CreatedAt:  r.createdAt,
			UpdatedAt:  ts,
		})
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE jira_issues SET message_id = ? WHERE id = ?`, msgID, r.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillJiraComments(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT jc.id, ji.context_id, jc.jira_id, jc.body, jc.author,
		       COALESCE(jc.jira_created, jc.created_at), jc.created_at,
		       (SELECT m.id FROM messages m
		        WHERE m.source_id='jira' AND m.external_id=ji.jira_key
		          AND m.context_id=ji.context_id LIMIT 1)
		FROM jira_comments jc
		JOIN jira_issues ji ON jc.issue_id = ji.id
		WHERE jc.message_id IS NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type row struct {
		id, contextID, jiraID, body, author, jiraCreated, createdAt string
		parentMsgID                                                  *string
	}
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.contextID, &r.jiraID, &r.body, &r.author, &r.jiraCreated, &r.createdAt, &r.parentMsgID); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range items {
		authorID := s.ResolveAuthor(ctx, "jira", r.author)
		args := UpsertMessageArgs{
			ContextID:  r.contextID,
			SourceID:   "jira",
			ExternalID: r.jiraID,
			Body:       r.body,
			RawAuthor:  r.author,
			AuthorID:   authorID,
			CreatedAt:  r.jiraCreated,
			UpdatedAt:  r.jiraCreated,
		}
		if r.parentMsgID != nil {
			args.ParentID = *r.parentMsgID
		}
		msgID, err := s.UpsertMessage(ctx, args)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE jira_comments SET message_id = ? WHERE id = ?`, msgID, r.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillSlackThreads(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, channel_name, COALESCE(content,''), thread_ts, created_at
		FROM slack_threads WHERE message_id IS NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type row struct{ id, contextID, channelName, content, threadTS, createdAt string }
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.contextID, &r.channelName, &r.content, &r.threadTS, &r.createdAt); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range items {
		msgID, err := s.UpsertMessage(ctx, UpsertMessageArgs{
			ContextID:  r.contextID,
			SourceID:   "slack",
			ExternalID: r.threadTS,
			Title:      r.channelName,
			Body:       r.content,
			CreatedAt:  r.createdAt,
			UpdatedAt:  r.createdAt,
		})
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE slack_threads SET message_id = ? WHERE id = ?`, msgID, r.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillVCSPRs(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT vpr.id, cvb.context_id,
		       r.owner || '/' || r.name || '#' || CAST(vpr.pr_number AS TEXT),
		       vpr.title, COALESCE(vpr.author,''),
		       vpr.pr_created_at, COALESCE(vpr.pr_updated_at, vpr.pr_created_at)
		FROM vcs_pull_requests vpr
		JOIN vcs_repos r ON vpr.repo_id = r.id
		JOIN context_vcs_branches cvb ON vpr.branch_id = cvb.id
		WHERE vpr.message_id IS NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type row struct{ id, contextID, externalID, title, author, createdAt, updatedAt string }
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.contextID, &r.externalID, &r.title, &r.author, &r.createdAt, &r.updatedAt); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range items {
		authorID := s.ResolveAuthor(ctx, "github", r.author)
		msgID, err := s.UpsertMessage(ctx, UpsertMessageArgs{
			ContextID:  r.contextID,
			SourceID:   "github",
			ExternalID: r.externalID,
			Title:      r.title,
			RawAuthor:  r.author,
			AuthorID:   authorID,
			CreatedAt:  r.createdAt,
			UpdatedAt:  r.updatedAt,
		})
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE vcs_pull_requests SET message_id = ? WHERE id = ?`, msgID, r.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillVCSPRComments(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT vpc.id, vpc.external_id, vpc.body, COALESCE(vpc.author,''),
		       vpc.comment_created_at, COALESCE(vpc.comment_updated_at, vpc.comment_created_at),
		       vpr.message_id
		FROM vcs_pr_comments vpc
		JOIN vcs_pull_requests vpr ON vpc.pr_id = vpr.id
		WHERE vpc.message_id IS NULL AND vpr.message_id IS NOT NULL
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type row struct {
		id, externalID, body, author, createdAt, updatedAt string
		parentMsgID                                        string
	}
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.externalID, &r.body, &r.author, &r.createdAt, &r.updatedAt, &r.parentMsgID); err != nil {
			return err
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range items {
		authorID := s.ResolveAuthor(ctx, "github", r.author)
		msgID, err := s.UpsertMessage(ctx, UpsertMessageArgs{
			SourceID:   "github",
			ExternalID: r.externalID,
			Body:       r.body,
			RawAuthor:  r.author,
			AuthorID:   authorID,
			ParentID:   r.parentMsgID,
			CreatedAt:  r.createdAt,
			UpdatedAt:  r.updatedAt,
		})
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE vcs_pr_comments SET message_id = ? WHERE id = ?`, msgID, r.id); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Build store package**

```bash
go build ./internal/store/...
```
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/store/user_repo.go internal/store/author_repo.go internal/store/message_repo.go
git commit -m "feat(store): add UserRepo, AuthorRepo, MessageRepo"
```

---

## Task 5: Add `MessageWriter` interface

**Files:**
- Create: `internal/store/message_writer.go`

- [ ] **Step 1: Write the file**

```go
package store

import "context"

// MessageWriter is a minimal interface for writing messages and resolving authors.
// Injected into provider services so they can persist messages without importing the full store.
type MessageWriter interface {
	ResolveAuthor(ctx context.Context, sourceID, externalName string) string
	UpsertMessage(ctx context.Context, args UpsertMessageArgs) (string, error)
}

// compile-time assertion
var _ MessageWriter = (*Store)(nil)
```

- [ ] **Step 2: Build**

```bash
go build ./internal/store/...
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/store/message_writer.go
git commit -m "feat(store): add MessageWriter interface"
```

---

## Task 6: Update Jira provider to write messages

**Files:**
- Modify: `internal/jira/store/models.go`
- Modify: `internal/jira/store/issue_repo.go`
- Modify: `internal/jira/store/comment_repo.go`
- Modify: `internal/jira/service/jira_provider.go`

- [ ] **Step 1: Add `MessageID *string` to Issue and Comment in models.go**

```go
type Issue struct {
	ID            string    `json:"id"`
	ContextID     string    `json:"contextId"`
	JiraKey       string    `json:"jiraKey"`
	Summary       string    `json:"summary"`
	Status        string    `json:"status"`
	Assignee      string    `json:"assignee"`
	IssueType     string    `json:"issueType"`
	Description   string    `json:"description"`
	JiraUpdated   string    `json:"jiraUpdated"`
	ParentKey     string    `json:"parentKey"`
	CreatedAt     string    `json:"createdAt"`
	IsActive      bool      `json:"isActive"`
	ViewedAt      *string   `json:"viewedAt"`
	AiProcessedAt *string   `json:"aiProcessedAt"`
	MessageID     *string   `json:"messageId,omitempty"`
	Comments      []Comment `json:"comments,omitempty"`
}

type Comment struct {
	ID            string  `json:"id"`
	IssueID       string  `json:"issueId"`
	JiraID        string  `json:"jiraId"`
	Author        string  `json:"author"`
	Body          string  `json:"body"`
	JiraCreated   string  `json:"jiraCreated"`
	CreatedAt     string  `json:"createdAt"`
	IsActive      bool    `json:"isActive"`
	ViewedAt      *string `json:"viewedAt"`
	AiProcessedAt *string `json:"aiProcessedAt"`
	MessageID     *string `json:"messageId,omitempty"`
}
```

- [ ] **Step 2: Write failing test for SetIssueMessageID**

Add to `internal/jira/store/repo_test.go`:

```go
func TestSetIssueMessageID(t *testing.T) {
	db := openTestDB(t)
	r := NewRepo(db)
	ctx := context.Background()

	issueID, _ := r.UpsertIssue(ctx, "ctx1", "PROJ-1", "Title", "open", "", "task", "", "", "")
	err := r.SetIssueMessageID(ctx, issueID, "msg-uuid-123")
	require.NoError(t, err)

	var got *string
	_ = db.QueryRowContext(ctx, "SELECT message_id FROM jira_issues WHERE id = ?", issueID).Scan(&got)
	require.NotNil(t, got)
	require.Equal(t, "msg-uuid-123", *got)
}
```

- [ ] **Step 3: Run test — confirm it fails**

```bash
go test ./internal/jira/store/... -run TestSetIssueMessageID -v
```
Expected: FAIL — `SetIssueMessageID` undefined.

- [ ] **Step 4: Implement SetIssueMessageID**

Append to `internal/jira/store/issue_repo.go`:

```go
// SetIssueMessageID sets the message_id FK on a jira_issues row.
func (r *Repo) SetIssueMessageID(ctx context.Context, issueID, messageID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE jira_issues SET message_id = ? WHERE id = ?", messageID, issueID)
	return err
}
```

- [ ] **Step 5: Run test — confirm it passes**

```bash
go test ./internal/jira/store/... -run TestSetIssueMessageID -v
```
Expected: PASS.

- [ ] **Step 6: Write failing test for SetCommentMessageID**

Add to `internal/jira/store/repo_test.go`:

```go
func TestSetCommentMessageID(t *testing.T) {
	db := openTestDB(t)
	r := NewRepo(db)
	ctx := context.Background()

	issueID, _ := r.UpsertIssue(ctx, "ctx1", "PROJ-1", "Title", "open", "", "task", "", "", "")
	commentID, _ := r.UpsertComment(ctx, issueID, "jira-cmt-1", "alice", "body", "2024-01-01T00:00:00Z")
	err := r.SetCommentMessageID(ctx, commentID, "msg-cmt-uuid")
	require.NoError(t, err)

	var got *string
	_ = db.QueryRowContext(ctx, "SELECT message_id FROM jira_comments WHERE id = ?", commentID).Scan(&got)
	require.NotNil(t, got)
	require.Equal(t, "msg-cmt-uuid", *got)
}
```

- [ ] **Step 7: Run test — confirm it fails**

```bash
go test ./internal/jira/store/... -run TestSetCommentMessageID -v
```
Expected: FAIL.

- [ ] **Step 8: Implement SetCommentMessageID**

Append to `internal/jira/store/comment_repo.go`:

```go
// SetCommentMessageID sets the message_id FK on a jira_comments row.
func (r *Repo) SetCommentMessageID(ctx context.Context, commentID, messageID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE jira_comments SET message_id = ? WHERE id = ?", messageID, commentID)
	return err
}
```

- [ ] **Step 9: Run test — confirm it passes**

```bash
go test ./internal/jira/store/... -run TestSetCommentMessageID -v
```
Expected: PASS.

- [ ] **Step 10: Inject MessageWriter into JiraProvider**

In `internal/jira/service/jira_provider.go`:

```go
import (
	appstore "github.com/artschekoff/context-consolidator/internal/store"
)

type JiraProvider struct {
	jiraRepo  *jirastore.Repo
	jira      JiraAPI
	msgWriter appstore.MessageWriter
	logger    *zap.Logger
}

func NewJiraProvider(jiraRepo *jirastore.Repo, jiraClient JiraAPI, msgWriter appstore.MessageWriter, logger *zap.Logger) *JiraProvider {
	return &JiraProvider{jiraRepo: jiraRepo, jira: jiraClient, msgWriter: msgWriter, logger: logger}
}
```

- [ ] **Step 11: Write messages with raw_author after UpsertIssue (Refresh + Reload, ~lines 56 and 108)**

After each `p.jiraRepo.UpsertIssue(...)` call:

```go
issueID, err = p.jiraRepo.UpsertIssue(ctx, contextID,
    detail.Key, detail.Summary, detail.Status, detail.Assignee,
    detail.IssueType, detail.Description, detail.Updated, detail.ParentKey,
)
if err != nil {
    p.logger.Warn("jira: upsert issue failed", zap.String("key", detail.Key), zap.Error(err))
    continue
}
authorID := p.msgWriter.ResolveAuthor(ctx, "jira", detail.Assignee)
msgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
    ContextID:  contextID,
    SourceID:   "jira",
    ExternalID: detail.Key,
    Title:      detail.Summary,
    Body:       detail.Description,
    RawAuthor:  detail.Assignee,
    AuthorID:   authorID,
    CreatedAt:  detail.Updated,
    UpdatedAt:  detail.Updated,
})
if msgID != "" {
    _ = p.jiraRepo.SetIssueMessageID(ctx, issueID, msgID)
}
// Write messages for comments.
for _, c := range detail.Comments {
    commentID, cErr := p.jiraRepo.UpsertComment(ctx, issueID, c.ID, c.Author, c.Body, c.Created)
    if cErr == nil && msgID != "" {
        cAuthorID := p.msgWriter.ResolveAuthor(ctx, "jira", c.Author)
        cMsgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
            ContextID:  contextID,
            SourceID:   "jira",
            ExternalID: c.ID,
            ParentID:   msgID,
            Body:       c.Body,
            RawAuthor:  c.Author,
            AuthorID:   cAuthorID,
            CreatedAt:  c.Created,
            UpdatedAt:  c.Created,
        })
        if cMsgID != "" {
            _ = p.jiraRepo.SetCommentMessageID(ctx, commentID, cMsgID)
        }
    }
}
```

- [ ] **Step 12: Build**

```bash
go build ./internal/jira/...
```
Expected: no errors (NewJiraProvider call-site in service.go fixed in Task 9).

- [ ] **Step 13: Commit**

```bash
git add internal/jira/store/models.go internal/jira/store/issue_repo.go internal/jira/store/comment_repo.go internal/jira/service/jira_provider.go
git commit -m "feat(jira): write message records with author resolution during sync"
```

---

## Task 7: Update Slack provider to write messages

**Files:**
- Modify: `internal/slack/store/models.go`
- Modify: `internal/slack/store/slack_repo.go`
- Modify: `internal/jira/service/slack_provider.go`

- [ ] **Step 1: Add `MessageID *string` to SlackThread and SlackDMConfig**

In `internal/slack/store/models.go`:

```go
type SlackThread struct {
	// ... existing fields unchanged ...
	MessageID *string `json:"messageId,omitempty"`
}

type SlackDMConfig struct {
	// ... existing fields unchanged ...
	MessageID *string `json:"messageId,omitempty"`
}
```

- [ ] **Step 2: Add SetThreadMessageID and SetDMConfigMessageID to slack_repo.go**

```go
// SetThreadMessageID sets the message_id FK on a slack_threads row.
func (r *Repo) SetThreadMessageID(ctx context.Context, threadID, messageID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE slack_threads SET message_id = ? WHERE id = ?", messageID, threadID)
	return err
}

// SetDMConfigMessageID sets the message_id FK on a slack_dm_configs row.
func (r *Repo) SetDMConfigMessageID(ctx context.Context, dmConfigID, messageID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE slack_dm_configs SET message_id = ? WHERE id = ?", messageID, dmConfigID)
	return err
}
```

- [ ] **Step 3: Inject MessageWriter into SlackProvider and write messages on upsert**

In `internal/jira/service/slack_provider.go`:

```go
import (
	appstore "github.com/artschekoff/context-consolidator/internal/store"
)

type SlackProvider struct {
	slackRepo *slackstore.Repo
	slack     SlackRunner
	msgWriter appstore.MessageWriter
	logger    *zap.Logger
}

func NewSlackProvider(slackRepo *slackstore.Repo, slack SlackRunner, msgWriter appstore.MessageWriter, logger *zap.Logger) *SlackProvider {
	return &SlackProvider{slackRepo: slackRepo, slack: slack, msgWriter: msgWriter, logger: logger}
}
```

After each `slackRepo.UpsertSlackThread(ctx, contextID, args)` call in Reload/Refresh:

```go
threadID, err := r.slackRepo.UpsertSlackThread(ctx, contextID, args)
if err != nil {
    // existing error handling
}
// The first message in args.Content is the thread author; Slack CLI serialises this as
// "{User}: {Text}" so we capture the raw string verbatim and resolve after.
// For threads loaded via channel search, the author comes from the first SlackMessage.User.
rawAuthor := "" // set from the specific Reload/Refresh context (the first message's User field)
authorID := p.msgWriter.ResolveAuthor(ctx, "slack", rawAuthor)
msgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
    ContextID:  contextID,
    SourceID:   "slack",
    ExternalID: args.ThreadTS,
    Title:      args.ChannelName,
    Body:       args.Content,
    RawAuthor:  rawAuthor,
    AuthorID:   authorID,
    CreatedAt:  slackTSToRFC3339(args.ThreadTS),
    UpdatedAt:  slackTSToRFC3339(args.ThreadTS),
})
if msgID != "" {
    _ = p.slackRepo.SetThreadMessageID(ctx, threadID, msgID)
}
```

Add the timestamp helper in `slack_provider.go`:

```go
// slackTSToRFC3339 converts "1746441180.000000" → RFC3339. Falls back to now on parse failure.
func slackTSToRFC3339(ts string) string {
	parts := strings.SplitN(ts, ".", 2)
	if len(parts) == 0 {
		return time.Now().UTC().Format(time.RFC3339)
	}
	var sec int64
	if _, err := fmt.Sscanf(parts[0], "%d", &sec); err != nil {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}
```

- [ ] **Step 4: Build**

```bash
go build ./internal/jira/... ./internal/slack/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/slack/store/models.go internal/slack/store/slack_repo.go internal/jira/service/slack_provider.go
git commit -m "feat(slack): write message records with author resolution during sync"
```

---

## Task 8: Update VCS provider to write messages

**Files:**
- Modify: `internal/vcs/store/models.go`
- Modify: `internal/vcs/store/pr_repo.go`
- Modify: `internal/vcs/service/vcs_provider.go`

- [ ] **Step 1: Add `MessageID *string` to PullRequest and PRComment**

```go
type PullRequest struct {
	// ... existing fields unchanged ...
	MessageID *string     `json:"messageId,omitempty"`
	Comments  []PRComment `json:"comments,omitempty"`
}

type PRComment struct {
	// ... existing fields unchanged ...
	MessageID *string     `json:"messageId,omitempty"`
	Replies   []PRComment `json:"replies,omitempty"`
}
```

- [ ] **Step 2: Add SetPRMessageID and SetPRCommentMessageID to pr_repo.go**

```go
// SetPRMessageID sets the message_id FK on a vcs_pull_requests row.
func (r *PRRepo) SetPRMessageID(ctx context.Context, prID, messageID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE vcs_pull_requests SET message_id = ? WHERE id = ?", messageID, prID)
	return err
}

// SetPRCommentMessageID sets the message_id FK on a vcs_pr_comments row.
func (r *PRRepo) SetPRCommentMessageID(ctx context.Context, commentID, messageID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE vcs_pr_comments SET message_id = ? WHERE id = ?", messageID, commentID)
	return err
}
```

- [ ] **Step 3: Inject MessageWriter into VCSProvider**

In `internal/vcs/service/vcs_provider.go`:

```go
import (
	appstore "github.com/artschekoff/context-consolidator/internal/store"
)

type VCSProvider struct {
	store     *vcsstore.Store
	runners   map[vcs.Platform]vcs.VCSRunner
	logger    *zap.Logger
	notify    NotifyFunc
	msgWriter appstore.MessageWriter
}

func NewVCSProvider(store *vcsstore.Store, runners map[vcs.Platform]vcs.VCSRunner, logger *zap.Logger, notify NotifyFunc, msgWriter appstore.MessageWriter) *VCSProvider {
	return &VCSProvider{store: store, runners: runners, logger: logger, notify: notify, msgWriter: msgWriter}
}
```

- [ ] **Step 4: Write message after UpsertPR (find the existing UpsertPR call ~line 195)**

```go
result, err := p.store.PullRequests.UpsertPR(ctx, repoID, branchID, prNumber,
    title, author, state, reviewStatus, url, headBranch, mergeable, ciStatus, prCreatedAt, prUpdatedAt)
if err != nil { /* existing error handling */ }

prExtID := repo.Owner + "/" + repo.Name + "#" + fmt.Sprint(prNumber)
prAuthorID := p.msgWriter.ResolveAuthor(ctx, "github", author)
prMsgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
    ContextID:  contextID,
    SourceID:   "github",
    ExternalID: prExtID,
    Title:      title,
    RawAuthor:  author,
    AuthorID:   prAuthorID,
    CreatedAt:  prCreatedAt,
    UpdatedAt:  prUpdatedAt,
})
if prMsgID != "" {
    _ = p.store.PullRequests.SetPRMessageID(ctx, result.ID, prMsgID)
}
```

- [ ] **Step 5: Write message after UpsertComment (find the existing UpsertComment call)**

```go
cID, err := p.store.PullRequests.UpsertComment(ctx, prID, c.ExternalID, c.ParentID, ...)
if err == nil && prMsgID != "" {
    cAuthorID := p.msgWriter.ResolveAuthor(ctx, "github", c.Author)
    cMsgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
        ContextID:  contextID,
        SourceID:   "github",
        ExternalID: c.ExternalID,
        ParentID:   prMsgID,
        Body:       c.Body,
        RawAuthor:  c.Author,
        AuthorID:   cAuthorID,
        CreatedAt:  c.CreatedAt,
        UpdatedAt:  c.UpdatedAt,
    })
    if cMsgID != "" {
        _ = p.store.PullRequests.SetPRCommentMessageID(ctx, cID, cMsgID)
    }
}
```

- [ ] **Step 6: Build**

```bash
go build ./internal/vcs/...
```

- [ ] **Step 7: Commit**

```bash
git add internal/vcs/store/models.go internal/vcs/store/pr_repo.go internal/vcs/service/vcs_provider.go
git commit -m "feat(vcs): write message records with author resolution during sync"
```

---

## Task 9: Wire MessageWriter into all providers at startup

**Files:**
- Modify: `internal/jira/service/service.go`
- Modify: `app.go`

- [ ] **Step 1: Update NewContextService to pass msgWriter to provider constructors**

In `internal/jira/service/service.go`, change the signature to accept `msgWriter appstore.MessageWriter` and pass it to `NewJiraProvider` and `NewSlackProvider`:

```go
func NewContextService(s *appstore.Store, j JiraAPI, jiraRepo *jirastore.Repo, slackRepo *slackstore.Repo,
    slack SlackRunner, msgWriter appstore.MessageWriter, logger *zap.Logger, providers ...provider.Provider) *ContextService {
```

Inside the function body, providers are now passed as variadic — the caller constructs them with `msgWriter` before calling `NewContextService`. No internal provider construction needed here (the caller already does it).

- [ ] **Step 2: Find the NewContextService call in app.go and update it**

```go
// Construct providers with MessageWriter (the store itself satisfies the interface).
jiraProvider  := service.NewJiraProvider(jiraRepo, jiraClient, s, logger)
slackProvider := service.NewSlackProvider(slackRepo, slackRunner, s, logger)
contextSvc    := service.NewContextService(s, jiraClient, jiraRepo, slackRepo, slackRunner, s, logger,
    jiraProvider, slackProvider)
```

- [ ] **Step 3: Find the NewVCSProvider call in app.go and add msgWriter**

```go
vcsProvider := vcsservice.NewVCSProvider(vcsStore, runners, logger, notifyFunc, s)
```

- [ ] **Step 4: Build entire project**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 5: Run existing tests**

```bash
go test ./... -timeout 60s
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/jira/service/service.go app.go
git commit -m "feat: wire MessageWriter into all provider constructors"
```

---

## Task 10: Update analysis service to use messages

**Files:**
- Modify: `internal/analysis/service.go`

- [ ] **Step 1: Add `listMessages` helper (append to service.go)**

```go
func (s *Service) listMessages(ctx context.Context, contextID string, since time.Time, sourceIDs ...string) ([]appstore.Message, error) {
	return s.store.ListMessagesSince(ctx, contextID, since, sourceIDs...)
}
```

- [ ] **Step 2: Add `formatMessages` helper**

```go
// formatMessages renders messages into a text block for AI prompts.
// Children (parent_id != nil) are indented under their parent.
func formatMessages(msgs []appstore.Message) string {
	if len(msgs) == 0 {
		return ""
	}
	byParent := make(map[string][]appstore.Message)
	var roots []appstore.Message
	for _, m := range msgs {
		if m.ParentID == nil {
			roots = append(roots, m)
		} else {
			byParent[*m.ParentID] = append(byParent[*m.ParentID], m)
		}
	}

	var b strings.Builder
	var write func(m appstore.Message, indent string)
	write = func(m appstore.Message, indent string) {
		author := m.AuthorDisplayName
		if author == "" {
			author = m.RawAuthor
		}
		if author == "" {
			author = "unknown"
		}
		label := m.SourceLabel
		if label == "" {
			label = strings.ToUpper(m.SourceID)
		}
		fmt.Fprintf(&b, "\n%s=== [%s] %s (by %s) ===\n", indent, label, m.CreatedAt, author)
		if m.Title != "" {
			fmt.Fprintf(&b, "%sTitle: %s\n", indent, m.Title)
		}
		if m.Body != "" {
			fmt.Fprintf(&b, "%s%s\n", indent, m.Body)
		}
		for _, child := range byParent[m.ID] {
			write(child, indent+"  ")
		}
	}
	for _, root := range roots {
		write(root, "")
	}
	return b.String()
}
```

- [ ] **Step 3: Replace FormatContext call #1 (line ~316, event extraction)**

Replace:
```go
var buf strings.Builder
if err := s.jiraSvc.FormatContext(ctx, &buf, contextID, "", deltaStart, true); err != nil {
    s.logger.Warn("analysis: format context failed", zap.String("contextID", contextID), zap.Error(err))
}
rawContent := buf.String()
```

With:
```go
msgs, err := s.listMessages(ctx, contextID, deltaStart)
if err != nil {
    s.logger.Warn("analysis: list messages failed", zap.String("contextID", contextID), zap.Error(err))
}
rawContent := formatMessages(msgs)
```

- [ ] **Step 4: Replace FormatContext calls #2–4 (~lines 533, 593, 649)**

These use `filter="jira"`. Replace each:
```go
var rawBuf strings.Builder
if err := s.jiraSvc.FormatContext(ctx, &rawBuf, contextID, "jira", time.Time{}, false); err != nil { ... }
rawContent := rawBuf.String()
```

With:
```go
allMsgs, err := s.listMessages(ctx, contextID, time.Time{}, "jira")
if err != nil {
    s.logger.Warn("analysis: list messages failed", zap.String("contextID", contextID), zap.Error(err))
}
rawContent := formatMessages(allMsgs)
```

- [ ] **Step 5: Build + test**

```bash
go build ./internal/analysis/...
go test ./... -timeout 60s
```
Expected: no errors, all tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/analysis/service.go
git commit -m "feat(analysis): replace FormatContext with message-layer listMessages + formatMessages"
```

---

## Task 11: Add app-layer methods + startup backfill

**Files:**
- Modify: `app.go`

- [ ] **Step 1: Add these methods to app.go**

```go
// ListOrgUsers returns all users for an org (empty orgID = all orgs).
func (a *App) ListOrgUsers(orgID string) ([]store.User, error) {
    return a.store.ListUsers(a.ctx, orgID)
}

// CreateOrgUser creates a new org-level user.
func (a *App) CreateOrgUser(displayName, email, orgID string) (*store.User, error) {
    return a.store.CreateUser(a.ctx, store.CreateUserArgs{
        OrgID: orgID, DisplayName: displayName, Email: email,
    })
}

// GetUnresolvedAuthors returns unresolved (source, rawAuthor) pairs for a context.
func (a *App) GetUnresolvedAuthors(contextID string) ([]store.UnresolvedAuthor, error) {
    return a.store.GetUnresolvedAuthors(a.ctx, contextID)
}

// AssignAuthorBatch records author mappings and back-fills author_id on messages.
func (a *App) AssignAuthorBatch(assignments []store.AuthorAssignment) error {
    return a.store.AssignAuthorBatch(a.ctx, assignments)
}

// CreateUserAndAssign creates a new org user and maps (sourceID, rawAuthor) → that user.
func (a *App) CreateUserAndAssign(displayName, email, orgID, sourceID, rawAuthor string) (*store.User, error) {
    return a.store.CreateUserAndAssign(a.ctx,
        store.CreateUserArgs{OrgID: orgID, DisplayName: displayName, Email: email},
        sourceID, rawAuthor,
    )
}

// ListMessages returns all messages for a context ordered by created_at ASC.
func (a *App) ListMessages(contextID string) ([]store.Message, error) {
    return a.store.ListMessagesSince(a.ctx, contextID, time.Time{})
}
```

- [ ] **Step 2: Add BackfillMessages to startup**

Find `func (a *App) startup(ctx context.Context)` and after the DB migration block:

```go
if err := a.store.BackfillMessages(ctx); err != nil {
    a.logger.Warn("startup: backfill messages failed", zap.Error(err))
}
```

- [ ] **Step 3: Regenerate Wails bindings**

```bash
wails generate module
```
Expected: `frontend/wailsjs/go/main/App.d.ts` updated with new methods.

- [ ] **Step 4: Build + test**

```bash
go build ./...
go test ./... -timeout 60s
```

- [ ] **Step 5: Commit**

```bash
git add app.go frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/main/App.js
git commit -m "feat(app): add user/author management endpoints and BackfillMessages on startup"
```

---

## Task 12: Frontend — `UnresolvedAuthorsModal`

**Files:**
- Create: `frontend/src/components/UnresolvedAuthorsModal.tsx`
- Modify: wherever `RefreshContext` / `RefreshAllContexts` is called (find via `grep -r "RefreshContext" frontend/src --include="*.tsx" -l`)

- [ ] **Step 1: Write the modal component**

```tsx
import { useState } from "react";
import {
  GetUnresolvedAuthors,
  AssignAuthorBatch,
  CreateUserAndAssign,
  ListOrgUsers,
} from "../../wailsjs/go/main/App";
import type {
  store_UnresolvedAuthor as UnresolvedAuthor,
  store_User as User,
  store_AuthorAssignment as AuthorAssignment,
} from "../../wailsjs/go/models";

interface Props {
  contextID: string;
  orgID: string;
  onClose: () => void;
}

type Resolution =
  | { type: "existing"; userID: string }
  | { type: "new"; displayName: string; email: string }
  | { type: "skip" };

export function UnresolvedAuthorsModal({ contextID, orgID, onClose }: Props) {
  const [unresolved, setUnresolved] = useState<UnresolvedAuthor[]>([]);
  const [orgUsers, setOrgUsers] = useState<User[]>([]);
  const [resolutions, setResolutions] = useState<Record<string, Resolution>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  // Load data on mount
  useState(() => {
    Promise.all([
      GetUnresolvedAuthors(contextID),
      ListOrgUsers(orgID),
    ]).then(([authors, users]) => {
      setUnresolved(authors ?? []);
      setOrgUsers(users ?? []);
      setLoading(false);
    });
  });

  const key = (a: UnresolvedAuthor) => `${a.sourceId}::${a.rawAuthor}`;

  const setResolution = (a: UnresolvedAuthor, r: Resolution) =>
    setResolutions((prev) => ({ ...prev, [key(a)]: r }));

  const handleConfirm = async () => {
    setSaving(true);
    const assignments: AuthorAssignment[] = [];

    for (const author of unresolved) {
      const r = resolutions[key(author)];
      if (!r || r.type === "skip") continue;

      if (r.type === "existing") {
        assignments.push({
          sourceId: author.sourceId,
          rawAuthor: author.rawAuthor,
          userId: r.userID,
        });
      } else if (r.type === "new" && r.displayName.trim()) {
        // Create user + mapping server-side and get back the new user.
        const newUser = await CreateUserAndAssign(
          r.displayName.trim(),
          r.email,
          orgID,
          author.sourceId,
          author.rawAuthor,
        );
        if (newUser) {
          setOrgUsers((prev) => [...prev, newUser]);
        }
      }
    }

    if (assignments.length > 0) {
      await AssignAuthorBatch(assignments);
    }
    setSaving(false);
    onClose();
  };

  if (loading) return null;
  if (unresolved.length === 0) { onClose(); return null; }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white dark:bg-zinc-900 rounded-xl shadow-2xl w-full max-w-lg max-h-[80vh] flex flex-col">
        <div className="px-6 py-4 border-b border-zinc-200 dark:border-zinc-700">
          <h2 className="text-base font-semibold">Assign Unknown Authors</h2>
          <p className="text-sm text-zinc-500 mt-0.5">
            {unresolved.length} author{unresolved.length !== 1 ? "s" : ""} found in this context with no org assignment.
          </p>
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4">
          {unresolved.map((author) => (
            <AuthorRow
              key={key(author)}
              author={author}
              orgUsers={orgUsers}
              resolution={resolutions[key(author)]}
              onChange={(r) => setResolution(author, r)}
            />
          ))}
        </div>

        <div className="px-6 py-4 border-t border-zinc-200 dark:border-zinc-700 flex justify-end gap-2">
          <button
            onClick={onClose}
            className="px-4 py-1.5 text-sm rounded-lg border border-zinc-300 dark:border-zinc-600 hover:bg-zinc-50 dark:hover:bg-zinc-800"
          >
            Skip all
          </button>
          <button
            onClick={handleConfirm}
            disabled={saving}
            className="px-4 py-1.5 text-sm rounded-lg bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-50"
          >
            {saving ? "Saving…" : "Confirm"}
          </button>
        </div>
      </div>
    </div>
  );
}

// --- AuthorRow sub-component ---

interface AuthorRowProps {
  author: UnresolvedAuthor;
  orgUsers: User[];
  resolution: Resolution | undefined;
  onChange: (r: Resolution) => void;
}

function AuthorRow({ author, orgUsers, resolution, onChange }: AuthorRowProps) {
  const [mode, setMode] = useState<"select" | "create">("select");
  const [newName, setNewName] = useState(author.rawAuthor);
  const [newEmail, setNewEmail] = useState("");

  return (
    <div className="rounded-lg border border-zinc-200 dark:border-zinc-700 p-3 space-y-2">
      <div className="flex items-center gap-2">
        <span className="text-xs uppercase tracking-wide text-zinc-400 w-14 shrink-0">
          {author.sourceLabel}
        </span>
        <span className="font-medium text-sm">{author.rawAuthor}</span>
        <span className="ml-auto text-xs text-zinc-400">
          {author.messageCount} msg{author.messageCount !== 1 ? "s" : ""}
        </span>
      </div>

      <div className="flex gap-2 text-xs">
        <button
          onClick={() => setMode("select")}
          className={`px-2 py-0.5 rounded ${mode === "select" ? "bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300" : "text-zinc-500 hover:text-zinc-700"}`}
        >
          Assign existing
        </button>
        <button
          onClick={() => setMode("create")}
          className={`px-2 py-0.5 rounded ${mode === "create" ? "bg-indigo-100 text-indigo-700 dark:bg-indigo-900 dark:text-indigo-300" : "text-zinc-500 hover:text-zinc-700"}`}
        >
          Create new
        </button>
      </div>

      {mode === "select" && (
        <select
          className="w-full text-sm rounded border border-zinc-300 dark:border-zinc-600 px-2 py-1 bg-white dark:bg-zinc-800"
          value={resolution?.type === "existing" ? resolution.userID : ""}
          onChange={(e) =>
            e.target.value
              ? onChange({ type: "existing", userID: e.target.value })
              : onChange({ type: "skip" })
          }
        >
          <option value="">— skip for now —</option>
          {orgUsers.map((u) => (
            <option key={u.id} value={u.id}>
              {u.displayName}
              {u.email ? ` (${u.email})` : ""}
            </option>
          ))}
        </select>
      )}

      {mode === "create" && (
        <div className="space-y-1">
          <input
            className="w-full text-sm rounded border border-zinc-300 dark:border-zinc-600 px-2 py-1 bg-white dark:bg-zinc-800"
            placeholder="Display name"
            value={newName}
            onChange={(e) => {
              setNewName(e.target.value);
              onChange({ type: "new", displayName: e.target.value, email: newEmail });
            }}
          />
          <input
            className="w-full text-sm rounded border border-zinc-300 dark:border-zinc-600 px-2 py-1 bg-white dark:bg-zinc-800"
            placeholder="Email (optional)"
            value={newEmail}
            onChange={(e) => {
              setNewEmail(e.target.value);
              onChange({ type: "new", displayName: newName, email: e.target.value });
            }}
          />
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Wire the modal into the refresh flow**

Find where `RefreshContext` result is handled. Search:

```bash
grep -r "RefreshContext\|RefreshAllContexts" frontend/src --include="*.tsx" -l
```

In that component, after the refresh resolves, check for unresolved authors:

```tsx
import { useState } from "react";
import { UnresolvedAuthorsModal } from "./UnresolvedAuthorsModal";

// Inside the component:
const [showAuthorModal, setShowAuthorModal] = useState(false);

// After RefreshContext() resolves:
const handleRefresh = async () => {
    await RefreshContext(contextID);
    // Check if there are unresolved authors; if yes, show modal.
    setShowAuthorModal(true);
};

// In JSX:
{showAuthorModal && (
    <UnresolvedAuthorsModal
        contextID={contextID}
        orgID={orgID}
        onClose={() => setShowAuthorModal(false)}
    />
)}
```

The modal self-dismisses if `GetUnresolvedAuthors` returns an empty list (no unknown authors).

- [ ] **Step 3: Verify no TypeScript errors**

```bash
cd frontend && npx tsc --noEmit
```
Expected: no errors.

- [ ] **Step 4: Start dev server and test manually**

```bash
wails dev
```

Test flow:
1. Open a context that has Jira or Slack data
2. Click Refresh
3. Modal appears showing any unresolved authors
4. Assign one to an existing user → click Confirm → modal closes
5. Refresh again → modal should not appear for the assigned author
6. Create a new user for an unresolved author → confirm → new user appears in org users list

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/UnresolvedAuthorsModal.tsx
git add -p  # stage only the refresh-trigger change in the context component
git commit -m "feat(frontend): add UnresolvedAuthorsModal for post-refresh author assignment"
```

---

## Task 13: Wire org users into `GeneratePersonAssignments`

**Files:**
- Modify: `internal/analysis/service.go:521-571` — `GeneratePersonAssignments`, `defaultPersonAssignmentPrompt`
- Modify: `app.go` — `GeneratePersonAssignments` app method (no signature change; wiring already done in Task 11)

### Context

Currently `GeneratePersonAssignments` asks the AI to *infer* people from Jira/Slack text: "Identify every person who is an assignee or has commented…". Now that `users` contains verified org members (Task 1), the AI must be given that roster explicitly so it only generates assignments for known people.

The `store.ListUsers` method created in Task 4 returns `[]User` with `ID` and `DisplayName`. The analysis service already holds a `store` reference.

---

- [ ] **Step 1: Write the failing test**

Add to `internal/analysis/service_test.go`:

```go
func TestGeneratePersonAssignments_UsesOrgUsers(t *testing.T) {
    // Arrange: seed two org users
    orgUsers := []appstore.User{
        {ID: "u1", DisplayName: "Alice"},
        {ID: "u2", DisplayName: "Bob"},
    }
    mockStore.ListUsersFunc = func(ctx context.Context, orgID string) ([]appstore.User, error) {
        return orgUsers, nil
    }

    var capturedPrompt string
    mockAI.ChatCompletionFunc = func(ctx context.Context, msgs []openai.Message) (*openai.Response, error) {
        capturedPrompt = msgs[0].Content
        return &openai.Response{Content: `[{"name":"Alice","body":"## Alice"}]`}, nil
    }

    // Act
    _, err := svc.GeneratePersonAssignments(context.Background(), "ctx-1")
    require.NoError(t, err)

    // Assert: roster injected into prompt
    assert.Contains(t, capturedPrompt, "Alice")
    assert.Contains(t, capturedPrompt, "Bob")
    // Assert: AI must NOT be told to discover people on its own
    assert.NotContains(t, capturedPrompt, "Identify every person")
}
```

Run: `go test ./internal/analysis/... -run TestGeneratePersonAssignments_UsesOrgUsers -v`
Expected: FAIL — `ListUsersFunc` field does not exist / prompt assertion fails.

- [ ] **Step 2: Add `ListUsers` method to the store mock**

The mock in `internal/analysis/service_test.go` (or wherever the test mock lives) needs a `ListUsersFunc` field. Add:

```go
// in the mock Store struct
ListUsersFunc func(ctx context.Context, orgID string) ([]appstore.User, error)

func (m *mockStore) ListUsers(ctx context.Context, orgID string) ([]appstore.User, error) {
    if m.ListUsersFunc != nil {
        return m.ListUsersFunc(ctx, orgID)
    }
    return nil, nil
}
```

Run: `go test ./internal/analysis/... -run TestGeneratePersonAssignments_UsesOrgUsers -v`
Expected: FAIL — prompt still contains "Identify every person".

- [ ] **Step 3: Add `{{org_users}}` placeholder to `defaultPersonAssignmentPrompt`**

In `internal/analysis/service.go`, replace the existing `defaultPersonAssignmentPrompt` constant:

```go
defaultPersonAssignmentPrompt = `You are an AI analyst generating detailed, technical task assignments for each participant in a project.

## Context key: {{context_key}}

## Org team members
The following people are the confirmed members of this organisation. Generate an assignment entry for EACH of them (do not add extra people):
{{org_users}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
For each person listed in "Org team members" produce an entry in the JSON array below.
If a person has no relevant work found in the source data, still include them with a brief "No active tasks found" body.

The "body" field must be rich Markdown covering ALL of the following that are relevant:
- **Current assignment**: which specific Jira issues/subtasks they own, their status
- **Technical context**: exact service names, API endpoints, DB tables, functions, modules, branches, PRs relevant to their work
- **Infrastructure details**: cloud services, queues, cron jobs, feature flags, env vars they need to touch
- **Business entities & rules**: domain objects, business logic constraints, data contracts they must respect
- **Methods & approaches**: specific algorithms, patterns, or architectural decisions already agreed on
- **Next concrete actions**: step-by-step tasks derived from issue state and event log
- **Blockers & dependencies**: open questions or blockers that directly affect them

Do NOT summarise vaguely. Include exact names, IDs, values, and references found in the source data.

Return ONLY valid JSON — no markdown fences, no preamble:
[
  {
    "name": "Person Name",
    "body": "## Person Name\n\n### Current Assignment\n...\n\n### Technical Context\n..."
  }
]`
```

- [ ] **Step 4: Build `formatOrgUsers` helper**

Add to `internal/analysis/service.go` (near `formatEventsForPrompt`):

```go
// formatOrgUsers renders org users as a simple bulleted list for prompt injection.
func formatOrgUsers(users []appstore.User) string {
    if len(users) == 0 {
        return "(no org users configured)"
    }
    var b strings.Builder
    for _, u := range users {
        b.WriteString("- ")
        b.WriteString(u.DisplayName)
        b.WriteByte('\n')
    }
    return b.String()
}
```

- [ ] **Step 5: Fetch org users and inject into `GeneratePersonAssignments`**

Replace the body of `GeneratePersonAssignments` in `internal/analysis/service.go`:

```go
func (s *Service) GeneratePersonAssignments(ctx context.Context, contextID string) ([]appstore.PersonAssignment, error) {
    ctxRow, err := s.store.GetContext(ctx, contextID)
    if err != nil {
        return nil, fmt.Errorf("get context: %w", err)
    }

    orgUsers, err := s.store.ListUsers(ctx, ctxRow.OrganizationID)
    if err != nil {
        s.logger.Warn("person assignment: list org users failed",
            zap.String("contextID", contextID), zap.Error(err))
        // proceed with empty list — AI will still process raw content
    }

    allEvents, err := s.store.ListEvents(ctx, contextID)
    if err != nil {
        return nil, fmt.Errorf("list events: %w", err)
    }

    var rawBuf strings.Builder
    if err := s.jiraSvc.FormatContext(ctx, &rawBuf, contextID, "jira", time.Time{}, false); err != nil {
        s.logger.Warn("person assignment: format jira context failed",
            zap.String("contextID", contextID), zap.Error(err))
    }

    promptEvents := make([]eventForPrompt, 0, len(allEvents))
    for _, e := range allEvents {
        promptEvents = append(promptEvents, eventForPrompt{
            ID: e.ID, Type: e.Type, Timestamp: e.Timestamp,
            SourceType: e.SourceType, Author: e.Author, Summary: e.Summary,
        })
    }

    tmpl := s.loadPrompt(ctx, "person_assignment", defaultPersonAssignmentPrompt)
    prompt := applyPlaceholders(tmpl, map[string]string{
        "context_key":         ctxRow.Key,
        "org_users":           formatOrgUsers(orgUsers),
        "raw_content":         rawBuf.String(),
        "events":              formatEventsForPrompt(promptEvents),
        "custom_instructions": "",
    })

    resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
    if err != nil {
        return nil, fmt.Errorf("ai person assignment: %w", err)
    }

    parsed, err := parsePersonAssignmentArray(resp.Content)
    if err != nil {
        return nil, fmt.Errorf("parse person assignment response: %w", err)
    }

    now := time.Now().UTC().Format(time.RFC3339)
    for i := range parsed {
        parsed[i].GeneratedAt = &now
    }

    return s.store.ReplacePersonAssignments(ctx, contextID, parsed)
}
```

Note: `ctxRow.OrganizationID` — confirm the field name on the `Context` struct in `internal/store/models.go`. If it is `OrgID`, adjust accordingly.

- [ ] **Step 6: Run tests**

```bash
go test ./internal/analysis/... -v
```

Expected: `TestGeneratePersonAssignments_UsesOrgUsers` PASS; all other analysis tests PASS.

- [ ] **Step 7: Compile check**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add internal/analysis/service.go internal/analysis/service_test.go
git commit -m "feat(analysis): inject org users roster into GeneratePersonAssignments prompt"
```

---

## Self-Review

### Spec Coverage

| Requirement | Task |
|---|---|
| `sources` dictionary table | Task 1 |
| `users` = org-level people only (no source accounts) | Task 1 |
| `author_mappings` table: (source, name) → user | Task 1 |
| `messages.raw_author` — verbatim author from source | Task 1 |
| `message_id` FK on all provider tables | Task 2 |
| Go models: `Source`, `User`, `AuthorMapping`, `UnresolvedAuthor`, `Message` | Task 3 |
| `UserRepo`, `AuthorRepo`, `MessageRepo` | Task 4 |
| `MessageWriter` interface (`ResolveAuthor` + `UpsertMessage`) | Task 5 |
| Jira sync writes messages with author resolution | Task 6 |
| Slack sync writes messages with author resolution | Task 7 |
| GitHub sync writes messages with author resolution | Task 8 |
| All providers wired at startup | Task 9 |
| AI pipeline reads messages only | Task 10 |
| App-layer endpoints for user/author management + backfill | Task 11 |
| Frontend modal: post-refresh, assign unknown authors | Task 12 |
| `GeneratePersonAssignments` uses org users roster | Task 13 |

### Type Consistency

- `UpsertMessageArgs.SourceID` / `AuthorID` / `RawAuthor` — consistent across Tasks 4–8 ✓
- `MessageWriter.ResolveAuthor` returns `string` (empty = not found) — used in Tasks 6–8 ✓
- `store.AuthorAssignment` used in both `AssignAuthorBatch` and frontend IPC ✓
- `formatMessages` uses `m.AuthorDisplayName` then `m.RawAuthor` as fallback ✓
- `formatOrgUsers([]appstore.User)` → `string` — used only in Task 13 `GeneratePersonAssignments` ✓
- `ctxRow.OrganizationID` — verify field name on `Context` struct before running (may be `OrgID`) ✓
- Modal self-dismisses when `GetUnresolvedAuthors` returns empty slice ✓
