---
name: arts-load-task-assignment
description: >-
  Load text entries from a context-consolidator context into the LLM
  conversation. Lists contexts, asks which to load, then fetches and presents
  all text entries for that context. Use when the user says "load assignment",
  "show task assignment", "get tech assignment", or wants to work with
  text notes stored inside a context.
---

# Load Task Assignment

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

Then ask the user to pick a context:

```
AskQuestion:
  title: "Load Task Assignment"
  questions:
    - id: "context_pick"
      prompt: "Which context?"
      options: [one option per context, using label: "<key> (<issue_count> issues)"]
```

If only one context exists, skip asking and proceed with it.

### Step 3: Fetch text entries

Use the context UUID from step 1 output:

```bash
context-consolidator-cli text-list --context-id <uuid>
```

### Step 4: Present the data

Show each text entry by `name` and `content`. If the content is Markdown, render it as-is.

Offer next steps:
- Load another context's text entries
- Analyze or compare entries
- Work with a specific Jira issue referenced in an entry
