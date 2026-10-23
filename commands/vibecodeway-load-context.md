---
name: vibecodeway-load-context
description: >-
  Load a saved vibecodeway context via vibecodeway-cli (local DB). FIRST
  ACTION: run vibecodeway-cli — not MCP, not gh, not Jira REST, not repo
  search. Use when the user says "load context", "get context", "show
  contexts", or invokes /vibecodeway-load-context [KEY].
---

# Load Context

`vibecodeway-cli` is a global binary (`/usr/local/bin/vibecodeway-cli`). It
reads the **local** consolidator DB. It is **always in scope** — even when
the active workspace/worktree is another repo, even when path guards block
`~/.claude` or other projects.

All operations use `vibecodeway-cli` (Bash). JSON on stdout; errors on stderr.
Follow **Safe JSON handling** in `/vibecodeway-prime` (prefer `jq` with
**single-quoted** programs; never `cli | python3 <<'PY'`).

## Hard rules (before any other tool)

1. **First tool call must be `vibecodeway-cli`.** If the user named a key
   (e.g. `/vibecodeway-load-context JH-5139`), run `context-get` immediately.
2. Do **not** search MCP, `ctx_search`, git worktrees, `.vibecodeway` /
   `.ponytail` folders, GitHub issues, or Jira REST as a substitute.
3. Do **not** call `jira-cli`, `acli`, `jira-refresh`, or `reload-all` unless
   the user explicitly asks to refresh from Jira/Slack.
4. Do **not** wait on `/vibecodeway-prime` when a key is already given. Load
   prime only if you need `describe` for an unknown flag.
5. If `command not found: vibecodeway-cli`, tell the user to run
   `make install-cli` from the vibecodeway repo and stop.
6. Pipe to `jq` — do not write `/tmp` if worktree scope may block files
   outside the active repo.
7. **Never invent top-level keys.** Real `context-get` payload is nested under
   `.context`, with siblings `issues`, `vcsBranches`, `pullRequests`,
   `contextTexts`, `uploadedFiles`, `slackThreads`, … — not flat
   `repos` / `branches` / `texts`. If unsure, project `. | keys` first.
8. Follow-up list commands (`text-list`, `file-list`, `slack-list-*`,
   `jira-refresh`, …) take **`--context-id` only**. Use `.context.id` from
   `context-get` — do **not** pass `--context-key` (unknown flag).

## Workflow

### Step 1: Resolve which context to load

**If the user already named a context key**, skip to Step 3.

**Otherwise**, list contexts:

```bash
vibecodeway-cli context-list | jq '[.[] | {key, status, id, issueCount}]'
```

### Step 2: Present contexts and ask which to load

(Only when Step 1 listed contexts.)

Show: key, status, issue count. Ask which to load. If only one exists, still confirm.

### Step 3: Fetch the full context

```bash
vibecodeway-cli context-get --context-key KEY | jq '{
  id: .context.id,
  key: .context.key,
  status: .context.status,
  snapshotStatus: .context.snapshotStatus,
  snapshotIsStale: .context.snapshotIsStale,
  issues: [.issues[]? | {
    jiraKey, summary, status, priority, assignee, issueType, parentKey,
    description,
    commentCount: ((.comments // []) | length)
  }],
  vcsBranches: [.vcsBranches[]? | {
    branch,
    label,
    autodetectPr,
    repo: (if .repo then (.repo.owner + "/" + .repo.name) else null end),
    platform: .repo.platform
  }],
  pullRequests: [.pullRequests[]? | {
    prNumber, title, state, url, headBranch, reviewStatus, ciStatus
  }],
  contextTexts: [.contextTexts[]? | {id, name, isActive, updatedAt}],
  uploadedFiles: [.uploadedFiles[]? | {id, name, isActive}],
  slackThreadCount: ((.slackThreads // []) | length)
}'
```

Use `--context-id UUID` when you already have it from `context-list`.

Keep `.context.id` — you need it for `text-list` / `file-list` / Slack list
commands and for resolving local worktrees via `/vibecodeway-list-repos`
(match `vcsBranches[].branch` → `repo-list` `.workspaces[]`).

If the user asked for the **last event / last comment / comments**, also project comment bodies:

```bash
vibecodeway-cli context-get --context-key KEY | jq '{
  snapshotIsStale: .context.snapshotIsStale,
  issues: [.issues[]? | {
    jiraKey, summary,
    comments: [(.comments // [])[] | {author, created, body}]
  }]
}'
```

If comments are empty and `snapshotIsStale` is true, say so and offer
`jira-sync-comments` / `jira-refresh` — do not run them unless asked.
Those sync commands require `--context-id` (from `.context.id`), not `--context-key`.

### Step 4: Present the context data

1. **Overview** — key, id, status, issue count, stale-snapshot warning if any.
2. **Issue list** — key, summary, status, comment count.
3. **VCS scope** — linked branches (`owner/name` + branch + label) and PRs.
4. **Texts / files** — names only unless the user asked for content
   (`text-list --context-id` for full bodies).
5. **Next steps** — analyze, resolve worktree via `repo-list`, or work a specific issue.

Keep the summary concise. Do not dump every comment unless asked.
