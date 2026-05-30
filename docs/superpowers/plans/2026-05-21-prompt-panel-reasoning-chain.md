# Tech Assignment Editor — Prompt Panel + Versioning Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a tech assignment (PersonAssignment) is opened, display the rendered LLM prompt between the source text and the output. Let the user (1) edit the output inline and save it, (2) edit the prompt and re-run it — with the new output previewed in a modal before it is committed as a new versioned assignment — and (3) browse/restore previous versions via a GitHub-style diff.

**Architecture overview:**
- `person_assignment_versions` table tracks all output versions per assignment. Each row carries the full `body`, `llm_prompt`, and an `is_active` boolean — only one row per assignment can be active at a time.
- `step_prompts` table (one row per reasoning-chain step) stores the rendered prompt; this is unchanged from the earlier design.
- The frontend for the PersonAssignment detail view gains three new regions: an editable OUTPUT block (Save button), a PROMPT block (Edit → Send → modal → Confirm/Cancel), and a VERSION SELECTOR (dropdown with ISO timestamps → diff view → Confirm/Cancel).

**Tech Stack:** Go + SQLite (store layer), Wails v2 IPC, React 19 + TypeScript (Vitest)

---

## File Map

| File | Change |
|------|--------|
| `internal/store/migrations/060_step_prompts.sql` | **Create** — `step_prompts` table (unchanged) |
| `internal/store/migrations/061_assignment_versions.sql` | **Create** — `person_assignment_versions` table |
| `internal/store/models.go` | **Modify** — add `LLMPrompt` to `ReasoningChainStep`; add `PersonAssignmentVersion` model |
| `internal/store/step_prompt_repo.go` | **Create** — `InsertStepPrompt`, `GetStepPrompt` |
| `internal/store/assignment_version_repo.go` | **Create** — `CreateVersion`, `ListVersions`, `SetActiveVersion` |
| `internal/store/reasoning_chain_repo.go` | **Modify** — LEFT JOIN `step_prompts` in `ListChainSteps`; set `LLMPrompt` in `AppendChainStep` |
| `internal/store/step_prompt_repo_test.go` | **Create** — store-layer tests for step_prompts |
| `internal/store/assignment_version_repo_test.go` | **Create** — store-layer tests for versioning |
| `internal/analysis/service.go` | **Modify** — set `step.LLMPrompt = prompt` in `RunStep`; add `SaveAssignmentOutput`, `RunAssignmentStepWithCustomPrompt`, `CommitAssignmentVersionFromCustomPrompt` |
| `internal/analysis/reasoning_chain.go` | **Modify** — `CommitStep` accepts `llmPrompt string` param |
| `app_workflow.go` | **Add** — IPC wrappers for versioning and custom-prompt replay |
| `frontend/wailsjs/go/main/App.d.ts` | **Modify** — declare new IPC methods |
| `frontend/wailsjs/go/main/App.js` | **Modify** — bind new IPC methods |
| `frontend/src/types/index.ts` | **Modify** — add `llmPrompt?` to `ReasoningChainStep`; add `PersonAssignmentVersion` type |
| `frontend/src/components/PersonAssignmentDetail.tsx` | **Create/Modify** — SOURCE → PROMPT → OUTPUT layout with edit/save/version UI |
| `frontend/src/components/PersonAssignmentDetail.test.tsx` | **Create** — Vitest tests |
| `frontend/src/components/VersionDiffModal.tsx` | **Create** — GitHub-style word/line diff modal |

---

## Task 1: DB Migrations

**Files:**
- Create: `internal/store/migrations/060_step_prompts.sql`
- Create: `internal/store/migrations/061_assignment_versions.sql`

- [ ] **Step 1: Create step_prompts migration**

```sql
-- 060_step_prompts.sql
CREATE TABLE IF NOT EXISTS step_prompts (
    id         TEXT PRIMARY KEY,
    step_id    TEXT NOT NULL UNIQUE,
    llm_prompt TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS sp_step_idx ON step_prompts (step_id);
```

- [ ] **Step 2: Create person_assignment_versions migration**

```sql
-- 061_assignment_versions.sql
CREATE TABLE IF NOT EXISTS person_assignment_versions (
    id                   TEXT PRIMARY KEY,
    person_assignment_id TEXT NOT NULL,
    body                 TEXT NOT NULL DEFAULT '',
    llm_prompt           TEXT NOT NULL DEFAULT '',
    is_active            INTEGER NOT NULL DEFAULT 0,
    created_at           TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS pav_pa_idx ON person_assignment_versions (person_assignment_id);
-- Partial unique index: only one active version per assignment
CREATE UNIQUE INDEX IF NOT EXISTS pav_active_idx ON person_assignment_versions (person_assignment_id) WHERE is_active = 1;
```

- [ ] **Step 3: Verify migrations load**

