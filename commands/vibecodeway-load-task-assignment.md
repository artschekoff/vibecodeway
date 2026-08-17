---
name: vibecodeway-load-task-assignment
description: >-
  Load text entries (task / tech assignment notes) from a vibecodeway
  context via vibecodeway-cli. FIRST ACTION: run vibecodeway-cli — not MCP,
  not gh, not Jira REST, not repo search. Use when the user says "load
  assignment", "show task assignment", "get tech assignment", or invokes
  /vibecodeway-load-task-assignment [KEY].
---

# Load Task Assignment

`vibecodeway-cli` is a global binary. It reads the **local** consolidator DB
and is **always in scope**, even when the active worktree is another repo.

All operations use `vibecodeway-cli` (Bash). JSON on stdout; errors on stderr.
Local reads only — do not call `jira-cli` / `jira-refresh` unless the user
asks to refresh. Follow **Safe JSON handling** in `/vibecodeway-prime`
(prefer `jq` with **single-quoted** programs; never `cli | python3 <<'PY'`).

## Hard rules (before any other tool)

1. **First tool call must be `vibecodeway-cli`.** If the user named a key,
   resolve it and run `text-list` immediately.
2. Do **not** search MCP, `ctx_search`, git worktrees, GitHub, or Jira REST
   as a substitute.
3. Do **not** wait on `/vibecodeway-prime` when a key is already given.
4. If `command not found: vibecodeway-cli`, tell the user to run
   `make install-cli` from the vibecodeway repo and stop.
5. Pipe to `jq` — do not write `/tmp` if worktree scope may block files
   outside the active repo.

## Workflow

### Step 1: Resolve which context

**If the user already named a context key**, skip to Step 3 (resolve UUID via
`context-list` if you only have the key).

**Otherwise**, list contexts:

```bash
vibecodeway-cli context-list | jq '[.[] | {key, status, id, issueCount}]'
```

### Step 2: Present contexts and ask which to load

(Only when Step 1 listed contexts.)

Show: key, status, issue count. Ask which context. If only one exists, proceed with it.

### Step 3: Fetch text entries

Use the context UUID from step 1:

```bash
vibecodeway-cli text-list --context-id UUID | jq '[.[] | {id, name, isActive, updatedAt, content}]'
```

**Input:** `--context-id UUID` (required)  
**Output:** JSON array of `{id, name, content, isActive, createdAt, updatedAt}`.

### Step 4: Present the data

Show each entry by `name` and `content`. Render Markdown content as-is.

Offer next steps:
- Load another context's text entries
- Analyze or compare entries
- Work with a Jira issue referenced in an entry
