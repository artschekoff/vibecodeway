---
name: context-consolidator-prime
description: >-
  Entry point for context-consolidator — load the full CLI schema via
  context-consolidator-cli describe, then route to the right slash command or
  CLI call. Use at the start of any context-consolidator task.
---

# Prime — Context Consolidator CLI Router

Load the authoritative CLI schema first, then route the user's request. All
operations go through `context-consolidator-cli` (Bash).

## Step 1 — Load the schema

```bash
context-consolidator-cli describe
```

**Output:** JSON

```json
{
  "binary": "context-consolidator-cli",
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

If the binary is missing (`command not found: context-consolidator-cli`), tell
the user to run `make install-cli` from the context-consolidator repo root and
stop.

## Step 2 — Quick auth check (when a task needs Jira/Slack)

```bash
context-consolidator-cli auth-status
```

**Output:** `{"status":"authenticated|unauthenticated|error"}`  
If not `authenticated`, stop and ask the user to run `acli jira auth login`.

## Step 3 — Route the user's request

| Task signal | Action |
|---|---|
| "Load / show / get a saved context" | `/context-consolidator-load-context` |
| "Load task assignment / tech assignment / text notes" | `/context-consolidator-load-task-assignment` |
| "List contexts", "show contexts" | `context-consolidator-cli context-list` |
| "Create / update / delete / archive a context" | drive CLI from schema (`context-create`, `context-update`, `context-delete`, `context-toggle-status`) |
| "Add / list / delete text entries" | `add-text`, `text-list`, `text-delete`, `text-toggle` |
| "Upload / list / delete files" | `add-file`, `file-list`, `file-delete`, `file-toggle` |
| "Search / add / sync / refresh Jira issues" | `jira-search`, `jira-add`, `jira-sync-comments`, `jira-refresh`, `jira-search-jql` |
| "Slack channels / threads / DMs" | `slack-*` commands from schema |
| "Show me the schema / all commands / what can this do" | `context-consolidator-cli describe` |

## Step 4 — Rules

- Always use `context-consolidator-cli` for context-consolidator work.
- For load-context flows, delegate to the slash commands above; do not duplicate their steps here.
- For everything else, drive the CLI using the schema from Step 1: read `input`, `flags`, and `output` to build the call and parse the result.
- Every command prints JSON to stdout on success; errors go to stderr with a non-zero exit.
- Prefer `--context-key` when the user names a context; use `--context-id` (UUID) when you already have it from `context-list`.
- If no row matches, ask the user to clarify what they want to do with context-consolidator.