Run: `go test ./internal/store/... -run TestMigrations -v`
Expected: PASS — no schema errors.

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/060_step_prompts.sql internal/store/migrations/061_assignment_versions.sql
git commit -m "feat(store): step_prompts and person_assignment_versions tables"
```

---

## Task 2: Go models

**Files:**
- Modify: `internal/store/models.go`

- [ ] **Step 1: Add `LLMPrompt` field to `ReasoningChainStep`**

```go
type ReasoningChainStep struct {
    // ... existing fields ...
    LLMPrompt string `json:"llmPrompt,omitempty"` // rendered prompt sent to LLM, loaded via JOIN
}
```

- [ ] **Step 2: Add `PersonAssignmentVersion` model**

```go
type PersonAssignmentVersion struct {
    ID                 string `json:"id"`
    PersonAssignmentID string `json:"personAssignmentId"`
    Body               string `json:"body"`
    LLMPrompt          string `json:"llmPrompt,omitempty"`
    IsActive           bool   `json:"isActive"`
    CreatedAt          string `json:"createdAt"`
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/store/models.go
git commit -m "feat(store): LLMPrompt on ReasoningChainStep, PersonAssignmentVersion model"
```

---

## Task 3: Store layer — step_prompts repo

**Files:**
- Create: `internal/store/step_prompt_repo.go`
- Create: `internal/store/step_prompt_repo_test.go`
- Modify: `internal/store/reasoning_chain_repo.go`

### Step 1: Write failing tests

`internal/store/step_prompt_repo_test.go`:

```go
func TestStepPrompt_RoundTrip(t *testing.T) {
    s := openTestStore(t)
    ctx := context.Background()

    step, err := s.AppendChainStep(ctx, ReasoningChainStep{
        EntityType:  ChainEntityPersonAssignment,
        EntityID:    "pa-test-1",
        SourceType:  "initial",
        InputState:  "",
        OutputState: "output body",
        IsCurrent:   true,
        LLMPrompt:   "You are an analyst. Generate a task assignment for Alice.",
    })
    require.NoError(t, err)
    assert.Equal(t, "You are an analyst. Generate a task assignment for Alice.", step.LLMPrompt)

    steps, err := s.ListChainSteps(ctx, ChainEntityPersonAssignment, "pa-test-1")
    require.NoError(t, err)
    require.Len(t, steps, 1)
    assert.Equal(t, "You are an analyst. Generate a task assignment for Alice.", steps[0].LLMPrompt)
}

func TestStepPrompt_EmptyWhenNotSet(t *testing.T) {
    s := openTestStore(t)
    ctx := context.Background()

    _, err := s.AppendChainStep(ctx, ReasoningChainStep{
        EntityType:  ChainEntityPersonAssignment,
        EntityID:    "pa-test-2",
        SourceType:  "initial",
        InputState:  "",
        OutputState: "output",
        IsCurrent:   true,
    })
    require.NoError(t, err)

    steps, err := s.ListChainSteps(ctx, ChainEntityPersonAssignment, "pa-test-2")
    require.NoError(t, err)
    require.Len(t, steps, 1)
    assert.Equal(t, "", steps[0].LLMPrompt)
}
```

- [ ] **Step 1: Run tests to verify they fail**

Run: `go test ./internal/store/... -run TestStepPrompt -v`
Expected: FAIL

### Step 2: Create `internal/store/step_prompt_repo.go`

```go
package store

import (
    "context"
    "fmt"
    "time"
    "github.com/google/uuid"
)

func (s *Store) InsertStepPrompt(ctx context.Context, stepID, llmPrompt string) error {
    _, err := s.db.ExecContext(ctx, `
        INSERT INTO step_prompts (id, step_id, llm_prompt, created_at)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(step_id) DO UPDATE SET llm_prompt = excluded.llm_prompt
    `, uuid.New().String(), stepID, llmPrompt, time.Now().UTC().Format(time.RFC3339))
    if err != nil {
        return fmt.Errorf("insert step prompt: %w", err)
    }
    return nil
}

func (s *Store) GetStepPrompt(ctx context.Context, stepID string) (string, error) {
    var prompt string
    err := s.db.QueryRowContext(ctx,
        `SELECT COALESCE(llm_prompt,'') FROM step_prompts WHERE step_id = ?`, stepID,
    ).Scan(&prompt)
    if err != nil {
        return "", nil
    }
    return prompt, nil
}
```

- [ ] **Step 2: Update `AppendChainStep`** — after main INSERT, if `step.LLMPrompt != ""`, call `InsertStepPrompt`.

- [ ] **Step 3: Update `ListChainSteps`** — LEFT JOIN `step_prompts` and scan `COALESCE(sp.llm_prompt,'')` into `&st.LLMPrompt`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/... -run TestStepPrompt -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/step_prompt_repo.go internal/store/step_prompt_repo_test.go internal/store/reasoning_chain_repo.go
git commit -m "feat(store): step_prompts repo — save and load LLM prompt per chain step"
```

---

## Task 4: Store layer — assignment_versions repo

**Files:**
- Create: `internal/store/assignment_version_repo.go`
- Create: `internal/store/assignment_version_repo_test.go`

### Step 1: Write failing tests

```go
func TestAssignmentVersion_CreateAndList(t *testing.T) {
    s := openTestStore(t)
    ctx := context.Background()

    paID := "pa-ver-1"
    v1, err := s.CreateAssignmentVersion(ctx, paID, "body v1", "prompt v1", true)
    require.NoError(t, err)
    assert.True(t, v1.IsActive)

    v2, err := s.CreateAssignmentVersion(ctx, paID, "body v2", "prompt v2", true)
    require.NoError(t, err)
    assert.True(t, v2.IsActive)

    // SetActiveVersion should deactivate v1
    versions, err := s.ListAssignmentVersions(ctx, paID)
    require.NoError(t, err)
    require.Len(t, versions, 2)
    activeCount := 0
    for _, v := range versions {
        if v.IsActive { activeCount++ }
    }
    assert.Equal(t, 1, activeCount, "only one version should be active")
}

func TestAssignmentVersion_SetActive(t *testing.T) {
    s := openTestStore(t)
    ctx := context.Background()

    paID := "pa-ver-2"
    v1, _ := s.CreateAssignmentVersion(ctx, paID, "body v1", "", true)
    v2, _ := s.CreateAssignmentVersion(ctx, paID, "body v2", "", true)
    _ = v2

    err := s.SetActiveAssignmentVersion(ctx, paID, v1.ID)
    require.NoError(t, err)

    versions, _ := s.ListAssignmentVersions(ctx, paID)
    for _, v := range versions {
        if v.ID == v1.ID { assert.True(t, v.IsActive) }
        if v.ID == v2.ID { assert.False(t, v.IsActive) }
    }
}
```

- [ ] **Step 1: Run tests to verify they fail**

Run: `go test ./internal/store/... -run TestAssignmentVersion -v`
Expected: FAIL

### Step 2: Create `internal/store/assignment_version_repo.go`

```go
package store

import (
    "context"
    "database/sql"
    "fmt"
    "time"
    "github.com/google/uuid"
    "errors"
)

// CreateAssignmentVersion inserts a new version. If setActive is true,
// deactivates all other versions for the same assignment first (in a transaction).
func (s *Store) CreateAssignmentVersion(ctx context.Context, personAssignmentID, body, llmPrompt string, setActive bool) (*PersonAssignmentVersion, error) {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return nil, fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback()

    if setActive {
        if _, err := tx.ExecContext(ctx,
            `UPDATE person_assignment_versions SET is_active = 0 WHERE person_assignment_id = ?`,
            personAssignmentID,
        ); err != nil {
            return nil, fmt.Errorf("deactivate versions: %w", err)
        }
    }

    id := uuid.New().String()
    now := time.Now().UTC().Format(time.RFC3339)
    active := 0
    if setActive { active = 1 }

    if _, err := tx.ExecContext(ctx, `
        INSERT INTO person_assignment_versions (id, person_assignment_id, body, llm_prompt, is_active, created_at)
        VALUES (?, ?, ?, ?, ?, ?)
    `, id, personAssignmentID, body, llmPrompt, active, now); err != nil {
        return nil, fmt.Errorf("insert version: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return nil, fmt.Errorf("commit: %w", err)
    }

    return &PersonAssignmentVersion{
        ID: id, PersonAssignmentID: personAssignmentID,
        Body: body, LLMPrompt: llmPrompt,
        IsActive: setActive, CreatedAt: now,
    }, nil
}

// ListAssignmentVersions returns all versions for an assignment, newest first.
func (s *Store) ListAssignmentVersions(ctx context.Context, personAssignmentID string) ([]PersonAssignmentVersion, error) {
    rows, err := s.db.QueryContext(ctx, `
        SELECT id, person_assignment_id, body, COALESCE(llm_prompt,''), is_active, created_at
        FROM person_assignment_versions
        WHERE person_assignment_id = ?
        ORDER BY created_at DESC
    `, personAssignmentID)
    if err != nil {
        return nil, fmt.Errorf("list versions: %w", err)
    }
    defer rows.Close()

    var versions []PersonAssignmentVersion
    for rows.Next() {
        var v PersonAssignmentVersion
        var active int
        if err := rows.Scan(&v.ID, &v.PersonAssignmentID, &v.Body, &v.LLMPrompt, &active, &v.CreatedAt); err != nil {
            return nil, fmt.Errorf("scan version: %w", err)
        }
        v.IsActive = active == 1
        versions = append(versions, v)
    }
    return versions, nil
}

// SetActiveAssignmentVersion deactivates all versions for the assignment and activates the given one.
func (s *Store) SetActiveAssignmentVersion(ctx context.Context, personAssignmentID, versionID string) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback()

    if _, err := tx.ExecContext(ctx,
        `UPDATE person_assignment_versions SET is_active = 0 WHERE person_assignment_id = ?`,
        personAssignmentID,
    ); err != nil {
        return fmt.Errorf("deactivate: %w", err)
    }
    res, err := tx.ExecContext(ctx,
        `UPDATE person_assignment_versions SET is_active = 1 WHERE id = ? AND person_assignment_id = ?`,
        versionID, personAssignmentID,
    )
    if err != nil {
        return fmt.Errorf("activate: %w", err)
    }
    if n, _ := res.RowsAffected(); n == 0 {
        return fmt.Errorf("version %s not found", versionID)
    }
    return tx.Commit()
}

// GetActiveAssignmentVersion returns the currently active version, or (nil, nil) if none.
func (s *Store) GetActiveAssignmentVersion(ctx context.Context, personAssignmentID string) (*PersonAssignmentVersion, error) {
    var v PersonAssignmentVersion
    var active int
    err := s.db.QueryRowContext(ctx, `
        SELECT id, person_assignment_id, body, COALESCE(llm_prompt,''), is_active, created_at
        FROM person_assignment_versions
        WHERE person_assignment_id = ? AND is_active = 1
        LIMIT 1
    `, personAssignmentID).Scan(&v.ID, &v.PersonAssignmentID, &v.Body, &v.LLMPrompt, &active, &v.CreatedAt)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, nil
    }
    if err != nil {
        return nil, fmt.Errorf("get active version: %w", err)
    }
    v.IsActive = true
    return &v, nil
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/store/... -run TestAssignmentVersion -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/store/assignment_version_repo.go internal/store/assignment_version_repo_test.go
git commit -m "feat(store): person_assignment_versions repo with create/list/set-active"
```

---

## Task 5: Analysis service methods

**Files:**
- Modify: `internal/analysis/service.go`
- Modify: `internal/analysis/reasoning_chain.go`

### 5a — `PersonAssignmentProcessor.RunStep` stores rendered prompt

The only change: after `prompt` is assigned, add `step.LLMPrompt = prompt` just before the AI call.

- [ ] **Step 1: Write failing test** in `internal/analysis/integration_test.go`:

```go
func TestPersonAssignmentStep_LLMPromptStored(t *testing.T) {
    svc, store := openTestService(t)
    ctx := context.Background()
    contextID := createTestContext(t, store, ctx)
    addTestMessage(t, store, ctx, contextID, "Alice is working on JIRA-1")

    pas, err := svc.GeneratePersonAssignments(ctx, contextID)
    require.NoError(t, err)
    require.NotEmpty(t, pas)

    steps, err := store.ListChainSteps(ctx, appstore.ChainEntityPersonAssignment, pas[0].ID)
    require.NoError(t, err)
    require.NotEmpty(t, steps)
    assert.NotEmpty(t, steps[0].LLMPrompt, "LLMPrompt must be stored after generation")
}
```

- [ ] **Step 2: Set `step.LLMPrompt = prompt` in `RunStep`** (before the AI call)

- [ ] **Step 3: Run test** — Expected: PASS

### 5b — `CommitStep` accepts custom `llmPrompt`

- [ ] **Step 1: Add `llmPrompt string` param to `CommitStep`** in `reasoning_chain.go`. Pass it as `LLMPrompt` in the `AppendChainStep` call. Update both callers in `service.go` to pass `""`.

### 5c — New service methods

- [ ] **Step 1: Write failing tests** in `internal/analysis/integration_test.go`:

```go
func TestSaveAssignmentOutput_UpdatesBody(t *testing.T) {
    svc, store := openTestService(t)
    ctx := context.Background()
    contextID := createTestContext(t, store, ctx)
    addTestMessage(t, store, ctx, contextID, "Bob on JIRA-2")

    pas, _ := svc.GeneratePersonAssignments(ctx, contextID)
    require.NotEmpty(t, pas)

    updated, err := svc.SaveAssignmentOutput(ctx, pas[0].ID, "Manually edited body")
    require.NoError(t, err)
    assert.Equal(t, "Manually edited body", updated.Body)

    // A new active version must exist
    ver, err := store.GetActiveAssignmentVersion(ctx, pas[0].ID)
    require.NoError(t, err)
    require.NotNil(t, ver)
    assert.Equal(t, "Manually edited body", ver.Body)
    assert.True(t, ver.IsActive)
}

func TestRunAssignmentStepWithCustomPrompt_NoDBWrite(t *testing.T) {
    svc, store := openTestService(t)
    ctx := context.Background()
    contextID := createTestContext(t, store, ctx)
    addTestMessage(t, store, ctx, contextID, "Carol on JIRA-3")

    pas, _ := svc.GeneratePersonAssignments(ctx, contextID)
    require.NotEmpty(t, pas)

    output, err := svc.RunAssignmentStepWithCustomPrompt(ctx, pas[0].ID, 0, "Return: CUSTOM_OUTPUT")
    require.NoError(t, err)
    assert.NotEmpty(t, output)

    steps, _ := store.ListChainSteps(ctx, appstore.ChainEntityPersonAssignment, pas[0].ID)
    assert.Len(t, steps, 1, "RunWithCustomPrompt must not write to DB")
}

func TestCommitAssignmentVersionFromCustomPrompt_SavesVersion(t *testing.T) {
    svc, store := openTestService(t)
    ctx := context.Background()
    contextID := createTestContext(t, store, ctx)
    addTestMessage(t, store, ctx, contextID, "Dave on JIRA-4")

    pas, _ := svc.GeneratePersonAssignments(ctx, contextID)
    require.NotEmpty(t, pas)

    pa, err := svc.CommitAssignmentVersionFromCustomPrompt(ctx, pas[0].ID, 0, "my custom prompt", "custom output body")
    require.NoError(t, err)
    assert.Equal(t, "custom output body", pa.Body)

    ver, _ := store.GetActiveAssignmentVersion(ctx, pas[0].ID)
    require.NotNil(t, ver)
    assert.Equal(t, "custom output body", ver.Body)
    assert.Equal(t, "my custom prompt", ver.LLMPrompt)
    assert.True(t, ver.IsActive)
}
```

- [ ] **Step 2: Implement methods in `service.go`**:

```go
// SaveAssignmentOutput saves a manually edited output as a new active version.
func (s *Service) SaveAssignmentOutput(ctx context.Context, personAssignmentID, body string) (*appstore.PersonAssignment, error) {
    if err := s.store.UpdatePersonAssignmentBody(ctx, personAssignmentID, body); err != nil {
        return nil, fmt.Errorf("update assignment body: %w", err)
    }
    if _, err := s.store.CreateAssignmentVersion(ctx, personAssignmentID, body, "", true); err != nil {
        return nil, fmt.Errorf("create version: %w", err)
    }
    return s.store.GetPersonAssignment(ctx, personAssignmentID)
}

// RunAssignmentStepWithCustomPrompt runs the AI with customPrompt verbatim.
// Does NOT write to the DB. Returns the AI output for preview.
func (s *Service) RunAssignmentStepWithCustomPrompt(ctx context.Context, personAssignmentID string, stepIndex int, customPrompt string) (string, error) {
    steps, err := s.store.ListChainSteps(ctx, appstore.ChainEntityPersonAssignment, personAssignmentID)
    if err != nil {
        return "", fmt.Errorf("list steps: %w", err)
    }
    if stepIndex >= len(steps) {
        return "", fmt.Errorf("step %d out of range (chain has %d steps)", stepIndex, len(steps))
    }
    resp, err := s.ai.ChatCompletion(ctx, []openai.Message{{Role: "user", Content: customPrompt}})
    if err != nil {
        return "", fmt.Errorf("custom prompt AI call: %w", err)
    }
    return resp.Content, nil
}

// CommitAssignmentVersionFromCustomPrompt saves the custom-prompted output as a new
// active version (and a new chain step), syncing PersonAssignment.body.
func (s *Service) CommitAssignmentVersionFromCustomPrompt(ctx context.Context, personAssignmentID string, stepIndex int, customPrompt, outputState string) (*appstore.PersonAssignment, error) {
    saved, err := s.chainSvc.CommitStep(ctx, appstore.ChainEntityPersonAssignment, personAssignmentID, stepIndex, "", outputState, customPrompt, s.paJudge)
    if err != nil {
        return nil, fmt.Errorf("commit step: %w", err)
    }
    if saved.IsCurrent {
        if err := s.store.UpdatePersonAssignmentBody(ctx, personAssignmentID, saved.OutputState); err != nil {
            return nil, fmt.Errorf("sync assignment body: %w", err)
        }
        now := time.Now().UTC().Format(time.RFC3339)
        _ = s.store.SetPersonAssignmentAnalyzedThrough(ctx, personAssignmentID, now)
    }
    if _, err := s.store.CreateAssignmentVersion(ctx, personAssignmentID, outputState, customPrompt, true); err != nil {
        return nil, fmt.Errorf("create version: %w", err)
    }
    return s.store.GetPersonAssignment(ctx, personAssignmentID)
}
```

- [ ] **Step 3: Run tests** — Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/analysis/service.go internal/analysis/reasoning_chain.go internal/analysis/integration_test.go
git commit -m "feat(analysis): SaveAssignmentOutput, RunAssignmentStepWithCustomPrompt, CommitAssignmentVersionFromCustomPrompt"
```

---

## Task 6: Wails IPC layer + bindings

**Files:**
- Modify: `app_workflow.go`
- Modify: `frontend/wailsjs/go/main/App.d.ts`
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/src/types/index.ts`

- [ ] **Step 1: Add IPC methods to `app_workflow.go`**

```go
func (a *App) SaveAssignmentOutput(personAssignmentID, body string) (*appstore.PersonAssignment, error) {
    return a.analysisSvc.SaveAssignmentOutput(a.ctx, personAssignmentID, body)
}

func (a *App) RunAssignmentStepWithCustomPrompt(personAssignmentID string, stepIndex int, customPrompt string) (string, error) {
    return a.analysisSvc.RunAssignmentStepWithCustomPrompt(a.ctx, personAssignmentID, stepIndex, customPrompt)
}

func (a *App) CommitAssignmentVersionFromCustomPrompt(personAssignmentID string, stepIndex int, customPrompt, outputState string) (*appstore.PersonAssignment, error) {
    return a.analysisSvc.CommitAssignmentVersionFromCustomPrompt(a.ctx, personAssignmentID, stepIndex, customPrompt, outputState)
}

func (a *App) ListAssignmentVersions(personAssignmentID string) ([]appstore.PersonAssignmentVersion, error) {
    return a.store.ListAssignmentVersions(a.ctx, personAssignmentID)
}

func (a *App) SetActiveAssignmentVersion(personAssignmentID, versionID string) error {
    if err := a.store.SetActiveAssignmentVersion(a.ctx, personAssignmentID, versionID); err != nil {
        return err
    }
    ver, err := a.store.GetActiveAssignmentVersion(a.ctx, personAssignmentID)
    if err != nil || ver == nil {
        return err
    }
    return a.store.UpdatePersonAssignmentBody(a.ctx, personAssignmentID, ver.Body)
}
```

- [ ] **Step 2: Declare in `App.d.ts`**

```typescript
export function SaveAssignmentOutput(arg1: string, arg2: string): Promise<store.PersonAssignment>;
export function RunAssignmentStepWithCustomPrompt(arg1: string, arg2: number, arg3: string): Promise<string>;
export function CommitAssignmentVersionFromCustomPrompt(arg1: string, arg2: number, arg3: string, arg4: string): Promise<store.PersonAssignment>;
export function ListAssignmentVersions(arg1: string): Promise<store.PersonAssignmentVersion[]>;
export function SetActiveAssignmentVersion(arg1: string, arg2: string): Promise<void>;
```

- [ ] **Step 3: Bind in `App.js`** (standard `window['go']['main']['App'][...]` pattern)

- [ ] **Step 4: Add types to `frontend/src/types/index.ts`**

```typescript
export interface PersonAssignmentVersion {
  id: string;
  personAssignmentId: string;
  body: string;
  llmPrompt?: string;
  isActive: boolean;
  createdAt: string;  // ISO 8601
}
```

Also add `llmPrompt?: string` to `ReasoningChainStep`.

- [ ] **Step 5: Build to verify no type errors**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add app_workflow.go frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/main/App.js frontend/src/types/index.ts
git commit -m "feat(ipc): assignment output save, custom-prompt replay, and version management"
```

---

## Task 7: Frontend — PersonAssignment detail layout

**UI layout (per assignment):**

```
┌─────────────────────────────────────────────┐
│ SOURCE                                       │
│ [context messages / initial data]            │
├─────────────────────────────────────────────┤
│ PROMPT                                   Edit│  ← shows stored llmPrompt; Edit enters edit mode
│ [rendered prompt text, truncated]            │
├─────────────────────────────────────────────┤
│ OUTPUT                                  Save │  ← editable textarea; Save calls SaveAssignmentOutput
│ [assignment body]                            │
│                     Versions ▾ (if any)      │  ← version selector dropdown
└─────────────────────────────────────────────┘
```

**Files:**
- Create/Modify: `frontend/src/components/PersonAssignmentDetail.tsx`
- Create: `frontend/src/components/PersonAssignmentDetail.test.tsx`

- [ ] **Step 1: Write failing tests**

`frontend/src/components/PersonAssignmentDetail.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { PersonAssignmentDetail } from "./PersonAssignmentDetail";
import * as App from "../../wailsjs/go/main/App";

vi.mock("../../wailsjs/go/main/App", () => ({
  SaveAssignmentOutput: vi.fn(),
  RunAssignmentStepWithCustomPrompt: vi.fn(),
  CommitAssignmentVersionFromCustomPrompt: vi.fn(),
  ListAssignmentVersions: vi.fn().mockResolvedValue([]),
  SetActiveAssignmentVersion: vi.fn(),
}));

const makePA = (overrides = {}) => ({
  id: "pa-1",
  contextId: "ctx-1",
  name: "Alice",
  body: "Original assignment body",
  isOutdated: false,
  updatedAt: new Date().toISOString(),
  ...overrides,
});

const makeStep = (overrides = {}) => ({
  id: "step-1",
  entityType: "person_assignment",
  entityId: "pa-1",
  stepIndex: 0,
  sourceType: "initial" as const,
  messageContent: "Alice context from Jira",
  inputState: "",
  outputState: "Original assignment body",
  compressedSummary: "",
  eventsSnapshot: "",
  promptVersion: "pa_initial_v1",
  isCurrent: true,
  createdAt: new Date().toISOString(),
  llmPrompt: "You are an analyst. Generate for Alice.",
  ...overrides,
});

describe("PersonAssignmentDetail", () => {
  beforeEach(() => vi.clearAllMocks());

  it("shows SOURCE, PROMPT, and OUTPUT sections", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    expect(screen.getByText(/source/i)).toBeInTheDocument();
    expect(screen.getByText(/prompt/i)).toBeInTheDocument();
    expect(screen.getByText(/output/i)).toBeInTheDocument();
  });

  it("output textarea contains current body", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    const textarea = screen.getByRole("textbox", { name: /output/i });
    expect((textarea as HTMLTextAreaElement).value).toBe("Original assignment body");
  });

  it("Save button calls SaveAssignmentOutput with edited body", async () => {
    (App.SaveAssignmentOutput as ReturnType<typeof vi.fn>).mockResolvedValue(makePA({ body: "Edited body" }));
    const onUpdated = vi.fn();
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={onUpdated} />);

    const textarea = screen.getByRole("textbox", { name: /output/i });
    fireEvent.change(textarea, { target: { value: "Edited body" } });
    fireEvent.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => expect(App.SaveAssignmentOutput).toHaveBeenCalledWith("pa-1", "Edited body"));
    expect(onUpdated).toHaveBeenCalled();
  });

  it("prompt is displayed truncated", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    expect(screen.getByText(/You are an analyst/)).toBeInTheDocument();
  });

  it("shows 'No prompt stored' when llmPrompt is absent", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep({ llmPrompt: undefined })]} onUpdated={vi.fn()} />);
    expect(screen.getByText(/no prompt stored/i)).toBeInTheDocument();
  });
});
```

Run: `cd frontend && npx vitest run src/components/PersonAssignmentDetail.test.tsx`
Expected: FAIL

- [ ] **Step 2: Implement `PersonAssignmentDetail.tsx`**

Key state:
```typescript
const [outputDraft, setOutputDraft] = useState(pa.body);
const [isSaving, setIsSaving] = useState(false);
```

Key handlers:
```typescript
const handleSave = async () => {
  setIsSaving(true);
  try {
    const updated = await SaveAssignmentOutput(pa.id, outputDraft);
    onUpdated(updated);
  } finally {
    setIsSaving(false);
  }
};
```

Layout (three labeled sections stacked vertically):
1. **SOURCE** — read-only, shows the most recent step's `messageContent`
2. **PROMPT** — shows `step.llmPrompt` (truncated, 400 chars); Edit button if prompt exists (see Task 8)
3. **OUTPUT** — `<textarea>` bound to `outputDraft`; Save button; version selector below (see Task 9)

- [ ] **Step 3: Run tests** — Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/PersonAssignmentDetail.tsx frontend/src/components/PersonAssignmentDetail.test.tsx
git commit -m "feat(ui): PersonAssignmentDetail with SOURCE/PROMPT/OUTPUT layout and Save"
```

