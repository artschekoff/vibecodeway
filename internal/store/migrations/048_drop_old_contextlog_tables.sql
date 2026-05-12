-- +goose Up
-- Remove the old context-log / workflow-proposal / open-questions system.
-- Replaced by event_log + context_snapshots (migrations 044-046).
DROP TABLE IF EXISTS context_log_files;
DROP TABLE IF EXISTS context_log_chat_messages;
DROP TABLE IF EXISTS context_open_questions;
DROP TABLE IF EXISTS consolidated_context_logs;
DROP TABLE IF EXISTS context_logs;
DROP TABLE IF EXISTS context_workflow_proposals;

-- +goose Down
SELECT 1; -- intentionally irreversible
