---
name: vibecodeway-list-repos
description: >-
  List connected vibecodeway repositories with their local paths and git
  worktrees via vibecodeway-cli (local DB). FIRST ACTION: run vibecodeway-cli —
  not MCP, not gh, not repo search. Use when the user asks "which repos are
  connected", "where are my repos/workspaces on disk", or invokes
  /vibecodeway-list-repos.
---

# List Repos — Connected Repositories, Paths & Workspaces

`vibecodeway-cli` is a global binary (`/usr/local/bin/vibecodeway-cli`) that
reads the **local** consolidator DB. It is **always in scope** — even when the
active workspace/worktree is another repo.

All operations use `vibecodeway-cli` (Bash). JSON on stdout; errors on stderr.
Follow **Safe JSON handling** in `/vibecodeway-prime` (prefer `jq` with
**single-quoted** programs; never `cli | python3 <<'PY'`).

## What this answers

- Which repositories are connected (tracked) in vibecodeway.
- Where each repo is checked out on disk (`paths[]` — one repo may have several).
- Which git-worktree **workspaces** exist per repo (`workspaces[]` — branch +
  on-disk `worktreePath` + `status`).

## Hard rules (before any other tool)

1. **First tool call must be `vibecodeway-cli repo-list`.** Do not search MCP,
   `ctx_search`, git worktrees, `.vibecodeway` / `.ponytail`, GitHub, or the
   filesystem to "find" repos — the local DB via the CLI is the only source.
2. This is a **read-only local** command. Do **not** call `jira-refresh`,
   `reload-all`, `gh`, or `git` to gather this data.
3. If `command not found: vibecodeway-cli`, tell the user to run
   `make install-cli` from the vibecodeway repo and stop.
4. Pipe to `jq` — do not write `/tmp` if worktree scope may block files outside
   the active repo.

## Workflow

### Step 1: List repos with paths and workspaces

```bash
vibecodeway-cli repo-list | jq '[.[] | {
  repo: (.owner + "/" + .name),
  platform,
  label,
  paths: [.paths[].path],
  workspaces: [.workspaces[] | {branch, worktreePath, status}]
}]'
```

**Output shape** (array; one entry per connected repo):

```json
[
  {
    "repo": "acme/api",
    "platform": "github",
    "label": "",
    "paths": ["/home/dev/src/api"],
    "workspaces": [
      { "branch": "feat/x", "worktreePath": "/home/dev/src/api/.worktrees/feat-x", "status": "active" }
    ]
  }
]
```

`paths` and `workspaces` are always arrays (possibly empty). `localPath` on the
raw object is **deprecated** — always use `paths[]`.

### Step 2: Present the results

1. **Repos** — `owner/name` (+ platform, label if set).
2. **Local paths** — the checkout directory/directories on disk per repo.
3. **Workspaces** — branch → worktree path (+ status). Note `should_delete`
   workspaces as stale if present.

If `repo-list` returns `[]`, tell the user no repositories are connected yet.

### Step 3 (optional): Filter to on-disk-only or a single repo

```bash
# Only repos that have at least one local path:
vibecodeway-cli repo-list | jq '[.[] | select((.paths | length) > 0) | {repo: (.owner + "/" + .name), paths: [.paths[].path]}]'

# One repo by name:
vibecodeway-cli repo-list | jq --arg name api '[.[] | select(.name == $name)]'
```

Keep the summary concise. Do not dump raw JSON into the chat — project with `jq`.