---

## Task 8: Frontend — Edit prompt → Send → Confirm/Cancel modal

When the user clicks **Edit** in the PROMPT section:
1. The PROMPT section expands to show a `<textarea>` pre-filled with the current prompt.
2. **Send** button calls `RunAssignmentStepWithCustomPrompt` — new output appears in a **modal**.
3. Modal has the new output (read-only) + **Confirm** / **Cancel**.
4. **Confirm** calls `CommitAssignmentVersionFromCustomPrompt`, closes modal, calls `onUpdated`.
5. **Cancel** closes modal with no changes.

**Files:**
- Modify: `frontend/src/components/PersonAssignmentDetail.tsx`
- Modify: `frontend/src/components/PersonAssignmentDetail.test.tsx`

- [ ] **Step 1: Write failing tests**

Add to `PersonAssignmentDetail.test.tsx`:

```tsx
describe("PersonAssignmentDetail — prompt edit + modal", () => {
  it("Edit button expands prompt textarea", () => {
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /edit/i }));
    const ta = screen.getByRole("textbox", { name: /prompt/i });
    expect((ta as HTMLTextAreaElement).value).toBe("You are an analyst. Generate for Alice.");
  });

  it("Send opens preview modal with LLM output", async () => {
    (App.RunAssignmentStepWithCustomPrompt as ReturnType<typeof vi.fn>).mockResolvedValue("Preview output from LLM");
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /edit/i }));
    fireEvent.click(screen.getByRole("button", { name: /send/i }));
    expect(await screen.findByText("Preview output from LLM")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /confirm/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /cancel/i })).toBeInTheDocument();
  });

  it("Confirm saves version and calls onUpdated", async () => {
    (App.RunAssignmentStepWithCustomPrompt as ReturnType<typeof vi.fn>).mockResolvedValue("Preview body");
    (App.CommitAssignmentVersionFromCustomPrompt as ReturnType<typeof vi.fn>).mockResolvedValue(makePA({ body: "Preview body" }));
    const onUpdated = vi.fn();
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={onUpdated} />);
    fireEvent.click(screen.getByRole("button", { name: /edit/i }));
    fireEvent.click(screen.getByRole("button", { name: /send/i }));
    await screen.findByText("Preview body");
    fireEvent.click(screen.getByRole("button", { name: /confirm/i }));
    await waitFor(() => expect(App.CommitAssignmentVersionFromCustomPrompt).toHaveBeenCalledWith(
      "pa-1", 0, "You are an analyst. Generate for Alice.", "Preview body"
    ));
    expect(onUpdated).toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: /confirm/i })).not.toBeInTheDocument();
  });

  it("Cancel on modal closes without saving", async () => {
    (App.RunAssignmentStepWithCustomPrompt as ReturnType<typeof vi.fn>).mockResolvedValue("Preview body");
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /edit/i }));
    fireEvent.click(screen.getByRole("button", { name: /send/i }));
    await screen.findByText("Preview body");
    fireEvent.click(screen.getByRole("button", { name: /cancel/i }));
    expect(App.CommitAssignmentVersionFromCustomPrompt).not.toHaveBeenCalled();
    expect(screen.queryByText("Preview body")).not.toBeInTheDocument();
  });
});
```

