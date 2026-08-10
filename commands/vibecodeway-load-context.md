---
name: vibecodeway-load-context
description: >-
  Load a saved context from vibecodeway into the conversation via
  vibecodeway-cli. Lists available contexts, asks which to load, then
  fetches and presents the full context data. Use when the user says "load
  context", "get context", "show contexts", or wants to work with a saved
  Jira/Slack context.
---

# Load Context

All operations use `vibecodeway-cli` (Bash). JSON on stdout; errors on
stderr. Prefer `/vibecodeway-prime` first if the CLI schema is not
already loaded.

`context-list` / `context-get` read the **local** consolidator DB. Do **not** call
`jira-cli`, `acli`, `jira-refresh`, or `reload-all` unless the user explicitly
asks to refresh from Jira/Slack.

Follow **Safe JSON handling** in `/vibecodeway-prime` (prefer `jq`;
never `cli | python3 <<'PY'`).

## Workflow

### Step 1: Resolve which context to load

**If the user already named a context key** (e.g. `/vibecodeway-load-context JH-4922`):
skip the list UI and go straight to Step 3 with that key.

**Otherwise**, list contexts:

```bash
vibecodeway-cli context-list | jq '[.[] | {key, status, id}]'
```

**Output:** JSON array of context objects (`id`, `key`, `status`, issue counts, …).

### Step 2: Present contexts and ask which to load

(Only when Step 1 listed contexts — skipped if the user already named a key.)

Display as a numbered list:
- **Key**
- **Status** (active / archived)
- **Issue count**
- **JQL / query** (truncated if long)

Ask which context to load. If only one exists, still confirm before loading.

### Step 3: Fetch the full context

```bash
vibecodeway-cli context-get --context-key <selected_key> > /tmp/cc-ctx.json
jq '{
  key: .context.key,
  status: .context.status,
  issues: [.issues[]? | {jiraKey, summary, status, priority, assignee, issueType, parentKey, description, commentCount: (.comments|length? // 0)}]
}' /tmp/cc-ctx.json
```

**Input:** `--context-key STRING` or `--context-id UUID` (one required)  
**Output:** JSON context detail (Jira config, Slack config, issues, counts, …).

To include comment bodies when needed:

```bash
jq '{issues: [.issues[]? | {jiraKey, summary, status, description, comments: [.comments[]? | {author, created, body}]}]}' /tmp/cc-ctx.json
```

### Step 4: Present the context data

Summarize for the user:

1. **Overview** — key, JQL, total issues, date range of activity.
2. **Issue list** — key, summary, status, comment count per issue.
3. **Next steps** — ask if they want to:
   - Analyze / summarize the context
   - Search for new issues (`vibecodeway-cli jira-search --context-key <key>`)
   - Work with a specific issue

Keep the summary concise. Full data is already in context — do not dump every
comment unless asked.
