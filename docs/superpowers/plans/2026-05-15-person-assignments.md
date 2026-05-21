# Person Assignments Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the single-blob task assignment feature with per-person assignment records — each with an extracted `name` and a rich `body` that includes technical implementation details, infrastructure specifics, business entities, and exact method/approach guidance.

**Architecture:** Migration 054 drops `context_task_assignments` and creates `context_person_assignments` (one row per person per context). The AI prompt returns a JSON array so the service can persist each person as a separate DB row. The frontend renders a scrollable list of person cards, each independently editable and refineable via AI instruction.

**Tech Stack:** Go 1.22, SQLite `database/sql`, Wails v2 IPC, React 19, TypeScript

---

## File Map

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `internal/store/migrations/054_person_assignments.sql` | Drop old table, create `context_person_assignments`, replace AI prompts |
| Modify | `internal/store/models.go` | Replace `TaskAssignment` with `PersonAssignment` |
| Replace | `internal/store/task_assignment_repo.go` → `internal/store/person_assignment_repo.go` | List, Replace, UpdateBody, GetOne |
| Modify | `internal/store/ai_prompts_repo.go` | Replace placeholder registrations |
| Modify | `internal/analysis/service.go` | New prompt constants, JSON parser, updated Generate/Refine methods |
| Modify | `app_workflow.go` | Remove 4 old commands, add 4 new |
| Modify | `frontend/wailsjs/go/main/App.js` | Replace bindings |
| Modify | `frontend/wailsjs/go/main/App.d.ts` | Replace type signatures |
| Modify | `frontend/src/types/index.ts` | Replace `TaskAssignment` with `PersonAssignment` |
| Modify | `frontend/src/pages/ContextDetail.tsx` | Replace single-text UI with person cards list |

---

## Task 1: DB Migration — New Table + Updated AI Prompt Seeds

**Files:**
- Create: `internal/store/migrations/054_person_assignments.sql`

- [ ] **Step 1: Write the migration file**

```sql
-- +goose Up
DROP TABLE IF EXISTS context_task_assignments;

CREATE TABLE IF NOT EXISTS context_person_assignments (
    id           TEXT PRIMARY KEY,
    context_id   TEXT NOT NULL,
    name         TEXT NOT NULL,
    body         TEXT NOT NULL DEFAULT '',
    generated_at TEXT,
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS context_person_assignments_ctx_idx
    ON context_person_assignments(context_id);

-- Replace old single-blob prompt with JSON-returning prompt
DELETE FROM ai_prompts WHERE id IN ('task_assignment', 'task_assignment_refine');

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'person_assignment',
'Person Assignment Generator',
'You are an AI analyst generating detailed, technical task assignments for each participant in a project.

## Context key: {{context_key}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
Identify every person who is an assignee or has commented in the Jira issues above.
For each person produce an entry in the JSON array below.

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
]',
'You are an AI analyst generating detailed, technical task assignments for each participant in a project.

## Context key: {{context_key}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
Identify every person who is an assignee or has commented in the Jira issues above.
For each person produce an entry in the JSON array below.

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
]',
datetime(''now'')
);

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'person_assignment_refine',
'Person Assignment Refine',
'You are refining a single person''s task assignment document.

## Person: {{person_name}}

## Current document
{{current_body}}

## User instruction
{{user_instruction}}

## Jira issues and activity (for reference)
{{raw_content}}

## Event log (for reference)
{{events}}

Apply the user instruction. Preserve all technical details (service names, endpoints, DB tables, methods, infrastructure specifics) unless the instruction explicitly says to remove them.
Return ONLY the updated Markdown for this person''s body — no JSON, no preamble.',
'You are refining a single person''s task assignment document.

## Person: {{person_name}}

## Current document
{{current_body}}

## User instruction
{{user_instruction}}

## Jira issues and activity (for reference)
{{raw_content}}

## Event log (for reference)
{{events}}

Apply the user instruction. Preserve all technical details (service names, endpoints, DB tables, methods, infrastructure specifics) unless the instruction explicitly says to remove them.
Return ONLY the updated Markdown for this person''s body — no JSON, no preamble.',
datetime(''now'')
);

-- +goose Down
DROP TABLE IF EXISTS context_person_assignments;
DELETE FROM ai_prompts WHERE id IN ('person_assignment', 'person_assignment_refine');
-- Restore old table (empty — data is not recoverable after drop)
CREATE TABLE IF NOT EXISTS context_task_assignments (
    context_id   TEXT PRIMARY KEY,
    content      TEXT NOT NULL DEFAULT '',
    generated_at TEXT,
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
```