Run: `cd frontend && npx vitest run src/components/PersonAssignmentDetail.test.tsx`
Expected: FAIL

- [ ] **Step 2: Add prompt-edit state + modal to `PersonAssignmentDetail.tsx`**

New state:
```typescript
type PromptEditState =
  | { mode: "view" }
  | { mode: "editing"; draftPrompt: string }
  | { mode: "preview"; draftPrompt: string; previewOutput: string; isSending: boolean };

const [promptState, setPromptState] = useState<PromptEditState>({ mode: "view" });
```

Handlers:
```typescript
const handleSend = async () => {
  if (promptState.mode !== "editing") return;
  setPromptState(s => ({ ...s, mode: "preview" as const, previewOutput: "", isSending: true }));
  const output = await RunAssignmentStepWithCustomPrompt(pa.id, currentStepIndex, promptState.draftPrompt);
  setPromptState(s => s.mode === "preview" ? { ...s, previewOutput: output, isSending: false } : s);
};

const handleConfirm = async () => {
  if (promptState.mode !== "preview") return;
  const updated = await CommitAssignmentVersionFromCustomPrompt(
    pa.id, currentStepIndex, promptState.draftPrompt, promptState.previewOutput
  );
  onUpdated(updated);
  setPromptState({ mode: "view" });
};
```

