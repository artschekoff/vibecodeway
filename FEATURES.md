# Context Consolidator

### Mission control for AI-assisted development.

One local, private desktop app that pulls your **Jira, Slack, GitHub/GitLab, code, and AI agents** into a single **context** — so you (and your agents) always work with the full picture, not fifteen open tabs.

---

## The problem

A developer's real task lives in a dozen places at once: the Jira ticket, the Slack thread where the decision was actually made, the PR and its review comments, the branch, the failing CI check, and the code itself. Every context switch costs focus — and every AI coding agent you use starts *blind*, with none of it.

**Context Consolidator makes "everything about this task" a first-class object** — assemble it once, keep it in sync, act on it in an integrated editor, and hand it to your AI agent in one command.

---

## What it does

### 🧩 Contexts — your unit of work
- Named workspaces (`sprint-42`, `feat-auth`) that **bundle everything** related to a task.
- Grouped by **organization** (multi-tenant), drag-to-reorder, archive/unarchive.
- **Workflow statuses** — configurable, color-coded, kanban-style, with a status log.
- **Context relations** — link sub-task → epic, or related work streams together.
- **Tabbed** — keep several contexts open side by side.
- **Create a context straight from a Jira ticket** in one click.

### 🔌 Integrations that stay in sync
- **Jira** — sync issues + comments via JQL, auto-discover subtasks & child issues, detect status changes, change a ticket's status back to Jira from the app, transition workflow.
- **Slack** — pull channels, threads, and DMs into the context.
- **GitHub / GitLab** — repos, branches, PRs, CI status, review decisions, comment threads.
- **Plain text & file uploads** — paste notes or attach files to any context.
- **Background delta sync** — incremental refresh with a change-count badge and toast when new activity lands.

### 💻 Integrated code workspace
A full editor built in — not a link that dumps you into another tool.
- **Monaco diff editor** — view working changes vs base with real syntax highlighting for every language your stack uses.
- **Accept / reject individual hunks** or the whole file — review and curate an AI agent's edits inline, VS Code / Cursor style.
- **Prev / next change navigation**, inline **git blame**, and **file history** at a commit.
- **Files & Git tabs** — a full repo file tree (VS Code–style, collapsible, filterable) alongside a git changes view (staged / unstaged / branch-vs-base), with per-file stage / unstage / discard.
- **Built-in terminal** — multiple tabs, **cmux-style hotkeys** (⌘T / ⌘W / ⌘⇧[ ] / ⌃1–9), reusable **command snippets** (with "send Enter" to auto-run), and **"open terminal here"** on any repo or folder.
- **Ask AI** — select code, and the app builds a precise prompt (path + lines + snippet) and drops it straight into your terminal agent.
- **Multi-worktree aware** — pick the right git worktree per repo; tab titles show `repo / branch` so you never lose track.

### 🔭 Code-review command center
- **My Board** — your personal dashboard: active tasks, assigned Jira to-dos (sortable/filterable by priority, status, date), your open PRs (with unresolved-comment counts and CI/ready badges), and review requests.
- **Pending Code Reviews** — every PR awaiting *your* review, grouped by repo, with CI / approval / conflict / "ready to merge" badges.
- **"Re-review" detection** — knows when an author has addressed your change-request and re-requested review, so it stops reading as a stale "Changes Requested".
- Unresolved-comment and changes-requested surfacing, cleanup of merged/closed items, and one-click **Checkout** or **Solve with AI**.

### 🧠 AI intelligence layer
- **Event extraction** — AI turns raw Jira/Slack/text into structured, typed events (decision, blocker, status change, next-step, open question…).
- **Context Snapshot** — an AI-maintained living brief: current step + confidence, summary, and short/mid/long-term next steps.
- **Handoff packet** — an inferred business-requirements document generated from the context, ready to paste into a ticket or share.
- **Time estimation** — AI estimate with a confidence range and its reasoning.
- **Person assignments** — generate a tailored task-assignment doc per teammate, with a multi-step reasoning chain, versioning, and per-step custom prompts.
- **Bring your own model** — OpenAI, Anthropic (Claude), or any OpenAI-compatible local endpoint.

### 🤖 Agent-native by design
- A first-class **CLI** (`context-consolidator-cli`) with JSON in/out — every capability an AI coding agent needs to read and update contexts.
- `describe` emits the **full machine-readable schema**, so agents self-configure.
- Ships **slash commands** (`/context-consolidator-prime`, load-context, load-task-assignment) so Claude Code / Cursor pick up the whole task context in one step.

### ⚡ Capture from anywhere
- **Global hotkey quick-attach** — select text in *any* app, hit the hotkey, pick a target context, and it's captured — with a system notification to confirm.

### 🔒 Local-first & private
- Everything lives in a **local SQLite database on your machine** — your tickets, threads, code, and credentials never pass through a third-party server.
- You supply your own Jira (`acli`), GitHub/GitLab, Slack, and AI credentials.
- Full-text search across everything you've captured.

---

## Who it's for

Professional developers and tech leads who live across Jira + Slack + GitHub and use AI coding agents — and are tired of re-explaining the same context to every tool, every time.

## Under the hood

Native desktop app (Wails: Go backend + React/TypeScript frontend), local SQLite with 90+ schema migrations, `acli` for Jira, `gh`/`glab` for VCS, Monaco editor, real PTY terminals.

---

## Where it's headed

Context Consolidator is evolving from "aggregator" into **the context layer for AI-assisted engineering** — the place your agents get grounded before they touch code, and where you triage what actually needs *you*. Next on the track: deeper codebase-aware analysis, ticket-to-plan breakdown, a daily "what should I work on now" focus view, and team-level context sharing.
