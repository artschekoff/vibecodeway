# Logs Preview in Settings — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a read-only logs viewer section to the Settings page that lists `context_logs` across all contexts, with filter by trigger (level), and displays date, trigger badge, message preview, context name, and org name.

**Architecture:** Add a `LogEntry` type that embeds `ContextLog` plus `ContextKey`/`OrgName` fields, back it with a new `ListAll` repo method using a LEFT JOIN to organizations, wire the full chain (CLI command → runner → App method → Wails binding → React component), and render a filter+table inside the existing Settings page.

**Tech Stack:** Go 1.22, Cobra CLI, SQLite (goose migrations), Wails v2, React 18 + TypeScript, Vite.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/contextlog/model.go` | Modify | Add `LogEntry` struct |
| `internal/contextlog/repo.go` | Modify | Add `ListAll(ctx, trigger, limit)` method |
| `internal/contextlog/repo_list_all_test.go` | Create | Tests for `ListAll` |
| `internal/contextlog/service.go` | Modify | Add `ListAllLogs(ctx, trigger, limit)` method |
| `cmd/cli/log_commands.go` | Modify | Add `newLogListAllCmd()` |
| `cmd/cli/main.go` | Modify | Register `newLogListAllCmd()` |
| `internal/cli/runner.go` | Modify | Add `ListAllLogs(ctx, trigger, limit)` |
| `app.go` | Modify | Add `ListAllLogs(trigger string, limit int)` |
| `frontend/wailsjs/go/main/App.js` | Modify | Add JS binding |
| `frontend/wailsjs/go/main/App.d.ts` | Modify | Add TS declaration |
| `frontend/src/types/index.ts` | Modify | Add `LogEntry` interface |
| `frontend/src/components/LogsPreview.tsx` | Create | Filter + table component |
| `frontend/src/pages/Settings.tsx` | Modify | Import and render `<LogsPreview />` |
| `frontend/src/style.css` | Modify | Add styles for logs preview section |

---

## Task 1: Add `LogEntry` type to model.go

**Files:**
- Modify: `internal/contextlog/model.go`

- [ ] **Step 1: Add LogEntry struct after ContextLog**

Open `internal/contextlog/model.go` and add after the `ContextLog` struct (after line 18):

```go
// LogEntry is a ContextLog enriched with its context's key and organization name.
// Returned by ListAll for cross-context log views.
type LogEntry struct {
	ContextLog
	ContextKey string `json:"contextKey"`
	OrgName    string `json:"orgName,omitempty"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/contextlog/model.go
git commit -m "feat(contextlog): add LogEntry type with context key and org name"
```

---

## Task 2: Add `ListAll` to repo with tests (TDD)

**Files:**
- Create: `internal/contextlog/repo_list_all_test.go`
- Modify: `internal/contextlog/repo.go`

- [ ] **Step 1: Write failing tests**

Create `internal/contextlog/repo_list_all_test.go`:

```go
package contextlog_test

import (
	"context"
	"testing"

	"github.com/artschekoff/context-consolidator/internal/contextlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAll_returnsLogsWithContextKey(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "sprint-42")
	require.NoError(t, err)

	_, err = svc.CreateManualLog(ctx, c.ID, "First note")
	require.NoError(t, err)

	repo := contextlog.NewRepo(s.DB())
	entries, err := repo.ListAll(ctx, "", 50)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "sprint-42", entries[0].ContextKey)
	assert.Equal(t, "First note", entries[0].SummaryMd)
	assert.Equal(t, "", entries[0].OrgName)
}

func TestListAll_joinsOrgName(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	org, err := s.CreateOrganization(ctx, "Acme Corp")
	require.NoError(t, err)

	c, err := s.CreateContext(ctx, "project-x")
	require.NoError(t, err)

	err = s.SetContextOrg(ctx, c.ID, org.ID)
	require.NoError(t, err)

	_, err = svc.CreateManualLog(ctx, c.ID, "Org note")
	require.NoError(t, err)

	repo := contextlog.NewRepo(s.DB())
	entries, err := repo.ListAll(ctx, "", 50)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Acme Corp", entries[0].OrgName)
	assert.Equal(t, "project-x", entries[0].ContextKey)
}

func TestListAll_filterByTrigger(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "ctx-a")
	require.NoError(t, err)

	_, err = svc.CreateManualLog(ctx, c.ID, "Manual entry")
	require.NoError(t, err)
	_, err = svc.CreateAgentLog(ctx, c.ID, "Agent entry")
	require.NoError(t, err)

	repo := contextlog.NewRepo(s.DB())

	manual, err := repo.ListAll(ctx, contextlog.TriggerManual, 50)
	require.NoError(t, err)
	require.Len(t, manual, 1)
	assert.Equal(t, contextlog.TriggerManual, manual[0].Trigger)

	agent, err := repo.ListAll(ctx, contextlog.TriggerAgent, 50)
	require.NoError(t, err)
	require.Len(t, agent, 1)
	assert.Equal(t, contextlog.TriggerAgent, agent[0].Trigger)

	all, err := repo.ListAll(ctx, "", 50)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestListAll_respectsLimit(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "ctx-b")
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err = svc.CreateManualLog(ctx, c.ID, "note")
		require.NoError(t, err)
	}

	repo := contextlog.NewRepo(s.DB())
	entries, err := repo.ListAll(ctx, "", 3)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestListAll_orderedNewestFirst(t *testing.T) {
	s, svc := openTestSvc(t)
	ctx := context.Background()

	c, err := s.CreateContext(ctx, "ctx-c")
	require.NoError(t, err)

	_, err = svc.CreateManualLog(ctx, c.ID, "first")
	require.NoError(t, err)
	_, err = svc.CreateManualLog(ctx, c.ID, "second")
	require.NoError(t, err)

	repo := contextlog.NewRepo(s.DB())
	entries, err := repo.ListAll(ctx, "", 50)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "second", entries[0].SummaryMd)
	assert.Equal(t, "first", entries[1].SummaryMd)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator
go test ./internal/contextlog/... -run TestListAll -v 2>&1 | head -30
```

Expected: compilation error — `repo.ListAll undefined`.

- [ ] **Step 3: Implement `ListAll` in repo.go**

Open `internal/contextlog/repo.go` and add this method before the final closing brace. Add it after the `ListLogs` method:

```go
// ListAll returns log entries across all contexts, optionally filtered by trigger.
// Pass an empty string for trigger to return all triggers. Results are newest-first.
// limit=0 is treated as 100 to avoid unbounded queries.
func (r *Repo) ListAll(ctx context.Context, trigger string, limit int) ([]LogEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			cl.id, cl.context_id, cl.trigger, cl.source, cl.model,
			cl.prompt_tokens, cl.completion_tokens, cl.diff_json,
			cl.summary_md, cl.is_manual, cl.edited_at, cl.created_at,
			c.key AS context_key,
			COALESCE(o.name, '') AS org_name
		FROM context_logs cl
		JOIN contexts c ON c.id = cl.context_id
		LEFT JOIN organizations o ON o.id = c.org_id
		WHERE (? = '' OR cl.trigger = ?)
		ORDER BY cl.created_at DESC
		LIMIT ?`,
		trigger, trigger, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing all logs: %w", err)
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var e LogEntry
		var editedAt sql.NullString
		err := rows.Scan(
			&e.ID, &e.ContextID, &e.Trigger, &e.Source, &e.Model,
			&e.PromptTokens, &e.CompletionTokens, &e.DiffJSON,
			&e.SummaryMd, &e.IsManual, &editedAt, &e.CreatedAt,
			&e.ContextKey, &e.OrgName,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning log entry: %w", err)
		}
		if editedAt.Valid {
			e.EditedAt = editedAt.String
		}
		e.IsManual = e.IsManual // already a bool via intToBool in existing scan helpers
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
```

Note: `IsManual` is stored as INTEGER in SQLite. Look at how existing `ListLogs` scans `is_manual` — it uses `intToBool`. Apply the same pattern here. Check `repo.go` for the `scanLog` or inline scan pattern and replicate it exactly for `is_manual`.

Open `internal/contextlog/repo.go` and check how `ListLogs` scans `is_manual`. Search for `is_manual` in the file. If it uses a helper like `intToBool`, replace `&e.IsManual` in the scan above with:

```go
var isManual int
// ... in Scan: &isManual
// ... after Scan: e.IsManual = isManual != 0
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/contextlog/... -run TestListAll -v
```

Expected: all 5 tests PASS.

- [ ] **Step 5: Run full test suite**

```bash
go test ./internal/contextlog/... -race -count=1
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/contextlog/repo.go internal/contextlog/repo_list_all_test.go
git commit -m "feat(contextlog): add Repo.ListAll for cross-context log listing with org join"
```

---

## Task 3: Add `ListAllLogs` to service

**Files:**
- Modify: `internal/contextlog/service.go`

- [ ] **Step 1: Add service method**

Open `internal/contextlog/service.go`. Find the `ListLogs` method. Add the following method immediately after it:

```go
// ListAllLogs returns log entries across all contexts, optionally filtered by trigger.
// Pass empty string for trigger to get all triggers. limit<=0 defaults to 100.
func (s *Service) ListAllLogs(ctx context.Context, trigger string, limit int) ([]LogEntry, error) {
	return s.repo.ListAll(ctx, trigger, limit)
}
```

- [ ] **Step 2: Run tests**

```bash
go test ./internal/contextlog/... -race -count=1
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/contextlog/service.go
git commit -m "feat(contextlog): add Service.ListAllLogs delegating to Repo.ListAll"
```

---

## Task 4: Add `log-list-all` CLI command

**Files:**
- Modify: `cmd/cli/log_commands.go`
- Modify: `cmd/cli/main.go`

- [ ] **Step 1: Add command to log_commands.go**

Open `cmd/cli/log_commands.go`. Add the following function at the end of the file, before the closing:

```go
func newLogListAllCmd() *cobra.Command {
	var trigger string
	var limit int
	cmd := &cobra.Command{
		Use:   "log-list-all",
		Short: "List log entries across all contexts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			entries, err := env.contextLog.ListAllLogs(ctx, trigger, limit)
			if err != nil {
				return fmt.Errorf("listing all logs: %w", err)
			}
			return writeJSON(entries)
		},
	}
	cmd.Flags().StringVar(&trigger, "trigger", "", "Filter by trigger: refresh|reload|manual|init|agent (empty = all)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of entries to return")
	return cmd
}
```

- [ ] **Step 2: Register command in main.go**

Open `cmd/cli/main.go`. Find the "Context logs & consolidated log" block (around line 81). Add the new command after `newLogListCmd()`:

```go
root.AddCommand(newLogListAllCmd())
```

So the block becomes:
```go
// Context logs & consolidated log
root.AddCommand(newLogListCmd())
root.AddCommand(newLogListAllCmd())   // ← add this line
root.AddCommand(newLogCreateCmd())
// ... rest unchanged
```

- [ ] **Step 3: Build CLI to verify it compiles**

```bash
go build ./cmd/cli/...
```

Expected: no errors.

- [ ] **Step 4: Smoke test the command**

```bash
./context-consolidator-cli log-list-all --limit 5 2>&1 | head -5
```

Expected: JSON output (possibly `null` or `[]` if no logs exist) with no error.

- [ ] **Step 5: Commit**

```bash
git add cmd/cli/log_commands.go cmd/cli/main.go
git commit -m "feat(cli): add log-list-all command with trigger filter and limit"
```

---

## Task 5: Add `ListAllLogs` to runner

**Files:**
- Modify: `internal/cli/runner.go`

- [ ] **Step 1: Add runner method**

Open `internal/cli/runner.go`. Find the `ListContextLogs` method (around line 630). Add the following method immediately after it:

```go
// ListAllLogs returns log entries across all contexts, optionally filtered by trigger.
// Pass empty string for trigger to return all triggers.
func (r *Runner) ListAllLogs(ctx context.Context, trigger string, limit int) ([]contextlog.LogEntry, error) {
	args := []string{"log-list-all", "--limit", fmt.Sprintf("%d", limit)}
	if trigger != "" {
		args = append(args, "--trigger", trigger)
	}
	out, err := r.run(ctx, args)
	if err != nil {
		return nil, err
	}
	return unmarshalSlice[contextlog.LogEntry](out, "log-list-all")
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/cli/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/cli/runner.go
git commit -m "feat(cli/runner): add ListAllLogs method"
```

---

## Task 6: Expose `ListAllLogs` in App

**Files:**
- Modify: `app.go`

- [ ] **Step 1: Add App method**

Open `app.go`. Find the `ListContextLogs` method (around line 749). Add the following method immediately after it:

```go
// ListAllLogs returns log entries across all contexts, optionally filtered by trigger.
// Pass empty string for trigger to return all triggers. limit<=0 defaults to 100.
func (a *App) ListAllLogs(trigger string, limit int) ([]contextlog.LogEntry, error) {
	return a.cliRunner.ListAllLogs(a.ctx, trigger, limit)
}
```

- [ ] **Step 2: Build desktop app to verify it compiles**

```bash
go build .
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add app.go
git commit -m "feat(app): expose ListAllLogs to Wails frontend"
```

---

## Task 7: Update Wails bindings

**Files:**
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/main/App.d.ts`

- [ ] **Step 1: Add JS binding**

Open `frontend/wailsjs/go/main/App.js`. Find the `ListContextLogs` export (search for `ListContextLogs`). Add the following export immediately after it:

```js
export function ListAllLogs(arg1, arg2) {
  return window['go']['main']['App']['ListAllLogs'](arg1, arg2);
}
```

- [ ] **Step 2: Add TS declaration**

Open `frontend/wailsjs/go/main/App.d.ts`. Find the `ListContextLogs` declaration. Add the following declaration immediately after it:

```ts
export function ListAllLogs(arg1:string,arg2:number):Promise<Array<contextlog.LogEntry>>;
```

- [ ] **Step 3: Commit**

```bash
git add frontend/wailsjs/go/main/App.js frontend/wailsjs/go/main/App.d.ts
git commit -m "feat(frontend): add Wails binding for ListAllLogs"
```

---

## Task 8: Add `LogEntry` TypeScript type

**Files:**
- Modify: `frontend/src/types/index.ts`

- [ ] **Step 1: Add LogEntry interface**

Open `frontend/src/types/index.ts`. Find the `ContextLog` interface (around line 256). Add the following interface immediately after the closing brace of `ContextLog`:

```ts
export interface LogEntry extends ContextLog {
  contextKey: string;
  orgName?: string;
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator/frontend
npm run build 2>&1 | tail -10
```

Expected: no TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/types/index.ts
git commit -m "feat(frontend/types): add LogEntry interface"
```

---

## Task 9: Add CSS styles for logs preview

**Files:**
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Add styles**

Open `frontend/src/style.css`. Append the following at the end of the file:

```css
/* ── Settings Logs Preview ─────────────────────────────────────── */

.settings-logs-section {
  margin-top: 2rem;
}

.settings-logs-filter-bar {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}

.settings-logs-filter-bar label {
  font-size: 0.8125rem;
  color: var(--text-muted);
  white-space: nowrap;
}

.settings-logs-filter-bar select {
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--border, #333);
  border-radius: 4px;
  padding: 0.25rem 0.5rem;
  font-size: 0.8125rem;
}

.settings-logs-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8125rem;
}

