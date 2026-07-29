# Context Consolidator — Current Features

Desktop app (Wails: Go backend + React frontend). Aggregates developer context from multiple sources into named workspaces.

---

## Context Management

- **Contexts** — named workspaces (e.g. "sprint-42", "feature-auth") that bundle all related information
- Organized by **organization** (multi-tenant)
- Drag-and-drop reordering
- Archive / unarchive
- **Workflow statuses** — kanban-style status badges (configurable colors, AI hints, status log with comments)
- **Context relations** — link contexts to each other (e.g. sub-task → epic)
- Tab-based navigation — open multiple contexts side by side

## Data Sources & Sync

- **Jira**: sync issues + comments via JQL, detect status changes, emit notifications on change
- **Slack**: channels, threads, DMs
- **GitHub / GitLab**: repos, PRs, branches, CI status, review status
- **Plain text + file uploads**: paste notes, attach files
- **Background reload** — incremental delta sync, toast on completion with change count badge

## Pull Request & Code Review

- **Pending Code Reviews** page — all PRs where your review is requested, grouped by repo
- PR detail: CI badge (pending / success / failure), review badge (approved / changes requested), merge conflict badge, "Ready to merge" composite badge
- Draft PR detection
- PR comment thread viewer
- Dismiss reviewed / "waiting for author" items

## AI Analysis

- **Event extraction** — AI extracts structured typed events (decision, blocker, status update, next-step recommendation, etc.) from raw Jira/Slack/text content
- **Context Snapshot** — AI-maintained 3-section state document: current step + confidence, summary, next steps (short / mid / long)
- **Handoff packet** — inferred business requirements document generated from events
- **Estimation panel** — AI time estimate with confidence range and basis text
- **Person Assignments** — AI generates a task assignment body per named person (see below)

## Task Assignment Module (PersonAssignment)

- AI-generated assignment document per person, derived from context events
- **Multi-step reasoning chain**: each step has input\_state → output\_state; steps are chained
- **Versioning**: multiple saved versions per assignment; activate any prior version
- **Custom prompt per step**: run with custom prompt for preview without saving; commit if good
- **Outdated flag**: marks assignment stale when context has been updated since last analysis
- Manual save / override of output body

## Quick Attach

- **Global hotkey**: select text in any app → hotkey captures selection → overlay picker to choose target context → text appended
- System notification on successful attach

## Branch & Workspace Management

- **VCS Workspaces** (git worktrees): create/delete per-branch worktree
- **Checkout**: open branch checkout from context, run optional per-repo `onCheckoutCommand` via shell
- **Terminal integration**: `openTerminalAt()` opens configured terminal app at worktree path
- Local branch listing per repo

## CLI (`context-consolidator-cli`)

- JSON in/out CLI for coding agents and terminal use (`describe` emits the full schema)
- Context CRUD, text/file entries, Jira search/add/sync/refresh, Slack channels/threads/DMs
- Slash command `/context-consolidator-prime` loads available tools via `describe`

## Settings & Configuration

- **Settings page**: VCS auth (GitHub/GitLab), Jira/Slack credentials, terminal app path, passphrase
- **Prompts page**: editable AI prompt templates stored in DB (event\_extraction, snapshot\_build, handoff\_packet)
- **App logs page**: live log viewer
- Hotkey configuration

---

## What's Notably Absent (gaps for planning-assistant evolution)

- No **today's focus / daily standup view** — what should I work on right now?
- No **priority scoring** across contexts — which task is most urgent?
- No **blocker tracking UI** — blockers exist as events but have no dedicated surface
- No **codebase-aware analysis** — AI doesn't know the actual code state
- No **inline code actions** — can't create a branch, run tests, or open a PR from within the app
- No **personal task inbox** — items explicitly assigned to _you_ across all contexts
- No **time tracking / focus sessions**
- PersonAssignment reasoning chain has no **natural-language task list** format — it's a free-form markdown blob
- No way to **break down a Jira ticket** into concrete coding steps with code context