**Note on `datetime('now')` in SQL:** SQLite string literals use single quotes. The `datetime('now')` call uses single quotes internally — in a Go raw string this is fine as-is. Verify the migration applies cleanly.

- [ ] **Step 2: Verify build**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator && go build ./...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/store/migrations/054_person_assignments.sql
git commit -m "feat: add context_person_assignments table, replace task_assignment prompts"
```

---

## Task 2: Replace Model + Repository

**Files:**
- Modify: `internal/store/models.go`
- Delete content of: `internal/store/task_assignment_repo.go` (replace entirely)
- Create: `internal/store/person_assignment_repo.go`

- [ ] **Step 1: Replace `TaskAssignment` with `PersonAssignment` in `internal/store/models.go`**

Find and replace the `TaskAssignment` struct (lines ~242–248):

Remove:
```go
// TaskAssignment stores the AI-generated (or user-edited) per-participant assignment text for a context.
type TaskAssignment struct {
	ContextID   string  `json:"contextId"`
	Content     string  `json:"content"`
	GeneratedAt *string `json:"generatedAt,omitempty"`
	UpdatedAt   string  `json:"updatedAt"`
}
```

Add in its place:
```go
// PersonAssignment stores one AI-generated (or user-edited) task assignment for a single person.
type PersonAssignment struct {
	ID          string  `json:"id"`
	ContextID   string  `json:"contextId"`
	Name        string  `json:"name"`
	Body        string  `json:"body"`
	GeneratedAt *string `json:"generatedAt,omitempty"`
	UpdatedAt   string  `json:"updatedAt"`
}
```

- [ ] **Step 2: Overwrite `internal/store/task_assignment_repo.go` with the new person assignment repo**

Replace the entire file content with:

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ListPersonAssignments returns all person assignments for a context ordered by name.
func (s *Store) ListPersonAssignments(ctx context.Context, contextID string) ([]PersonAssignment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, context_id, name, body, generated_at, updated_at
		FROM context_person_assignments
		WHERE context_id = ?
		ORDER BY name ASC
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list person assignments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []PersonAssignment
	for rows.Next() {
		var pa PersonAssignment
		var generatedAt sql.NullString
		if err := rows.Scan(&pa.ID, &pa.ContextID, &pa.Name, &pa.Body, &generatedAt, &pa.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan person assignment: %w", err)
		}
		if generatedAt.Valid {
			pa.GeneratedAt = &generatedAt.String
		}
		result = append(result, pa)
	}
	return result, rows.Err()
}

// GetPersonAssignment returns a single person assignment by ID.
func (s *Store) GetPersonAssignment(ctx context.Context, id string) (*PersonAssignment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, context_id, name, body, generated_at, updated_at
		FROM context_person_assignments WHERE id = ?
	`, id)
	var pa PersonAssignment
	var generatedAt sql.NullString
	if err := row.Scan(&pa.ID, &pa.ContextID, &pa.Name, &pa.Body, &generatedAt, &pa.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get person assignment: %w", err)
	}
	if generatedAt.Valid {
		pa.GeneratedAt = &generatedAt.String
	}
	return &pa, nil
}

