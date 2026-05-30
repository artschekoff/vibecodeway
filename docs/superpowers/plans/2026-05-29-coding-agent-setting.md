# Coding Agent App Setting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a global `codingAgentApp` setting (default: `cursor`) so that checkout/open actions launch the coding agent at the repo path, while `onCheckoutCommand` is now executed visibly inside the terminal app window instead of silently in the background.

**Architecture:** A new `coding_agent_app` column is added to `app_settings`. The unified `openWithCheckout(onCheckoutCommand, path string)` function in `app_vcs.go` replaces all separate `runOnCheckoutCommand` + `openTerminalAt` call sites. When a terminal is configured, a temp shell script (containing the checkout command and the coding agent invocation) is written to `/tmp` and opened via `open -a <terminalApp> <script>` — the terminal runs the script visibly. If no terminal is configured, the coding agent is launched directly.

**Tech Stack:** Go (Wails), SQLite migrations (goose), React/TypeScript frontend

---

## File Map

| File | Change |
|---|---|
| `internal/store/migrations/063_coding_agent_app.sql` | Create — new column |
| `internal/store/models.go` | Modify — add `CodingAgentApp` to `AppSettings` |
| `internal/store/app_settings_repo.go` | Modify — include new field in `GET`/`SET` |
| `internal/store/app_settings_terminal_test.go` | Modify — extend with `CodingAgentApp` coverage |
| `app_settings.go` | Modify — add `codingAgentApp` param to `SetAppSettings` |
| `app_vcs.go` | Modify — replace `openTerminalAt` + `runOnCheckoutCommand` with `openWithCheckout` |
| `frontend/src/pages/Settings.tsx` | Modify — add Coding Agent input field in the Terminal section |

---

## Task 1: DB Migration

**Files:**
- Create: `internal/store/migrations/063_coding_agent_app.sql`

- [ ] **Step 1: Write the migration file**

```sql
-- +goose Up
ALTER TABLE app_settings ADD COLUMN coding_agent_app TEXT NOT NULL DEFAULT 'cursor';

-- +goose Down
ALTER TABLE app_settings DROP COLUMN coding_agent_app;
```

- [ ] **Step 2: Verify migration file is in place**

Run: `ls internal/store/migrations/ | grep 063`
Expected: `063_coding_agent_app.sql`

- [ ] **Step 3: Commit**

```bash
git add internal/store/migrations/063_coding_agent_app.sql
git commit -m "feat(settings): add coding_agent_app migration"
```

---

## Task 2: Model + Store Layer

**Files:**
- Modify: `internal/store/models.go:126-133`
- Modify: `internal/store/app_settings_repo.go`

- [ ] **Step 1: Write the failing test**

In `internal/store/app_settings_terminal_test.go`, add a new test function after `TestAppSettings_Terminal`:

```go
func TestAppSettings_CodingAgent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	t.Run("default CodingAgentApp is cursor", func(t *testing.T) {
		settings, err := s.GetAppSettings(ctx)
		require.NoError(t, err)
		assert.Equal(t, "cursor", settings.CodingAgentApp)
	})

	t.Run("SetAppSettings persists CodingAgentApp", func(t *testing.T) {
		err := s.SetAppSettings(ctx, AppSettings{
			ContextRefetchEnabled:     false,
			ContextRefetchIntervalMin: 30,
			ReviewsRefetchEnabled:     false,
			ReviewsRefetchIntervalMin: 30,
			TerminalAppPath:           "kitty",
			CodingAgentApp:            "code",
		})
		require.NoError(t, err)

		got, err := s.GetAppSettings(ctx)
		require.NoError(t, err)
		assert.Equal(t, "code", got.CodingAgentApp)
	})
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/store/... -run TestAppSettings_CodingAgent -v`
Expected: FAIL — `CodingAgentApp` field does not exist yet

- [ ] **Step 3: Add `CodingAgentApp` field to the model**

In `internal/store/models.go`, update the `AppSettings` struct:

```go
// AppSettings holds application-level preferences stored in the local database.
type AppSettings struct {
	ContextRefetchEnabled     bool   `json:"contextRefetchEnabled"`
	ContextRefetchIntervalMin int    `json:"contextRefetchIntervalMin"`
	ReviewsRefetchEnabled     bool   `json:"reviewsRefetchEnabled"`
	ReviewsRefetchIntervalMin int    `json:"reviewsRefetchIntervalMin"`
	TerminalAppPath           string `json:"terminalAppPath"`
	CodingAgentApp            string `json:"codingAgentApp"`
	UpdatedAt                 string `json:"updatedAt"`
}
```

