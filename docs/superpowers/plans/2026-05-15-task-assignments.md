# Task Assignments Feature Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Task Assignments" section to the Context Detail page that uses AI to generate per-participant action summaries from Jira issues and the event log, with support for manual editing and AI-driven refinement via a custom prompt.

**Architecture:** A new `context_task_assignments` DB table (one row per context) stores the generated content independently from the analysis snapshot. The existing `analysis.Service` gains two new methods (`GenerateTaskAssignments`, `RefineTaskAssignments`) that format Jira issues + events into prompts and call the AI. Four new Wails commands wire these to the frontend, where a new collapsible section in `ContextDetail` provides Generate / Edit / Ask AI flows.

**Tech Stack:** Go 1.22, SQLite (rusqlite via `database/sql`), Wails v2 IPC, React 19, TypeScript, Tailwind CSS v4

---

## File Map

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `internal/store/migrations/053_task_assignments.sql` | Table + AI prompt seeds |
| Modify | `internal/store/models.go` | Add `TaskAssignment` struct |
| Create | `internal/store/task_assignment_repo.go` | CRUD for `context_task_assignments` |
| Modify | `internal/store/ai_prompts_repo.go` | Register placeholder metadata for new prompts |
| Modify | `internal/analysis/service.go` | Add `GenerateTaskAssignments` + `RefineTaskAssignments` |
| Modify | `app_workflow.go` | 4 new Wails commands |
| Modify | `frontend/wailsjs/go/main/App.js` | New JS bindings |
| Modify | `frontend/wailsjs/go/main/App.d.ts` | New TS type signatures |
| Modify | `frontend/src/types/index.ts` | `TaskAssignment` interface |
| Modify | `frontend/src/pages/ContextDetail.tsx` | Task Assignments UI section |

---

## Task 1: DB Migration — Table + AI Prompt Seeds

**Files:**
- Create: `internal/store/migrations/053_task_assignments.sql`

- [ ] **Step 1: Write the migration file**

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS context_task_assignments (
    context_id   TEXT PRIMARY KEY,
    content      TEXT NOT NULL DEFAULT '',
    generated_at TEXT,
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'task_assignment',
'Task Assignment Generator',
'You are an AI analyst generating personalized task assignments for project participants.

## Context key: {{context_key}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
Identify every person mentioned as an assignee or commenter in the Jira issues above.
For each person write a section (## Person Name) containing:
1. Their current responsibility based on assigned issues and status
2. Specific next actions derived from the event log and issue state
3. Any blockers or open questions directly relevant to them

Skip people mentioned only in passing. If no Jira participants are found, write a brief summary of open work items without per-person sections.

Return ONLY Markdown, no JSON, no preamble.',
'You are an AI analyst generating personalized task assignments for project participants.

## Context key: {{context_key}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
Identify every person mentioned as an assignee or commenter in the Jira issues above.
For each person write a section (## Person Name) containing:
1. Their current responsibility based on assigned issues and status
2. Specific next actions derived from the event log and issue state
3. Any blockers or open questions directly relevant to them

Skip people mentioned only in passing. If no Jira participants are found, write a brief summary of open work items without per-person sections.

Return ONLY Markdown, no JSON, no preamble.',
datetime('now')
);

INSERT OR IGNORE INTO ai_prompts (id, label, body, default_body, updated_at) VALUES (
'task_assignment_refine',
'Task Assignment Refine',
'You have the following task assignment document:

## Current document
{{current_content}}

## User instruction
{{user_instruction}}

## Jira issues and activity (for reference)
{{raw_content}}

## Event log (for reference)
{{events}}

Apply the user instruction to modify the document. Return ONLY the updated Markdown, no preamble.',
'You have the following task assignment document:

## Current document
{{current_content}}

## User instruction
{{user_instruction}}

## Jira issues and activity (for reference)
{{raw_content}}

## Event log (for reference)
{{events}}

Apply the user instruction to modify the document. Return ONLY the updated Markdown, no preamble.',
datetime('now')
);

-- +goose Down
DROP TABLE IF EXISTS context_task_assignments;
DELETE FROM ai_prompts WHERE id IN ('task_assignment', 'task_assignment_refine');
```

- [ ] **Step 2: Verify migration applies**

```bash
go build ./...
```

Expected: no compilation errors (migrations are embedded files, build confirms they're valid Go embeds).

- [ ] **Step 3: Commit**

```bash
git add internal/store/migrations/053_task_assignments.sql
git commit -m "feat: add context_task_assignments table and AI prompt seeds"
```

---

## Task 2: Go Model + Repository

**Files:**
- Modify: `internal/store/models.go`
- Create: `internal/store/task_assignment_repo.go`

- [ ] **Step 1: Add `TaskAssignment` struct to `models.go`**

Open `internal/store/models.go`. After the `EstimationLog` struct (around line 240), add:

```go
// TaskAssignment stores the AI-generated (or user-edited) per-participant assignment text for a context.
type TaskAssignment struct {
	ContextID   string  `json:"contextId"`
	Content     string  `json:"content"`
	GeneratedAt *string `json:"generatedAt,omitempty"`
	UpdatedAt   string  `json:"updatedAt"`
}
```

- [ ] **Step 2: Create `task_assignment_repo.go`**

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// GetTaskAssignment returns the task assignment record for a context, or nil if none exists yet.
func (s *Store) GetTaskAssignment(ctx context.Context, contextID string) (*TaskAssignment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT context_id, content, generated_at, updated_at
		FROM context_task_assignments WHERE context_id = ?
	`, contextID)
	var ta TaskAssignment
	var generatedAt sql.NullString
	if err := row.Scan(&ta.ContextID, &ta.Content, &generatedAt, &ta.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get task assignment: %w", err)
	}
	if generatedAt.Valid {
		ta.GeneratedAt = &generatedAt.String
	}
	return &ta, nil
}