// ReplacePersonAssignments deletes all existing assignments for a context and inserts the new set.
// Runs in a single transaction. Returns the inserted rows with their generated IDs.
func (s *Store) ReplacePersonAssignments(ctx context.Context, contextID string, assignments []PersonAssignment) ([]PersonAssignment, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin replace person assignments: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM context_person_assignments WHERE context_id = ?`, contextID,
	); err != nil {
		return nil, fmt.Errorf("delete old person assignments: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO context_person_assignments (id, context_id, name, body, generated_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare insert person assignment: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	result := make([]PersonAssignment, 0, len(assignments))
	for _, a := range assignments {
		a.ID = uuid.New().String()
		a.ContextID = contextID
		a.UpdatedAt = now
		if _, err := stmt.ExecContext(ctx, a.ID, a.ContextID, a.Name, a.Body, a.GeneratedAt, a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("insert person assignment %q: %w", a.Name, err)
		}
		result = append(result, a)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit replace person assignments: %w", err)
	}
	return result, nil
}

// UpdatePersonAssignmentBody updates the body of a single person assignment (user manual edit).
func (s *Store) UpdatePersonAssignmentBody(ctx context.Context, id, body string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE context_person_assignments SET body = ?, updated_at = ? WHERE id = ?`,
		body, now, id,
	)
	if err != nil {
		return fmt.Errorf("update person assignment body: %w", err)
	}
	return nil
}
```

Note: rename the file from `task_assignment_repo.go` to `person_assignment_repo.go` by overwriting (the file name change is cosmetic since Go uses package names, but rename for clarity).

Actually — just overwrite `task_assignment_repo.go` in place; Go doesn't care about file names within a package.

- [ ] **Step 3: Verify build (will fail — fix dependents in later tasks)**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator && go build ./...
```

Expected: compile errors referencing `TaskAssignment`, `GetTaskAssignment`, `UpsertTaskAssignment`, `UpdateTaskAssignmentContent` in `service.go` and `app_workflow.go`. That is correct — those will be fixed in Tasks 3 and 4.

- [ ] **Step 4: Commit partial (do not include broken files)**

```bash
git add internal/store/models.go internal/store/task_assignment_repo.go
git commit -m "feat: replace TaskAssignment with PersonAssignment model and repo"
```

---

## Task 3: Update AI Prompt Placeholders + Analysis Service

**Files:**
- Modify: `internal/store/ai_prompts_repo.go`
- Modify: `internal/analysis/service.go`

### 3a — Update placeholder registrations

- [ ] **Step 1: Update `knownPlaceholders` in `internal/store/ai_prompts_repo.go`**

Find and remove the entries for `"task_assignment"` and `"task_assignment_refine"`. Replace with:

```go
"person_assignment": {
    {Name: "context_key", Description: "Project/module key of the context"},
    {Name: "raw_content", Description: "Formatted Jira issues and activity for this context"},
    {Name: "events", Description: "Full event log in chronological order"},
    {Name: "custom_instructions", Description: "Optional extra instructions section (empty by default)"},
},
"person_assignment_refine": {
    {Name: "person_name", Description: "The name of the person whose assignment is being refined"},
    {Name: "current_body", Description: "The current Markdown body for this person"},
    {Name: "user_instruction", Description: "The user's modification instruction"},
    {Name: "raw_content", Description: "Formatted Jira issues and activity (reference)"},
    {Name: "events", Description: "Full event log (reference)"},
},
```

### 3b — Replace analysis service methods

- [ ] **Step 2: Replace prompt constants in `internal/analysis/service.go`**

Find the two existing constants `defaultTaskAssignmentPrompt` and `defaultTaskAssignmentRefinePrompt` inside the `const (...)` block (lines ~124–159). Replace both with:

```go
defaultPersonAssignmentPrompt = `You are an AI analyst generating detailed, technical task assignments for each participant in a project.

## Context key: {{context_key}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
Identify every person who is an assignee or has commented in the Jira issues above.
For each person produce an entry in the JSON array below.

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

