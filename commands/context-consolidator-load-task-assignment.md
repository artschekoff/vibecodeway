---
name: context-consolidator-load-task-assignment
description: >-
  Load text entries (task / tech assignment notes) from a context-consolidator
  context via context-consolidator-cli. Lists contexts, asks which to load, then
  fetches and presents text entries. Use when the user says "load assignment",
  "show task assignment", "get tech assignment", or wants text notes from a
  context.
---

# Load Task Assignment

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
- **Status**
- **Issue count**

Ask which context. If only one exists, proceed with it.

### Step 3: Fetch text entries

Use the context UUID from step 1:

```bash
context-consolidator-cli text-list --context-id <uuid>
```

**Input:** `--context-id UUID` (required)  
**Output:** JSON array of `{id, name, content, isActive, createdAt, updatedAt}`.

### Step 4: Present the data

Show each entry by `name` and `content`. Render Markdown content as-is.

Offer next steps:
- Load another context's text entries
- Analyze or compare entries
- Work with a Jira issue referenced in an entry
