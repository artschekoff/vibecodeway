---
name: arts-load-context
description: >-
  Load a saved context from context-consolidator into the LLM conversation.
  Lists available contexts via CLI, asks the user which one to load, then
  fetches and presents the full context data. Use when the user says
  "load context", "get context", "show contexts", or wants to work with
  a saved Jira/Slack context.
---

# Load Context

## Workflow

### Step 1: List available contexts

Run:

```bash
context-consolidator-cli context-list
```

### Step 2: Present contexts and ask which to load

Display the returned contexts as a numbered list showing:
- **Key** (user-assigned name)
- **Status** (e.g. active, archived)
- **Issue count**
- **JQL query** (truncated if long)

Then use `AskQuestion` to let the user pick one:

```
AskQuestion:
  title: "Load Context"
  questions:
    - id: "context_pick"
      prompt: "Which context do you want to load?"
      options: [one option per context, using label: "<key> (<issue_count> issues)"]
```

If only one context exists, still confirm with the user before loading.

### Step 3: Fetch the full context

Run with the chosen context's key:

```bash
context-consolidator-cli context-get --context-key <selected_key>
```

### Step 4: Present the context data

Summarize the loaded context for the user:

1. **Context overview** — key, JQL, total issues, date range of activity.
2. **Issue list** — for each issue show key, summary, status, and comment count.
3. **Offer next steps** — ask if the user wants to:
   - Analyze/summarize the context
   - Search for new issues to add (`context-consolidator-cli jira-search`)
   - Work with a specific issue

Keep the summary concise. The full data is now in your context window — do not
repeat every comment verbatim unless the user asks.
