# on_checkout_command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an `on_checkout_command` field to `vcs_repos` that runs a shell command in the worktree directory after `CheckoutInWorkspace` succeeds, and expose it as an editable field in the Repositories tab.

**Architecture:** The field lives on `vcs_repos` as `on_checkout_command TEXT NOT NULL DEFAULT ''`. `CheckoutInWorkspace` in `app_vcs.go` fetches the repo after creating the workspace record and fire-and-forgets the command in the worktree directory via `sh -c`. The repo edit form in `ContextList.tsx` gains a third input for this field.

**Tech Stack:** Go 1.22, SQLite (goose migrations), Wails v2 (auto-generated TS bindings), React 18 + TypeScript

---

## File Map

| File | Change |
|------|--------|
| `internal/store/migrations/041_vcs_repo_checkout_command.sql` | **Create** — ALTER TABLE adds column |
| `internal/vcs/store/models.go` | **Modify** — add `OnCheckoutCommand` to `VCSRepo` |
| `internal/vcs/store/repo_repo.go` | **Modify** — SELECT/Scan in 3 methods + extend `UpdateRepoMeta` |
| `internal/vcs/store/repo_repo_test.go` | **Modify** — update existing tests + add new test |
| `app_vcs.go` | **Modify** — extend `UpdateTrackedRepoMeta`, add `runOnCheckoutCommand`, call from `CheckoutInWorkspace` |
| `frontend/wailsjs/go/models.ts` | **Modify** — add `onCheckoutCommand` to `VCSRepo` class |
| `frontend/wailsjs/go/main/App.d.ts` | **Modify** — update `UpdateTrackedRepoMeta` signature to 4 args |
| `frontend/wailsjs/go/main/App.js` | **Modify** — update `UpdateTrackedRepoMeta` call to pass 4 args |
| `frontend/src/pages/ContextList.tsx` | **Modify** — add `editOnCheckoutCommand` state, form field, pass to save |

---

## Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/041_vcs_repo_checkout_command.sql`

- [ ] **Step 1: Create migration file**

```sql
-- +goose Up
ALTER TABLE vcs_repos ADD COLUMN on_checkout_command TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; this is intentionally a no-op.
```

- [ ] **Step 2: Verify migration runs**

```bash
make db-reinit && make build-mcp
```

Expected: Build succeeds, no migration errors in output.

- [ ] **Step 3: Commit**

```bash
git add internal/store/migrations/041_vcs_repo_checkout_command.sql
git commit -m "feat(vcs): add on_checkout_command column to vcs_repos"
```

---

## Task 2: Model and Store Layer (TDD)

**Files:**
- Modify: `internal/vcs/store/models.go:14-24`
- Modify: `internal/vcs/store/repo_repo.go`
- Modify: `internal/vcs/store/repo_repo_test.go`

- [ ] **Step 1: Write failing tests**

Replace the contents of `internal/vcs/store/repo_repo_test.go` with:

```go
package vcsstore_test

import (
	"context"
	"testing"

	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRepoMeta(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "owner", "myrepo")
	require.NoError(t, err)
	assert.Equal(t, "", repo.Label)
	assert.Equal(t, "", repo.Description)
	assert.Equal(t, "", repo.OnCheckoutCommand)

	err = vcsStore.Repos.UpdateRepoMeta(ctx, repo.ID, "My Repo", "A test repository", "code .")
	require.NoError(t, err)

	updated, err := vcsStore.Repos.GetRepo(ctx, repo.ID)
	require.NoError(t, err)
	assert.Equal(t, "My Repo", updated.Label)
	assert.Equal(t, "A test repository", updated.Description)
	assert.Equal(t, "code .", updated.OnCheckoutCommand)
}

func TestUpdateRepoMetaNotFound(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()
	err := vcsStore.Repos.UpdateRepoMeta(ctx, "nonexistent-id", "label", "desc", "")
	assert.ErrorIs(t, err, vcsstore.ErrNotFound)
}

func TestOnCheckoutCommandRoundTrip(t *testing.T) {
	_, vcsStore := openTestDB(t)
	ctx := context.Background()

	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "acme", "backend")
	require.NoError(t, err)

	err = vcsStore.Repos.UpdateRepoMeta(ctx, repo.ID, "", "", "claude --dangerously-skip-permissions")
	require.NoError(t, err)

	fetched, err := vcsStore.Repos.GetRepo(ctx, repo.ID)
	require.NoError(t, err)
	assert.Equal(t, "claude --dangerously-skip-permissions", fetched.OnCheckoutCommand)

	// Clearing the command works too.
	err = vcsStore.Repos.UpdateRepoMeta(ctx, repo.ID, "", "", "")
	require.NoError(t, err)

	fetched, err = vcsStore.Repos.GetRepo(ctx, repo.ID)
	require.NoError(t, err)
	assert.Equal(t, "", fetched.OnCheckoutCommand)
}
```