defaultPersonAssignmentRefinePrompt = `You are refining a single person's task assignment document.

## Person: {{person_name}}

## Current document
{{current_body}}

## User instruction
{{user_instruction}}

## Jira issues and activity (for reference)
{{raw_content}}

## Event log (for reference)
{{events}}

Apply the user instruction. Preserve all technical details (service names, endpoints, DB tables, methods, infrastructure specifics) unless the instruction explicitly says to remove them.
Return ONLY the updated Markdown for this person's body — no JSON, no preamble.`
```

- [ ] **Step 3: Replace `GenerateTaskAssignments` with `GeneratePersonAssignments`**

Find the existing `GenerateTaskAssignments` method (lines ~445–493) and replace it entirely:

```go
// GeneratePersonAssignments generates per-person technical assignment records from Jira issues
// and the event log. Replaces all existing assignments for the context and returns the new set.
func (s *Service) GeneratePersonAssignments(ctx context.Context, contextID string) ([]appstore.PersonAssignment, error) {
	ctxRow, err := s.store.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get context: %w", err)
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
	rawContent := rawBuf.String()

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
		"raw_content":         rawContent,
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

- [ ] **Step 4: Replace `RefineTaskAssignments` with `RefinePersonAssignment`**

Find the existing `RefineTaskAssignments` method (lines ~495–545) and replace it entirely:

```go
// RefinePersonAssignment refines a single person's assignment body based on a user instruction.
// Returns the updated PersonAssignment.
func (s *Service) RefinePersonAssignment(ctx context.Context, contextID, personID, userInstruction string) (*appstore.PersonAssignment, error) {
	existing, err := s.store.GetPersonAssignment(ctx, personID)
	if err != nil {
		return nil, fmt.Errorf("get person assignment: %w", err)
	}
	currentBody := ""
	personName := ""
	if existing != nil {
		currentBody = existing.Body
		personName = existing.Name
	}

	allEvents, err := s.store.ListEvents(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	var rawBuf strings.Builder
	if err := s.jiraSvc.FormatContext(ctx, &rawBuf, contextID, "jira", time.Time{}, false); err != nil {
		s.logger.Warn("person assignment refine: format jira context failed",
			zap.String("contextID", contextID), zap.Error(err))
	}
	rawContent := rawBuf.String()

	promptEvents := make([]eventForPrompt, 0, len(allEvents))
	for _, e := range allEvents {
		promptEvents = append(promptEvents, eventForPrompt{
			ID: e.ID, Type: e.Type, Timestamp: e.Timestamp,
			SourceType: e.SourceType, Author: e.Author, Summary: e.Summary,
		})
	}

	tmpl := s.loadPrompt(ctx, "person_assignment_refine", defaultPersonAssignmentRefinePrompt)
	prompt := applyPlaceholders(tmpl, map[string]string{
		"person_name":      personName,
		"current_body":     currentBody,
		"user_instruction": userInstruction,
		"raw_content":      rawContent,
		"events":           formatEventsForPrompt(promptEvents),
	})

	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, fmt.Errorf("ai person assignment refine: %w", err)
	}

	if err := s.store.UpdatePersonAssignmentBody(ctx, personID, resp.Content); err != nil {
		return nil, fmt.Errorf("save refined person assignment: %w", err)
	}

	return s.store.GetPersonAssignment(ctx, personID)
}
```

- [ ] **Step 5: Add `parsePersonAssignmentArray` helper**

Add this function after `parsePacketResponse` (or after the `extractJSON` helper):

```go
// parsePersonAssignmentArray parses the AI's JSON array of {name, body} objects.
func parsePersonAssignmentArray(content string) ([]appstore.PersonAssignment, error) {
	content = extractJSON(content, '[')
	var raw []struct {
		Name string `json:"name"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("unmarshal person assignments: %w (content: %.300s)", err, content)
	}
	result := make([]appstore.PersonAssignment, 0, len(raw))
	for _, r := range raw {
		if r.Name == "" {
			continue
		}
		result = append(result, appstore.PersonAssignment{Name: r.Name, Body: r.Body})
	}
	return result, nil
}
```

- [ ] **Step 6: Verify build (will still fail on app_workflow.go — fix in Task 4)**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator && go build ./...
```

Expected: errors only in `app_workflow.go` referencing old command names. `internal/analysis/service.go` and `internal/store/` should be clean.

- [ ] **Step 7: Commit**

```bash
git add internal/store/ai_prompts_repo.go internal/analysis/service.go
git commit -m "feat: add GeneratePersonAssignments and RefinePersonAssignment, replace AI prompts"
```

---

## Task 4: Replace App Commands

**Files:**
- Modify: `app_workflow.go`