.settings-logs-table th {
  text-align: left;
  padding: 0.375rem 0.5rem;
  color: var(--text-muted);
  font-weight: 500;
  border-bottom: 1px solid var(--border, #333);
  white-space: nowrap;
}

.settings-logs-table td {
  padding: 0.375rem 0.5rem;
  border-bottom: 1px solid var(--border-subtle, #222);
  vertical-align: top;
}

.settings-logs-table tr:hover td {
  background: var(--surface);
}

.settings-logs-cell-date {
  white-space: nowrap;
  color: var(--text-muted);
  min-width: 9rem;
}

.settings-logs-cell-level {
  white-space: nowrap;
  min-width: 5.5rem;
}

.settings-logs-cell-context {
  white-space: nowrap;
  min-width: 7rem;
  font-weight: 500;
}

.settings-logs-cell-org {
  white-space: nowrap;
  min-width: 7rem;
  color: var(--text-muted);
}

.settings-logs-cell-message {
  color: var(--text);
  max-width: 30rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.settings-logs-trigger {
  display: inline-block;
  padding: 0.1rem 0.45rem;
  border-radius: 3px;
  font-size: 0.75rem;
  border: 1px solid var(--text-muted);
  color: var(--text-muted);
}

.settings-logs-trigger--refresh,
.settings-logs-trigger--reload {
  border-color: var(--accent);
  color: var(--accent);
}

.settings-logs-trigger--manual {
  border-color: var(--text-muted);
  color: var(--text-muted);
}

.settings-logs-trigger--init {
  border-color: #9C6;
  color: #9C6;
}

.settings-logs-trigger--agent {
  border-color: #6af;
  color: #6af;
}

.settings-logs-empty {
  text-align: center;
  padding: 1.5rem 0;
  color: var(--text-muted);
  font-size: 0.875rem;
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/style.css
git commit -m "feat(frontend/style): add logs preview section styles"
```

---

## Task 10: Create `LogsPreview` component

**Files:**
- Create: `frontend/src/components/LogsPreview.tsx`

- [ ] **Step 1: Create component**

Create `frontend/src/components/LogsPreview.tsx`:

```tsx
import { useEffect, useState } from "react";
import { ListAllLogs } from "../../wailsjs/go/main/App";
import type { LogEntry } from "../types";

const TRIGGERS = [
  { value: "", label: "All" },
  { value: "refresh", label: "Refresh" },
  { value: "reload", label: "Reload" },
  { value: "manual", label: "Manual" },
  { value: "init", label: "Init" },
  { value: "agent", label: "Agent" },
];

const LIMIT = 100;

function formatDate(iso: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return iso;
  }
}

export default function LogsPreview() {
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [trigger, setTrigger] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    ListAllLogs(trigger, LIMIT)
      .then((data) => setEntries(data ?? []))
      .finally(() => setLoading(false));
  }, [trigger]);

  return (
    <div className="settings-logs-section">
      <div className="settings-section-header">Logs</div>

      <div className="settings-logs-filter-bar">
        <label htmlFor="logs-level-filter">Level</label>
        <select
          id="logs-level-filter"
          value={trigger}
          onChange={(e) => setTrigger(e.target.value)}
        >
          {TRIGGERS.map((t) => (
            <option key={t.value} value={t.value}>
              {t.label}
            </option>
          ))}
        </select>
      </div>

      {loading ? (
        <div className="settings-logs-empty">Loading…</div>
      ) : entries.length === 0 ? (
        <div className="settings-logs-empty">No log entries found.</div>
      ) : (
        <table className="settings-logs-table">
          <thead>
            <tr>
              <th className="settings-logs-cell-date">Date</th>
              <th className="settings-logs-cell-level">Level</th>
              <th className="settings-logs-cell-context">Context</th>
              <th className="settings-logs-cell-org">Organization</th>
              <th className="settings-logs-cell-message">Message</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((e) => (
              <tr key={e.id}>
                <td className="settings-logs-cell-date">
                  {formatDate(e.createdAt)}
                </td>
                <td className="settings-logs-cell-level">
                  <span
                    className={`settings-logs-trigger settings-logs-trigger--${e.trigger}`}
                  >
                    {e.trigger}
                  </span>
                </td>
                <td className="settings-logs-cell-context">{e.contextKey}</td>
                <td className="settings-logs-cell-org">{e.orgName ?? "—"}</td>
                <td className="settings-logs-cell-message" title={e.summaryMd}>
                  {e.summaryMd}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Build frontend to verify it compiles**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator/frontend
npm run build 2>&1 | tail -15
```

Expected: no TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/LogsPreview.tsx
git commit -m "feat(frontend): add LogsPreview component with trigger filter"
```

---

## Task 11: Add LogsPreview to Settings page

**Files:**
- Modify: `frontend/src/pages/Settings.tsx`

- [ ] **Step 1: Import LogsPreview**

Open `frontend/src/pages/Settings.tsx`. Find the existing imports at the top. Add the import after the `WorkflowStatusManager` import:

```ts
import LogsPreview from "../components/LogsPreview";
```

- [ ] **Step 2: Add LogsPreview section to JSX**

In `Settings.tsx`, find the last section rendered in the return JSX (search for `WorkflowStatusManager` — it's likely near the end). Add `<LogsPreview />` immediately after the `<WorkflowStatusManager />` element:

```tsx
<WorkflowStatusManager />
<LogsPreview />
```

- [ ] **Step 3: Build frontend to verify**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator/frontend
npm run build 2>&1 | tail -15
```

Expected: no errors.

- [ ] **Step 4: Run full Go test suite**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator
go test ./... -race -count=1 2>&1 | tail -20
```

Expected: all tests PASS.

- [ ] **Step 5: Start dev server and verify in UI**

```bash
make dev
```

Open the app, navigate to Settings, scroll to the bottom. Verify:
- "Logs" section is visible
- The level dropdown shows: All, Refresh, Reload, Manual, Init, Agent
- Selecting a level filters the table
- Each row shows: date, level badge (colored by trigger type), context key, org name (or —), message preview

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/Settings.tsx
git commit -m "feat(settings): add LogsPreview section showing cross-context logs"
```

---

## Self-Review Checklist

- [x] **LogEntry type** — defined in model.go, mirrored in types/index.ts
- [x] **SQL JOIN** — context_logs → contexts (for key) → organizations (for name, LEFT JOIN)
- [x] **Trigger filter** — empty string = all; non-empty = WHERE cl.trigger = ?
- [x] **Limit** — default 100, configurable via CLI flag and runner parameter
- [x] **is_manual scanning** — note in Task 2 Step 3 to replicate existing `intToBool` pattern
- [x] **Wails bindings** — both App.js and App.d.ts updated manually (not auto-generated)
- [x] **Full chain wired** — repo → service → CLI command → runner → App.go → JS binding → React
- [x] **Context name shown** — `contexts.key` as `ContextKey`
- [x] **Org name shown** — `organizations.name` via LEFT JOIN, empty string if no org
- [x] **Filter by level** — trigger dropdown in LogsPreview
- [x] **Display date** — formatted in `formatDate()` helper
- [x] **Display message** — `summaryMd` truncated with `text-overflow: ellipsis`
- [x] **Styled trigger badges** — colored per trigger type, matching existing log-entry-trigger pattern
