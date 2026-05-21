---
name: load-task-assignment
description: >-
  Load a person's current task assignment from context-consolidator into the
  LLM conversation. Lists contexts, asks which context and person to load, then
  fetches and presents the full technical assignment. Use when the user says
  "load assignment", "show task assignment", "get tech assignment", or wants to
  work with a specific person's assigned tasks inside a context.
---

# Load Task Assignment

## Workflow

### Step 1: List available contexts

Call the `context_list` MCP tool to get all saved contexts:

```
CallMcpTool:
  server: "user-context-consolidator"
  toolName: "context_list"
  arguments: {}
```

### Step 2: Present contexts and ask which to load

Display the returned contexts as a numbered list showing:
- **Key** (user-assigned name)
- **Status** (e.g. active, archived)
- **Issue count**

Then ask the user to pick a context and provide a person name:

```
AskQuestion:
  title: "Load Task Assignment"
  questions:
    - id: "context_pick"
      prompt: "Which context?"
      options: [one option per context, using label: "<key> (<issue_count> issues)"]
    - id: "person_name"
      prompt: "Person name (leave blank to list all):"
```

If only one context exists, skip asking for the context and proceed with it.

### Step 3: Fetch the assignment

**If a person name was provided**, call `load_task_assignment` for the current assignment body:

```
CallMcpTool:
  server: "user-context-consolidator"
  toolName: "load_task_assignment"
  arguments: { "context_key": "<selected_key>", "person_name": "<person_name>" }
```

**If no person name was provided**, call `context_assignments` to list all assignments:

```
CallMcpTool:
  server: "user-context-consolidator"
  toolName: "context_assignments"
  arguments: { "context_key": "<selected_key>" }
```

### Step 4: Present the assignment data

**Single person:** Present the full assignment body verbatim — it is Markdown and should be rendered as-is. Note if the assignment is marked outdated (new activity has occurred since it was generated).

**All people:** Show a table with each person's name and a one-line summary of their top task, then offer to load one person's full assignment.

Offer next steps:
- Load another person's assignment
- Analyze or compare assignments
- Work with a specific Jira issue referenced in the assignment
