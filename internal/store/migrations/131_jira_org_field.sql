-- +goose Up
-- Jira custom field (customfield_NNNNN) holding a ticket's scope/service, used
-- to split My Board by "org". Was a hardcoded instance-specific id; now a
-- setting, empty = feature off.
ALTER TABLE app_settings ADD COLUMN jira_org_field TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE app_settings DROP COLUMN jira_org_field;