// UpsertTaskAssignment saves or replaces a task assignment for a context.
func (s *Store) UpsertTaskAssignment(ctx context.Context, ta TaskAssignment) (*TaskAssignment, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	ta.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO context_task_assignments (context_id, content, generated_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(context_id) DO UPDATE SET
			content      = excluded.content,
			generated_at = excluded.generated_at,
			updated_at   = excluded.updated_at
	`, ta.ContextID, ta.Content, ta.GeneratedAt, ta.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert task assignment: %w", err)
	}
	return &ta, nil
}

// UpdateTaskAssignmentContent updates only the content field (for user manual edits).
func (s *Store) UpdateTaskAssignmentContent(ctx context.Context, contextID, content string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE context_task_assignments SET content = ?, updated_at = ? WHERE context_id = ?`,
		content, now, contextID,
	)
	if err != nil {
		return fmt.Errorf("update task assignment content: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/store/models.go internal/store/task_assignment_repo.go
git commit -m "feat: add TaskAssignment model and repository"
```

---

## Task 3: Register AI Prompt Placeholders

**Files:**
- Modify: `internal/store/ai_prompts_repo.go`

- [ ] **Step 1: Open `internal/store/ai_prompts_repo.go` and locate `knownPlaceholders`**

Find the `var knownPlaceholders = map[string][]PromptPlaceholder{` declaration. Add two new entries for the new prompt IDs after the existing entries:

```go
"task_assignment": {
    {Name: "context_key", Description: "Project/module key of the context"},
    {Name: "raw_content", Description: "Formatted Jira issues and activity for this context"},
    {Name: "events", Description: "Full event log in chronological order"},
    {Name: "custom_instructions", Description: "Optional extra instructions section (empty by default)"},
},
"task_assignment_refine": {
    {Name: "current_content", Description: "The existing task assignment document to refine"},
    {Name: "user_instruction", Description: "The user's modification instruction"},
    {Name: "raw_content", Description: "Formatted Jira issues and activity (reference)"},
    {Name: "events", Description: "Full event log (reference)"},
},
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/store/ai_prompts_repo.go
git commit -m "feat: register task_assignment prompt placeholders"
```

---

## Task 4: Analysis Service Methods

**Files:**
- Modify: `internal/analysis/service.go`

- [ ] **Step 1: Add `GenerateTaskAssignments` method to `Service`**

Open `internal/analysis/service.go`. After the `GenerateHandoffPacket` method, add:

```go
// GenerateTaskAssignments generates per-participant task assignment text from Jira issues
// and the event log, saves it to the DB, and returns the result.
func (s *Service) GenerateTaskAssignments(ctx context.Context, contextID string) (*appstore.TaskAssignment, error) {
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
		s.logger.Warn("task assignment: format jira context failed",
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

	tmpl := s.loadPrompt(ctx, "task_assignment", defaultTaskAssignmentPrompt)
	prompt := applyPlaceholders(tmpl, map[string]string{
		"context_key":          ctxRow.Key,
		"raw_content":          rawContent,
		"events":               formatEventsForPrompt(promptEvents),
		"custom_instructions":  "",
	})

	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, fmt.Errorf("ai task assignment: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	ta := appstore.TaskAssignment{
		ContextID:   contextID,
		Content:     resp.Content,
		GeneratedAt: &now,
	}
	return s.store.UpsertTaskAssignment(ctx, ta)
}

// RefineTaskAssignments refines the existing task assignment document based on a user instruction.
func (s *Service) RefineTaskAssignments(ctx context.Context, contextID, userInstruction string) (*appstore.TaskAssignment, error) {
	ctxRow, err := s.store.GetContext(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get context: %w", err)
	}

	existing, err := s.store.GetTaskAssignment(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("get existing task assignment: %w", err)
	}
	currentContent := ""
	if existing != nil {
		currentContent = existing.Content
	}

	allEvents, err := s.store.ListEvents(ctx, contextID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	var rawBuf strings.Builder
	if err := s.jiraSvc.FormatContext(ctx, &rawBuf, contextID, "jira", time.Time{}, false); err != nil {
		s.logger.Warn("task assignment refine: format jira context failed",
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

	tmpl := s.loadPrompt(ctx, "task_assignment_refine", defaultTaskAssignmentRefinePrompt)
	prompt := applyPlaceholders(tmpl, map[string]string{
		"current_content":  currentContent,
		"user_instruction": userInstruction,
		"raw_content":      rawContent,
		"events":           formatEventsForPrompt(promptEvents),
	})

	resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return nil, fmt.Errorf("ai task assignment refine: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	ta := appstore.TaskAssignment{
		ContextID:   contextID,
		Content:     resp.Content,
		GeneratedAt: &now,
	}
	return s.store.UpsertTaskAssignment(ctx, ta)
}
```

- [ ] **Step 2: Add default prompt constants to `service.go`**

Near the top of `service.go`, after the existing `defaultHandoffPacketPrompt` constant, add:

```go
defaultTaskAssignmentPrompt = `You are an AI analyst generating personalized task assignments for project participants.

## Context key: {{context_key}}

## Jira issues and activity
{{raw_content}}

## Event log
{{events}}
{{custom_instructions}}
## Task
Identify every person mentioned as an assignee or commenter in the Jira issues above.
For each person write a section (## Person Name) containing:
1. Their current responsibility based on assigned issues and status
2. Specific next actions derived from the event log and issue state
3. Any blockers or open questions directly relevant to them

Skip people mentioned only in passing. If no Jira participants are found, write a brief summary of open work items without per-person sections.

Return ONLY Markdown, no JSON, no preamble.`

defaultTaskAssignmentRefinePrompt = `You have the following task assignment document:

## Current document
{{current_content}}

## User instruction
{{user_instruction}}

## Jira issues and activity (for reference)
{{raw_content}}

## Event log (for reference)
{{events}}

Apply the user instruction to modify the document. Return ONLY the updated Markdown, no preamble.`
```

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/analysis/service.go
git commit -m "feat: add GenerateTaskAssignments and RefineTaskAssignments to analysis service"
```

---

## Task 5: Wails App Commands

**Files:**
- Modify: `app_workflow.go`

- [ ] **Step 1: Add 4 new commands to `app_workflow.go`**

After the `UpdateUserEvent` function, add:

```go
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

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add app_workflow.go
git commit -m "feat: add GetTaskAssignment, GenerateTaskAssignment, RefineTaskAssignment, SaveTaskAssignment commands"
```

---

## Task 6: Wails Bindings + TypeScript Types

**Files:**
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/main/App.d.ts`
- Modify: `frontend/src/types/index.ts`

- [ ] **Step 1: Add bindings to `App.js`**

Find `export function GenerateHandoffPacket` in `App.js`. After that function's closing brace, add:

```js
export function GetTaskAssignment(arg1) {
  return window['go']['main']['App']['GetTaskAssignment'](arg1);
}

export function GenerateTaskAssignment(arg1) {
  return window['go']['main']['App']['GenerateTaskAssignment'](arg1);
}

export function RefineTaskAssignment(arg1, arg2) {
  return window['go']['main']['App']['RefineTaskAssignment'](arg1, arg2);
}

export function SaveTaskAssignment(arg1, arg2) {
  return window['go']['main']['App']['SaveTaskAssignment'](arg1, arg2);
}
```

- [ ] **Step 2: Add type signatures to `App.d.ts`**

Find `export function GenerateHandoffPacket` in `App.d.ts`. After it, add:

```typescript
export function GetTaskAssignment(arg1:string):Promise<store.TaskAssignment|null>;

export function GenerateTaskAssignment(arg1:string):Promise<store.TaskAssignment>;

export function RefineTaskAssignment(arg1:string,arg2:string):Promise<store.TaskAssignment>;

export function SaveTaskAssignment(arg1:string,arg2:string):Promise<void>;
```

- [ ] **Step 3: Add `TaskAssignment` interface to `frontend/src/types/index.ts`**

After the `EstimationLog` interface (around line 281), add:

```typescript
export interface TaskAssignment {
  contextId: string;
  content: string;
  generatedAt?: string;
  updatedAt: string;
}
```

- [ ] **Step 4: TypeScript check**

```bash
cd frontend && npx tsc -b --noEmit
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts frontend/src/types/index.ts
git commit -m "feat: add task assignment bindings and TypeScript types"
```

---

## Task 7: Frontend UI — Task Assignments Section in ContextDetail

**Files:**
- Modify: `frontend/src/pages/ContextDetail.tsx`

### 7a: Imports and State

- [ ] **Step 1: Add Wails imports**

Find the existing import block from `../../wailsjs/go/main/App`. Add the four new functions:

```typescript
// existing line – find it and add to the list:
  GetTaskAssignment,
  GenerateTaskAssignment,
  RefineTaskAssignment,
  SaveTaskAssignment,
```

- [ ] **Step 2: Add `TaskAssignment` to the type import line**

Find:
```typescript
import type { ... ContextEvent, ... } from "../types";
```

Add `TaskAssignment` to the list.

- [ ] **Step 3: Add state variables**

After the existing `const [isAnalyzing, setIsAnalyzing] = useState(false);` line, add:

```typescript
const [taskAssignment, setTaskAssignment] = useState<TaskAssignment | null>(null);
const [isGeneratingAssignment, setIsGeneratingAssignment] = useState(false);
const [isEditingAssignment, setIsEditingAssignment] = useState(false);
const [editAssignmentContent, setEditAssignmentContent] = useState("");
const [showAssignmentAiForm, setShowAssignmentAiForm] = useState(false);
const [assignmentAiPrompt, setAssignmentAiPrompt] = useState("");
const [isRefiningAssignment, setIsRefiningAssignment] = useState(false);
```

### 7b: Load task assignment alongside snapshot

- [ ] **Step 4: Extend `loadSnapshot` to also fetch task assignment**

Find the `loadSnapshot` callback. It currently does:
```typescript
const [snap, evts] = await Promise.all([
  GetContextSnapshot(id) as Promise<ContextSnapshot>,
  ListEvents(id) as Promise<ContextEvent[]>,
]);
setSnapshot(snap);
setEvents(evts ?? []);
```

Replace with:
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

### 7c: Task Assignments UI section

- [ ] **Step 5: Add the section inside the Analysis `detail-section`**

Find the "Copy Handoff Packet" button div in the snapshot section:
```tsx
<div style={{ display: "flex", gap: 8, marginTop: 4 }}>
  <button
    className="btn-secondary"
    style={{ fontSize: 12, padding: "2px 10px" }}
    onClick={async () => { ... GenerateHandoffPacket ... }}
  >
    Copy Handoff Packet
  </button>
</div>
```

After the closing `</div>` of that button div, add the Task Assignments section:

```tsx
{/* Task Assignments */}
<details style={{ marginTop: 8 }}>
  <summary style={{ cursor: "pointer", fontWeight: 500, fontSize: 13, userSelect: "none" }}>
    Task Assignments
  </summary>
  <div style={{ marginTop: 6, display: "flex", flexDirection: "column", gap: 6 }}>
    {/* Content area */}
    {isEditingAssignment ? (
      <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
        <textarea
          className="passphrase-input"
          style={{ fontSize: 12, minHeight: 160, resize: "vertical", width: "100%", fontFamily: "inherit" }}
          value={editAssignmentContent}
          onChange={(e) => setEditAssignmentContent(e.target.value)}
        />
        <div style={{ display: "flex", gap: 4 }}>
          <button
            className="btn-primary"
            style={{ fontSize: 12, padding: "2px 10px" }}
            onClick={async () => {
              await SaveTaskAssignment(id, editAssignmentContent);
              setTaskAssignment({ ...(taskAssignment ?? { contextId: id, updatedAt: "" }), content: editAssignmentContent });
              setIsEditingAssignment(false);
            }}
          >
            Save
          </button>
          <button
            className="btn-secondary"
            style={{ fontSize: 12, padding: "2px 10px" }}
            onClick={() => setIsEditingAssignment(false)}
          >
            Cancel
          </button>
        </div>
      </div>
    ) : taskAssignment?.content ? (
      <pre style={{ whiteSpace: "pre-wrap", fontSize: 12, margin: 0, color: "var(--text-muted)", lineHeight: 1.5 }}>
        {taskAssignment.content}
      </pre>
    ) : (
      <div style={{ fontSize: 12, color: "var(--text-muted)" }}>
        Not generated yet. Click Generate to create task assignments from Jira issues and events.
      </div>
    )}

    {/* Action buttons row */}
    {!isEditingAssignment && (
      <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
        <button
          className="btn-secondary"
          style={{ fontSize: 12, padding: "2px 10px" }}
          disabled={isGeneratingAssignment}
          onClick={async () => {
            setIsGeneratingAssignment(true);
            try {
              const ta = await GenerateTaskAssignment(id) as TaskAssignment;
              setTaskAssignment(ta);
            } finally {
              setIsGeneratingAssignment(false);
            }
          }}
        >
          {isGeneratingAssignment ? "Generating…" : "Generate"}
        </button>
        {taskAssignment?.content && (
          <button
            className="btn-secondary"
            style={{ fontSize: 12, padding: "2px 10px" }}
            onClick={() => {
              setEditAssignmentContent(taskAssignment.content);
              setIsEditingAssignment(true);
            }}
          >
            Edit
          </button>
        )}
        {taskAssignment?.content && (
          <button
            className="btn-secondary"
            style={{ fontSize: 12, padding: "2px 10px" }}
            onClick={() => setShowAssignmentAiForm((v) => !v)}
          >
            Ask AI
          </button>
        )}
      </div>
    )}

    {/* Ask AI sub-form */}
    {showAssignmentAiForm && !isEditingAssignment && (
      <div style={{ borderTop: "1px solid var(--border)", paddingTop: 6, display: "flex", flexDirection: "column", gap: 4 }}>
        <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 2 }}>
          Describe how to change the document (e.g. "Focus on backend engineers only", "Translate to Spanish")
        </div>
        <textarea
          className="passphrase-input"
          style={{ fontSize: 12, minHeight: 56, resize: "vertical", width: "100%", fontFamily: "inherit" }}
          placeholder="Your instruction…"
          value={assignmentAiPrompt}
          onChange={(e) => setAssignmentAiPrompt(e.target.value)}
        />
        <div style={{ display: "flex", gap: 4 }}>
          <button
            className="btn-primary"
            style={{ fontSize: 12, padding: "2px 10px" }}
            disabled={isRefiningAssignment || !assignmentAiPrompt.trim()}
            onClick={async () => {
              if (!assignmentAiPrompt.trim()) return;
              setIsRefiningAssignment(true);
              try {
                const ta = await RefineTaskAssignment(id, assignmentAiPrompt.trim()) as TaskAssignment;
                setTaskAssignment(ta);
                setAssignmentAiPrompt("");
                setShowAssignmentAiForm(false);
              } finally {
                setIsRefiningAssignment(false);
              }
            }}
          >
            {isRefiningAssignment ? "Applying…" : "Apply"}
          </button>
          <button
            className="btn-secondary"
            style={{ fontSize: 12, padding: "2px 10px" }}
            onClick={() => { setShowAssignmentAiForm(false); setAssignmentAiPrompt(""); }}
          >
            Cancel
          </button>
        </div>
      </div>
    )}
  </div>
</details>
```

- [ ] **Step 6: TypeScript check**

```bash
cd frontend && npx tsc -b --noEmit
```

Expected: no errors.

- [ ] **Step 7: Go build check**

```bash
go build ./...
```

Expected: success.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/pages/ContextDetail.tsx
git commit -m "feat: add Task Assignments section to ContextDetail (generate, edit, ask AI)"
```

---

## Task 8: Smoke Test

- [ ] **Step 1: Start the app**

```bash
wails dev
```

- [ ] **Step 2: Open a context that has at least one Jira issue with an assignee**

Navigate to a Context Detail page.

- [ ] **Step 3: Verify Generate flow**

- Find the "Task Assignments" collapsible section in the Analysis area
- Click to expand
- Click "Generate"
- Spinner shows "Generating…"
- After a few seconds, markdown content appears with `## Person Name` sections

- [ ] **Step 4: Verify Edit flow**

- Click "Edit"
- Textarea appears with existing content
- Modify some text and click "Save"
- Pre-formatted text updates to show saved content

- [ ] **Step 5: Verify Ask AI flow**

- Click "Ask AI"
- Sub-form appears with textarea
- Type "Add a section for any unassigned open tasks"
- Click "Apply"
- "Applying…" shows
- Content updates with the refined text

- [ ] **Step 6: Verify persistence**

- Reload the page (or navigate away and back)
- Task assignment content is still shown (loaded from DB via `loadSnapshot`)

---

## Self-Review Checklist

- **Spec coverage:** Generate ✓, re-generate with custom prompt ✓ (Ask AI), display ✓, edit ✓, persist ✓
- **No placeholders:** All code blocks are complete; no TBDs
- **Type consistency:** `TaskAssignment` struct (Go) → `store.TaskAssignment` (Wails) → `TaskAssignment` (TS) — camelCase JSON throughout via `json:"camelCase"` tags on Go struct
- **Method name consistency:** `GenerateTaskAssignments` (service) vs `GenerateTaskAssignment` (app command) — the app command drops the plural since it operates on a single context; this is intentional and consistent with the existing pattern (`AnalyzeContext` not `AnalyzeContexts`)
- **`GetTaskAssignment` null handling:** Returns `nil` for no-row case and the TS type is `Promise<store.TaskAssignment|null>` — frontend handles null with `??` fallback
- **Migration number:** 053 — confirmed as next after 052_keybindings.sql