- [ ] **Step 1: Remove old task assignment commands and add new person assignment commands**

Find the `--- Task Assignments ---` section in `app_workflow.go`:

```go
// --- Task Assignments ---

// GetTaskAssignment returns the saved task assignment for a context, or nil if none exists yet.
func (a *App) GetTaskAssignment(contextID string) (*appstore.TaskAssignment, error) {
	return a.db.GetTaskAssignment(context.Background(), contextID)
}

// GenerateTaskAssignment runs the AI task assignment pipeline and saves the result.
func (a *App) GenerateTaskAssignment(contextID string) (*appstore.TaskAssignment, error) {
	return a.analysisSvc.GenerateTaskAssignments(a.ctx, contextID)
}

// RefineTaskAssignment refines the existing task assignment document using the provided instruction.
func (a *App) RefineTaskAssignment(contextID, instruction string) (*appstore.TaskAssignment, error) {
	return a.analysisSvc.RefineTaskAssignments(a.ctx, contextID, instruction)
}

// SaveTaskAssignment persists a manually edited task assignment content.
func (a *App) SaveTaskAssignment(contextID, content string) error {
	return a.db.UpdateTaskAssignmentContent(context.Background(), contextID, content)
}
```

Replace that entire block with:

```go
// --- Person Assignments ---

// GetPersonAssignments returns all person assignment records for a context.
func (a *App) GetPersonAssignments(contextID string) ([]appstore.PersonAssignment, error) {
	return a.db.ListPersonAssignments(context.Background(), contextID)
}

// GeneratePersonAssignments runs the AI pipeline to produce per-person technical assignment records.
// Replaces all existing assignments for the context.
func (a *App) GeneratePersonAssignments(contextID string) ([]appstore.PersonAssignment, error) {
	return a.analysisSvc.GeneratePersonAssignments(a.ctx, contextID)
}

// RefinePersonAssignment refines a single person's assignment body using the provided instruction.
func (a *App) RefinePersonAssignment(contextID, personID, instruction string) (*appstore.PersonAssignment, error) {
	return a.analysisSvc.RefinePersonAssignment(a.ctx, contextID, personID, instruction)
}

// SavePersonAssignment persists a manually edited body for a person assignment.
func (a *App) SavePersonAssignment(personID, body string) error {
	return a.db.UpdatePersonAssignmentBody(context.Background(), personID, body)
}
```

- [ ] **Step 2: Verify full build**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator && go build ./...
```

Expected: **success** — no compile errors.

- [ ] **Step 3: Commit**

```bash
git add app_workflow.go
git commit -m "feat: replace task assignment commands with person assignment commands"
```

---

## Task 5: Wails Bindings + TypeScript Types

**Files:**
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/main/App.d.ts`
- Modify: `frontend/src/types/index.ts`

- [ ] **Step 1: Update `App.js` — remove old bindings, add new**

Find all four old task assignment functions in `App.js`:
```js
export function GetTaskAssignment(arg1) { ... }
export function GenerateTaskAssignment(arg1) { ... }
export function RefineTaskAssignment(arg1, arg2) { ... }
export function SaveTaskAssignment(arg1, arg2) { ... }
```

Replace them with:
```js
export function GetPersonAssignments(arg1) {
  return window['go']['main']['App']['GetPersonAssignments'](arg1);
}

export function GeneratePersonAssignments(arg1) {
  return window['go']['main']['App']['GeneratePersonAssignments'](arg1);
}

export function RefinePersonAssignment(arg1, arg2, arg3) {
  return window['go']['main']['App']['RefinePersonAssignment'](arg1, arg2, arg3);
}

export function SavePersonAssignment(arg1, arg2) {
  return window['go']['main']['App']['SavePersonAssignment'](arg1, arg2);
}
```

- [ ] **Step 2: Update `App.d.ts` — remove old signatures, add new**

Find the four old signatures:
```typescript
export function GetTaskAssignment(arg1:string):Promise<store.TaskAssignment|null>;
export function GenerateTaskAssignment(arg1:string):Promise<store.TaskAssignment>;
export function RefineTaskAssignment(arg1:string,arg2:string):Promise<store.TaskAssignment>;
export function SaveTaskAssignment(arg1:string,arg2:string):Promise<void>;
```