- [ ] **Step 2: Run tests and confirm they fail**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator
go test ./internal/vcs/store/... -run "TestUpdateRepoMeta|TestOnCheckoutCommand" -v
```

Expected: FAIL — `too many arguments` or `undefined field OnCheckoutCommand`.

- [ ] **Step 3: Add `OnCheckoutCommand` to the `VCSRepo` struct**

In `internal/vcs/store/models.go`, change lines 14-24 from:

```go
type VCSRepo struct {
	ID          string `json:"id"`
	Platform    string `json:"platform"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	LocalPath   string `json:"localPath"`
	Label       string `json:"label"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
}
```

to:

```go
type VCSRepo struct {
	ID                string `json:"id"`
	Platform          string `json:"platform"`
	Owner             string `json:"owner"`
	Name              string `json:"name"`
	LocalPath         string `json:"localPath"`
	Label             string `json:"label"`
	Description       string `json:"description"`
	OnCheckoutCommand string `json:"onCheckoutCommand"`
	CreatedAt         string `json:"createdAt"`
}
```

- [ ] **Step 4: Update `GetRepo` in `repo_repo.go`**

Change the query and scan in `GetRepo` (lines 22-33):

```go
func (r *RepoRepo) GetRepo(ctx context.Context, id string) (*VCSRepo, error) {
	var repo VCSRepo
	err := r.db.QueryRowContext(ctx,
		`SELECT id, platform, owner, name, local_path, label, description, on_checkout_command, created_at FROM vcs_repos WHERE id = ?`, id,
	).Scan(&repo.ID, &repo.Platform, &repo.Owner, &repo.Name, &repo.LocalPath, &repo.Label, &repo.Description, &repo.OnCheckoutCommand, &repo.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get vcs repo: %w", err)
	}
	return &repo, nil
}
```

- [ ] **Step 5: Update `EnsureRepo` SELECT path in `repo_repo.go`**

Change the SELECT query and its Scan in `EnsureRepo` (the first db.QueryRowContext call, lines 39-44):

```go
err := r.db.QueryRowContext(ctx,
    `SELECT id, platform, owner, name, local_path, label, description, on_checkout_command, created_at FROM vcs_repos WHERE platform = ? AND owner = ? AND name = ?`,
    platform, owner, name,
).Scan(&repo.ID, &repo.Platform, &repo.Owner, &repo.Name, &repo.LocalPath, &repo.Label, &repo.Description, &repo.OnCheckoutCommand, &repo.CreatedAt)
```

- [ ] **Step 6: Update `ListRepos` scan in `repo_repo.go`**

Change the SELECT query string and Scan in `ListRepos` (lines 64 and 81):

Query string (line 64):
```go
q := `SELECT id, platform, owner, name, local_path, label, description, on_checkout_command, created_at FROM vcs_repos`
```

Scan call (line 81):
```go
if err := rows.Scan(&repo.ID, &repo.Platform, &repo.Owner, &repo.Name, &repo.LocalPath, &repo.Label, &repo.Description, &repo.OnCheckoutCommand, &repo.CreatedAt); err != nil {
```

- [ ] **Step 7: Update `UpdateRepoMeta` in `repo_repo.go`**

Change the `UpdateRepoMeta` method (lines 113-125):

```go
func (r *RepoRepo) UpdateRepoMeta(ctx context.Context, id, label, description, onCheckoutCommand string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE vcs_repos SET label = ?, description = ?, on_checkout_command = ? WHERE id = ?`,
		label, description, onCheckoutCommand, id,
	)
	if err != nil {
		return fmt.Errorf("update vcs repo meta: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Step 8: Run tests and confirm they pass**

```bash
go test ./internal/vcs/store/... -run "TestUpdateRepoMeta|TestOnCheckoutCommand" -v
```

Expected: PASS for all three tests.

- [ ] **Step 9: Run full test suite to catch any regressions**

```bash
make test
```

Expected: All tests pass. If `UpdateRepoMeta` is called with 3 args elsewhere (in `app_vcs.go`), it will fail to compile here — that's expected and fixed in Task 3.

- [ ] **Step 10: Commit**

```bash
git add internal/vcs/store/models.go internal/vcs/store/repo_repo.go internal/vcs/store/repo_repo_test.go
git commit -m "feat(vcs): add OnCheckoutCommand to VCSRepo model and store"
```

---

## Task 3: App Method + Checkout Command Execution

**Files:**
- Modify: `app_vcs.go`

- [ ] **Step 1: Update `UpdateTrackedRepoMeta` in `app_vcs.go`**

Change the method signature and body (lines 240-243):

```go
// UpdateTrackedRepoMeta updates the human-readable label, description, and on-checkout command for a tracked repo.
func (a *App) UpdateTrackedRepoMeta(repoID, label, description, onCheckoutCommand string) error {
	return a.vcsStore.Repos.UpdateRepoMeta(a.ctx, repoID, label, description, onCheckoutCommand)
}
```

- [ ] **Step 2: Add `runOnCheckoutCommand` helper in `app_vcs.go`**

Add this method after `openTerminalAt` (after line 345):

```go
// runOnCheckoutCommand executes command in workDir via sh -c. Fire-and-forget.
func (a *App) runOnCheckoutCommand(command, workDir string) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = workDir
	if err := cmd.Start(); err != nil {
		a.logger.Sugar().Warnf("on_checkout_command failed to start: cmd=%q workDir=%q err=%v", command, workDir, err)
	}
}
```

- [ ] **Step 3: Call the command in `CheckoutInWorkspace`**

In `CheckoutInWorkspace` (lines 380-408), add the command execution after the `vcsStore.Workspaces.Create` call and before `openTerminalAt`:

Change lines 403-407 from:

```go
	if err := a.vcsStore.Workspaces.Create(a.ctx, w); err != nil {
		return fmt.Errorf("storing workspace record: %w", err)
	}
	return a.openTerminalAt(worktreePath)
```

to:

```go
	if err := a.vcsStore.Workspaces.Create(a.ctx, w); err != nil {
		return fmt.Errorf("storing workspace record: %w", err)
	}
	repo, err := a.vcsStore.Repos.GetRepo(a.ctx, repoPath.RepoID)
	if err == nil && repo.OnCheckoutCommand != "" {
		a.runOnCheckoutCommand(repo.OnCheckoutCommand, worktreePath)
	}
	return a.openTerminalAt(worktreePath)
```

- [ ] **Step 4: Build to confirm no compile errors**

```bash
make build-mcp
```

Expected: Builds successfully (or if Wails bindings haven't been updated yet, the frontend won't compile — that's fixed in Task 4).

- [ ] **Step 5: Run Go tests only**

```bash
go build ./...
```

Expected: No errors.

- [ ] **Step 6: Commit**

```bash
git add app_vcs.go
git commit -m "feat(vcs): run on_checkout_command after CheckoutInWorkspace"
```

---

## Task 4: Update Wails-Generated Bindings

Wails generates these files from Go types — update them manually to match the new `VCSRepo` struct and `UpdateTrackedRepoMeta` signature.

**Files:**
- Modify: `frontend/wailsjs/go/models.ts`
- Modify: `frontend/wailsjs/go/main/App.d.ts`
- Modify: `frontend/wailsjs/go/main/App.js`

- [ ] **Step 1: Update `VCSRepo` class in `models.ts`**

In `frontend/wailsjs/go/models.ts`, change the `VCSRepo` class (lines 1172-1197) from:

```typescript
	export class VCSRepo {
	    id: string;
	    platform: string;
	    owner: string;
	    name: string;
	    localPath: string;
	    label: string;
	    description: string;
	    createdAt: string;

	    static createFrom(source: any = {}) {
	        return new VCSRepo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.platform = source["platform"];
	        this.owner = source["owner"];
	        this.name = source["name"];
	        this.localPath = source["localPath"];
	        this.label = source["label"];
	        this.description = source["description"];
	        this.createdAt = source["createdAt"];
	    }
	}
```

to:

```typescript
	export class VCSRepo {
	    id: string;
	    platform: string;
	    owner: string;
	    name: string;
	    localPath: string;
	    label: string;
	    description: string;
	    onCheckoutCommand: string;
	    createdAt: string;

	    static createFrom(source: any = {}) {
	        return new VCSRepo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.platform = source["platform"];
	        this.owner = source["owner"];
	        this.name = source["name"];
	        this.localPath = source["localPath"];
	        this.label = source["label"];
	        this.description = source["description"];
	        this.onCheckoutCommand = source["onCheckoutCommand"] ?? "";
	        this.createdAt = source["createdAt"];
	    }
	}
```

- [ ] **Step 2: Update `UpdateTrackedRepoMeta` signature in `App.d.ts`**

In `frontend/wailsjs/go/main/App.d.ts`, change line 275 from:

```typescript
export function UpdateTrackedRepoMeta(arg1:string,arg2:string,arg3:string):Promise<void>;
```

to:

```typescript
export function UpdateTrackedRepoMeta(arg1:string,arg2:string,arg3:string,arg4:string):Promise<void>;
```

- [ ] **Step 3: Update `UpdateTrackedRepoMeta` call in `App.js`**

In `frontend/wailsjs/go/main/App.js`, change lines 529-530 from:

```javascript
export function UpdateTrackedRepoMeta(arg1, arg2, arg3) {
  return window['go']['main']['App']['UpdateTrackedRepoMeta'](arg1, arg2, arg3);
```

to:

```javascript
export function UpdateTrackedRepoMeta(arg1, arg2, arg3, arg4) {
  return window['go']['main']['App']['UpdateTrackedRepoMeta'](arg1, arg2, arg3, arg4);
```

- [ ] **Step 4: Commit**

```bash
git add frontend/wailsjs/go/models.ts frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/main/App.js
git commit -m "feat(vcs): update Wails bindings for onCheckoutCommand and UpdateTrackedRepoMeta"
```

---

## Task 5: Frontend — Repo Edit Form

**Files:**
- Modify: `frontend/src/pages/ContextList.tsx`

All line references below are approximate — find the exact lines by searching for the surrounding context shown.

- [ ] **Step 1: Add `editOnCheckoutCommand` state**

Find the four state declarations near line 251:

```tsx
  const [editingRepoId, setEditingRepoId] = useState<string | null>(null);
  // ...
  const [editLabel, setEditLabel] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [editSaving, setEditSaving] = useState(false);
  const [editError, setEditError] = useState("");
```

Add one line after `editDescription`:

```tsx
  const [editOnCheckoutCommand, setEditOnCheckoutCommand] = useState("");
```

- [ ] **Step 2: Initialize the new state in `startEdit`**

Find `startEdit` (near line 730):

```tsx
  const startEdit = (repo: VCSRepo) => {
    setEditingRepoId(repo.id);
    setEditLabel(repo.label ?? "");
    setEditDescription(repo.description ?? "");
    setEditError("");
  };
```

Change to:

```tsx
  const startEdit = (repo: VCSRepo) => {
    setEditingRepoId(repo.id);
    setEditLabel(repo.label ?? "");
    setEditDescription(repo.description ?? "");
    setEditOnCheckoutCommand(repo.onCheckoutCommand ?? "");
    setEditError("");
  };
```

- [ ] **Step 3: Pass `onCheckoutCommand` in `handleUpdateRepoMeta`**

Find `handleUpdateRepoMeta` (near line 716):

```tsx
  const handleUpdateRepoMeta = async (repoId: string) => {
    setEditSaving(true);
    setEditError("");
    try {
      await UpdateTrackedRepoMeta(repoId, editLabel.trim(), editDescription.trim());
```

Change the `UpdateTrackedRepoMeta` call to:

```tsx
      await UpdateTrackedRepoMeta(repoId, editLabel.trim(), editDescription.trim(), editOnCheckoutCommand.trim());
```

- [ ] **Step 4: Add the input field in the edit form**

Find the Description input block in the edit form (around line 1298-1313):

```tsx
                          <div className="repo-card-edit-row">
                            <label>Description</label>
                            <input
                              value={editDescription}
                              onChange={(e) => setEditDescription(e.target.value)}
                              placeholder="Short description (optional)"
                              autoCorrect="off"
                              autoCapitalize="none"
                              spellCheck={false}
                              onKeyDown={(e) => {
                                if (e.key === "Enter") { e.preventDefault(); void handleUpdateRepoMeta(repo.id); }
                                if (e.key === "Escape") cancelEdit();
                              }}
                            />
                          </div>
```

Add a new row **after** the Description block (before the `{editError &&` line):

```tsx
                          <div className="repo-card-edit-row">
                            <label>On checkout</label>
                            <input
                              value={editOnCheckoutCommand}
                              onChange={(e) => setEditOnCheckoutCommand(e.target.value)}
                              placeholder="Command to run after checkout (e.g. code .)"
                              autoCorrect="off"
                              autoCapitalize="none"
                              spellCheck={false}
                              onKeyDown={(e) => {
                                if (e.key === "Enter") { e.preventDefault(); void handleUpdateRepoMeta(repo.id); }
                                if (e.key === "Escape") cancelEdit();
                              }}
                            />
                          </div>
```

- [ ] **Step 5: Build and verify no TypeScript errors**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator/frontend && npm run build
```

Expected: Build succeeds, no TypeScript errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/ContextList.tsx
git commit -m "feat(vcs): add on_checkout_command field to repository edit form"
```

---

## Task 6: Full Integration Build

- [ ] **Step 1: Run full test suite**

```bash
cd /Users/artschekoff/Documents/jiro/context-consolidator && make test
```

Expected: All tests pass.

- [ ] **Step 2: Build all binaries**

```bash
make build
```

Expected: All three binaries build successfully.

- [ ] **Step 3: Manual smoke test**

1. Start the app (`make dev`)
2. Open Settings > Repositories tab
3. Click the Edit button on any tracked repo
4. Confirm three fields appear: Label, Description, On checkout
5. Enter `echo hello` in the On checkout field, click Save
6. Open a context with that repo's branch, click Checkout
7. Confirm `echo hello` runs (no visible output in UI, but no crash either)

- [ ] **Step 4: Final commit (if any stragglers)**

```bash
git status
# If clean, nothing to do. Otherwise:
git add -p
git commit -m "chore: final integration fixes for on_checkout_command"
```