Modal: a simple overlay `<dialog>` or `<div role="dialog">` showing the preview output + Confirm/Cancel buttons.

- [ ] **Step 3: Run tests** — Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/PersonAssignmentDetail.tsx frontend/src/components/PersonAssignmentDetail.test.tsx
git commit -m "feat(ui): prompt edit + Send → modal preview → Confirm/Cancel for custom prompts"
```

---

## Task 9: Frontend — Version selector + GitHub-style diff

When versions are available, a **Versions** dropdown appears below the OUTPUT section. Selecting a version shows a `VersionDiffModal`:
- Diff between selected version's body and the current body (word + line level, GitHub style: `−` red / `+` green)
- **Confirm** calls `SetActiveAssignmentVersion`, closes modal, calls `onUpdated`
- **Cancel** closes modal with no changes

**Files:**
- Create: `frontend/src/components/VersionDiffModal.tsx`
- Modify: `frontend/src/components/PersonAssignmentDetail.tsx`
- Modify: `frontend/src/components/PersonAssignmentDetail.test.tsx`

### VersionDiffModal

Use a word-diff algorithm (split both texts into words/tokens, run Myers diff, render `<del>`/`<ins>` spans per word). For line-level structure, first split into lines, then diff words within each changed line.

```tsx
// VersionDiffModal.tsx
import { diffWords } from "diff";  // npm package "diff"