Replace with:
```typescript
export function GetPersonAssignments(arg1:string):Promise<Array<store.PersonAssignment>>;

export function GeneratePersonAssignments(arg1:string):Promise<Array<store.PersonAssignment>>;

export function RefinePersonAssignment(arg1:string,arg2:string,arg3:string):Promise<store.PersonAssignment>;

export function SavePersonAssignment(arg1:string,arg2:string):Promise<void>;
```

- [ ] **Step 3: Update `frontend/src/types/index.ts` — replace interface**

Find and remove:
```typescript
export interface TaskAssignment {
  contextId: string;
  content: string;
  generatedAt?: string;
  updatedAt: string;
}
```

Replace with:
```typescript
export interface PersonAssignment {
  id: string;
  contextId: string;
  name: string;
  body: string;
  generatedAt?: string;
  updatedAt: string;
}
```

- [ ] **Step 4: TypeScript check**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator/frontend && npx tsc -b --noEmit
```

Expected: errors in `ContextDetail.tsx` only (referencing old types/functions). That is correct — fixed in Task 6.

- [ ] **Step 5: Commit**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator
git add frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts frontend/src/types/index.ts
git commit -m "feat: replace task assignment bindings and types with person assignment"
```

---

## Task 6: Frontend UI — Person Assignment Cards

**Files:**
- Modify: `frontend/src/pages/ContextDetail.tsx`

This task replaces the entire Task Assignments section with a per-person card list.

### 6a: Imports

- [ ] **Step 1: Update Wails imports in `ContextDetail.tsx`**

In the import block from `../../wailsjs/go/main/App`, remove:
```typescript
  GetTaskAssignment,
  GenerateTaskAssignment,
  RefineTaskAssignment,
  SaveTaskAssignment,
```

Add:
```typescript
  GetPersonAssignments,
  GeneratePersonAssignments,
  RefinePersonAssignment,
  SavePersonAssignment,
```

- [ ] **Step 2: Update type import**

In the `import type { ... }` line, remove `TaskAssignment` and add `PersonAssignment`.

### 6b: State

- [ ] **Step 3: Replace state variables**

Find the 7 old task assignment state variables:
```typescript
const [taskAssignment, setTaskAssignment] = useState<TaskAssignment | null>(null);
const [isGeneratingAssignment, setIsGeneratingAssignment] = useState(false);
const [isEditingAssignment, setIsEditingAssignment] = useState(false);
const [editAssignmentContent, setEditAssignmentContent] = useState("");
const [showAssignmentAiForm, setShowAssignmentAiForm] = useState(false);
const [assignmentAiPrompt, setAssignmentAiPrompt] = useState("");
const [isRefiningAssignment, setIsRefiningAssignment] = useState(false);
```

Replace with:
```typescript
const [personAssignments, setPersonAssignments] = useState<PersonAssignment[]>([]);
const [isGeneratingAssignments, setIsGeneratingAssignments] = useState(false);
const [editingPersonId, setEditingPersonId] = useState<string | null>(null);
const [editPersonBody, setEditPersonBody] = useState("");
const [refiningPersonId, setRefiningPersonId] = useState<string | null>(null);
const [personAiPrompt, setPersonAiPrompt] = useState("");
const [isRefiningPerson, setIsRefiningPerson] = useState(false);
```

### 6c: loadSnapshot

- [ ] **Step 4: Update `loadSnapshot` to use new command**

Find the `Promise.all` in `loadSnapshot` that calls `GetTaskAssignment`. It currently looks like:
```typescript
const [snap, evts, ta] = await Promise.all([
  GetContextSnapshot(id) as Promise<ContextSnapshot>,
  ListEvents(id) as Promise<ContextEvent[]>,
  GetTaskAssignment(id).catch(() => null) as Promise<TaskAssignment | null>,
]);
setSnapshot(snap);
setEvents(evts ?? []);
setTaskAssignment(ta);
```

