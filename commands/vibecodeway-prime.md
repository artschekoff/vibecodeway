---
name: vibecodeway-prime
description: >-
  Entry point for vibecodeway — load the full CLI schema via
  vibecodeway-cli describe, then route to the right slash command or
  CLI call. Use at the start of any vibecodeway task.
---

# Prime — VibeCodeWay CLI Router

Load the authoritative CLI schema first, then route the user's request. All
operations go through `vibecodeway-cli` (Bash).

## Step 1 — Load the schema

```bash
vibecodeway-cli describe
```

**Output:** JSON

```json
{
  "binary": "vibecodeway-cli",
  "version": "string",
  "commands": [
    {
      "name": "context-list",
      "short": "string",
      "description": "string",
      "input": "string",
      "output": "json shape string",
      "example": "string",
      "flags": [{ "name": "string", "type": "string", "default": "string", "description": "string" }]
    }
  ]
}
```

Every command documents **description**, **input**, **output**, and **flags**.
Keep this as the source of truth — prefer it over `--help`.

If the binary is missing (`command not found: vibecodeway-cli`), tell
the user to run `make install-cli` from the vibecodeway repo root and
stop.

## Step 2 — Quick auth check (when a task needs Jira/Slack)

```bash
vibecodeway-cli auth-status
```

**Output:** `{"status":"authenticated|unauthenticated|error"}`  
If not `authenticated`, stop and ask the user to run `acli jira auth login`.

Only run this when the task will hit Jira/Slack (`jira-refresh`, `jira-search`,
`jira-add`, `reload-all`, Slack commands). **Do not** run auth checks for
local reads (`context-list`, `context-get`, `text-list`, …).

## Step 3 — Route the user's request

| Task signal | Action |
|---|---|
| "Load / show / get a saved context" | `/vibecodeway-load-context` |
| "Load task assignment / tech assignment / text notes" | `/vibecodeway-load-task-assignment` |
| "List contexts", "show contexts" | `vibecodeway-cli context-list` |
| "Which repos are connected / where are my repos / workspaces on disk" or `/vibecodeway-list-repos` | `/vibecodeway-list-repos` — `vibecodeway-cli repo-list` |
| "Create / update / delete / archive a context" | drive CLI from schema (`context-create`, `context-update`, `context-delete`, `context-toggle-status`) |
| "Add / list / delete text entries" | `add-text`, `text-list`, `text-delete`, `text-toggle` |
| "Upload / list / delete files" | `add-file`, `file-list`, `file-delete`, `file-toggle` |
| "Search / add / sync / refresh Jira issues" | `jira-search`, `jira-add`, `jira-sync-comments`, `jira-refresh`, `jira-search-jql` |
| "Slack channels / threads / DMs" | `slack-*` commands from schema |
| "Show me the schema / all commands / what can this do" | `vibecodeway-cli describe` |

## Step 4 — Rules

- Always use `vibecodeway-cli` for vibecodeway work.
- For load-context flows, delegate to the slash commands above; do not duplicate their steps here.
- For everything else, drive the CLI using the schema from Step 1: read `input`, `flags`, and `output` to build the call and parse the result.
- Every command prints JSON to stdout on success; errors go to stderr with a non-zero exit.
- Prefer `--context-key` when the user names a context; use `--context-id` (UUID) when you already have it from `context-list`.
- If no row matches, ask the user to clarify what they want to do with vibecodeway.

## Step 5 — Safe JSON handling (required)

CLI output is JSON. Follow these rules whenever you inspect or summarize it.

### Preferred: `jq`

```bash
vibecodeway-cli context-list | jq '[.[] | {key, status, id}]'
vibecodeway-cli context-get --context-key KEY | jq '{key: .context.key, issues: [.issues[]? | {jiraKey, summary, status, assignee}]}'
```

### Allowed: Python via temp file

```bash
vibecodeway-cli context-get --context-key KEY > /tmp/cc-ctx.json
python3 -c 'import json; d=json.load(open("/tmp/cc-ctx.json")); print(d["context"]["key"])'
```

### Forbidden (causes real agent failures)

| Anti-pattern | Why it breaks |
|---|---|
| `cli … \| python3 <<'PY' … PY` | Pipe + heredoc fight over stdin; Python may **execute JSON as code** → `NameError: true is not defined` |
| `python3 -c` with nested `\"` inside f-strings | Shell escaping corrupts the script → `SyntaxError` |
| Feeding CLI JSON straight into `python3` with no `-c` / script file | JSON is executed as Python |
| Calling `jira-cli` / `acli` to re-fetch data that `context-get` already has | Wrong tool; local consolidator DB is the source for load flows |

When summarizing large payloads, project with `jq` first — do not dump raw JSON into the chat.