interface Props {
  currentBody: string;
  selectedVersion: PersonAssignmentVersion;
  onConfirm: () => void;
  onCancel: () => void;
}

export function VersionDiffModal({ currentBody, selectedVersion, onConfirm, onCancel }: Props) {
  const parts = diffWords(currentBody, selectedVersion.body);

  return (
    <div role="dialog" aria-modal style={{ /* overlay styles */ }}>
      <div style={{ /* modal box styles */ }}>
        <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 8 }}>
          Version from {new Date(selectedVersion.createdAt).toLocaleString()}
        </div>
        <pre style={{ whiteSpace: "pre-wrap", fontSize: 11, lineHeight: 1.5 }}>
          {parts.map((part, i) =>
            part.added ? (
              <ins key={i} style={{ background: "#16a34a22", color: "#16a34a", textDecoration: "none" }}>{part.value}</ins>
            ) : part.removed ? (
              <del key={i} style={{ background: "#dc262622", color: "#dc2626" }}>{part.value}</del>
            ) : (
              <span key={i}>{part.value}</span>
            )
          )}
        </pre>
        <div style={{ display: "flex", gap: 8, marginTop: 12 }}>
          <button className="btn-primary" onClick={onConfirm}>Confirm</button>
          <button className="btn-secondary" onClick={onCancel}>Cancel</button>
        </div>
      </div>
    </div>
  );
}
```

Install the `diff` package:
```bash
cd frontend && npm install diff && npm install -D @types/diff
```

- [ ] **Step 1: Write failing tests** (in `PersonAssignmentDetail.test.tsx`):

```tsx
describe("PersonAssignmentDetail — version selector", () => {
  const v1: PersonAssignmentVersion = {
    id: "v1",
    personAssignmentId: "pa-1",
    body: "Old body text",
    isActive: false,
    createdAt: "2026-05-20T10:00:00Z",
  };

  it("shows version selector when versions exist", async () => {
    (App.ListAssignmentVersions as ReturnType<typeof vi.fn>).mockResolvedValue([v1]);
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    expect(await screen.findByRole("combobox", { name: /versions/i })).toBeInTheDocument();
  });

  it("selecting a version opens diff modal", async () => {
    (App.ListAssignmentVersions as ReturnType<typeof vi.fn>).mockResolvedValue([v1]);
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    const select = await screen.findByRole("combobox", { name: /versions/i });
    fireEvent.change(select, { target: { value: "v1" } });
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });

  it("Confirm in diff modal calls SetActiveAssignmentVersion", async () => {
    (App.ListAssignmentVersions as ReturnType<typeof vi.fn>).mockResolvedValue([v1]);
    (App.SetActiveAssignmentVersion as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    (App.ListAssignmentVersions as ReturnType<typeof vi.fn>).mockResolvedValue([]);
    const onUpdated = vi.fn();
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={onUpdated} />);
    const select = await screen.findByRole("combobox", { name: /versions/i });
    fireEvent.change(select, { target: { value: "v1" } });
    fireEvent.click(await screen.findByRole("button", { name: /confirm/i }));
    await waitFor(() => expect(App.SetActiveAssignmentVersion).toHaveBeenCalledWith("pa-1", "v1"));
  });

  it("Cancel in diff modal closes without saving", async () => {
    (App.ListAssignmentVersions as ReturnType<typeof vi.fn>).mockResolvedValue([v1]);
    render(<PersonAssignmentDetail pa={makePA()} steps={[makeStep()]} onUpdated={vi.fn()} />);
    const select = await screen.findByRole("combobox", { name: /versions/i });
    fireEvent.change(select, { target: { value: "v1" } });
    fireEvent.click(await screen.findByRole("button", { name: /cancel/i }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(App.SetActiveAssignmentVersion).not.toHaveBeenCalled();
  });
});
```

Run: `cd frontend && npx vitest run src/components/PersonAssignmentDetail.test.tsx`
Expected: FAIL

- [ ] **Step 2: Install `diff` package**

```bash
cd frontend && npm install diff && npm install -D @types/diff
```

- [ ] **Step 3: Create `VersionDiffModal.tsx`** (see code above)

- [ ] **Step 4: Add version state to `PersonAssignmentDetail.tsx`**

```typescript
const [versions, setVersions] = useState<PersonAssignmentVersion[]>([]);
const [selectedVersionId, setSelectedVersionId] = useState<string | null>(null);

useEffect(() => {
  ListAssignmentVersions(pa.id).then(setVersions).catch(() => {});
}, [pa.id]);

const selectedVersion = versions.find(v => v.id === selectedVersionId) ?? null;

const handleVersionConfirm = async () => {
  if (!selectedVersion) return;
  await SetActiveAssignmentVersion(pa.id, selectedVersion.id);
  const updated = await GetPersonAssignment(pa.id); // or reload via onUpdated
  onUpdated(updated);
  setSelectedVersionId(null);
  ListAssignmentVersions(pa.id).then(setVersions);
};
```

Version selector JSX (below OUTPUT textarea):
```tsx
{versions.length > 0 && (
  <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 8 }}>
    <label htmlFor="version-select" style={{ fontSize: 11, color: "var(--text-muted)" }}>Versions</label>
    <select
      id="version-select"
      aria-label="Versions"
      value={selectedVersionId ?? ""}
      onChange={e => setSelectedVersionId(e.target.value || null)}
      style={{ fontSize: 11 }}
    >
      <option value="">Select version…</option>
      {versions.map(v => (
        <option key={v.id} value={v.id}>
          {new Date(v.createdAt).toLocaleString()}{v.isActive ? " (active)" : ""}
        </option>
      ))}
    </select>
  </div>
)}