Replace with:
```typescript
const [snap, evts, pas] = await Promise.all([
  GetContextSnapshot(id) as Promise<ContextSnapshot>,
  ListEvents(id) as Promise<ContextEvent[]>,
  GetPersonAssignments(id).catch(() => []) as Promise<PersonAssignment[]>,
]);
setSnapshot(snap);
setEvents(evts ?? []);
setPersonAssignments(pas ?? []);
```

### 6d: UI Section

- [ ] **Step 5: Replace the Task Assignments `<details>` section**

Find the entire `{/* Task Assignments */}` block (it starts with `<details style={{ marginTop: 8 }}>` and ends with its closing `</details>`). Replace the entire block with:

```tsx
{/* Person Assignments */}
<details style={{ marginTop: 8 }}>
  <summary style={{ cursor: "pointer", fontWeight: 500, fontSize: 13, userSelect: "none" }}>
    Person Assignments
    {personAssignments.length > 0 && (
      <span style={{ fontWeight: 400, color: "var(--text-muted)", marginLeft: 6, fontSize: 11 }}>
        ({personAssignments.length})
      </span>
    )}
  </summary>
  <div style={{ marginTop: 6, display: "flex", flexDirection: "column", gap: 8 }}>
    {/* Generate button */}
    <div>
      <button
        className="btn-secondary"
        style={{ fontSize: 12, padding: "2px 10px" }}
        disabled={isGeneratingAssignments}
        onClick={async () => {
          setIsGeneratingAssignments(true);
          try {
            const pas = await GeneratePersonAssignments(id) as PersonAssignment[];
            setPersonAssignments(pas ?? []);
          } finally {
            setIsGeneratingAssignments(false);
          }
        }}
      >
        {isGeneratingAssignments ? "Generating…" : personAssignments.length > 0 ? "Re-generate" : "Generate"}
      </button>
    </div>

    {personAssignments.length === 0 && !isGeneratingAssignments && (
      <div style={{ fontSize: 12, color: "var(--text-muted)" }}>
        Not generated yet. Click Generate to create per-person assignments from Jira issues and events.
      </div>
    )}

    {/* One card per person */}
    {personAssignments.map((pa) => (
      <div
        key={pa.id}
        style={{ border: "1px solid var(--border)", borderRadius: 6, padding: "8px 10px", display: "flex", flexDirection: "column", gap: 6 }}
      >
        {/* Card header: name + action buttons */}
        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <span style={{ fontWeight: 600, fontSize: 13, flex: 1 }}>{pa.name}</span>
          {editingPersonId !== pa.id && refiningPersonId !== pa.id && (
            <>
              <button
                style={{ background: "none", border: "none", cursor: "pointer", padding: 2, color: "var(--text-muted)", lineHeight: 1 }}
                title="Edit"
                onClick={() => { setEditingPersonId(pa.id); setEditPersonBody(pa.body); }}
              >
                <Pencil size={12} />
              </button>
              <button
                style={{ background: "none", border: "none", cursor: "pointer", padding: 2, color: "var(--text-muted)", lineHeight: 1, fontSize: 11 }}
                title="Ask AI"
                onClick={() => { setRefiningPersonId(pa.id); setPersonAiPrompt(""); }}
              >
                AI
              </button>
            </>
          )}
        </div>

        {/* Edit mode */}
        {editingPersonId === pa.id ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <textarea
              className="passphrase-input"
              style={{ fontSize: 12, minHeight: 180, resize: "vertical", width: "100%", fontFamily: "inherit" }}
              value={editPersonBody}
              onChange={(e) => setEditPersonBody(e.target.value)}
            />
            <div style={{ display: "flex", gap: 4 }}>
              <button
                className="btn-primary"
                style={{ fontSize: 12, padding: "2px 10px" }}
                onClick={async () => {
                  await SavePersonAssignment(pa.id, editPersonBody);
                  setPersonAssignments((prev) =>
                    prev.map((p) => p.id === pa.id ? { ...p, body: editPersonBody } : p)
                  );
                  setEditingPersonId(null);
                }}
              >
                Save
              </button>
              <button
                className="btn-secondary"
                style={{ fontSize: 12, padding: "2px 10px" }}
                onClick={() => setEditingPersonId(null)}
              >
                Cancel
              </button>
            </div>
          </div>
        ) : refiningPersonId === pa.id ? (
          /* Ask AI mode */
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <div style={{ fontSize: 11, color: "var(--text-muted)" }}>
              Describe how to change {pa.name}{"'"}s assignment (e.g. "Add infrastructure details", "Focus on blockers")
            </div>
            <textarea
              className="passphrase-input"
              style={{ fontSize: 12, minHeight: 56, resize: "vertical", width: "100%", fontFamily: "inherit" }}
              placeholder="Your instruction…"
              value={personAiPrompt}
              onChange={(e) => setPersonAiPrompt(e.target.value)}
              autoFocus
            />
            <div style={{ display: "flex", gap: 4 }}>
              <button
                className="btn-primary"
                style={{ fontSize: 12, padding: "2px 10px" }}
                disabled={isRefiningPerson || !personAiPrompt.trim()}
                onClick={async () => {
                  if (!personAiPrompt.trim()) return;
                  setIsRefiningPerson(true);
                  try {
                    const updated = await RefinePersonAssignment(id, pa.id, personAiPrompt.trim()) as PersonAssignment;
                    setPersonAssignments((prev) =>
                      prev.map((p) => p.id === pa.id ? updated : p)
                    );
                    setRefiningPersonId(null);
                    setPersonAiPrompt("");
                  } finally {
                    setIsRefiningPerson(false);
                  }
                }}
              >
                {isRefiningPerson ? "Applying…" : "Apply"}
              </button>
              <button
                className="btn-secondary"
                style={{ fontSize: 12, padding: "2px 10px" }}
                onClick={() => { setRefiningPersonId(null); setPersonAiPrompt(""); }}
              >
                Cancel
              </button>
            </div>
            {/* Show current body for reference while editing */}
            <pre style={{ whiteSpace: "pre-wrap", fontSize: 11, margin: "4px 0 0", color: "var(--text-muted)", lineHeight: 1.4, maxHeight: 200, overflow: "auto" }}>
              {pa.body}
            </pre>
          </div>
        ) : (
          /* Display mode */
          <pre style={{ whiteSpace: "pre-wrap", fontSize: 12, margin: 0, color: "var(--text-muted)", lineHeight: 1.5 }}>
            {pa.body}
          </pre>
        )}
      </div>
    ))}
  </div>
</details>
```

