# Agent Tooling

Generated from the tool registry — do not edit by hand. Regenerate with `make tooling-doc`.

These are the actions the Ask AI agent can take. The CLI mirrors them:
`context-consolidator-cli agent-tool list` and `... agent-tool call --name <tool> --args '{...}'`.
Tools marked **Write: yes** mutate state and require confirmation (Ask AI) or `--yes` (CLI).

## search_contexts

Full-text search across ALL contexts and their content (Jira issues/comments, Slack threads, texts, files, branches). Returns matching contexts with their id and key. This is the same search the app's search bar uses.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `query` | string | yes | keywords to search for |

```sh
context-consolidator-cli agent-tool call --name search_contexts --args '{}'
```

## get_context

List the elements inside one context: Jira issues (key/summary/status), Slack threads, text notes, uploaded files, and relations. Use item ids from here with get_item.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `contextId` | string | yes | the context id |

```sh
context-consolidator-cli agent-tool call --name get_context --args '{}'
```

## search_within_context

Find the elements inside a single context whose text matches a query. Returns matching item ids grouped by type.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `contextId` | string | yes | the context id |
| `query` | string | yes | keywords to search for |

```sh
context-consolidator-cli agent-tool call --name search_within_context --args '{}'
```

## get_item

Read the full text of one element. itemType is one of: issue, thread, text, file.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `contextId` | string | yes | the context id |
| `itemId` | string | yes | the element id |
| `itemType` | string | yes | issue | thread | text | file |

```sh
context-consolidator-cli agent-tool call --name get_item --args '{}'
```

## list_related_contexts

List contexts related to the given context (dependencies / links), with the relation type and target key.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `contextId` | string | yes | the context id |

```sh
context-consolidator-cli agent-tool call --name list_related_contexts --args '{}'
```

## edit_context

Edit a context's scalar attributes. Provide only the fields to change; omitted fields are left as-is. status is 'active' or 'archived'.

**Write:** yes

| Param | Type | Required | Description |
|---|---|---|---|
| `contextId` | string | yes | the context id |
| `key` | string | no | new key (optional) |
| `status` | string | no | active | archived (optional) |

```sh
context-consolidator-cli agent-tool call --name edit_context --args '{}'
```

## add_event

Append a timeline event to a context (e.g. a decision, status update, or note). type is one of the event categories; summary is a short human sentence.

**Write:** yes

| Param | Type | Required | Description |
|---|---|---|---|
| `author` | string | no | who/what produced the event (optional) |
| `contextId` | string | yes | the context id |
| `rawReference` | string | no | original text or URL (optional) |
| `sourceType` | string | no | jira | slack | github | chat | user (default user) |
| `summary` | string | yes | 1-2 sentence summary of what happened |
| `type` | string | yes | event type, e.g. user_note | decision_made | status_update | blocker_identified |

```sh
context-consolidator-cli agent-tool call --name add_event --args '{}'
```