{selectedVersion && (
  <VersionDiffModal
    currentBody={pa.body}
    selectedVersion={selectedVersion}
    onConfirm={() => void handleVersionConfirm()}
    onCancel={() => setSelectedVersionId(null)}
  />
)}
```

- [ ] **Step 5: Run all tests**

```bash
cd frontend && npx vitest run src/components/PersonAssignmentDetail.test.tsx
go test ./... -count=1
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/VersionDiffModal.tsx \
        frontend/src/components/PersonAssignmentDetail.tsx \
        frontend/src/components/PersonAssignmentDetail.test.tsx \
        frontend/package.json frontend/package-lock.json
git commit -m "feat(ui): version selector + GitHub-style diff modal for assignment versions"
```

---

## Self-Review

**Spec coverage:**

| Requirement | Task |
|---|---|
| Initial tech assignment generated from initial context data | Pre-existing (GeneratePersonAssignments) |
| Display LLM output, editable with Save | Task 7 (OUTPUT textarea + Save button → SaveAssignmentOutput) |
| Prompt shown between source and output | Task 7 (PROMPT section, truncated display) |
| Edit prompt → Save prompt → Send → new output in modal | Task 8 (edit mode, Send → RunAssignmentStepWithCustomPrompt → modal) |
| Modal: Confirm saves as new version with is_active=true | Task 8 (CommitAssignmentVersionFromCustomPrompt → CreateAssignmentVersion with setActive=true) |
| Modal: Cancel dismisses without saving | Task 8 (cancel handler, no API call) |
| Version selector with date+time next to output | Task 9 (select dropdown, ISO date displayed) |
| Selecting version shows GitHub-style word/row diff | Task 9 (VersionDiffModal with `diff` word-diff) |
| Confirm in diff → selected version set as is_active | Task 9 (SetActiveAssignmentVersion) |
| Cancel in diff → diff view dismissed, no change | Task 9 (cancel sets selectedVersionId = null) |

**Type consistency:** `PersonAssignmentVersion` model flows Go → JSON → TypeScript consistently. `is_active` (SQLite int) → `IsActive` (Go bool) → `isActive` (TS bool).