- [ ] **Step 6: TypeScript check**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator/frontend && npx tsc -b --noEmit
```

Expected: no errors.

- [ ] **Step 7: Go build check**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator && go build ./...
```

Expected: success.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/pages/ContextDetail.tsx
git commit -m "feat: replace task assignment UI with per-person assignment cards"
```

---

## Self-Review

**Spec coverage:**
- "separate entity in database, with name and body" → `context_person_assignments` table, `PersonAssignment.Name` + `.Body` ✅
- "not hide technical details" → AI prompt explicitly instructs inclusion of service names, endpoints, DB tables, functions, infrastructure, business entities, methods ✅
- "re-generate" → Generate button replaces all assignments via `ReplacePersonAssignments` ✅
- Each person independently editable (Edit) and AI-refineable (Ask AI) ✅
- Persistence via `loadSnapshot` using `GetPersonAssignments` ✅

**Placeholder scan:** No TBDs or vague steps. All code blocks are complete.

**Type consistency:**
- `PersonAssignment` struct (Go) uses `json:"id"`, `json:"contextId"`, `json:"name"`, `json:"body"` — matches TypeScript interface field names throughout
- `ReplacePersonAssignments` returns `[]PersonAssignment` — matches `GeneratePersonAssignments` return type in service and app command
- `RefinePersonAssignment` takes `(contextID, personID, instruction)` in service and app command — consistent
- `App.d.ts` `RefinePersonAssignment(arg1, arg2, arg3)` matches 3-arg Go signature ✅
- `UpdatePersonAssignmentBody(ctx, id, body)` in repo matches `SavePersonAssignment(personID, body)` app command ✅