- [ ] **Step 4: Update `defaultAppSettings` in `app_settings_repo.go`**

Change the `defaultAppSettings` function:

```go
func defaultAppSettings() *AppSettings {
	return &AppSettings{
		ContextRefetchEnabled:     false,
		ContextRefetchIntervalMin: 30,
		ReviewsRefetchEnabled:     false,
		ReviewsRefetchIntervalMin: 30,
		TerminalAppPath:           "",
		CodingAgentApp:            "cursor",
	}
}
```

- [ ] **Step 5: Update `GetAppSettings` in `app_settings_repo.go`**

Add `codingAgentApp` variable and include `coding_agent_app` in the SELECT:

```go
func (s *Store) GetAppSettings(ctx context.Context) (*AppSettings, error) {
	var (
		contextEnabled     int
		contextIntervalMin int
		reviewsEnabled     int
		reviewsIntervalMin int
		terminalAppPath    string
		codingAgentApp     string
		updatedAt          string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT context_refetch_enabled, context_refetch_interval_min,
		        reviews_refetch_enabled, reviews_refetch_interval_min,
		        terminal_app_path, coding_agent_app, updated_at
		 FROM app_settings WHERE id = ?`,
		appSettingsSingletonID,
	).Scan(&contextEnabled, &contextIntervalMin, &reviewsEnabled, &reviewsIntervalMin, &terminalAppPath, &codingAgentApp, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultAppSettings(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading app settings: %w", err)
	}
	return &AppSettings{
		ContextRefetchEnabled:     contextEnabled != 0,
		ContextRefetchIntervalMin: contextIntervalMin,
		ReviewsRefetchEnabled:     reviewsEnabled != 0,
		ReviewsRefetchIntervalMin: reviewsIntervalMin,
		TerminalAppPath:           terminalAppPath,
		CodingAgentApp:            codingAgentApp,
		UpdatedAt:                 updatedAt,
	}, nil
}
```

- [ ] **Step 6: Update `SetAppSettings` in `app_settings_repo.go`**

Add `coding_agent_app` to the upsert:

```go
func (s *Store) SetAppSettings(ctx context.Context, settings AppSettings) error {
	now := time.Now().UTC().Format(time.RFC3339)

	contextEnabled := 0
	if settings.ContextRefetchEnabled {
		contextEnabled = 1
	}
	reviewsEnabled := 0
	if settings.ReviewsRefetchEnabled {
		reviewsEnabled = 1
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO app_settings
		     (id, context_refetch_enabled, context_refetch_interval_min,
		      reviews_refetch_enabled, reviews_refetch_interval_min,
		      terminal_app_path, coding_agent_app, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		     context_refetch_enabled      = excluded.context_refetch_enabled,
		     context_refetch_interval_min = excluded.context_refetch_interval_min,
		     reviews_refetch_enabled      = excluded.reviews_refetch_enabled,
		     reviews_refetch_interval_min = excluded.reviews_refetch_interval_min,
		     terminal_app_path            = excluded.terminal_app_path,
		     coding_agent_app             = excluded.coding_agent_app,
		     updated_at                   = excluded.updated_at`,
		appSettingsSingletonID,
		contextEnabled, settings.ContextRefetchIntervalMin,
		reviewsEnabled, settings.ReviewsRefetchIntervalMin,
		settings.TerminalAppPath,
		settings.CodingAgentApp,
		now,
	)
	if err != nil {
		return fmt.Errorf("saving app settings: %w", err)
	}
	return nil
}
```

- [ ] **Step 7: Run the test to verify it passes**

Run: `go test ./internal/store/... -run TestAppSettings_CodingAgent -v`
Expected: PASS

- [ ] **Step 8: Run full store tests to check for regressions**

Run: `go test ./internal/store/... -v`
Expected: all PASS

- [ ] **Step 9: Commit**

```bash
git add internal/store/models.go internal/store/app_settings_repo.go internal/store/app_settings_terminal_test.go
git commit -m "feat(settings): add CodingAgentApp to AppSettings model and store"
```

---

## Task 3: Wails API Layer

**Files:**
- Modify: `app_settings.go:31-54`

- [ ] **Step 1: Update `SetAppSettings` to accept `codingAgentApp`**

Replace the function in `app_settings.go`:

```go
// SetAppSettings persists new application settings and immediately restarts the
// background refresh goroutine so changes take effect without an app restart.
func (a *App) SetAppSettings(
	contextEnabled bool, contextIntervalMin int,
	reviewsEnabled bool, reviewsIntervalMin int,
	terminalAppPath string,
	codingAgentApp string,
) error {
	if contextIntervalMin < 1 {
		contextIntervalMin = 1
	}
	if reviewsIntervalMin < 1 {
		reviewsIntervalMin = 1
	}
	settings := appstore.AppSettings{
		ContextRefetchEnabled:     contextEnabled,
		ContextRefetchIntervalMin: contextIntervalMin,
		ReviewsRefetchEnabled:     reviewsEnabled,
		ReviewsRefetchIntervalMin: reviewsIntervalMin,
		TerminalAppPath:           terminalAppPath,
		CodingAgentApp:            codingAgentApp,
	}
	if err := a.db.SetAppSettings(a.ctx, settings); err != nil {
		return err
	}
	a.restartBackgroundRefresh(settings)
	return nil
}
```

- [ ] **Step 2: Verify the app compiles**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Regenerate Wails bindings**

Run: `wails generate module`
Expected: `frontend/wailsjs/go/main/App.js` and `App.d.ts` updated — `SetAppSettings` now has 6 params

- [ ] **Step 4: Commit**

```bash
git add app_settings.go frontend/wailsjs/
git commit -m "feat(settings): add codingAgentApp param to SetAppSettings Wails binding"
```

---

## Task 4: Checkout Logic in `app_vcs.go`

**Files:**
- Modify: `app_vcs.go:337-366` (replace `openTerminalAt` and `runOnCheckoutCommand`)
- Modify: call sites at lines 334, 462, 484-486, 497-499, 508

The new unified function `openWithCheckout(onCheckoutCommand, path string)` replaces all separate `runOnCheckoutCommand` + `openTerminalAt` combos.

**New behavior:**
- Terminal configured + onCheckoutCommand set: write temp script (`cd path; <cmd>; <codingAgent> path`), open via `open -a <terminal> <script>`
- Terminal configured, no onCheckoutCommand: temp script is just `cd path; <codingAgent> path` — terminal opens and immediately launches agent
- Terminal configured, no codingAgent: temp script is just `cd path; <cmd>` (or empty / just opens terminal at path)
- No terminal, codingAgent configured: exec `codingAgentApp path` directly
- Neither: no-op

**PATH note:** The coding agent (e.g. `cursor`, `code`) may not be in the app's PATH. Use the same `resolveBinPath` pattern from `internal/jira/acli/runner.go`.

- [ ] **Step 1: Add `openWithCheckout` and helpers**

In `app_vcs.go`, replace the `openTerminalAt` function (lines 337-353) and `runOnCheckoutCommand` function (lines 355-366) with:

```go
// knownAgentDirs is the minimal set of directories checked when the app's
// PATH doesn't include Homebrew or user-local installs.
var knownAgentDirs = []string{
	"/opt/homebrew/bin",
	"/usr/local/bin",
	"/usr/bin",
}

// resolveAgentPath resolves a coding agent executable name to an absolute path,
// checking known extra dirs when exec.LookPath fails (Wails apps have a stripped PATH).
func resolveAgentPath(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	for _, dir := range knownAgentDirs {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return name
}

// openWithCheckout runs onCheckoutCommand (if set) and opens the configured
// coding agent at path. When a terminal is configured the command and agent
// launch are bundled into a temp script that is executed inside the terminal
// window so the user can see output. Falls back to launching the agent directly
// when no terminal is configured.
func (a *App) openWithCheckout(onCheckoutCommand, path string) error {
	settings, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		a.logger.Sugar().Warnf("openWithCheckout: failed to load settings: %v", err)
		return nil
	}

	terminalApp := settings.TerminalAppPath
	agentApp := settings.CodingAgentApp

	if terminalApp == "" && agentApp == "" {
		return nil
	}

	// No terminal configured: launch coding agent directly.
	if terminalApp == "" {
		agentBin := resolveAgentPath(agentApp)
		cmd := exec.Command(agentBin, path)
		if err := cmd.Start(); err != nil {
			a.logger.Sugar().Warnf("open coding agent failed: app=%s err=%v", agentApp, err)
		}
		return nil
	}

	// Terminal configured: compose a script and open it inside the terminal window.
	script := a.buildCheckoutScript(onCheckoutCommand, agentApp, path)
	scriptFile, err := os.CreateTemp("", "jiro-checkout-*.sh")
	if err != nil {
		a.logger.Sugar().Warnf("openWithCheckout: failed to create temp script: %v", err)
		return nil
	}
	if _, err := scriptFile.WriteString(script); err != nil {
		_ = scriptFile.Close()
		a.logger.Sugar().Warnf("openWithCheckout: failed to write temp script: %v", err)
		return nil
	}
	_ = scriptFile.Close()
	if err := os.Chmod(scriptFile.Name(), 0o755); err != nil {
		a.logger.Sugar().Warnf("openWithCheckout: chmod failed: %v", err)
		return nil
	}

	cmd := exec.Command("open", "-a", terminalApp, scriptFile.Name())
	if err := cmd.Start(); err != nil {
		a.logger.Sugar().Warnf("open terminal failed: app=%s err=%v", terminalApp, err)
	}
	return nil
}

// buildCheckoutScript returns the content of the temp shell script that the
// terminal will execute. The script: cds into path, runs onCheckoutCommand
// (if set), then launches agentApp (if set). It deletes itself on exit.
func (a *App) buildCheckoutScript(onCheckoutCommand, agentApp, path string) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "cd %q\n", path)
	if onCheckoutCommand != "" {
		b.WriteString(onCheckoutCommand)
		b.WriteString("\n")
	}
	if agentApp != "" {
		agentBin := resolveAgentPath(agentApp)
		fmt.Fprintf(&b, "%q %q\n", agentBin, path)
	}
	// Self-cleanup so /tmp doesn't accumulate scripts.
	b.WriteString(`rm -- "$0"` + "\n")
	return b.String()
}
```

Make sure `"strings"` is imported in `app_vcs.go` (add to imports if not already present).

- [ ] **Step 2: Replace all `openTerminalAt` + `runOnCheckoutCommand` call sites**

Update `CheckoutBranch` (line 327):
```go
func (a *App) CheckoutBranch(localPath, branch string) error {
	if _, err := os.Stat(localPath); err != nil {
		return fmt.Errorf("local path not accessible: %w", err)
	}
	if err := vcs.GitCheckout(localPath, branch); err != nil {
		return err
	}
	return a.openWithCheckout("", localPath)
}
```

Update `CheckoutInWorkspace` — replace the two blocks that called `runOnCheckoutCommand` + `openTerminalAt` (lines 458-462 and 482-486):

First block (reused worktree):
```go
repo, err := a.vcsStore.Repos.GetRepo(a.ctx, repoPath.RepoID)
onCheckout := ""
if err == nil {
    onCheckout = repo.OnCheckoutCommand
}
return a.openWithCheckout(onCheckout, wt.Path)
```

Second block (new worktree):
```go
repo, err := a.vcsStore.Repos.GetRepo(a.ctx, repoPath.RepoID)
onCheckout := ""
if err == nil {
    onCheckout = repo.OnCheckoutCommand
}
return a.openWithCheckout(onCheckout, worktreePath)
```

Update `OpenWorkspaceTerminal` (line 489):
```go
func (a *App) OpenWorkspaceTerminal(workspaceID string) error {
	w, err := a.vcsStore.Workspaces.Get(a.ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}
	repo, err := a.vcsStore.Repos.GetRepo(a.ctx, w.RepoID)
	onCheckout := ""
	if err == nil {
		onCheckout = repo.OnCheckoutCommand
	}
	return a.openWithCheckout(onCheckout, w.WorktreePath)
}
```

Update `OpenRepoPathTerminal` (line 502):
```go
func (a *App) OpenRepoPathTerminal(pathID string) error {
	rp, err := a.vcsStore.RepoPaths.Get(a.ctx, pathID)
	if err != nil {
		return fmt.Errorf("repo path not found: %w", err)
	}
	return a.openWithCheckout("", rp.Path)
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: no errors (all call sites updated, no unused functions)

- [ ] **Step 4: Commit**

```bash
git add app_vcs.go
git commit -m "feat(vcs): replace openTerminalAt+runOnCheckoutCommand with openWithCheckout"
```

---

## Task 5: Frontend — Settings Page

**Files:**
- Modify: `frontend/src/pages/Settings.tsx`

- [ ] **Step 1: Add `codingAgentApp` state variable**

After line 70 (`const [terminalAppPath, setTerminalAppPath] = useState("");`), add:

```tsx
const [codingAgentApp, setCodingAgentApp] = useState("cursor");
```

- [ ] **Step 2: Populate state from loaded settings**

In the `useEffect` block where `terminalAppPath` is set (line ~101), add:

```tsx
setCodingAgentApp(typedAppS.codingAgentApp ?? "cursor");
```

- [ ] **Step 3: Update `handleSaveTerminal` to pass the new field**

Update the `SetAppSettings` call in `handleSaveTerminal` to include `codingAgentApp`:

```tsx
const handleSaveTerminal = async () => {
  setSavingTerminal(true);
  setTerminalMessage("");
  try {
    await (SetAppSettings as (
      a: boolean, b: number, c: boolean, d: number, e: string, f: string
    ) => Promise<void>)(
      contextRefetchEnabled,
      Math.max(1, contextRefetchInterval),
      reviewsRefetchEnabled,
      Math.max(1, reviewsRefetchInterval),
      terminalAppPath.trim(),
      codingAgentApp.trim(),
    );
    const updated = await GetAppSettings() as unknown as AppSettings;
    setAppSettings(updated);
    setTerminalAppPath(updated.terminalAppPath ?? "");
    setCodingAgentApp(updated.codingAgentApp ?? "cursor");
    setTerminalMessage("Saved.");
  } catch (e) {
    setError(String(e));
  } finally {
    setSavingTerminal(false);
  }
};
```

Also update `handleSaveRefetch` similarly (same `SetAppSettings` call at line ~161):

```tsx
await (SetAppSettings as (
  a: boolean, b: number, c: boolean, d: number, e: string, f: string
) => Promise<void>)(
  contextRefetchEnabled,
  Math.max(1, contextRefetchInterval),
  reviewsRefetchEnabled,
  Math.max(1, reviewsRefetchInterval),
  terminalAppPath.trim(),
  codingAgentApp.trim(),
);
const updated = await GetAppSettings() as unknown as AppSettings;
setAppSettings(updated);
setTerminalAppPath(updated.terminalAppPath ?? "");
setCodingAgentApp(updated.codingAgentApp ?? "cursor");
```

- [ ] **Step 4: Add the Coding Agent input to the Terminal section**

In the Terminal section (after the existing terminal app input and before the Save button, around line 436), add a second field:

```tsx
<div>
  <label style={{ display: "block", fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
    Coding agent executable
  </label>
  <input
    className="passphrase-input"
    placeholder="cursor"
    value={codingAgentApp}
    onChange={(e) => setCodingAgentApp(e.target.value)}
    style={{ width: "100%" }}
  />
  <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>
    Opened at the repo path after checkout. CLI executable name (e.g. <code>cursor</code>,{" "}
    <code>code</code>, <code>windsurf</code>).
  </div>
</div>
```

This goes between the existing terminal app `<div>` and the Save `<div>`.

- [ ] **Step 5: Type-check the frontend**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/Settings.tsx
git commit -m "feat(settings): add Coding Agent field to Terminal settings section"
```

---

## Task 6: `AppSettings` Type in Frontend

**Files:**
- Find the `AppSettings` type definition used in `Settings.tsx`

- [ ] **Step 1: Locate the AppSettings interface**

Run: `grep -rn "codingAgentApp\|AppSettings" frontend/src --include="*.ts" --include="*.tsx" | grep -v "node_modules" | grep "interface\|type\|codingAgentApp"`

Expected: find where `AppSettings` is declared (likely auto-generated in `wailsjs/go/main/App.js` or a local types file)

- [ ] **Step 2: Add `codingAgentApp` to the interface if it's a handwritten type**

If `AppSettings` is declared manually somewhere (not auto-generated), add:
```ts
codingAgentApp: string;
```

If it's sourced from the Wails-generated `App.d.ts`, verify the generated file already includes `codingAgentApp` after step Task 3 Step 3 (wails generate module). If not, re-run `wails generate module`.

- [ ] **Step 3: Re-run type check**

Run: `cd frontend && npx tsc --noEmit`
Expected: no errors

---

## Self-Review

**Spec coverage check:**
- ✅ New `codingAgentApp` setting in Settings → Terminal
- ✅ Default value `cursor`
- ✅ `onCheckoutCommand` now executes inside terminal window (via temp script)
- ✅ Path opens in coding agent, not terminal app
- ✅ Flow: checkout → terminal runs script (onCheckoutCommand + open agent)
- ✅ DB migration
- ✅ Store layer (GET/SET)
- ✅ Wails binding
- ✅ All checkout call sites updated
- ✅ `handleSaveRefetch` also passes new field (avoids overwriting on refetch settings save)

**Placeholder scan:** None found.

**Type consistency:**
- `CodingAgentApp` (Go) ↔ `codingAgentApp` (JSON/TS) — consistent throughout
- `buildCheckoutScript` called only in `openWithCheckout` — consistent signature
- All `openTerminalAt` references removed and replaced with `openWithCheckout`
