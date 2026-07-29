---
name: context-consolidator-load-context
description: >-
  Load a saved context from context-consolidator into the conversation via
  context-consolidator-cli. Lists available contexts, asks which to load, then
  fetches and presents the full context data. Use when the user says "load
  context", "get context", "show contexts", or wants to work with a saved
  Jira/Slack context.
---

# Load Context

All operations use `context-consolidator-cli` (Bash). JSON on stdout; errors on
stderr. Prefer `/context-consolidator-prime` first if the CLI schema is not
already loaded.

## Workflow

### Step 1: List available contexts

```bash
context-consolidator-cli context-list
```

**Output:** JSON array of context objects (`id`, `key`, `status`, issue counts, …).

### Step 2: Present contexts and ask which to load

Display as a numbered list:
- **Key**
- **Status** (active / archived)
- **Issue count**
- **JQL / query** (truncated if long)

Ask which context to load. If only one exists, still confirm before loading.

### Step 3: Fetch the full context

```bash
context-consolidator-cli context-get --context-key <selected_key>
```

**Input:** `--context-key STRING` or `--context-id UUID` (one required)  
**Output:** JSON context detail (Jira config, Slack config, issues, counts, …).

### Step 4: Present the context data

Summarize for the user:

1. **Overview** — key, JQL, total issues, date range of activity.
2. **Issue list** — key, summary, status, comment count per issue.
3. **Next steps** — ask if they want to:
   - Analyze / summarize the context
   - Search for new issues (`context-consolidator-cli jira-search --context-key <key>`)
   - Work with a specific issue

Keep the summary concise. Full data is already in context — do not dump every
comment unless asked.
