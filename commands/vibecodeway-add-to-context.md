---
name: vibecodeway-add-to-context
description: >-
  Add one thing to a vibecodeway context via vibecodeway-cli — a Jira issue, a
  GitHub repo/branch/PR, or free text. FIRST ACTION: run vibecodeway-cli — not
  MCP, not gh, not Jira REST, not repo search. Use when the user says "add to
  context", "attach this to", "put this into <context>", or invokes
  /vibecodeway-add-to-context [CONTEXT] [STRING].
---

# Add to Context

Takes two inputs: a **context key** and one **free-form string**. Classifies the
string, then writes it into the context with a single `vibecodeway-cli` call.

All operations use `vibecodeway-cli` (Bash). JSON on stdout; errors on stderr.
Follow **Safe JSON handling** in `/vibecodeway-prime` (prefer `jq` with
**single-quoted** programs; never `cli | python3 <<'PY'`).

## Hard rules (before any other tool)

1. **First tool call must be `vibecodeway-cli`.**
2. Never create a context without the user explicitly choosing to.
3. Never guess where plain text goes — ask (Step 4).
4. One invocation writes **one** thing. Several strings → run the flow per string.
5. If `command not found: vibecodeway-cli`, tell the user to run
   `make install-cli` from the vibecodeway repo and stop.

## Step 1 — Classify the string

Match in this order and stop at the first hit:

| Input looks like | Kind | Extracted values |
|---|---|---|
| `JH-5139`, `ABC-12` (bare issue key) | **jira** | key = as typed |
| `https://<host>/browse/JH-5139` | **jira** | key = last path segment |
| `https://<host>/…/JH-5139` (any Atlassian URL containing a key) | **jira** | key = the `[A-Z][A-Z0-9]+-\d+` match |
| `https://github.com/<owner>/<repo>/pull/<n>` | **pr** | owner/repo, prNumber = n |
| `https://github.com/<owner>/<repo>/tree/<branch>` | **branch** | owner/repo, branch (URL-decoded, may contain `/`) |
| `https://github.com/<owner>/<repo>` | **repo** | owner/repo |
| anything else | **text** | the string verbatim |

Print one line stating the classification before writing, e.g.
`Detected: GitHub PR juna-health/frontend#397`.

## Step 2 — Resolve the context

```bash
vibecodeway-cli context-list | jq '[.[] | {key, id, orgId, status}]'
```

- **Exact key match** → use it, go to Step 3.
- **No exact match** → do NOT create anything yet. Show the closest keys
  (case-insensitive substring match on what the user typed) and ask:

  > Контекст `<typed>` не найден. Похожие: `A`, `B`. Выбрать один, или создать
  > новый контекст `<typed>`?

- **User picks an existing key** → continue with it.
- **User chooses "новый контекст"** → Step 2a.

### Step 2a — Create the context (only on explicit confirmation)

```bash
vibecodeway-cli org-list | jq '[.[] | {id, name}]'
```

One org → use it. More than one → ask which. Then:

```bash
vibecodeway-cli context-create --key <key> --org-id <org-id>
```

Continue with the new key.

## Step 3 — Write, by kind

### jira

```bash
vibecodeway-cli jira-add --context-key <key> --jira-keys JH-5139
```

Needs Jira auth — run `vibecodeway-cli auth-status` first; if it is not
`authenticated`, stop and ask the user to run `acli jira auth login`.
Report `added` and `newComments` from the JSON.

### branch

```bash
vibecodeway-cli branch-add --context-key <key> --owner-repo juna-health/frontend --branch feat/JH-123-login
```

### pr

`branch-add` resolves the head branch itself via `gh` — do not call `gh` yourself:

```bash
vibecodeway-cli branch-add --context-key <key> --owner-repo juna-health/frontend --pr-number 397
```

If it fails with a gh auth error, tell the user to run `gh auth login` and stop.

### repo

No branch — tracks every PR of the repo:

```bash
vibecodeway-cli branch-add --context-key <key> --owner-repo juna-health/frontend
```

### text

Go to Step 4 first.

## Step 4 — Plain text: ask where it goes

Never decide this alone. Ask:

> Куда добавить этот текст в `<key>`?
> 1. **Событие** — попадёт в Activity Timeline (`event-add`)
> 2. **Текстовый материал** — задание/заметка в материалах контекста (`add-text`)

**Event:**

```bash
vibecodeway-cli event-add --context-key <key> --summary "<the string>" --type user_note
```

Pick a more specific `--type` when the wording is unambiguous — `blocker_identified`
for "blocked by …", `decision_made` for "we decided …", `work_completed` for
"finished …". When in doubt use `user_note`. Run `vibecodeway-cli describe` and
read `event-add`'s `input` for the full list.

**Text material:**

```bash
vibecodeway-cli add-text --context-key <key> --name "<short title>" --content "<the string>"
```

Ask for a short `--name` if the user has not implied one.

## Step 5 — Report

One line: what was added, to which context, plus the identifier the CLI returned
(issue count, branch id, event id, or text entry id). Do not dump raw JSON.
