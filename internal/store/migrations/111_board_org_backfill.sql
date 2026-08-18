-- +goose Up
-- The org/scope custom field can't come back from a Jira search (acli rejects
-- custom fields there), so it's backfilled one key at a time via `workitem view`.
-- org_checked marks "we already asked about this key", which is what distinguishes
-- an issue with no org set from one we haven't looked at yet — without it, every
-- org-less issue would be re-fetched on every refresh forever.
ALTER TABLE board_jira_assignments ADD COLUMN org_checked INTEGER NOT NULL DEFAULT 0;

-- Rows that already carry an org came from the days when search returned it.
UPDATE board_jira_assignments SET org_checked = 1 WHERE org <> '';

-- +goose Down
ALTER TABLE board_jira_assignments DROP COLUMN org_checked;
