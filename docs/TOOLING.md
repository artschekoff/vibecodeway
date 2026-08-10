# Agent Tooling

Generated from the tool registry — do not edit by hand. Regenerate with `make tooling-doc`.

These are the actions the Ask AI agent can take. The CLI mirrors them:
`vibecodeway-cli agent-tool list` and `... agent-tool call --name <tool> --args '{...}'`.
Tools marked **Write: yes** mutate state and require confirmation (Ask AI) or `--yes` (CLI).

## search_contexts

Full-text search across ALL contexts and their content (Jira issues/comments, Slack threads, texts, files, branches). Returns matching contexts with their id and key. This is the same search the app's search bar uses.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `query` | string | yes | keywords to search for |

```sh
vibecodeway-cli agent-tool call --name search_contexts --args '{}'
```

## get_context

List the elements inside one context: Jira issues (key/summary/status), Slack threads, text notes, uploaded files, and relations. Use item ids from here with get_item.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `contextId` | string | yes | the context id |

```sh
vibecodeway-cli agent-tool call --name get_context --args '{}'
```

## search_within_context

Find the elements inside a single context whose text matches a query. Returns matching item ids grouped by type.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `contextId` | string | yes | the context id |
| `query` | string | yes | keywords to search for |

```sh
vibecodeway-cli agent-tool call --name search_within_context --args '{}'
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
vibecodeway-cli agent-tool call --name get_item --args '{}'
```

## list_related_contexts

List contexts related to the given context (dependencies / links), with the relation type and target key.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `contextId` | string | yes | the context id |

```sh
vibecodeway-cli agent-tool call --name list_related_contexts --args '{}'
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
vibecodeway-cli agent-tool call --name edit_context --args '{}'
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
vibecodeway-cli agent-tool call --name add_event --args '{}'
```

## list_contexts

List ALL contexts (not a keyword search) with their id, key and status. Use this to enumerate everything before filtering or bulk operations. By default only active contexts are returned; set includeArchived to include archived ones.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `includeArchived` | boolean | no | include archived contexts (default false) |
| `status` | string | no | optional filter: active | archived |

```sh
vibecodeway-cli agent-tool call --name list_contexts --args '{}'
```

## find_contexts_by_jira_status

Find every context that has at least one Jira issue in the given status (e.g. 'Closed', 'Done', 'In Progress'). Match is case-insensitive. Returns the matching contexts' id, key and status.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `jiraStatus` | string | yes | the Jira issue status to match, e.g. 'Closed' |

```sh
vibecodeway-cli agent-tool call --name find_contexts_by_jira_status --args '{}'
```

## archive_contexts

Archive (deactivate) multiple contexts at once by id. Skips any that are already archived. Returns how many were changed. Use find_contexts_by_jira_status or list_contexts first to gather the ids.

**Write:** yes

| Param | Type | Required | Description |
|---|---|---|---|
| `contextIds` | array | yes | the context ids to archive |

```sh
vibecodeway-cli agent-tool call --name archive_contexts --args '{}'
```

## list_review_requests

List open pull requests in a repo that are awaiting review from a user. Defaults to the authenticated user (@me); pass a login to see another user's requested reviews.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `repo` | string | yes | repository as 'owner/name' |
| `user` | string | no | GitHub login to check (optional, default @me) |

```sh
vibecodeway-cli agent-tool call --name list_review_requests --args '{}'
```

## list_changes_requested

List open pull requests in a repo where a user left a 'changes requested' review. Defaults to the authenticated user (@me).

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `repo` | string | yes | repository as 'owner/name' |
| `user` | string | no | GitHub login to check (optional, default @me) |

```sh
vibecodeway-cli agent-tool call --name list_changes_requested --args '{}'
```

## get_pr

Get one pull request's metadata (title, author, state, review status, CI status, mergeability, reviewers) by number.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `number` | number | yes | the pull request number |
| `repo` | string | yes | repository as 'owner/name' |

```sh
vibecodeway-cli agent-tool call --name get_pr --args '{}'
```

## get_pr_comments

List the comments on a pull request (conversation, review bodies, and inline review comments) with author and body.

**Write:** no

| Param | Type | Required | Description |
|---|---|---|---|
| `number` | number | yes | the pull request number |
| `repo` | string | yes | repository as 'owner/name' |

```sh
vibecodeway-cli agent-tool call --name get_pr_comments --args '{}'
```

## approve_pr

Approve a pull request, optionally with a message. Submits an approving review as the authenticated user.

**Write:** yes

| Param | Type | Required | Description |
|---|---|---|---|
| `body` | string | no | approval message (optional) |
| `number` | number | yes | the pull request number |
| `repo` | string | yes | repository as 'owner/name' |

```sh
vibecodeway-cli agent-tool call --name approve_pr --args '{}'
```

## request_changes_pr

Request changes on a pull request with a required message explaining what needs to change. Submits a changes-requested review.

**Write:** yes

| Param | Type | Required | Description |
|---|---|---|---|
| `body` | string | yes | what needs to change (required) |
| `number` | number | yes | the pull request number |
| `repo` | string | yes | repository as 'owner/name' |

```sh
vibecodeway-cli agent-tool call --name request_changes_pr --args '{}'
```

## comment_pr

Leave a comment on a pull request (a review with no approval state). Requires a message.

**Write:** yes

| Param | Type | Required | Description |
|---|---|---|---|
| `body` | string | yes | the comment text (required) |
| `number` | number | yes | the pull request number |
| `repo` | string | yes | repository as 'owner/name' |

```sh
vibecodeway-cli agent-tool call --name comment_pr --args '{}'
```

